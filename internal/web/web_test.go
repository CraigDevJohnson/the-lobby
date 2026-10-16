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
	"privacy.html":           {Data: []byte("privacy")},
	"soccer.html":            {Data: []byte("soccer")},
	"minecraft.html":         {Data: []byte("minecraft")},
	"foundry.html":           {Data: []byte("foundry")},
	"signed-out.html":        {Data: []byte("signed out")},
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

// Sign-out ends the Sign-in session, marks the browser as just signed out,
// and lands on the signed-out page, which ends Cloudflare Access's sign-in
// too. Without that mark, or with a Sign-in session, the page is not shown,
// so opening it directly or from another site ends nothing and claims nothing.
func TestSignoutLandsOnTheSignedOutPage(t *testing.T) {
	srv, st := newServer(t, fakeAccess{emails: map[string]string{"good": "craig@example.com"}})
	_ = st.PutMember(context.Background(), store.Member{Email: "craig@example.com", Tools: []string{"soccer"}})
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/signin", nil)
	req.Header.Set(access.Header, "good")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	signedIn := res.Cookies()

	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/signout", nil)
	for _, c := range signedIn {
		req.AddCookie(c)
	}
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/signed-out" {
		t.Fatalf("sign-out: %d to %q, want 303 to /signed-out", res.StatusCode, res.Header.Get("Location"))
	}
	cleared := false
	var mark *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			cleared = true
		}
		if c.Name == signedOutCookie {
			mark = c
		}
	}
	if !cleared {
		t.Fatalf("session cookie not cleared: %v", res.Cookies())
	}
	if mark == nil || mark.MaxAge != 60 || !mark.HttpOnly || mark.SameSite != http.SameSiteLaxMode || mark.Path != "/" {
		t.Fatalf("signed-out mark: %+v", mark)
	}
	for _, c := range signedIn {
		if c.Name == session.CookieName {
			if _, err := st.GetSession(context.Background(), c.Value); !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("session record left behind: %v", err)
			}
		}
	}

	res, body := get(t, srv, "/signed-out", mark)
	if res.StatusCode != http.StatusOK || body != "signed out" {
		t.Fatalf("just signed out: %d %q", res.StatusCode, body)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control %q", cc)
	}

	for _, cookies := range [][]*http.Cookie{nil, signedIn, append([]*http.Cookie{mark}, signedIn...)} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/signed-out", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
			t.Fatalf("without the mark or with a session, cookies %v: %d to %q, want 303 to /", cookies, res.StatusCode, res.Header.Get("Location"))
		}
	}
}

// Cloudflare may still accept the Access sign-in for up to 30 seconds after
// its logout. A sign-in in the minute after signing out goes back to the
// signed-out page, which ends Access's sign-in again, instead of starting a
// Sign-in session without a new code.
func TestSigninJustAfterSignoutGoesBackToTheSignedOutPage(t *testing.T) {
	srv, st := newServer(t, fakeAccess{emails: map[string]string{"good": "craig@example.com"}})
	_ = st.PutMember(context.Background(), store.Member{Email: "craig@example.com", Tools: []string{"soccer"}})
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/signin", nil)
	req.Header.Set(access.Header, "good")
	req.AddCookie(&http.Cookie{Name: signedOutCookie, Value: "1"})
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/signed-out" {
		t.Fatalf("sign-in just after sign-out: %d to %q, want 303 to /signed-out", res.StatusCode, res.Header.Get("Location"))
	}
	if len(res.Cookies()) != 0 {
		t.Fatalf("a held sign-in set cookies: %v", res.Cookies())
	}
}

type undeletableStore struct{ store.Store }

func (undeletableStore) DeleteSession(context.Context, string) error {
	return errors.New("dynamodb unavailable")
}

