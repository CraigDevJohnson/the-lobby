package foundry

import (
	"bytes"
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
	method, path, member, body string
}

// record keeps what the fake backend saw, which grants the access check was
// asked about (each check's grants joined with " or ") and what the site
// logged, for the test to read once the answer is in.
type record struct {
	mu     sync.Mutex
	calls  []seen
	grants []string
	log    bytes.Buffer
}

func (rec *record) Write(p []byte) (int, error) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.log.Write(p)
}

func (rec *record) saw(s seen) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.calls = append(rec.calls, s)
}

func (rec *record) asked(grant string) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.grants = append(rec.grants, grant)
}

func (rec *record) askedFor() []string {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return append([]string(nil), rec.grants...)
}

func (rec *record) seen() []seen {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return append([]seen(nil), rec.calls...)
}

func (rec *record) logged() string {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.log.String()
}

// newSite serves Foundry's routes in front of a fake backend, with the
// access check answering as given for whichever grant it is asked about.
// configure, if given, changes the Tool before it serves.
func newSite(t *testing.T, access Access, checkErr error, backend http.HandlerFunc, configure ...func(*Tool)) (*httptest.Server, *record) {
	t.Helper()
	rec := &record{}
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.saw(seen{r.Method, r.URL.Path, r.Header.Get(MemberHeader), string(body)})
		r.Body = io.NopCloser(bytes.NewReader(body))
		backend(w, r)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{
		Backend: be.URL,
		Check: func(_ http.ResponseWriter, _ *http.Request, grants ...string) (Access, string, error) {
			rec.asked(strings.Join(grants, " or "))
			if access == SignedOut {
				return access, "", checkErr
			}
			return access, "player@example.com", checkErr
		},
		Log: slog.New(slog.NewTextHandler(rec, nil)),
	}
	for _, c := range configure {
		c(tool)
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	return site, rec
}

func call(t *testing.T, method, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
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

const ready = `{"state":"ready","join_url":"https://vtt.example.net:30000/join","started_at":"2026-10-13T18:00:00Z","checked_at":"2026-10-13T18:03:12Z","checks":{"host":"running","agent":"online","foundry":"responding"}}`

func statusBackend(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, ready)
}

const statusPath = "/api/foundry/server"

// Nothing about the server reaches anyone without Foundry's grant, and the
// backend is never asked on their behalf.
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
		site, rec := newSite(t, c.access, c.err, statusBackend)
		res, body := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != c.status || code(t, body) != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, res.StatusCode, body, c.status, c.code)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: must not be cached: %q", c.name, res.Header.Get("Cache-Control"))
		}
		if strings.Contains(body, "vtt.example.net") || strings.Contains(body, "running") {
			t.Errorf("%s: server details leaked: %s", c.name, body)
		}
		if calls := rec.seen(); len(calls) != 0 {
			t.Errorf("%s: backend was called: %v", c.name, calls)
		}
	}

	site, rec := newSite(t, Granted, nil, statusBackend, func(tool *Tool) { tool.Check = nil })
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "access_unavailable" || len(rec.seen()) != 0 {
		t.Errorf("no access check: %d %s, backend calls %v", res.StatusCode, body, rec.seen())
	}
}

// A granted Member gets the backend's status as it answered, never cached,
// and the backend is told who is asking.
func TestGrantedMemberGetsTheStatus(t *testing.T) {
	site, rec := newSite(t, Granted, nil, statusBackend)
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusOK || body != ready {
		t.Fatalf("status: %d %s", res.StatusCode, body)
	}
	if res.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content type: %q", res.Header.Get("Content-Type"))
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("status must not be cached: %q", res.Header.Get("Cache-Control"))
	}
	want := seen{http.MethodGet, "/server", "player@example.com", ""}
	if calls := rec.seen(); len(calls) != 1 || calls[0] != want {
		t.Fatalf("backend saw %v, want %v", calls, want)
	}
	if grants := rec.askedFor(); len(grants) != 1 || grants[0] != Grant+" or "+AdminGrant {
		t.Errorf("status asked about grants %v, want %s or %s", grants, Grant, AdminGrant)
	}
}

