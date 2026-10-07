// Package web is the site's HTTP surface for the trial: a placeholder page,
// who-am-I, sign-in through Cloudflare Access, and sign-out. The React app
// replaces the placeholder page later (ADR 0003).
package web

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
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
	Log      *slog.Logger
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/healthz", h.healthz)
	r.Get("/", h.index)
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
// record, which is re-read every time so removal takes effect at once.
func (h *Handler) identify(ctx context.Context, w http.ResponseWriter, r *http.Request) whoami {
	s, err := h.Sessions.Current(ctx, w, r)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.Log.Error("session lookup", "err", err)
		}
		return whoami{Role: "visitor"}
	}
	m, err := h.Store.GetMember(ctx, s.Email)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.Log.Error("member lookup", "err", err)
		}
		return whoami{Role: "visitor"}
	}
	tools := m.Tools
	if tools == nil {
		tools = []string{}
	}
	return whoami{Role: "member", Email: m.Email, Tools: tools}
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(h.identify(r.Context(), w, r))
}

// signin runs only behind Cloudflare Access. Access has already proved who
// the person is; the site checks the token, requires a Member record, then
// starts its own Sign-in session.
func (h *Handler) signin(w http.ResponseWriter, r *http.Request) {
	if h.Access == nil {
		http.Error(w, "sign-in is not configured", http.StatusNotImplemented)
		return
	}
	tok := access.TokenFromRequest(r)
	if tok == "" {
		http.Error(w, "sign-in must go through Cloudflare Access", http.StatusUnauthorized)
		return
	}
	id, err := h.Access.Verify(r.Context(), tok)
	if err != nil {
		h.Log.Warn("access token rejected", "err", err)
		http.Error(w, "sign-in could not be verified", http.StatusUnauthorized)
		return
	}
	if _, err := h.Store.GetMember(r.Context(), id.Email); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Log.Info("sign-in by non-member", "email", id.Email)
			http.Error(w, "this address has not been invited", http.StatusForbidden)
			return
		}
		h.Log.Error("member lookup", "err", err)
		http.Error(w, "sign-in failed", http.StatusInternalServerError)
		return
	}
	if _, err := h.Sessions.Start(r.Context(), w, id.Email); err != nil {
		h.Log.Error("start session", "err", err)
		http.Error(w, "sign-in failed", http.StatusInternalServerError)
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

var indexPage = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>craigdevjohnson.com ({{.Env}})</title>
<style>body{font:16px/1.5 system-ui,sans-serif;margin:2rem auto;max-width:36rem;padding:0 1rem}button{font:inherit}</style>
</head>
<body>
<h1>craigdevjohnson.com</h1>
<p>Trial build, {{.Env}} environment.</p>
<p id="who">Checking who you are…</p>
<p id="actions"></p>
<script>
fetch('/api/me',{credentials:'same-origin'}).then(r=>r.json()).then(me=>{
  const who=document.getElementById('who'),act=document.getElementById('actions');
  if(me.role==='member'){
    who.textContent='Signed in as '+me.email+'. Tools: '+(me.tools.length?me.tools.join(', '):'none yet')+'.';
    act.innerHTML='<form method="post" action="/signout"><button>Sign out</button></form>';
  }else{
    who.textContent='You are a Visitor.';
    act.innerHTML='<a href="/signin">Sign in</a>';
  }
}).catch(()=>{document.getElementById('who').textContent='Could not reach the site.'});
</script>
</body>
</html>
`))

func (h *Handler) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = indexPage.Execute(w, map[string]string{"Env": h.Env})
}
