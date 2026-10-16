package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// dist holds the screens built from web/ by `task ui:build` (ADR 0003). In a
// checkout where they have not been built it holds only .gitkeep, and the
// site answers with a plain notice instead of the screens.
//
//go:embed all:dist
var dist embed.FS

// UI returns the built screens.
func UI() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

const (
	// The public welcome, rendered in full at build time.
	welcomePage = "index.html"
	// The VIP Lobby's checking state, served while a Sign-in session cookie
	// is present so a Member never sees the public welcome first.
	checkingPage = "checking.html"
	// The public privacy note, the same for Visitors and Members.
	privacyPage = "privacy.html"
	// The Let's Plan Soccer screen, public and the same for everyone.
	soccerPage = "soccer.html"
	// The Minecraft Launcher's checking state. It holds nothing private: the
	// server's name, status and address arrive only after the page's own
	// call has passed the access check.
	minecraftPage = "minecraft.html"
	// Foundry's checking state. Like the Launcher's, it holds nothing
	// private: the server's status and join link arrive only after the
	// page's own call has passed the access check.
	foundryPage = "foundry.html"
	// Where sign-out lands: the public welcome saying the Member is signed
	// out, with a hidden frame that ends Cloudflare Access's sign-in too.
	// Served only just after a sign-out; see signedOutCookie.
	signedOutPage = "signed-out.html"
)

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	name := welcomePage
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		name = checkingPage
	}
	h.page(w, name)
}

// privacy never looks at the Sign-in session, so an expired or failing one
// cannot keep anyone from reading it.
func (h *Handler) privacy(w http.ResponseWriter, _ *http.Request) {
	h.page(w, privacyPage)
}

// soccer never looks at the Sign-in session either: the Visitor flow needs no
// sign-in, and an expired or failing session must not get in its way.
func (h *Handler) soccer(w http.ResponseWriter, _ *http.Request) {
	h.page(w, soccerPage)
}

func (h *Handler) minecraft(w http.ResponseWriter, _ *http.Request) {
	h.page(w, minecraftPage)
}

func (h *Handler) foundry(w http.ResponseWriter, _ *http.Request) {
	h.page(w, foundryPage)
}

// signedOut is shown only just after a sign-out, and never to a browser that
// carries a Sign-in session, so opening it at any other time, or from another
// site, neither ends Cloudflare's sign-in nor says the browser is signed out
// when it may not be. Those go to the front page, which picks the welcome or
// The VIP Lobby as usual.
func (h *Handler) signedOut(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); !justSignedOut(r) || (err == nil && c.Value != "") {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.page(w, signedOutPage)
}

func (h *Handler) page(w http.ResponseWriter, name string) {
	page, err := fs.ReadFile(h.UI, name)
	if err != nil {
		h.Log.Error("screens are not built", "page", name, "err", err)
		http.Error(w, "The Lobby's screens have not been built (task ui:build).", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(page)
}

// assets serves the built files. Names under assets/ carry a content hash,
// so they can be cached for good; anything else is revalidated.
func (h *Handler) assets(w http.ResponseWriter, r *http.Request) {
	// A missing file is answered uncached, so a cache never keeps a 404 for a
	// hashed name that a later or rolled-back build serves again.
	if _, err := fs.Stat(h.UI, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.FileServerFS(h.UI).ServeHTTP(w, r)
}
