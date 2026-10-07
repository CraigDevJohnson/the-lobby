package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CraigDevJohnson/website/internal/store"
)

func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	m := &Manager{Store: store.NewMemory(), Now: func() time.Time { return clock }}

	rec := httptest.NewRecorder()
	s, err := m.Start(ctx, rec, "Craig@Example.com")
	if err != nil {
		t.Fatal(err)
	}
	if s.Email != "craig@example.com" {
		t.Fatalf("email not normalised: %q", s.Email)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected cookies: %+v", cookies)
	}

	withCookie := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(cookies[0])
		return r
	}

	// Same day: found, not renewed.
	rec2 := httptest.NewRecorder()
	got, err := m.Current(ctx, rec2, withCookie())
	if err != nil || got.ID != s.ID {
		t.Fatalf("current: %v %+v", err, got)
	}
	if len(rec2.Result().Cookies()) != 0 {
		t.Fatal("cookie must not be rewritten within a day")
	}

	// Two days later: renewed, cookie rewritten, expiry moved.
	clock = clock.Add(48 * time.Hour)
	rec3 := httptest.NewRecorder()
	got, err = m.Current(ctx, rec3, withCookie())
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpiresAt != clock.Add(Lifetime) {
		t.Fatalf("not renewed: %v", got.ExpiresAt)
	}
	if len(rec3.Result().Cookies()) != 1 {
		t.Fatal("renewal must rewrite the cookie")
	}

	// 91 days of silence: gone.
	clock = clock.Add(91 * 24 * time.Hour)
	if _, err := m.Current(ctx, httptest.NewRecorder(), withCookie()); err != store.ErrNotFound {
		t.Fatalf("expired session returned: %v", err)
	}

	// Sign out clears the cookie.
	rec4 := httptest.NewRecorder()
	if err := m.End(ctx, rec4, withCookie()); err != nil {
		t.Fatal(err)
	}
	if c := rec4.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", c)
	}
}

func TestNoCookie(t *testing.T) {
	m := &Manager{Store: store.NewMemory()}
	_, err := m.Current(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if err != store.ErrNotFound {
		t.Fatalf("got %v", err)
	}
}