// Each request is signed after the Member is named, so the signature covers
// who is asking, and a request that can't be signed is never sent.
func TestRequestsAreSignedBeforeTheyLeave(t *testing.T) {
	var (
		mu        sync.Mutex
		signedFor string
	)
	sign := func(tool *Tool) {
		tool.Sign = func(r *http.Request, body []byte) error {
			if body != nil {
				return errors.New("a status request has no body")
			}
			mu.Lock()
			signedFor = r.Header.Get(MemberHeader)
			mu.Unlock()
			r.Header.Set("Authorization", "test-signature")
			return nil
		}
	}
	site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "test-signature" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		statusBackend(w, r)
	}, sign)
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	mu.Lock()
	who := signedFor
	mu.Unlock()
	if res.StatusCode != http.StatusOK || body != ready || who != "player@example.com" || len(rec.seen()) != 1 {
		t.Fatalf("signed status: %d %s, signed for %q, backend calls %v", res.StatusCode, body, who, rec.seen())
	}

	site, rec = newSite(t, Granted, nil, statusBackend, func(tool *Tool) {
		tool.Sign = func(*http.Request, []byte) error { return errors.New("no credentials") }
	})
	res, body = call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || len(rec.seen()) != 0 {
		t.Fatalf("unsigned: %d %s, backend calls %v", res.StatusCode, body, rec.seen())
	}
	if !strings.Contains(rec.logged(), "no credentials") {
		t.Errorf("signing failure not logged: %s", rec.logged())
	}
}

// The backend's own "no server" answer comes through for the page to explain.
func TestNoServerComesThrough(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"code":"no_server"}`)
	})
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusNotFound || code(t, body) != "no_server" {
		t.Fatalf("no server: %d %s", res.StatusCode, body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("no server must not be cached: %q", res.Header.Get("Cache-Control"))
	}
}

// A 404 that isn't the backend's "no server", such as one for a route it
// doesn't know, is reported as a failed check, and the player still gets the
// join page.
func TestOnlyNoServerComesThroughAsA404(t *testing.T) {
	for _, answer := range []string{`{"code":"not_found"}`, `{"message":"Not Found"}`, `[]`, `{"code":"no_server"} and more`, `{"code":"no_server"}{"code":"no_server"}`, `{"CODE":"no_server"}`} {
		site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, answer)
		}, func(tool *Tool) { tool.JoinURL = joinPage })
		res, body := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || !strings.Contains(body, joinPage) {
			t.Errorf("404 %s: %d %s, want tool_unavailable with the join page", answer, res.StatusCode, body)
		}
		if !strings.Contains(rec.logged(), "foundry backend") {
			t.Errorf("404 %s: not logged: %s", answer, rec.logged())
		}
	}
}

// When the backend answers something the page can't read, including AWS
// refusing the site's signature, the page is told so in its own terms and
// never handed a guess, and the trouble is logged.
func TestBackendTroubleIsReportedNotGuessed(t *testing.T) {
	for _, status := range []int{
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusConflict,
		http.StatusInternalServerError,
		http.StatusBadGateway,
	} {
		site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, ready)
		})
		res, body := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
			t.Errorf("backend %d: %d %s", status, res.StatusCode, body)
		}
		if strings.Contains(body, "vtt.example.net") {
			t.Errorf("backend %d: its body came through: %s", status, body)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("backend %d: must not be cached: %q", status, res.Header.Get("Cache-Control"))
		}
		if !strings.Contains(rec.logged(), "level=ERROR") || !strings.Contains(rec.logged(), "foundry backend") {
			t.Errorf("backend %d: not logged as an error: %s", status, rec.logged())
		}
	}
}

// With no backend configured the page is told so, and with one that is down
// or too slow for the site's client the page is told the status can't be
// checked.
func TestMissingOrUnreachableBackendIsReported(t *testing.T) {
	site, rec := newSite(t, Granted, nil, statusBackend, func(tool *Tool) { tool.Backend = "" })
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "not_configured" || len(rec.seen()) != 0 {
		t.Fatalf("no backend: %d %s, backend calls %v", res.StatusCode, body, rec.seen())
	}
	if !strings.Contains(body, "Foundry has no server configured") {
		t.Errorf("no backend: %s", body)
	}

	site, rec = newSite(t, Granted, nil, statusBackend, func(tool *Tool) { tool.Backend = "http://127.0.0.1:1" })
	res, body = call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("unreachable backend: %d %s", res.StatusCode, body)
	}
	if !strings.Contains(rec.logged(), "foundry backend") {
		t.Errorf("unreachable backend not logged: %s", rec.logged())
	}

	site, _ = newSite(t, Granted, nil, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		statusBackend(w, r)
	}, func(tool *Tool) { tool.Client = &http.Client{Timeout: 50 * time.Millisecond} })
	res, body = call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("slow backend: %d %s", res.StatusCode, body)
	}
}

// Only the status, start and stop are forwarded, each by its own method, and
// a wrong method or another path never reaches the backend.
func TestOnlyTheKnownRoutesAreForwarded(t *testing.T) {
	site, rec := newSite(t, Granted, nil, statusBackend)
	for _, c := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPost, statusPath, http.StatusMethodNotAllowed},
		{http.MethodDelete, statusPath, http.StatusMethodNotAllowed},
		{http.MethodGet, startPath, http.StatusMethodNotAllowed},
		{http.MethodPut, stopPath, http.StatusMethodNotAllowed},
		{http.MethodPost, statusPath + "/restart", http.StatusNotFound},
		{http.MethodPost, stopPath + "/now", http.StatusNotFound},
	} {
		if res, body := call(t, c.method, site.URL+c.path); res.StatusCode != c.status {
			t.Errorf("%s %s: %d %s, want %d", c.method, c.path, res.StatusCode, body, c.status)
		}
	}
	if calls := rec.seen(); len(calls) != 0 {
		t.Errorf("backend was called: %v", calls)
	}
}

// An answer up to 64 KiB comes through whole; a longer one is refused, not
// cut into JSON the page can't read.
// padded is a status of exactly n bytes.
func padded(n int) string {
	const head = `{"state":"ready","join_url":"https://vtt.example.net:30000/join","pad":"`
	return head + strings.Repeat("x", n-len(head)-len(`"}`)) + `"}`
}

