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
		Check: func(_ http.ResponseWriter, _ *http.Request, grants ...string) (Access, string, error) {
			if len(grants) != 1 || grants[0] != Grant {
				t.Errorf("the Launcher's routes asked for %v, want only %q", grants, Grant)
			}
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

	tool := &Tool{Check: func(http.ResponseWriter, *http.Request, ...string) (Access, string, error) {
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
		Check: func(http.ResponseWriter, *http.Request, ...string) (Access, string, error) {
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

// worldSite serves the Launcher's routes for a granted Member in front of a
// backend that records each request's body, and a signer that records what
// it was asked to sign.
type sent struct {
	path, contentType, body string
	signed                  []byte
}

func worldSite(t *testing.T, backend http.HandlerFunc) (*httptest.Server, *[]sent) {
	t.Helper()
	var calls []sent
	var signed [][]byte
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c := sent{path: r.URL.Path, contentType: r.Header.Get("Content-Type"), body: string(b)}
		if n := len(signed); n > 0 {
			c.signed = signed[n-1]
		}
		calls = append(calls, c)
		backend(w, r)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{
		Backend: be.URL,
		Check: func(http.ResponseWriter, *http.Request, ...string) (Access, string, error) {
			return Granted, "kid@example.com", nil
		},
		Sign: func(_ *http.Request, body []byte) error {
			signed = append(signed, body)
			return nil
		},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	return site, &calls
}

func post(t *testing.T, url, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// A start or stop names its world in a body the site builds, and that same
// body is what gets signed. Without a world the backend gets no body, which
// it reads as its first world, as before there were worlds.
func TestStartAndStopNameTheirWorld(t *testing.T) {
	site, calls := worldSite(t, statusBackend)
	for _, path := range []string{"/server/start", "/server/stop"} {
		*calls = nil
		res, body := post(t, site.URL+"/api/minecraft"+path, `{"world":"creative"}`)
		if res.StatusCode != http.StatusAccepted {
			t.Fatalf("%s: %d %s", path, res.StatusCode, body)
		}
		want := `{"world":"creative"}`
		if len(*calls) != 1 {
			t.Fatalf("%s: backend saw %v", path, *calls)
		}
		c := (*calls)[0]
		if c.path != path || c.body != want || c.contentType != "application/json" || string(c.signed) != want {
			t.Errorf("%s: backend got %+v (signed %q), want body %s signed as sent", path, c, c.signed, want)
		}

		*calls = nil
		for _, none := range []string{"", "  ", "{}"} {
			if res, body := post(t, site.URL+"/api/minecraft"+path, none); res.StatusCode != http.StatusAccepted {
				t.Fatalf("%s with %q: %d %s", path, none, res.StatusCode, body)
			}
		}
		for _, c := range *calls {
			if c.body != "" || c.contentType != "" || c.signed != nil {
				t.Errorf("%s without a world: backend got %+v, want no body", path, c)
			}
		}
	}
}

// Only the world's id reaches the backend: a force or a restore archive sent
// from a browser is dropped, because they are operator-only.
func TestABrowsersForceNeverReachesTheBackend(t *testing.T) {
	site, calls := worldSite(t, statusBackend)
	post(t, site.URL+"/api/minecraft/server/stop", `{"world":"creative","force":true,"archive":"x"}`)
	post(t, site.URL+"/api/minecraft/server/stop", `{"force":true}`)
	post(t, site.URL+"/api/minecraft/server/start", `{"archive":"instances/creative/crafty/old.tar.zst"}`)
	if len(*calls) != 3 {
		t.Fatalf("backend saw %v", *calls)
	}
	for i, want := range []string{`{"world":"creative"}`, "", ""} {
		if got := (*calls)[i].body; got != want {
			t.Errorf("request %d: backend got %q, want %q", i, got, want)
		}
	}
}

// A body that isn't a world's id is refused by the site, and the backend is
// never asked.
func TestABadWorldIsRefusedBeforeTheBackend(t *testing.T) {
	site, calls := worldSite(t, statusBackend)
	for _, body := range []string{
		`{"world":"Creative"}`,
		`{"world":""}`,
		`{"world":"creative world"}`,
		`{"world":"a123456789012345678901234567890123"}`,
		`{"world":7}`,
		`{"world":null}`,
		`[]`,
		`null`,
		`"creative"`,
		`not json`,
		`{"world":"creative"`,
		`{"world":"creative","pad":"` + strings.Repeat("x", 1100) + `"}`,
	} {
		for _, path := range []string{"/server/start", "/server/stop"} {
			res, answer := post(t, site.URL+"/api/minecraft"+path, body)
			if res.StatusCode != http.StatusBadRequest || code(t, answer) != "bad_request" {
				t.Errorf("%s %.40q: %d %s, want 400 bad_request", path, body, res.StatusCode, answer)
			}
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("backend was asked: %v", *calls)
	}
}

// A world the backend doesn't have comes back as its reason, for the page to
// read the list again.
func TestUnknownWorldComesThrough(t *testing.T) {
	site, _ := worldSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"code":"unknown_world","world":"moon"}`)
	})
	for _, path := range []string{"/server/start", "/server/stop"} {
		res, body := post(t, site.URL+"/api/minecraft"+path, `{"world":"moon"}`)
		if res.StatusCode != http.StatusConflict || strings.TrimSpace(body) != `{"code":"unknown_world"}` {
			t.Errorf("%s: %d %s, want 409 unknown_world alone", path, res.StatusCode, body)
		}
	}
}

// The worlds come through as the backend listed them, in its order.
func TestTheWorldsComeThrough(t *testing.T) {
	const worlds = `{"name":"Survival","state":"stopped","connection":{},"worlds":[` +
		`{"id":"survival","name":"Survival","state":"stopped","connection":{}},` +
		`{"id":"creative","name":"Creative","state":"ready","connection":{"java":{"address":"creative.mc.example.net"}}}]}`
	site, calls := worldSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, worlds)
	})
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/server", nil)
	if res.StatusCode != http.StatusOK || body != worlds {
		t.Fatalf("status: %d %s", res.StatusCode, body)
	}
	if len(*calls) != 1 || (*calls)[0].body != "" || (*calls)[0].signed != nil {
		t.Fatalf("a status check sends no body: %+v", *calls)
	}
}

// modsSite serves the routes for a Member holding the given grants, in front
// of a backend that records each request and answers the mods list.
type modsCall struct {
	method, path, member, body string
}

const modsList = `{"worlds":[{"id":"creative","name":"Creative","mods":[{"id":"fawe","name":"FastAsyncWorldEdit","version":"2.16.2","summary":"Bulk building tools.","source":"https://modrinth.com/mod/fawe","default":true,"enabled":false}]}]}`

func modsSite(t *testing.T, grants []string, backend http.HandlerFunc) (*httptest.Server, *[]modsCall) {
	t.Helper()
	var calls []modsCall
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, modsCall{r.Method, r.URL.Path, r.Header.Get(MemberHeader), string(b)})
		backend(w, r)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{
		Backend: be.URL,
		Check: func(_ http.ResponseWriter, _ *http.Request, want ...string) (Access, string, error) {
			if grants == nil {
				return SignedOut, "", nil
			}
			for _, g := range want {
				for _, h := range grants {
					if g == h {
						return Granted, "craig@example.com", nil
					}
				}
			}
			return NoAccess, "craig@example.com", nil
		},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	return site, &calls
}

func modsBackend(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, modsList)
}

// The mods routes are Minecraft Admin's: the Launcher's grant doesn't reach
// them, and minecraft-admin doesn't reach the Launcher's.
func TestEachGrantReachesOnlyItsOwnRoutes(t *testing.T) {
	for _, c := range []struct {
		name   string
		grants []string
		status int
		code   string
	}{
		{"signed out", nil, http.StatusUnauthorized, "signed_out"},
		{"no grant", []string{"soccer"}, http.StatusForbidden, "no_access"},
		{"launcher only", []string{Grant}, http.StatusForbidden, "no_access"},
	} {
		site, calls := modsSite(t, c.grants, modsBackend)
		res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/mods", nil)
		if res.StatusCode != c.status || code(t, body) != c.code {
			t.Errorf("%s, GET mods: %d %s", c.name, res.StatusCode, body)
		}
		res, body = post(t, site.URL+"/api/minecraft/mods", `{"world":"creative","mod":"fawe","enabled":false}`)
		if res.StatusCode != c.status || code(t, body) != c.code {
			t.Errorf("%s, POST mods: %d %s", c.name, res.StatusCode, body)
		}
		if len(*calls) != 0 {
			t.Errorf("%s: backend was asked: %v", c.name, *calls)
		}
	}
	site, calls := modsSite(t, []string{AdminGrant}, modsBackend)
	for _, path := range []string{"/server", "/server/start", "/server/stop"} {
		method := http.MethodPost
		if path == "/server" {
			method = http.MethodGet
		}
		res, body := call(t, method, site.URL+"/api/minecraft"+path, map[string]string{"Sec-Fetch-Site": "same-origin"})
		if res.StatusCode != http.StatusForbidden || code(t, body) != "no_access" {
			t.Errorf("minecraft-admin only, %s %s: %d %s", method, path, res.StatusCode, body)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("backend was asked for an admin-only Member: %v", *calls)
	}
}

// Minecraft Admin reads the mods and switches one; the backend gets the
// site-built switch and never the Member's name.
func TestAdminReadsAndSwitchesMods(t *testing.T) {
	site, calls := modsSite(t, []string{"soccer", AdminGrant}, modsBackend)
	res, body := call(t, http.MethodGet, site.URL+"/api/minecraft/mods", nil)
	if res.StatusCode != http.StatusOK || body != modsList || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET mods: %d %s", res.StatusCode, body)
	}
	res, body = post(t, site.URL+"/api/minecraft/mods", `{"enabled":false,"world":"creative","mod":"fawe","member":"x"}`)
	if res.StatusCode != http.StatusOK || body != modsList {
		t.Fatalf("POST mods: %d %s", res.StatusCode, body)
	}
	want := []modsCall{
		{http.MethodGet, "/mods", "", ""},
		{http.MethodPost, "/mods", "", `{"world":"creative","mod":"fawe","enabled":false}`},
	}
	if len(*calls) != 2 || (*calls)[0] != want[0] || (*calls)[1] != want[1] {
		t.Fatalf("backend saw %+v, want %+v", *calls, want)
	}
}

// A switch that isn't one is refused by the site, and the backend is never
// asked; so is one from another site.
func TestABadSwitchIsRefusedBeforeTheBackend(t *testing.T) {
	site, calls := modsSite(t, []string{AdminGrant}, modsBackend)
	for _, body := range []string{
		``,
		`{}`,
		`{"world":"creative","mod":"fawe"}`,
		`{"world":"creative","mod":"fawe","enabled":"false"}`,
		`{"world":"creative","mod":"fawe","enabled":null}`,
		`{"world":"Creative","mod":"fawe","enabled":true}`,
		`{"world":"creative","mod":"../fawe","enabled":true}`,
		`{"mod":"fawe","enabled":true}`,
		`[]`,
		`not json`,
		`{"world":"creative","mod":"fawe","enabled":true,"pad":"` + strings.Repeat("x", 1100) + `"}`,
	} {
		res, answer := post(t, site.URL+"/api/minecraft/mods", body)
		if res.StatusCode != http.StatusBadRequest || code(t, answer) != "bad_request" {
			t.Errorf("%.40q: %d %s, want 400 bad_request", body, res.StatusCode, answer)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, site.URL+"/api/minecraft/mods", strings.NewReader(`{"world":"creative","mod":"fawe","enabled":true}`))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site switch: %d", res.StatusCode)
	}
	if len(*calls) != 0 {
		t.Fatalf("backend was asked: %+v", *calls)
	}
}

// A world or mod the backend doesn't know comes back as its reason alone.
func TestUnknownModComesThrough(t *testing.T) {
	for _, reason := range []string{"unknown_world", "unknown_mod"} {
		site, _ := modsSite(t, []string{AdminGrant}, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"code":"`+reason+`","mod":"moon"}`)
		})
		res, body := post(t, site.URL+"/api/minecraft/mods", `{"world":"creative","mod":"moon","enabled":true}`)
		if res.StatusCode != http.StatusConflict || strings.TrimSpace(body) != `{"code":"`+reason+`"}` {
			t.Errorf("%s: %d %s", reason, res.StatusCode, body)
		}
	}
}
