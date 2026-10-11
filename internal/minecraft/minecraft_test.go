package minecraft

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type seen struct {
	method, path, member string
}

// newSite serves the Launcher's routes in front of a fake backend, with the
// access check answering as given.
func newSite(t *testing.T, access Access, checkErr error, backend http.HandlerFunc) (*httptest.Server, *[]seen) {
	t.Helper()
	var calls []seen
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, seen{r.Method, r.URL.Path, r.Header.Get(MemberHeader)})
		backend(w, r)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{
		Backend: be.URL,
		Check: func(http.ResponseWriter, *http.Request) (Access, string, error) {
			if access == SignedOut {
				return access, "", checkErr
			}
			return access, "kid@example.com", checkErr
		},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	return site, &calls
}

func call(t *testing.T, method, url string, header map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
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

func code(t *testing.T, body string) string {
	t.Helper()
	var e struct{ Code string }
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("not an error body: %q", body)
	}
	return e.Code
}

const stopped = `{"name":"Family server","state":"stopped","connection":{"java":{"address":"mc.example.net"}}}`

func statusBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, strings.Replace(stopped, "stopped", "starting", 1))
		return
	}
	_, _ = io.WriteString(w, stopped)
}

// Nothing about the server reaches anyone without the Launcher's grant, and
// the backend is never asked on their behalf.
func TestOnlyGrantedMembersReachTheServer(t *testing.T) {
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
		site, calls := newSite(t, c.access, c.err, statusBackend)
		for _, req := range []struct{ method, path string }{
			{http.MethodGet, "/api/minecraft/server"},
			{http.MethodPost, "/api/minecraft/server/start"},
		} {
			res, body := call(t, req.method, site.URL+req.path, nil)
			if res.StatusCode != c.status || code(t, body) != c.code {
				t.Errorf("%s %s %s: %d %s, want %d %s", c.name, req.method, req.path, res.StatusCode, body, c.status, c.code)
			}
			if strings.Contains(body, "Family server") || strings.Contains(body, "mc.example.net") {
				t.Errorf("%s: server details leaked: %s", c.name, body)
			}
		}
		if len(*calls) != 0 {
			t.Errorf("%s: backend was called: %v", c.name, *calls)
		}
	}
}

// A granted Member gets the backend's status, and the backend is told who
// is asking.
func TestGrantedMemberGetsStatusAndStarts(t *testing.T) {
	site, calls := newSite(t, Granted, nil, statusBackend)
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusOK || body != stopped {
		t.Fatalf("status: %d %s", res.StatusCode, body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status must not be cached: %q", res.Header.Get("Cache-Control"))
	}
	res, body = call(t, http.MethodPost, site.URL+"/api/minecraft/server/start", map[string]string{"Sec-Fetch-Site": "same-origin"})
	if res.StatusCode != http.StatusAccepted || !strings.Contains(body, `"starting"`) {
		t.Fatalf("start: %d %s", res.StatusCode, body)
	}
	want := []seen{{http.MethodGet, "/server", "kid@example.com"}, {http.MethodPost, "/server/start", "kid@example.com"}}
	if len(*calls) != 2 || (*calls)[0] != want[0] || (*calls)[1] != want[1] {
		t.Fatalf("backend saw %v, want %v", *calls, want)
	}
}

// Another site cannot start the server through a Member's browser.
func TestStartRefusesCrossSiteRequests(t *testing.T) {
	site, calls := newSite(t, Granted, nil, statusBackend)
	res, body := call(t, http.MethodPost, site.URL+"/api/minecraft/server/start", map[string]string{"Sec-Fetch-Site": "cross-site"})
	if res.StatusCode != http.StatusForbidden || code(t, body) != "cross_site" || len(*calls) != 0 {
		t.Fatalf("cross-site start: %d %s, backend calls %v", res.StatusCode, body, *calls)
	}
}

// When the backend is missing, down or answers something the page can't
// read, the page is told so in its own terms and never handed a guess.
func TestBackendTroubleIsReportedNotGuessed(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"state":"ready"}`)
	})
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("backend error: %d %s", res.StatusCode, body)
	}

	tool := &Tool{Check: func(http.ResponseWriter, *http.Request) (Access, string, error) {
		return Granted, "kid@example.com", nil
	}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	r := chi.NewRouter()
	tool.Routes(r)
	unset := httptest.NewServer(r)
	t.Cleanup(unset.Close)
	res, body = call(t, http.MethodGet, unset.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "not_configured" {
		t.Fatalf("no backend: %d %s", res.StatusCode, body)
	}

	down := &Tool{Backend: "http://127.0.0.1:1", Check: tool.Check, Log: tool.Log}
	r = chi.NewRouter()
	down.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	res, body = call(t, http.MethodGet, srv.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("unreachable backend: %d %s", res.StatusCode, body)
	}
}

// The backend's own "no server" answer comes through for the page to explain.
func TestNoServerComesThrough(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"code":"no_server","message":"no server is configured"}`)
	})
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusNotFound || code(t, body) != "no_server" {
		t.Fatalf("no server: %d %s", res.StatusCode, body)
	}
}
