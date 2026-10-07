package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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

func newServer(t *testing.T, verifier access.Verifier) (*httptest.Server, store.Store) {
	t.Helper()
	st := store.NewMemory()
	h := &Handler{Env: "test", Store: st, Sessions: &session.Manager{Store: st}, Access: verifier, Log: slog.Default()}
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
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("signin without token: %d", res.StatusCode)
	}

	// A verified stranger is refused: Access proves identity, the site decides membership.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/signin", nil)
	req.Header.Set(access.Header, "stranger")
	res, _ = client.Do(req)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("stranger: %d", res.StatusCode)
	}

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

	// Removing the Member takes effect on the next request.
	_ = st.DeleteMember(context.Background(), "craig@example.com")
	if got := me(cookies); got.Role != "visitor" {
		t.Fatalf("removed member still recognised: %+v", got)
	}
}

func TestSigninUnconfigured(t *testing.T) {
	srv, _ := newServer(t, nil)
	res, _ := srv.Client().Get(srv.URL + "/signin")
	if res.StatusCode != http.StatusNotImplemented {
		t.Fatalf("got %d", res.StatusCode)
	}
}
