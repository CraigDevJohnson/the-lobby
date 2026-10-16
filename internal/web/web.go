// Package web is the site's HTTP surface: the screens built from web/ (the
// public welcome, The VIP Lobby, Privacy, Let's Plan Soccer, the Minecraft
// Launcher and Foundry, ADR 0003), who-am-I, sign-in through Cloudflare
// Access, and sign-out.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

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

// A successful sign-out marks the browser for a minute with signedOutCookie.
// Cloudflare documents that Access may still accept a sign-in for 20 to 30
// seconds after its logout, and in that time /signin could start a new Sign-in
// session without a new code. While the mark lasts, /signed-out shows and
// ends Access's sign-in, and /signin goes back there instead of signing in.
// Without the mark, /signed-out has nothing to finish.
const (
	signedOutCookie = "site_signed_out"
	signedOutFor    = time.Minute
)

func justSignedOut(r *http.Request) bool {
	c, err := r.Cookie(signedOutCookie)
	return err == nil && c.Value != ""
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/healthz", h.healthz)
	r.Get("/", h.index)
	r.Get("/privacy", h.privacy)
	r.Get("/soccer", h.soccer)
	r.Get("/minecraft", h.minecraft)
	r.Get("/foundry", h.foundry)
	r.Get("/assets/*", h.assets)
	r.Get("/favicon.svg", h.assets)
	r.Get("/api/me", h.me)
	r.Get("/signin", h.signin)
	r.Post("/signout", h.signout)
	r.Get("/signed-out", h.signedOut)
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
			return h.visitor(ctx, w, r), nil
		}
		return whoami{}, fmt.Errorf("session lookup: %w", err)
	}
	m, err := h.Store.GetMember(ctx, s.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return h.visitor(ctx, w, r), nil
		}
		return whoami{}, fmt.Errorf("member lookup: %w", err)
	}
	tools := m.Tools
	if tools == nil {
		tools = []string{}
	}
	return whoami{Role: "member", Email: m.Email, Tools: tools}, nil
}

// visitor ends a Sign-in session that no longer counts (expired, deleted, or
// its Member removed), so the cookie stops sending later visits to The VIP
// Lobby's checking state.
func (h *Handler) visitor(ctx context.Context, w http.ResponseWriter, r *http.Request) whoami {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := h.Sessions.End(ctx, w, r); err != nil {
			h.Log.Error("end stale session", "err", err)
		}
	}
	return whoami{Role: "visitor"}
}

// Member says who is asking, for a Tool's own access check: the Member's
// email and grants, or an empty email for a Visitor. An error means access
// could not be checked.
func (h *Handler) Member(w http.ResponseWriter, r *http.Request) (email string, tools []string, err error) {
	who, err := h.identify(r.Context(), w, r)
	if err != nil {
		return "", nil, err
	}
	return who.Email, who.Tools, nil
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
	if justSignedOut(r) {
		// Access may have let this through on the sign-in being ended; the
		// signed-out page ends it again, and says when to try.
		h.Log.Info("sign-in just after sign-out")
		http.Redirect(w, r, "/signed-out", http.StatusSeeOther)
		return
	}
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
	http.Redirect(w, r, returnTo(r), http.StatusSeeOther)
}

// returnPaths are the pages a sign-in may come back to, named with
// /signin?next=. Anything else returns to The VIP Lobby, so the parameter
// can never send someone off the site.
var returnPaths = map[string]bool{"/minecraft": true, "/foundry": true}

func returnTo(r *http.Request) string {
	if next := r.URL.Query().Get("next"); returnPaths[next] {
		return next
	}
	return "/"
}

// signout ends the site's Sign-in session, then lands on the signed-out
// page, which ends Cloudflare Access's sign-in on this browser too, so the
// next Sign in asks for a new code. If the session could not be ended, the
// browser is still signed in, so it goes back to The VIP Lobby instead of a
// page saying otherwise.
//
// Only a signed-in browser on The Lobby's own pages signs out: another
// site's form, which the browser marks as such and which never carries the
// SameSite=Lax session cookie, goes to the front page with nothing changed,
// so it can neither end a Member's Access sign-in nor hold their next one.
func (h *Handler) signout(w http.ResponseWriter, r *http.Request) {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if c, err := r.Cookie(sessionCookie); err != nil || c.Value == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := h.Sessions.End(r.Context(), w, r); err != nil {
		h.Log.Error("end session", "err", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     signedOutCookie,
		Value:    "1",
		Path:     "/",
		MaxAge:   int(signedOutFor.Seconds()),
		HttpOnly: true,
		Secure:   h.Sessions.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/signed-out", http.StatusSeeOther)
}
