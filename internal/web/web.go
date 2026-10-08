// Package web is the site's HTTP surface: the screens built from web/ (the
// public welcome and The VIP Lobby, ADR 0003), who-am-I, sign-in through
// Cloudflare Access, and sign-out.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

type Handler struct {
	Env      string
	Store    store.Store
	Sessions *session.Manager
	Access   access.Verifier // nil when Access is not configured
	UI       fs.FS           // the built screens; see UI()
	Log      *slog.Logger
}

const sessionCookie = session.CookieName

func (h *Handler) Routes(r chi.Router) {
	r.Get("/healthz", h.healthz)
	r.Get("/", h.index)
	r.Get("/assets/*", h.assets)
	r.Get("/favicon.svg", h.assets)
	r.Get("/api/me", h.me)
	r.Get("/signin", h.signin)
	r.Post("/signout", h.signout)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok\n"))
}

type whoami struct {
	Role  string   `json:"role"`
	Email string   `json:"email,omitempty"`
	Tools []string `json:"tools,omitempty"`
}

// identify answers "who is this" from the Sign-in session and the Member
// record, which is re-read every time so removal takes effect at once. A
// store failure is an error, not a Visitor: The VIP Lobby must not mistake
// "could not check" for "signed out" or "no Tools".
func (h *Handler) identify(ctx context.Context, w http.ResponseWriter, r *http.Request) (whoami, error) {
	s, err := h.Sessions.Current(ctx, w, r)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return whoami{Role: "visitor"}, nil
		}
		return whoami{}, fmt.Errorf("session lookup: %w", err)
	}
	m, err := h.Store.GetMember(ctx, s.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return whoami{Role: "visitor"}, nil
		}
		return whoami{}, fmt.Errorf("member lookup: %w", err)
	}
	tools := m.Tools
	if tools == nil {
		tools = []string{}
	}
	return whoami{Role: "member", Email: m.Email, Tools: tools}, nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	who, err := h.identify(r.Context(), w, r)
	if err != nil {
		h.Log.Error("identify", "err", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access could not be checked"})
		return
	}
	_ = json.NewEncoder(w).Encode(who)
}

// Sign-in outcomes the public welcome explains (web/src/Welcome.tsx).
const (
	signinFailed      = "failed"
	signinNotInvited  = "not-invited"
	signinUnavailable = "unavailable"
)

// signinProblem sends the person back to the public welcome with a plain
// explanation instead of a bare error page.
func signinProblem(w http.ResponseWriter, r *http.Request, outcome string) {
	http.Redirect(w, r, "/?signin="+outcome, http.StatusSeeOther)
}

// signin runs only behind Cloudflare Access. Access has already proved who
// the person is; the site checks the token, requires a Member record, then
// starts its own Sign-in session.
func (h *Handler) signin(w http.ResponseWriter, r *http.Request) {
	if h.Access == nil {
		h.Log.Warn("sign-in attempted but Access is not configured")
		signinProblem(w, r, signinUnavailable)
		return
	}
	tok := access.TokenFromRequest(r)
	if tok == "" {
		h.Log.Warn("sign-in without an Access token")
		signinProblem(w, r, signinFailed)
		return
	}
	id, err := h.Access.Verify(r.Context(), tok)
	if err != nil {
		h.Log.Warn("access token rejected", "err", err)
		signinProblem(w, r, signinFailed)
		return
	}
	if _, err := h.Store.GetMember(r.Context(), id.Email); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Log.Info("sign-in by non-member", "email", id.Email)
			signinProblem(w, r, signinNotInvited)
			return
		}
		h.Log.Error("member lookup", "err", err)
		signinProblem(w, r, signinFailed)
		return
	}
	if _, err := h.Sessions.Start(r.Context(), w, id.Email); err != nil {
		h.Log.Error("start session", "err", err)
		signinProblem(w, r, signinFailed)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) signout(w http.ResponseWriter, r *http.Request) {
	if err := h.Sessions.End(r.Context(), w, r); err != nil {
		h.Log.Error("end session", "err", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
