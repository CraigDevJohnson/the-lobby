package foundry

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const (
	startPath = "/api/foundry/server/start"
	stopPath  = "/api/foundry/server/stop"
	nobody    = `{"nobody_playing":true}`
)

const (
	starting = `{"state":"starting","join_url":"https://vtt.example.net:30000/join","started_at":null,"checked_at":"2026-10-14T01:00:00Z","checks":{"host":"pending","agent":"not_checked","foundry":"not_checked","stop":"not_checked"}}`
	stopping = `{"state":"stopping","join_url":"https://vtt.example.net:30000/join","started_at":"2026-10-13T18:00:00Z","checked_at":"2026-10-14T01:00:00Z","checks":{"host":"running","agent":"online","foundry":"responding","stop":"in_progress"}}`
)

// changeBackend answers a start or stop the way the controller does once it
// has begun.
func changeBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if strings.HasSuffix(r.URL.Path, "/stop") {
		_, _ = io.WriteString(w, stopping)
		return
	}
	_, _ = io.WriteString(w, starting)
}

// send makes a request with a body and headers, as the page's fetch would.
func send(t *testing.T, method, url, body string, header map[string]string) (*http.Response, string) {
	t.Helper()
	var content io.Reader
	if body != "" {
		content = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, content)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// change starts or stops the server the way the page does, the stop with
// the Member's confirmation.
func change(t *testing.T, siteURL, path string) (*http.Response, string) {
	t.Helper()
	body := ""
	if path == stopPath {
		body = nobody
	}
	return send(t, http.MethodPost, siteURL+path, body, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"})
}

// Starting and stopping need the foundry or foundry-admin grant, like the
// status, and nobody without one reaches the backend.
func TestChangesNeedAGrant(t *testing.T) {
	for _, path := range []string{startPath, stopPath} {
		for _, c := range []struct {
			name   string
			access Access
			err    error
			status int
			code   string
		}{
			{"signed out", SignedOut, nil, http.StatusUnauthorized, "signed_out"},
			{"no grant", NoAccess, nil, http.StatusForbidden, "no_access"},
			{"check failed", SignedOut, errors.New("store down"), http.StatusServiceUnavailable, "access_unavailable"},
		} {
			site, rec := newSite(t, c.access, c.err, changeBackend)
			res, body := change(t, site.URL, path)
			if res.StatusCode != c.status || code(t, body) != c.code {
				t.Errorf("%s %s: %d %s, want %d %s", path, c.name, res.StatusCode, body, c.status, c.code)
			}
			if res.Header.Get("Cache-Control") != "no-store" {
				t.Errorf("%s %s: must not be cached: %q", path, c.name, res.Header.Get("Cache-Control"))
			}
			if calls := rec.seen(); len(calls) != 0 {
				t.Errorf("%s %s: backend was called: %v", path, c.name, calls)
			}
			if grants := rec.askedFor(); len(grants) != 1 || grants[0] != Grant+" or "+AdminGrant {
				t.Errorf("%s %s: asked about grants %v, want %s or %s", path, c.name, grants, Grant, AdminGrant)
			}
		}

		site, rec := newSite(t, Granted, nil, changeBackend, func(tool *Tool) { tool.Check = nil })
		res, body := change(t, site.URL, path)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "access_unavailable" || len(rec.seen()) != 0 {
			t.Errorf("%s with no access check: %d %s, backend calls %v", path, res.StatusCode, body, rec.seen())
		}
	}
}

// A start or stop the browser says came from another site is refused before
// anything else happens. A request without the header, from a client that
// doesn't send it, goes on to the grant check.
func TestChangesFromAnotherSiteAreRefused(t *testing.T) {
	for _, path := range []string{startPath, stopPath} {
		for _, from := range []string{"cross-site", "same-site", "none"} {
			site, rec := newSite(t, Granted, nil, changeBackend)
			res, body := send(t, http.MethodPost, site.URL+path, nobody, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": from})
			if res.StatusCode != http.StatusForbidden || code(t, body) != "cross_site" {
				t.Errorf("%s from %s: %d %s", path, from, res.StatusCode, body)
			}
			if calls, grants := rec.seen(), rec.askedFor(); len(calls) != 0 || len(grants) != 0 {
				t.Errorf("%s from %s: backend calls %v, grants asked %v", path, from, calls, grants)
			}
		}

		site, rec := newSite(t, Granted, nil, changeBackend)
		body := ""
		if path == stopPath {
			body = nobody
		}
		res, answer := send(t, http.MethodPost, site.URL+path, body, nil)
		if res.StatusCode != http.StatusAccepted || len(rec.seen()) != 1 {
			t.Errorf("%s with no Sec-Fetch-Site: %d %s, backend calls %v", path, res.StatusCode, answer, rec.seen())
		}
	}
}

// A Member's start reaches the backend with no body, and a stop with
// exactly the confirmation, whatever else the page sent with it; both name
// the Member and pass the backend's status on as it answered.
func TestAdminChangesReachTheBackend(t *testing.T) {
	for _, c := range []struct {
		path, sent string
		want       seen
	}{
		{startPath, "", seen{http.MethodPost, "/server/start", "player@example.com", ""}},
		{stopPath, nobody, seen{http.MethodPost, "/server/stop", "player@example.com", nobody}},
		{stopPath, `{"note":"from the page", "nobody_playing" : true}`, seen{http.MethodPost, "/server/stop", "player@example.com", nobody}},
	} {
		var (
			mu          sync.Mutex
			contentType string
		)
		site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			contentType = r.Header.Get("Content-Type")
			mu.Unlock()
			changeBackend(w, r)
		})
		res, body := send(t, http.MethodPost, site.URL+c.path, c.sent, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"})
		begun := starting
		if c.path == stopPath {
			begun = stopping
		}
		if res.StatusCode != http.StatusAccepted || body != begun {
			t.Errorf("%s %q: %d %s", c.path, c.sent, res.StatusCode, body)
		}
		if res.Header.Get("Content-Type") != "application/json" || res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: headers %v", c.path, res.Header)
		}
		if calls := rec.seen(); len(calls) != 1 || calls[0] != c.want {
			t.Errorf("%s %q: backend saw %v, want %v", c.path, c.sent, calls, c.want)
		}
		mu.Lock()
		got := contentType
		mu.Unlock()
		if wantType := map[bool]string{true: "application/json", false: ""}[c.want.body != ""]; got != wantType {
			t.Errorf("%s: content type %q, want %q", c.path, got, wantType)
		}
	}
}

// A stop without the Member's confirmation that nobody is playing is
// refused at the site and never reaches the backend.
func TestAStopNeedsTheConfirmation(t *testing.T) {
	for _, sent := range []string{
		"",
		"null",
		"{}",
		"[true]",
		`{"nobody_playing":false}`,
		`{"nobody_playing":"true"}`,
		`{"nobody_playing":1}`,
		`{"nobody_playing":true} {}`,
		`{"nobody_playing":true`,
		`{"nobody_playing":true,"pad":"` + strings.Repeat("x", 1100) + `"}`,
	} {
		site, rec := newSite(t, Granted, nil, changeBackend)
		res, body := send(t, http.MethodPost, site.URL+stopPath, sent, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"})
		if res.StatusCode != http.StatusBadRequest || code(t, body) != "confirmation_required" {
			t.Errorf("stop with %.40q: %d %s", sent, res.StatusCode, body)
		}
		if calls := rec.seen(); len(calls) != 0 {
			t.Errorf("stop with %.40q: backend was called: %v", sent, calls)
		}
	}
}

// Each change is signed with the body it carries, after the Member is named,
// and one that can't be signed is never sent.
func TestChangesAreSignedWithTheirBody(t *testing.T) {
	for _, c := range []struct {
		path, body string
	}{
		{startPath, ""},
		{stopPath, nobody},
	} {
		var (
			mu         sync.Mutex
			signedBody []byte
			signedNil  bool
			signedFor  string
		)
		site, rec := newSite(t, Granted, nil, changeBackend, func(tool *Tool) {
			tool.Sign = func(r *http.Request, body []byte) error {
				mu.Lock()
				defer mu.Unlock()
				signedBody, signedNil, signedFor = body, body == nil, r.Header.Get(MemberHeader)
				return nil
			}
		})
		res, body := change(t, site.URL, c.path)
		mu.Lock()
		if res.StatusCode != http.StatusAccepted || string(signedBody) != c.body || signedNil != (c.body == "") || signedFor != "player@example.com" {
			t.Errorf("%s: %d %s, signed %q (nil %v) for %q", c.path, res.StatusCode, body, signedBody, signedNil, signedFor)
		}
		mu.Unlock()
		if calls := rec.seen(); len(calls) != 1 || calls[0].body != c.body {
			t.Errorf("%s: backend saw %v", c.path, calls)
		}

		site, rec = newSite(t, Granted, nil, changeBackend, func(tool *Tool) {
			tool.Sign = func(*http.Request, []byte) error { return errors.New("no credentials") }
		})
		res, body = change(t, site.URL, c.path)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || len(rec.seen()) != 0 {
			t.Errorf("%s unsigned: %d %s, backend calls %v", c.path, res.StatusCode, body, rec.seen())
		}
		if !strings.Contains(rec.logged(), "no credentials") {
			t.Errorf("%s: signing failure not logged: %s", c.path, rec.logged())
		}
	}
}

// The backend's answers the page can read come through: the status once the
// change began or when nothing needed changing, no server, and a refusal's
// reason, with nothing else from the refusal. Anything else is reported as
// the server not being changeable right now, without the join page, which
// only the status offers.
func TestChangeAnswersArePassedOnOrReported(t *testing.T) {
	answer := func(status int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}
	}
	joinPage := func(tool *Tool) { tool.JoinURL = "https://vtt.example.net:30000/join" }
	for _, path := range []string{startPath, stopPath} {
		// The reason this route gives for refusing, and one only the other
		// route gives.
		given, other := "stopping", "agent_offline"
		// The status the route began its change in, and the other route's.
		begun, otherBegun := starting, stopping
		if path == stopPath {
			given, other = "agent_offline", "stopping"
			begun, otherBegun = stopping, starting
		}
		for _, c := range []struct {
			name       string
			backend    http.HandlerFunc
			status     int
			body, code string
		}{
			{"began", answer(http.StatusAccepted, begun), http.StatusAccepted, begun, ""},
			{"began the other change", answer(http.StatusAccepted, otherBegun), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"began in a state no change begins in", answer(http.StatusAccepted, ready), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"nothing to change", answer(http.StatusOK, ready), http.StatusOK, ready, ""},
			{"no server", answer(http.StatusNotFound, `{"code":"no_server","message":"no host i-0abc"}`), http.StatusNotFound, "{\"code\":\"no_server\"}\n", ""},
			{"another 404", answer(http.StatusNotFound, `{"code":"not_found"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"no server with more after it", answer(http.StatusNotFound, `{"code":"no_server"}{"code":"no_server"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"refused", answer(http.StatusConflict, `{"code":"`+given+`","message":"the host i-0abc can't change","state":"running"}`), http.StatusConflict, "{\"code\":\"" + given + "\"}\n", ""},
			{"refused for the other route's reason", answer(http.StatusConflict, `{"code":"`+other+`"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"refused for an unknown reason", answer(http.StatusConflict, `{"code":"exploded"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"refused with more after it", answer(http.StatusConflict, `{"code":"`+given+`"} and more`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"refused without a reason", answer(http.StatusConflict, `{"message":"no"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"refused with the reason under another case", answer(http.StatusConflict, `{"Code":"`+given+`"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"refused unreadably", answer(http.StatusConflict, `stopping`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"status unknown", answer(http.StatusServiceUnavailable, `{"code":"status_unknown"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"backend refused the request", answer(http.StatusBadRequest, `{"code":"confirmation_required"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"AWS refused the signature", answer(http.StatusForbidden, `{"Message":"Forbidden"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"no content", answer(http.StatusNoContent, ""), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"too long", answer(http.StatusAccepted, `{"pad":"`+strings.Repeat("x", maxResponseBody)+`"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"began without JSON", answer(http.StatusAccepted, ""), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"nothing to change without JSON", answer(http.StatusOK, starting[:30]), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"began without a status", answer(http.StatusAccepted, `{}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"began in an unknown state", answer(http.StatusAccepted, strings.Replace(starting, `"starting"`, `"booting"`, 1)), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"nothing to change without the join page", answer(http.StatusOK, `{"state":"ready"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"began with its status under other keys", answer(http.StatusAccepted, `{"STATE":"starting","JOIN_URL":"https://vtt.example.net/join"}`), http.StatusServiceUnavailable, "", "tool_unavailable"},
			{"no server without JSON", answer(http.StatusNotFound, "404 page not found"), http.StatusServiceUnavailable, "", "tool_unavailable"},
		} {
			site, rec := newSite(t, Granted, nil, c.backend, joinPage)
			res, body := change(t, site.URL, path)
			if res.StatusCode != c.status {
				t.Errorf("%s %s: %d %s, want %d", path, c.name, res.StatusCode, body, c.status)
				continue
			}
			if c.code == "" && body != c.body {
				t.Errorf("%s %s: body %s, want %s", path, c.name, body, c.body)
			}
			if c.code != "" {
				if code(t, body) != c.code || !strings.Contains(body, "Foundry's server can't be changed right now") {
					t.Errorf("%s %s: %s", path, c.name, body)
				}
				if strings.Contains(body, "join_url") || strings.Contains(body, "vtt.example.net") {
					t.Errorf("%s %s: the join page or the backend's body came through: %s", path, c.name, body)
				}
				if !strings.Contains(rec.logged(), "level=ERROR") || !strings.Contains(rec.logged(), "foundry backend") {
					t.Errorf("%s %s: not logged as an error: %s", path, c.name, rec.logged())
				}
			}
			if res.Header.Get("Content-Type") != "application/json" || res.Header.Get("Cache-Control") != "no-store" {
				t.Errorf("%s %s: headers %v", path, c.name, res.Header)
			}
		}

		site, rec := newSite(t, Granted, nil, changeBackend, func(tool *Tool) { tool.Backend = "" })
		res, body := change(t, site.URL, path)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "not_configured" || len(rec.seen()) != 0 {
			t.Errorf("%s with no backend: %d %s, backend calls %v", path, res.StatusCode, body, rec.seen())
		}

		site, _ = newSite(t, Granted, nil, changeBackend, func(tool *Tool) { tool.Backend = "http://127.0.0.1:1" })
		res, body = change(t, site.URL, path)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || strings.Contains(body, "join_url") {
			t.Errorf("%s with an unreachable backend: %d %s", path, res.StatusCode, body)
		}
	}
}
