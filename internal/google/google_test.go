package google

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestSealOpensOnlyForItsMember(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	sealed, err := Seal(key, "a@example.com", "refresh-token")
	if err != nil || bytes.Contains([]byte(sealed), []byte("refresh-token")) {
		t.Fatalf("sealed %q, %v", sealed, err)
	}
	if got, err := Open(key, "a@example.com", sealed); err != nil || got != "refresh-token" {
		t.Fatalf("open = %q, %v", got, err)
	}
	// Copied to another Member's item, or opened with another key, it
	// doesn't open.
	if _, err := Open(key, "b@example.com", sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("another Member: %v", err)
	}
	if _, err := Open(bytes.Repeat([]byte{2}, 32), "a@example.com", sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("another key: %v", err)
	}
	if _, err := Open(key, "a@example.com", "not sealed"); !errors.Is(err, ErrSealed) {
		t.Fatalf("garbage: %v", err)
	}
	if _, err := Seal(key[:16], "a@example.com", "x"); err == nil {
		t.Fatal("a short key sealed")
	}
}

func TestDailyWritesToGoogle(t *testing.T) {
	events, err := Events([]byte(`{"events":[{"id":"123456","summary":"ROVERS vs OWLS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	game := events[0]
	const owned = `{"etag":"version1","extendedProperties":{"private":{"game_id":"123456"}}}`
	const cancelled = `{"status":"cancelled","etag":"version2","id":"123456"}`
	tests := []struct {
		name, operation       string
		readStatus, putStatus int
		readBody              string
		want                  bool
		wantErr               error
		calls                 string
	}{
		{"new", "create", 0, 0, "", true, nil, "POST"},
		{"existing", "create", 200, 0, owned, false, nil, "POST GET"},
		{"previously deleted", "create", 200, 0, cancelled, false, nil, "POST GET"},
		{"unrelated collision", "create", 200, 0, `{}`, false, ErrEvent, "POST GET"},
		{"move", "replace", 200, 200, owned, true, nil, "GET PUT"},
		{"member deleted", "replace", 200, 0, cancelled, false, nil, "GET"},
		{"deleted with a retained marker", "replace", 200, 0, `{"status":"cancelled","extendedProperties":{"private":{"game_id":"123456"}}}`, false, nil, "GET"},
		{"tombstone gone", "replace", 410, 0, "", false, nil, "GET"},
		{"event missing", "replace", 404, 0, "", false, nil, "GET LIST"},
		{"calendar now inaccessible", "replace", 404, 404, "", false, ErrCalendar, "GET LIST"},
		{"calendar read only", "replace", 403, 0, `{}`, false, ErrCalendar, "GET"},
		{"Google busy", "replace", 429, 0, "", false, ErrBusy, "GET"},
		{"quota exceeded", "replace", 403, 0, `{"error":{"errors":[{"reason":"userRateLimitExceeded"}]}}`, false, ErrBusy, "GET"},
		{"unowned", "replace", 200, 0, `{"extendedProperties":{"private":{"game_id":"another"}}}`, false, ErrEvent, "GET"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method)
				switch r.Method {
				case "POST":
					if tc.readStatus != 0 {
						w.WriteHeader(409)
					}
				case "GET":
					if r.URL.Path == "/calendars/family/events" {
						calls[len(calls)-1] = "LIST"
						if tc.putStatus != 0 {
							w.WriteHeader(tc.putStatus)
						}
						return
					}
					w.WriteHeader(tc.readStatus)
					_, _ = io.WriteString(w, tc.readBody)
				case "PUT":
					if r.Header.Get("If-Match") != "version1" || r.URL.Path != "/calendars/family/events/123456" {
						t.Errorf("replace lacked its event path or read version")
					}
					w.WriteHeader(tc.putStatus)
				}
			}))
			defer srv.Close()
			client := &Client{HTTP: srv.Client(), CalendarURL: srv.URL}
			op := client.Create
			if tc.operation == "replace" {
				op = client.Replace
			}
			got, err := op(t.Context(), "access", "family", game)
			if got != tc.want || !errors.Is(err, tc.wantErr) || strings.Join(calls, " ") != tc.calls {
				t.Errorf("got %v, %v, calls %v", got, err, calls)
			}
		})
	}
}

func TestRefreshFailureKeepsClientErrorsSeparateFromMemberConsent(t *testing.T) {
	for _, tc := range []struct {
		status        int
		code          string
		expired, busy bool
	}{
		{400, "invalid_grant", true, false},
		{401, "invalid_client", false, false},
		{429, "rate_limit_exceeded", false, true},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, `{"error":"`+tc.code+`","error_description":"test-sensitive-response"}`)
		}))
		client := &Client{HTTP: srv.Client(), Endpoint: oauth2.Endpoint{TokenURL: srv.URL, AuthStyle: oauth2.AuthStyleInParams}}
		_, err := client.Access(t.Context(), Settings{ClientID: "test", ClientSecret: "test"}, "test-refresh")
		srv.Close()
		if err == nil || errors.Is(err, ErrExpired) != tc.expired || errors.Is(err, ErrBusy) != tc.busy || strings.Contains(err.Error(), "test-sensitive-response") {
			t.Fatalf("wrong refresh error classification for %s", tc.code)
		}
	}
}