func TestALongAnswerIsRefused(t *testing.T) {
	site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, padded(maxResponseBody))
	})
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusOK || len(body) != maxResponseBody {
		t.Fatalf("64 KiB answer: %d, %d bytes, want 200 and %d", res.StatusCode, len(body), maxResponseBody)
	}

	site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, padded(maxResponseBody+1))
	})
	res, body = call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("long answer: %d %s", res.StatusCode, body)
	}
	if !strings.Contains(rec.logged(), "more than 64 KiB") {
		t.Errorf("long answer not logged: %s", rec.logged())
	}
}

// An answer that breaks off after its status is reported as trouble, not
// passed on as a 200 with half a body.
func TestABrokenAnswerIsReported(t *testing.T) {
	site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n" + ready[:40])
		_ = buf.Flush()
	})
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
		t.Fatalf("broken answer: %d %s", res.StatusCode, body)
	}
	if !strings.Contains(rec.logged(), "foundry backend") {
		t.Errorf("broken answer not logged: %s", rec.logged())
	}
}

// An answer the backend sent whole but not as JSON, with a status the page
// reads, is reported, never passed on for the page to fail on.
func TestAnAnswerThatIsNotJSONIsReported(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
	}{
		{http.StatusOK, ""},
		{http.StatusOK, ready[:40]},
		{http.StatusNotFound, "404 page not found"},
		{http.StatusNotFound, "<html>Not Found</html>"},
	} {
		site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		}, func(tool *Tool) { tool.JoinURL = joinPage })
		res, body := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || !strings.Contains(body, joinPage) {
			t.Errorf("%d %q: %d %s", c.status, c.body, res.StatusCode, body)
		}
		if !strings.Contains(rec.logged(), "without JSON") {
			t.Errorf("%d %q: not logged: %s", c.status, c.body, rec.logged())
		}
	}
}

// A 200 that isn't a status the page can show, with one of its states and
// the join page, is reported with the join page, never passed on.
func TestAnAnswerThatIsNotAStatusIsReported(t *testing.T) {
	for _, body := range []string{
		`null`,
		`{}`,
		`[]`,
		`"ready"`,
		`{"code":"status_unknown"}`,
		`{"state":"ready"}`,
		`{"state":"ready","join_url":""}`,
		strings.Replace(ready, `"https://vtt.example.net:30000/join"`, `"not a URL"`, 1),
		strings.Replace(ready, `"https://vtt.example.net:30000/join"`, `"http://vtt.example.net:30000/join"`, 1),
		strings.Replace(ready, `"https://vtt.example.net:30000/join"`, `"https://vtt.example.net:30000/join?next=https://elsewhere.example"`, 1),
		strings.Replace(ready, `"https://vtt.example.net:30000/join"`, `"javascript:alert(1)//https://vtt.example.net/join"`, 1),
		strings.Replace(ready, `"https://vtt.example.net:30000/join"`, `"https://vtt.example.net:30000/game"`, 1),
		strings.Replace(ready, `"ready"`, `"exploded"`, 1),
		strings.Replace(ready, `"state":"ready"`, `"state":7`, 1),
		`{"STATE":"ready","JOIN_URL":"https://vtt.example.net/join"}`,
		strings.Replace(ready, `"state"`, `"State"`, 1),
		strings.Replace(ready, `"join_url"`, `"Join_URL"`, 1),
	} {
		site, rec := newSite(t, Granted, nil, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		}, func(tool *Tool) { tool.JoinURL = joinPage })
		res, got := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, got) != "tool_unavailable" || !strings.Contains(got, joinPage) {
			t.Errorf("%s: %d %s", body, res.StatusCode, got)
		}
		if !strings.Contains(rec.logged(), "without a status") {
			t.Errorf("%s: not logged: %s", body, rec.logged())
		}
	}
}

