package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CraigDevJohnson/the-lobby/internal/foundry"
	"github.com/CraigDevJohnson/the-lobby/internal/minecraft"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
	"github.com/CraigDevJohnson/the-lobby/internal/web"
)

// A calendar link's token stays out of the request log whether or not the
// request matched a route: a wrong method or a longer path is logged as the
// route too.
func TestRequestLogLeavesOutLinkTokens(t *testing.T) {
	var logged bytes.Buffer
	h, err := New(context.Background(), Config{Env: "local", SoccerURL: "http://127.0.0.1:1"}, slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/soccer/link/c2VjcmV0dG9rZW4.ics"},
		{http.MethodPost, "/soccer/link/c2VjcmV0dG9rZW4.ics"},
		{http.MethodGet, "/soccer/link/c2VjcmV0dG9rZW4/extra"},
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
	}
	if n := strings.Count(logged.String(), "path=/soccer/link/{token}"); n != 3 {
		t.Fatalf("%d of 3 link requests were logged as the route: %s", n, logged.String())
	}
	if strings.Contains(logged.String(), "c2VjcmV0dG9rZW4") {
		t.Fatalf("log carries the token: %s", logged.String())
	}
}

// Requests to the soccer backend are signed everywhere but a developer's
// computer unless SOCCER_BACKEND_SIGNING says otherwise: "lambda" signs even
// locally, "off" never signs, and anything else stops the site from starting.
func TestSoccerSigningFollowsTheEnvironmentUnlessTold(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	const backend = "https://backend.example"
	for _, c := range []struct {
		env, signing string
		signed       bool
	}{
		{"local", "", false},
		{"dev", "", true},
		{"prod", "", true},
		{"local", "lambda", true},
		{"dev", "off", false},
		{"prod", "lambda", true},
	} {
		tool, err := newSoccer(context.Background(), Config{Env: c.env, SoccerURL: backend, SoccerSigning: c.signing}, log)
		if err != nil {
			t.Fatalf("%s %q: %v", c.env, c.signing, err)
		}
		if got := tool.Sign != nil; got != c.signed {
			t.Errorf("env %s, SOCCER_BACKEND_SIGNING %q: signed %v, want %v", c.env, c.signing, got, c.signed)
		}
	}
	for _, env := range []string{"local", "prod"} {
		if _, err := newSoccer(context.Background(), Config{Env: env, SoccerURL: backend, SoccerSigning: "Lambda "}, log); err == nil {
			t.Errorf("env %s: an unknown SOCCER_BACKEND_SIGNING was accepted", env)
		}
	}
	// With no backend to call there is nothing to sign, but a bad value is still refused.
	if _, err := newSoccer(context.Background(), Config{Env: "prod", SoccerSigning: "sometimes"}, log); err == nil {
		t.Error("an unknown SOCCER_BACKEND_SIGNING was accepted when no backend is set")
	}
}

// The Launcher's access check reads the Member record: no session is signed
// out, a Member without the minecraft grant has no access (minecraft-admin
// does not count), and one with it is named for the backend.
func TestLauncherAccessFollowsTheGrant(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	sessions := &session.Manager{Store: st}
	h := &web.Handler{Store: st, Sessions: sessions, Log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))}
	check := launcherAccess(h)

	ask := func(email string) (minecraft.Access, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/minecraft/server", nil)
		if email != "" {
			rec := httptest.NewRecorder()
			if _, err := sessions.Start(ctx, rec, email); err != nil {
				t.Fatal(err)
			}
			for _, c := range rec.Result().Cookies() {
				req.AddCookie(c)
			}
		}
		a, who, err := check(httptest.NewRecorder(), req)
		if err != nil {
			t.Fatal(err)
		}
		return a, who
	}
	_ = st.PutMember(ctx, store.Member{Email: "admin@example.com", Tools: []string{"soccer", "minecraft-admin"}})
	_ = st.PutMember(ctx, store.Member{Email: "kid@example.com", Tools: []string{"minecraft"}})

	if a, _ := ask(""); a != minecraft.SignedOut {
		t.Errorf("no session: %v", a)
	}
	if a, _ := ask("admin@example.com"); a != minecraft.NoAccess {
		t.Errorf("minecraft-admin only: %v", a)
	}
	if a, who := ask("kid@example.com"); a != minecraft.Granted || who != "kid@example.com" {
		t.Errorf("granted: %v %q", a, who)
	}
}

// The Minecraft backend's signing setting is read like the soccer one's, and
// a bad value stops the site from starting even before a backend is named.
func TestMinecraftSigningFollowsTheEnvironmentUnlessTold(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	h := &web.Handler{}
	for _, c := range []struct {
		env, signing string
		signed       bool
	}{
		{"local", "", false},
		{"prod", "", true},
		{"local", "lambda", true},
		{"dev", "off", false},
	} {
		tool, err := newMinecraft(context.Background(), Config{Env: c.env, MinecraftURL: "https://backend.example", MinecraftSigning: c.signing}, h, log)
		if err != nil {
			t.Fatalf("%s %q: %v", c.env, c.signing, err)
		}
		if got := tool.Sign != nil; got != c.signed {
			t.Errorf("env %s, MINECRAFT_BACKEND_SIGNING %q: signed %v, want %v", c.env, c.signing, got, c.signed)
		}
	}
	if _, err := newMinecraft(context.Background(), Config{Env: "prod", MinecraftSigning: "always"}, h, log); err == nil {
		t.Error("an unknown MINECRAFT_BACKEND_SIGNING was accepted")
	}
	tool, err := newMinecraft(context.Background(), Config{Env: "local"}, h, log)
	if err != nil || tool.Backend != "" {
		t.Errorf("no MINECRAFT_BACKEND_URL locally: backend %q, err %v; want none", tool.Backend, err)
	}
}