// When the session can't be ended, the browser is still signed in, so
// sign-out goes back to The VIP Lobby rather than to a page saying it is
// signed out, and Cloudflare's sign-in is left alone.
func TestAFailedSignoutDoesNotClaimToHaveSignedOut(t *testing.T) {
	st := undeletableStore{store.NewMemory()}
	h := &Handler{Store: st, Sessions: &session.Manager{Store: st}, UI: testUI, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	r := chi.NewRouter()
	h.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/signout", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "anything"})
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
		t.Fatalf("failed sign-out: %d to %q, want 303 to /", res.StatusCode, res.Header.Get("Location"))
	}
	for _, c := range res.Cookies() {
		if c.Name == signedOutCookie {
			t.Fatalf("a failed sign-out marked the browser as signed out: %+v", c)
		}
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

// Privacy is public: the same page with or without a Sign-in session, and
// still readable when the session store is failing.
func TestPrivacyIsPublic(t *testing.T) {
	srv, _ := newServer(t, nil)
	for _, cookies := range [][]*http.Cookie{nil, {{Name: session.CookieName, Value: "expired"}}} {
		res, body := get(t, srv, "/privacy", cookies...)
		if res.StatusCode != http.StatusOK || body != "privacy" {
			t.Fatalf("cookies %v: %d %q", cookies, res.StatusCode, body)
		}
		if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("content-type %q", ct)
		}
	}

	st := failingStore{store.NewMemory()}
	h := &Handler{Store: st, Sessions: &session.Manager{Store: st}, UI: testUI, Log: slog.Default()}
	r := chi.NewRouter()
	h.Routes(r)
	failing := httptest.NewServer(r)
	t.Cleanup(failing.Close)
	res, body := get(t, failing, "/privacy", &http.Cookie{Name: session.CookieName, Value: "anything"})
	if res.StatusCode != http.StatusOK || body != "privacy" {
		t.Fatalf("failing store: %d %q", res.StatusCode, body)
	}
}

// Let's Plan Soccer is public too: no Sign-in session is needed, and an
// expired one or a failing session store does not get in the way.
func TestLetsPlanSoccerIsPublic(t *testing.T) {
	srv, _ := newServer(t, nil)
	for _, cookies := range [][]*http.Cookie{nil, {{Name: session.CookieName, Value: "expired"}}} {
		res, body := get(t, srv, "/soccer", cookies...)
		if res.StatusCode != http.StatusOK || body != "soccer" {
			t.Fatalf("cookies %v: %d %q", cookies, res.StatusCode, body)
		}
	}

	st := failingStore{store.NewMemory()}
	h := &Handler{Store: st, Sessions: &session.Manager{Store: st}, UI: testUI, Log: slog.Default()}
	r := chi.NewRouter()
	h.Routes(r)
	failing := httptest.NewServer(r)
	t.Cleanup(failing.Close)
	res, body := get(t, failing, "/soccer", &http.Cookie{Name: session.CookieName, Value: "anything"})
	if res.StatusCode != http.StatusOK || body != "soccer" {
		t.Fatalf("failing store: %d %q", res.StatusCode, body)
	}
}

// The Minecraft Launcher's and Foundry's pages are the same checking state
// for everyone: they carry nothing private, so they are served without
// looking at the session, and each page's own call does the access check.
func TestToolPagesHoldNothingPrivate(t *testing.T) {
	srv, _ := newServer(t, nil)
	for _, page := range []string{"minecraft", "foundry"} {
		for _, cookies := range [][]*http.Cookie{nil, {{Name: session.CookieName, Value: "anything"}}} {
			res, body := get(t, srv, "/"+page, cookies...)
			if res.StatusCode != http.StatusOK || body != page || res.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("%s, cookies %v: %d %q", page, cookies, res.StatusCode, body)
			}
		}
	}
}

// Sign-in comes back to the Minecraft Launcher or Foundry when it started
// there, and to The VIP Lobby for anything else, so ?next= can never lead
// off the site.
func TestSigninReturnsToKnownPagesOnly(t *testing.T) {
	srv, st := newServer(t, fakeAccess{emails: map[string]string{"good": "kid@example.com"}})
	_ = st.PutMember(context.Background(), store.Member{Email: "kid@example.com", Tools: []string{"minecraft"}})
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for next, want := range map[string]string{
		"/minecraft":            "/minecraft",
		"/foundry":              "/foundry",
		"":                      "/",
		"https://evil.example/": "/",
		"//evil.example/":       "/",
		"/minecraft/admin":      "/",
		"/foundry/admin":        "/",
	} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/signin?next="+url.QueryEscape(next), nil)
		req.Header.Set(access.Header, "good")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != want {
			t.Errorf("next %q: %d to %q, want %q", next, res.StatusCode, res.Header.Get("Location"), want)
		}
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
	res, _ = get(t, srv, "/assets/missing.js")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing asset: %d", res.StatusCode)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("missing asset cache-control %q", cc)
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