// A redirect from the backend is never followed, so the Member's email goes
// nowhere else and the page is told the status can't be checked.
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
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		site, _ := newSite(t, Granted, nil, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/server", status)
		})
		res, body := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" {
			t.Errorf("redirect %d: %d %s", status, res.StatusCode, body)
		}
	}
	elsewhere.Lock()
	defer elsewhere.Unlock()
	if len(reached) != 0 {
		t.Errorf("a redirect was followed, carrying %q", reached)
	}
}

const joinPage = "https://vtt.example.net:30000/join"

func downBackend(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusBadGateway)
}

// A Member with the grant whose status check fails is still given the join
// page, when one is configured, so the check never keeps them from joining.
func TestAFailedCheckStillOffersTheJoinPage(t *testing.T) {
	site, _ := newSite(t, Granted, nil, downBackend, func(tool *Tool) { tool.JoinURL = joinPage })
	res, body := call(t, http.MethodGet, site.URL+statusPath)
	var got struct {
		Code, Message string
		JoinURL       string `json:"join_url"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("not JSON: %q", body)
	}
	if res.StatusCode != http.StatusServiceUnavailable || got.Code != "tool_unavailable" || got.Message == "" || got.JoinURL != joinPage {
		t.Fatalf("failed check: %d %s, want 503 tool_unavailable with join_url %s", res.StatusCode, body, joinPage)
	}

	site, _ = newSite(t, Granted, nil, downBackend)
	res, body = call(t, http.MethodGet, site.URL+statusPath)
	if res.StatusCode != http.StatusServiceUnavailable || code(t, body) != "tool_unavailable" || strings.Contains(body, "join_url") {
		t.Errorf("no join page configured: %d %s, want tool_unavailable without join_url", res.StatusCode, body)
	}
}

// The join page goes to Members with the grant alone, and only when the
// status check itself failed: never with any other answer of the site's.
func TestTheJoinPageGoesOnlyToGrantedMembers(t *testing.T) {
	noServer := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"code":"no_server"}`)
	}
	for _, c := range []struct {
		name      string
		access    Access
		err       error
		backend   http.HandlerFunc
		configure func(*Tool)
		status    int
		code      string
	}{
		{"signed out", SignedOut, nil, downBackend, nil, http.StatusUnauthorized, "signed_out"},
		{"no grant", NoAccess, nil, downBackend, nil, http.StatusForbidden, "no_access"},
		{"check failed", SignedOut, errors.New("store down"), downBackend, nil, http.StatusServiceUnavailable, "access_unavailable"},
		{"no check", Granted, nil, downBackend, func(tool *Tool) { tool.Check = nil }, http.StatusServiceUnavailable, "access_unavailable"},
		{"no backend", Granted, nil, downBackend, func(tool *Tool) { tool.Backend = "" }, http.StatusServiceUnavailable, "not_configured"},
		{"no server", Granted, nil, noServer, nil, http.StatusNotFound, "no_server"},
	} {
		site, _ := newSite(t, c.access, c.err, c.backend, func(tool *Tool) {
			tool.JoinURL = joinPage
			if c.configure != nil {
				c.configure(tool)
			}
		})
		res, body := call(t, http.MethodGet, site.URL+statusPath)
		if res.StatusCode != c.status || code(t, body) != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, res.StatusCode, body, c.status, c.code)
		}
		if strings.Contains(body, "join_url") || strings.Contains(body, "vtt.example.net") {
			t.Errorf("%s: the join page was given: %s", c.name, body)
		}
	}
}

// The join page setting is held to the controller's own rule for it.
func TestJoinPageMustBeExactlyTheJoinPage(t *testing.T) {
	for _, ok := range []string{joinPage, "https://vtt.example.net/join", "https://vtt.example.net:443/join"} {
		if !ValidJoinURL(ok) {
			t.Errorf("%q was refused", ok)
		}
	}
	for _, bad := range []string{"", "http://vtt.example.net/join", "https://user:secret@vtt.example.net/join", "https://vtt.example.net/", "https://vtt.example.net/join/", "https://vtt.example.net/join?x=1", "https://vtt.example.net/join?", "https://vtt.example.net/join#x", "https://vtt.example.net/%6Aoin", "https://vtt.example.net:0/join", "https://vtt.example.net:99999/join", "https:///join", "vtt.example.net/join"} {
		if ValidJoinURL(bad) {
			t.Errorf("%q was accepted", bad)
		}
	}
}