// Foundry's access check reads the Member record like the Launcher's, for
// the grant the route asks about: no session is signed out, a Member without
// that grant has no access (neither Foundry grant includes the other, and the
// minecraft grant counts for neither), and one with it is named for the
// backend.
func TestFoundryAccessFollowsTheGrant(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	sessions := &session.Manager{Store: st}
	h := &web.Handler{Store: st, Sessions: sessions, Log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))}
	check := foundryAccess(h)

	ask := func(email, grant string) (foundry.Access, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/foundry/server", nil)
		if email != "" {
			rec := httptest.NewRecorder()
			if _, err := sessions.Start(ctx, rec, email); err != nil {
				t.Fatal(err)
			}
			for _, c := range rec.Result().Cookies() {
				req.AddCookie(c)
			}
		}
		a, who, err := check(httptest.NewRecorder(), req, grant)
		if err != nil {
			t.Fatal(err)
		}
		return a, who
	}
	_ = st.PutMember(ctx, store.Member{Email: "admin@example.com", Tools: []string{"soccer", "foundry-admin"}})
	_ = st.PutMember(ctx, store.Member{Email: "kid@example.com", Tools: []string{"minecraft"}})
	_ = st.PutMember(ctx, store.Member{Email: "player@example.com", Tools: []string{"foundry"}})

	for _, grant := range []string{foundry.Grant, foundry.AdminGrant} {
		if a, _ := ask("", grant); a != foundry.SignedOut {
			t.Errorf("%s, no session: %v", grant, a)
		}
		if a, _ := ask("kid@example.com", grant); a != foundry.NoAccess {
			t.Errorf("%s, minecraft only: %v", grant, a)
		}
	}
	if a, _ := ask("admin@example.com", foundry.Grant); a != foundry.NoAccess {
		t.Errorf("status, foundry-admin only: %v", a)
	}
	if a, who := ask("player@example.com", foundry.Grant); a != foundry.Granted || who != "player@example.com" {
		t.Errorf("status, foundry: %v %q", a, who)
	}
	if a, _ := ask("player@example.com", foundry.AdminGrant); a != foundry.NoAccess {
		t.Errorf("changes, foundry only: %v", a)
	}
	if a, who := ask("admin@example.com", foundry.AdminGrant); a != foundry.Granted || who != "admin@example.com" {
		t.Errorf("changes, foundry-admin: %v %q", a, who)
	}
}

// Foundry's backend signing setting is read like the other backends', and a
// bad value stops the site from starting even before a backend is named.
// There is no local default: without FOUNDRY_BACKEND_URL nothing is called.
func TestFoundrySigningFollowsTheEnvironmentUnlessTold(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	h := &web.Handler{}
	for _, c := range []struct {
		env, signing string
		signed       bool
	}{
		{"local", "", false},
		{"dev", "", true},
		{"prod", "", true},
		{"local", "lambda", true},
		{"dev", "off", false},
	} {
		tool, err := newFoundry(context.Background(), Config{Env: c.env, FoundryURL: "https://backend.example/", FoundrySigning: c.signing}, h, log)
		if err != nil {
			t.Fatalf("%s %q: %v", c.env, c.signing, err)
		}
		if got := tool.Sign != nil; got != c.signed {
			t.Errorf("env %s, FOUNDRY_BACKEND_SIGNING %q: signed %v, want %v", c.env, c.signing, got, c.signed)
		}
		if tool.Backend != "https://backend.example" {
			t.Errorf("env %s: backend %q, want it without the trailing slash", c.env, tool.Backend)
		}
	}
	if _, err := newFoundry(context.Background(), Config{Env: "prod", FoundrySigning: "always"}, h, log); err == nil {
		t.Error("an unknown FOUNDRY_BACKEND_SIGNING was accepted")
	}
	tool, err := newFoundry(context.Background(), Config{Env: "local"}, h, log)
	if err != nil || tool.Backend != "" || tool.Sign != nil {
		t.Errorf("no FOUNDRY_BACKEND_URL locally: backend %q, signed %v, err %v; want none", tool.Backend, tool.Sign != nil, err)
	}
}

// FOUNDRY_JOIN_URL reaches the Tool as it is, and one that is not exactly
// Foundry's join page stops the site from starting, as the controller does.
func TestFoundryJoinPageIsCheckedAtStart(t *testing.T) {
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	h := &web.Handler{}
	tool, err := newFoundry(context.Background(), Config{Env: "local", FoundryJoinURL: "https://vtt.example.net:30000/join"}, h, log)
	if err != nil || tool.JoinURL != "https://vtt.example.net:30000/join" {
		t.Fatalf("join page: %q, err %v", tool.JoinURL, err)
	}
	if tool, err = newFoundry(context.Background(), Config{Env: "local"}, h, log); err != nil || tool.JoinURL != "" {
		t.Errorf("unset: join page %q, err %v; want none", tool.JoinURL, err)
	}
	for _, bad := range []string{"http://vtt.example.net/join", "https://vtt.example.net/", "https://vtt.example.net/join?world=x"} {
		if _, err := newFoundry(context.Background(), Config{Env: "local", FoundryJoinURL: bad}, h, log); err == nil {
			t.Errorf("FOUNDRY_JOIN_URL %q was accepted", bad)
		}
	}
}

// The site serves Foundry's status route behind its access check: a Visitor
// is told to sign in, and nothing is cached.
func TestFoundryStatusIsServedBehindItsGrant(t *testing.T) {
	h, err := New(context.Background(), Config{Env: "local"}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/foundry/server", nil))
	var body struct{ Code string }
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusUnauthorized || body.Code != "signed_out" {
		t.Fatalf("visitor: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("must not be cached: %q", rec.Header().Get("Cache-Control"))
	}
}
