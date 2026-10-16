package minecraft

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
			{http.MethodPost, "/api/minecraft/server/stop"},
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

// Another site cannot start or stop the server through a Member's browser.
func TestStartAndStopRefuseCrossSiteRequests(t *testing.T) {
	site, calls := newSite(t, Granted, nil, statusBackend)
	for _, path := range []string{"/api/minecraft/server/start", "/api/minecraft/server/stop"} {
		res, body := call(t, http.MethodPost, site.URL+path, map[string]string{"Sec-Fetch-Site": "cross-site"})
		if res.StatusCode != http.StatusForbidden || code(t, body) != "cross_site" || len(*calls) != 0 {
			t.Fatalf("cross-site %s: %d %s, backend calls %v", path, res.StatusCode, body, *calls)
		}
	}
}

const stopping = `{"name":"Family server","state":"stopping","connection":{"java":{"address":"mc.example.net"}}}`

// A stop that began, one that changed nothing and the backend's refusals
// all reach the page as the backend answered them, a refusal with only its
// reason and never how many are playing, and the backend is told who asked.
func TestStopRelaysTheBackendsAnswer(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"stop began", http.StatusAccepted, stopping, stopping},
		{"nothing changed", http.StatusOK, stopping, stopping},
		{"players online", http.StatusConflict, `{"code":"players_online","players":2}`, `{"code":"players_online"}` + "\n"},
		{"players unknown", http.StatusConflict, `{"code":"players_unknown"}`, `{"code":"players_unknown"}` + "\n"},
		{"still starting", http.StatusConflict, `{"code":"starting"}`, `{"code":"starting"}` + "\n"},
	} {
		site, calls := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		})
		res, body := call(t, http.MethodPost, site.URL+"/api/minecraft/server/stop", map[string]string{"Sec-Fetch-Site": "same-origin"})
		if res.StatusCode != c.status || body != c.want {
			t.Errorf("%s: %d %q, want %d %q", c.name, res.StatusCode, body, c.status, c.want)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: must not be cached: %q", c.name, res.Header.Get("Cache-Control"))
		}
		want := seen{http.MethodPost, "/server/stop", "kid@example.com"}
		if len(*calls) != 1 || (*calls)[0] != want {
			t.Errorf("%s: backend saw %v, want %v", c.name, *calls, want)
		}
	}
}

// A stop the backend could not confirm is reported as such, never passed
// off as a refusal or a status. So is a refusal that gives no reason.
func TestStopTroubleIsReportedNotGuessed(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
	}{
		{"backend error", http.StatusInternalServerError, `{"code":"players_online","players":1}`},
		{"refusal without a reason", http.StatusConflict, `{"players":1}`},
		{"unreadable refusal", http.StatusConflict, `players online`},
	} {
		site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		})
		res, body := call(t, http.MethodPost, site.URL+"/api/minecraft/server/stop", map[string]string{"Sec-Fetch-Site": "same-origin"})
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
			t.Errorf("%s: %d %s", c.name, res.StatusCode, body)
		}
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

// An answer of exactly 64 KiB comes through whole; one byte more is refused
// rather than cut short and passed on.
func TestALongAnswerIsRefused(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, padded(maxResponseBody))
	})
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusOK || len(body) != maxResponseBody {
		t.Fatalf("64 KiB answer: %d, %d bytes, want 200 and %d", res.StatusCode, len(body), maxResponseBody)
	}

	site, _ = newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, padded(maxResponseBody+1))
	})
	res, body = call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("long answer: %d %s", res.StatusCode, body)
	}
}

// padded is the stopped status, padded with spaces to exactly n bytes.
func padded(n int) string {
	return stopped + strings.Repeat(" ", n-len(stopped))
}

// An answer that breaks off after its status is reported as trouble, not
// passed on as a 200 with half a body.
func TestABrokenAnswerIsReported(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n" + stopped[:30])
		_ = buf.Flush()
	})
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("broken answer: %d %s", res.StatusCode, body)
	}
}

