package members

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/web"
)

// pings counts each Tool's checks, which run at once.
type pings struct {
	mu    sync.Mutex
	asked []string
}

func (p *pings) ping(tool string, wait time.Duration, err error) func(context.Context) error {
	return func(ctx context.Context) error {
		p.mu.Lock()
		p.asked = append(p.asked, tool)
		p.mu.Unlock()
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("asked without a time limit")
		}
		time.Sleep(wait)
		return err
	}
}

func (p *pings) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.asked)
}

// statusSite serves Lobby Admin with a Site card for dev, calendars offered,
// whose soccer backend answers slowly, Minecraft's doesn't answer and
// Foundry's isn't set up.
func statusSite(t *testing.T) (*site, *pings) {
	t.Helper()
	s := newSite(t)
	p := &pings{}
	log := slog.New(slog.NewTextHandler(s.log, nil))
	h := &web.Handler{Store: s.store, Sessions: s.sessions, Log: log}
	r := chi.NewRouter()
	(&Tool{
		Store:       s.store,
		Check:       h.Member,
		Log:         log,
		Environment: "dev",
		Calendars:   true,
		Backends: []Backend{
			{Tool: "soccer", Configured: true, Ping: p.ping("soccer", 50*time.Millisecond, nil)},
			{Tool: "minecraft", Configured: true, Ping: p.ping("minecraft", 0, errors.New("the backend answered 503 Service Unavailable"))},
			{Tool: "foundry", Configured: false, Ping: p.ping("foundry", 0, nil)},
		},
	}).Routes(r)
	s.handler = r
	return s, p
}

// The Site card needs lobby-admin like the Members do: a Visitor is told to
// sign in, a Member without it is refused, and no backend is asked for
// either.
func TestTheSiteCardNeedsLobbyAdmin(t *testing.T) {
	s, p := statusSite(t)
	code, body := s.do(http.MethodGet, StatusPath, nil, "")
	if code != http.StatusUnauthorized || body["code"] != "signed_out" {
		t.Errorf("signed out: %d %v", code, body)
	}
	code, body = s.do(http.MethodGet, StatusPath, s.signIn(kid), "")
	if code != http.StatusForbidden || body["code"] != "no_access" {
		t.Errorf("without the grant: %d %v", code, body)
	}
	if _, ok := body["backends"]; ok {
		t.Error("a Member without lobby-admin saw the backends")
	}
	if n := p.count(); n != 0 {
		t.Errorf("%d backends were asked for someone without lobby-admin", n)
	}
}

// The Site card names the environment, the version, whether the calendars
// are offered, and each backend in the order given: one that answered, one
// that didn't, and one that isn't set up and so isn't asked. A failed check
// is logged with its Tool, never with who asked.
func TestTheSiteCardChecksEachBackend(t *testing.T) {
	s, p := statusSite(t)
	req := httptest.NewRequest(http.MethodGet, StatusPath, nil)
	req.AddCookie(s.signIn(owner))
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status: %d, Cache-Control %q: %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	// The tests record no version, so the version's fields are empty, but
	// they are there.
	want := `{"environment":"dev","version":{"commit":"","committed":"","modified":false},"calendars":true,"backends":[` +
		`{"tool":"soccer","configured":true,"answered":true},` +
		`{"tool":"minecraft","configured":true,"answered":false},` +
		`{"tool":"foundry","configured":false,"answered":false}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("answer:\n got %s\nwant %s", got, want)
	}
	if asked := slices.Sorted(slices.Values(p.asked)); !slices.Equal(asked, []string{"minecraft", "soccer"}) {
		t.Errorf("asked %v, want soccer and minecraft only", p.asked)
	}
	logged := s.log.String()
	if !strings.Contains(logged, "backend not answering") || !strings.Contains(logged, "tool=minecraft") || !strings.Contains(logged, "503") {
		t.Errorf("the failed check wasn't logged with its Tool: %s", logged)
	}
	if strings.Contains(logged, owner) {
		t.Errorf("the log names a Member: %s", logged)
	}
}

// The version is the commit go build recorded, when it was made and
// whether the checkout had changes that weren't committed.
func TestTheVersionIsTheBuildsCommit(t *testing.T) {
	got := versionOf([]debug.BuildSetting{
		{Key: "-trimpath", Value: "true"},
		{Key: "vcs", Value: "git"},
		{Key: "vcs.revision", Value: "4d0e5f7c0ffee4d0e5f7c0ffee4d0e5f7c0ffee4"},
		{Key: "vcs.time", Value: "2026-11-15T10:21:08Z"},
		{Key: "vcs.modified", Value: "true"},
	})
	want := Version{Commit: "4d0e5f7c0ffee4d0e5f7c0ffee4d0e5f7c0ffee4", Committed: "2026-11-15T10:21:08Z", Modified: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := versionOf(nil); got != (Version{}) {
		t.Errorf("no settings: %+v", got)
	}
}
