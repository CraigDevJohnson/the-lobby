// Package devices serves the family device profiles at /admin: a page that
// lists the two profiles with their install steps, the profile files
// themselves, and the small answer the page uses to learn whether they are
// configured yet. The routes sit behind their own Cloudflare Access
// application, which allows only the adults, and the server checks that
// application's token on every request; the Sign-in session and the Member
// record play no part (ADR 0010).
package devices

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/profile"
)

// Where the routes live. The Access application is on PagePath and covers
// every path under it, which takes in the files.
const (
	PagePath   = "/admin"
	FilesPath  = "/admin/profiles/"
	StatusPath = "/api/devices/profiles"

	// The page, prerendered from web/ (ADR 0003). It holds nothing private:
	// two links and the install steps.
	pageFile = "admin.html"

	// What iOS expects a configuration profile to be served as.
	contentType = "application/x-apple-aspen-config"
)

type Handler struct {
	// Env is dev, prod, or local. On a developer's computer, where Access is
	// not configured, the routes serve without the check; anywhere else a
	// missing Access application is refused.
	Env string
	// Access checks the /admin application's token. nil means the
	// application is not configured (ACCESS_ADMIN_AUD unset).
	Access access.Verifier
	// The two profiles, rendered from the server's settings at request time.
	Kids  profile.Kids
	Adult profile.Adult
	UI    fs.FS // the built screens, for the page
	Log   *slog.Logger
}

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.guard)
		r.Get(PagePath, h.page)
		r.Get(FilesPath+"{name}", h.file)
		r.Get(StatusPath, h.status)
	})
}

// guard is the server's own check of the Access application's token, behind
// Cloudflare's. The origin is already closed to anything that did not come
// through Cloudflare (ADR 0005), so this guards against the application
// being misconfigured or removed, not against direct origin access.
func (h *Handler) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.Access == nil {
			if h.Env != "local" {
				writeError(w, http.StatusServiceUnavailable, "access_not_configured", "the family devices page has no Access application yet")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		tok := access.TokenFromRequest(r)
		if tok == "" {
			h.Log.Warn("family devices request without an Access token", "path", r.URL.Path)
			writeError(w, http.StatusForbidden, "forbidden", "this page is reached through Cloudflare Access")
			return
		}
		if _, err := h.Access.Verify(r.Context(), tok); err != nil {
			h.Log.Warn("family devices access token rejected", "path", r.URL.Path, "err", err)
			writeError(w, http.StatusForbidden, "forbidden", "this page is reached through Cloudflare Access")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) page(w http.ResponseWriter, _ *http.Request) {
	page, err := fs.ReadFile(h.UI, pageFile)
	if err != nil {
		h.Log.Error("screens are not built", "page", pageFile, "err", err)
		http.Error(w, "The Lobby's screens have not been built (task ui:build).", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(page)
}

// entry is one profile as the page lists them.
type entry struct {
	Device string `json:"device"` // kids or adult
	File   string `json:"file"`
	Href   string `json:"href"`
}

var entries = []entry{
	{Device: "kids", File: profile.KidsFileName, Href: FilesPath + profile.KidsFileName},
	{Device: "adult", File: profile.AdultFileName, Href: FilesPath + profile.AdultFileName},
}

var errUnknownProfile = errors.New("devices: no such profile")

// render builds the named profile from the server's settings. A name that is
// not one of the two is unknown, not an empty file.
func (h *Handler) render(name string) ([]byte, error) {
	switch name {
	case profile.KidsFileName:
		return h.Kids.Render()
	case profile.AdultFileName:
		return h.Adult.Render()
	}
	return nil, errUnknownProfile
}

// status tells the page whether both profiles can be served: the two files,
// or 503 not_configured while a setting is unset. Nothing with a placeholder
// in it is ever served, so the page says so instead.
func (h *Handler) status(w http.ResponseWriter, _ *http.Request) {
	for _, e := range entries {
		if _, err := h.render(e.File); err != nil {
			h.notConfigured(w, e.File, err)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"profiles": entries})
}

// file serves one profile with the headers iOS wants: the profile content
// type, and the file's own name for the download.
func (h *Handler) file(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	data, err := h.render(name)
	if errors.Is(err, errUnknownProfile) {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.notConfigured(w, name, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// notConfigured answers 503 not_configured. An unset setting is expected
// until the Zero Trust root has been applied and its outputs carried over;
// anything else is a setting with a wrong value, and is logged as such.
func (h *Handler) notConfigured(w http.ResponseWriter, file string, err error) {
	if errors.Is(err, profile.ErrNotConfigured) {
		h.Log.Info("family device profile not configured", "file", file, "err", err)
	} else {
		h.Log.Error("family device profile cannot be rendered", "file", file, "err", err)
	}
	writeError(w, http.StatusServiceUnavailable, "not_configured", "the device profiles are not configured yet")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