// An answer the backend sent whole but not as JSON is reported, never
// passed on for the page to fail on.
func TestAnAnswerThatIsNotJSONIsReported(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
	}{
		{http.StatusOK, ""},
		{http.StatusOK, stopped[:30]},
		{http.StatusAccepted, "<html>Accepted</html>"},
		{http.StatusNotFound, "404 page not found"},
	} {
		site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		})
		res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
			t.Errorf("%d %q: %d %s", c.status, c.body, res.StatusCode, body)
		}
	}
}

// A redirect from the backend is never followed, so the Member's email never
// goes anywhere but the backend, and it is reported rather than passed on.
func TestRedirectsAreNotFollowed(t *testing.T) {
	var elsewhere sync.Mutex
	var reached []string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere.Lock()
		reached = append(reached, r.Header.Get(MemberHeader))
		elsewhere.Unlock()
		statusBackend(w, r)
	}))
	t.Cleanup(other.Close)
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+r.URL.Path, status)
		})
		for _, req := range []struct{ method, path string }{
			{http.MethodGet, "/api/minecraft/server"},
			{http.MethodPost, "/api/minecraft/server/start"},
			{http.MethodPost, "/api/minecraft/server/stop"},
		} {
			res, body := call(t, req.method, site.URL+req.path, map[string]string{"Sec-Fetch-Site": "same-origin"})
			if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
				t.Errorf("redirect %d on %s %s: %d %s", status, req.method, req.path, res.StatusCode, body)
			}
		}
	}
	elsewhere.Lock()
	defer elsewhere.Unlock()
	if len(reached) != 0 {
		t.Errorf("a redirect was followed, carrying %q", reached)
	}
}

// The deployed site passes its own client, with a timeout. Its redirect
// policy is overridden on a copy, never on the client itself, and its
// timeout still holds.
func TestAConfiguredClientKeepsItsTimeoutButNotItsRedirects(t *testing.T) {
	var elsewhere sync.Mutex
	followed := 0
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		elsewhere.Lock()
		followed++
		elsewhere.Unlock()
	}))
	t.Cleanup(other.Close)
	stall := make(chan struct{})
	defer close(stall) // before the servers close, which wait for the stalled answer
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.Redirect(w, r, other.URL+"/server", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		http.NewResponseController(w).Flush()
		<-stall
	}))
	t.Cleanup(be.Close)
	follow := func(*http.Request, []*http.Request) error { return nil }
	client := &http.Client{Timeout: 200 * time.Millisecond, CheckRedirect: follow}
	tool := &Tool{
		Backend: be.URL,
		Check: func(http.ResponseWriter, *http.Request) (Access, string, error) {
			return Granted, "kid@example.com", nil
		},
		Client: client,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)

	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("redirect: %d %s", res.StatusCode, body)
	}
	elsewhere.Lock()
	if followed != 0 {
		t.Errorf("the configured client followed a redirect")
	}
	elsewhere.Unlock()
	// follow allows every redirect; an override set on the client itself would refuse it.
	if client.CheckRedirect == nil || client.CheckRedirect(nil, nil) != nil || client.Timeout != 200*time.Millisecond {
		t.Errorf("the configured client was changed: %+v", client)
	}

	start := time.Now()
	res, body = call(t, http.MethodPost, site.URL+"/api/minecraft/server/start", map[string]string{"Sec-Fetch-Site": "same-origin"})
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || time.Since(start) > 5*time.Second {
		t.Fatalf("stalled answer: %d %s after %s, want 503 once the client's timeout passes", res.StatusCode, body, time.Since(start))
	}
}

// A refusal that runs past 64 KiB is reported rather than read in part.
func TestALongRefusalIsReported(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		refusal := `{"code":"players_online"}`
		_, _ = io.WriteString(w, refusal+strings.Repeat(" ", maxResponseBody+1-len(refusal)))
	})
	res, body := call(t, http.MethodPost, site.URL+"/api/minecraft/server/stop", map[string]string{"Sec-Fetch-Site": "same-origin"})
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("long refusal: %d %s", res.StatusCode, body)
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
