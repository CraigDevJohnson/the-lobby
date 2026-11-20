package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
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

type deleteBeforeRenewStore struct{ store.Store }

func (s deleteBeforeRenewStore) RenewSession(ctx context.Context, id string, currentExpiresAt, expiresAt time.Time) error {
	if err := s.Store.DeleteSession(ctx, id); err != nil {
		return err
	}
	return s.Store.RenewSession(ctx, id, currentExpiresAt, expiresAt)
}

type renewBeforeRenewStore struct{ store.Store }

func (s renewBeforeRenewStore) RenewSession(ctx context.Context, id string, currentExpiresAt, expiresAt time.Time) error {
	if err := s.Store.RenewSession(ctx, id, currentExpiresAt, expiresAt); err != nil {
		return err
	}
	return s.Store.RenewSession(ctx, id, currentExpiresAt, expiresAt)
}

func TestRenewalCannotRestoreDeletedSession(t *testing.T) {
	ctx := context.Background()
	// The store checks a session's expiry against the real clock, so the
	// test's clock starts from it and not from a fixed date.
	clock := time.Now()
	kept := store.NewMemory()
	m := &Manager{Store: deleteBeforeRenewStore{Store: kept}, Now: func() time.Time { return clock }}
	rec := httptest.NewRecorder()
	s, err := m.Start(ctx, rec, "craig@example.com")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(rec.Result().Cookies()[0])

	clock = clock.Add(48 * time.Hour)
	if _, err := m.Current(ctx, httptest.NewRecorder(), r); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("renewal after deletion: %v, want ErrNotFound", err)
	}
	if _, err := kept.GetSession(ctx, s.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("renewal restored the deleted session: %v", err)
	}
}

func TestConcurrentRenewalKeepsSession(t *testing.T) {
	ctx := context.Background()
	// As above: the store checks expiry against the real clock.
	clock := time.Now()
	kept := store.NewMemory()
	m := &Manager{Store: renewBeforeRenewStore{Store: kept}, Now: func() time.Time { return clock }}
	rec := httptest.NewRecorder()
	s, err := m.Start(ctx, rec, "craig@example.com")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(rec.Result().Cookies()[0])

	clock = clock.Add(48 * time.Hour)
	renewed := httptest.NewRecorder()
	got, err := m.Current(ctx, renewed, r)
	if err != nil || got.ID != s.ID {
		t.Fatalf("current after another renewal: %+v, %v", got, err)
	}
	if len(renewed.Result().Cookies()) != 1 {
		t.Fatal("successful concurrent renewal must rewrite the cookie")
	}
}

func TestNoCookie(t *testing.T) {
	m := &Manager{Store: store.NewMemory()}
	_, err := m.Current(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if err != store.ErrNotFound {
		t.Fatalf("got %v", err)
	}
}
