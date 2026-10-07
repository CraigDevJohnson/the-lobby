// Package session issues and checks the site's own Sign-in session: a random
// id in a cookie, looked up in the store on every request. Removing a Member
// takes effect at once because the Member record is checked each time too.
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

const (
	CookieName = "site_session"
	Lifetime   = 90 * 24 * time.Hour
	// A session is renewed at most once a day, so the store is not written on
	// every request.
	renewAfter = 24 * time.Hour
)

type Manager struct {
	Store  store.Store
	Secure bool // false only on a developer's computer over plain http
	Now    func() time.Time
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Start creates a session for the email and sets the cookie.
func (m *Manager) Start(ctx context.Context, w http.ResponseWriter, email string) (store.Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return store.Session{}, err
	}
	now := m.now()
	s := store.Session{
		ID:        base64.RawURLEncoding.EncodeToString(raw),
		Email:     store.NormalizeEmail(email),
		CreatedAt: now,
		ExpiresAt: now.Add(Lifetime),
	}
	if err := m.Store.PutSession(ctx, s); err != nil {
		return store.Session{}, err
	}
	m.setCookie(w, s.ID, Lifetime)
	return s, nil
}

// Current returns the session behind the request's cookie, renewing it when
// it is more than a day old. It returns store.ErrNotFound for no session.
func (m *Manager) Current(ctx context.Context, w http.ResponseWriter, r *http.Request) (store.Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return store.Session{}, store.ErrNotFound
	}
	s, err := m.Store.GetSession(ctx, c.Value)
	if err != nil {
		return store.Session{}, err
	}
	now := m.now()
	if !s.ExpiresAt.After(now) {
		return store.Session{}, store.ErrNotFound
	}
	if s.ExpiresAt.Sub(now) < Lifetime-renewAfter {
		s.ExpiresAt = now.Add(Lifetime)
		if err := m.Store.PutSession(ctx, s); err != nil {
			return store.Session{}, err
		}
		m.setCookie(w, s.ID, Lifetime)
	}
	return s, nil
}

// End deletes the session and clears the cookie.
func (m *Manager) End(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(CookieName)
	if err == nil && c.Value != "" {
		if err := m.Store.DeleteSession(ctx, c.Value); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	m.setCookie(w, "", -time.Second)
	return nil
}

func (m *Manager) setCookie(w http.ResponseWriter, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
