package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

type fakeAccess struct{ emails map[string]string }

func (f fakeAccess) Verify(_ context.Context, tok string) (access.Identity, error) {
	if e, ok := f.emails[tok]; ok {
		return access.Identity{Email: e}, nil
	}
	return access.Identity{}, errors.New("bad token")
}

var testUI = fstest.MapFS{
	"index.html":             {Data: []byte("welcome")},
	"checking.html":          {Data: []byte("checking")},
	"favicon.svg":            {Data: []byte("<svg/>")},
	"assets/index-abc123.js": {Data: []byte("app")},
}

// signinOutcome asserts a redirect to the public welcome with the outcome it explains.
func signinOutcome(t *testing.T, res *http.Response, want string) {
	t.Helper()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", res.StatusCode)
	}
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil || loc.Path != "/" || loc.Query().Get("signin") != want {
		t.Fatalf("redirect to %q, want /?signin=%s", res.Header.Get("Location"), want)
	}
	if len(res.Cookies()) != 0 {
		t.Fatalf("a failed sign-in set cookies: %v", res.Cookies())
	}
}

func newServer(t *testing.T, verifier access.Verifier) (*httptest.Server, store.Store) {
	t.Helper()
	st := store.NewMemory()
	h := &Handler{Env: "test", Store: st, Sessions: &session.Manager{Store: st}, Access: verifier, UI: testUI, Log: slog.Default()}
	r := chi.NewRouter()
	h.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, st
}

func TestVisitorThenMember(t *testing.T) {
	srv, st := newServer(t, fakeAccess{emails: map[string]string{"good": "craig@example.com", "stranger": "nobody@example.com"}})
	_ = st.PutMember(context.Background(), store.Member{Email: "craig@example.com", Tools: []string{"soccer"}})
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	me := func(cookies []*http.Cookie) whoami {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var w whoami
		_ = json.NewDecoder(res.Body).Decode(&w)
		return w
	}
	if got := me(nil); got.Role != "visitor" {
		t.Fatalf("no cookie: %+v", got)
	}

	// Sign-in without Access's token is refused.
	res, _ := client.Get(srv.URL + "/signin")
	signinOutcome(t, res, "failed")

	// So is a token Access did not issue.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/signin", nil)
	req.Header.Set(access.Header, "forged")
	res, _ = client.Do(req)
	signinOutcome(t, res, "failed")

	// A verified stranger is refused: Access proves identity, the site decides membership.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/signin", nil)
	req.Header.Set(access.Header, "stranger")
	res, _ = client.Do(req)
	signinOutcome(t, res, "not-invited")

	// A Member gets a session and is redirected home.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/signin", nil)
	req.Header.Set(access.Header, "good")
	res, _ = client.Do(req)
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("member signin: %d", res.StatusCode)
	}
	cookies := res.Cookies()
	got := me(cookies)
	if got.Role != "member" || got.Email != "craig@example.com" || len(got.Tools) != 1 {
		t.Fatalf("member: %+v", got)
	}

	// Removing the Member takes effect on the next request, and the stale
	// cookie is cleared so later visits start at the public welcome.
	_ = st.DeleteMember(context.Background(), "craig@example.com")
	if got := me(cookies); got.Role != "visitor" {
		t.Fatalf("removed member still recognised: %+v", got)
	}
	res, _ = get(t, srv, "/api/me", cookies...)
	cleared := false
	for _, c := range res.Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("stale session cookie not cleared: %v", res.Cookies())
	}
}

func TestSigninUnconfigured(t *testing.T) {
	srv, _ := newServer(t, nil)
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, _ := client.Get(srv.URL + "/signin")
	signinOutcome(t, res, "unavailable")
}

func get(t *testing.T, srv *httptest.Server, path string, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var b strings.Builder
	_, _ = io.Copy(&b, res.Body)
	return res, b.String()
}

// The public welcome is served to Visitors; a request carrying a Sign-in
// session cookie gets The VIP Lobby's checking state instead, which shows no
// private Tool names until /api/me answers.
func TestIndexPicksScreenBySessionCookie(t *testing.T) {
	srv, _ := newServer(t, nil)

	res, body := get(t, srv, "/")
	if res.StatusCode != http.StatusOK || body != "welcome" {
		t.Fatalf("visitor: %d %q", res.StatusCode, body)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("visitor cache-control %q", cc)
	}

	res, body = get(t, srv, "/", &http.Cookie{Name: session.CookieName, Value: "anything"})
	if res.StatusCode != http.StatusOK || body != "checking" {
		t.Fatalf("with cookie: %d %q", res.StatusCode, body)
	}
}

func TestIndexWithoutBuiltScreens(t *testing.T) {
	st := store.NewMemory()
	h := &Handler{Store: st, Sessions: &session.Manager{Store: st}, UI: fstest.MapFS{}, Log: slog.Default()}
	r := chi.NewRouter()
	h.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	if res, _ := get(t, srv, "/"); res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %d", res.StatusCode)
	}
}

func TestAssetsCaching(t *testing.T) {
	srv, _ := newServer(t, nil)
	res, body := get(t, srv, "/assets/index-abc123.js")
	if res.StatusCode != http.StatusOK || body != "app" {
		t.Fatalf("asset: %d %q", res.StatusCode, body)
	}
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("hashed asset cache-control %q", cc)
	}
	if res, _ := get(t, srv, "/favicon.svg"); res.StatusCode != http.StatusOK {
		t.Fatalf("favicon: %d", res.StatusCode)
	}
	if res, _ := get(t, srv, "/assets/missing.js"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing asset: %d", res.StatusCode)
	}
}

type failingStore struct{ store.Store }

func (failingStore) GetSession(context.Context, string) (store.Session, error) {
	return store.Session{}, errors.New("dynamodb unavailable")
}

// A store failure must not read as "Visitor": The VIP Lobby shows a retry
// instead of signing the Member out or showing an empty collection.
func TestMeReportsAccessCheckFailure(t *testing.T) {
	st := failingStore{store.NewMemory()}
	h := &Handler{Store: st, Sessions: &session.Manager{Store: st}, UI: testUI, Log: slog.Default()}
	r := chi.NewRouter()
	h.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	res, body := get(t, srv, "/api/me", &http.Cookie{Name: session.CookieName, Value: "anything"})
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %d %q", res.StatusCode, body)
	}
	if strings.Contains(body, "visitor") {
		t.Fatalf("failure reported as a role: %q", body)
	}
}
