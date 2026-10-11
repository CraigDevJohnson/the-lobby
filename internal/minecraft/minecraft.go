// Package minecraft is the site's side of the Minecraft Launcher (ADR 0001).
// It checks that the person holds the Launcher's grant, then forwards the
// server's status and the Start server request to the Tool's backend, naming
// the Member in a header (ADR 0006). Whether the server is running, and
// starting it, are the backend's; the site never reaches the game host, its
// Crafty Controller or AWS on a Member's behalf.
//
// The backend answers two requests, described in docs/minecraft-backend.md:
//
//	GET  /server        the server's current status
//	POST /server/start  start it if it is stopped; answers the status after
package minecraft

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

// Grant is the access a Member needs for every route here. The separate
// minecraft-admin grant does not include it.
const Grant = "minecraft"

// MemberHeader names the Member in each request to the backend. The backend
// believes it because only the site can reach the backend (ADR 0006).
const MemberHeader = "X-Lobby-Member"

// Access is what the site knows about the person asking.
type Access int

const (
	// SignedOut: no Sign-in session, or one that no longer counts.
	SignedOut Access = iota
	// NoAccess: a Member without the Launcher's grant.
	NoAccess
	// Granted: a Member with the Launcher's grant.
	Granted
)

// Tool forwards Launcher requests to the Minecraft backend.
type Tool struct {
	// Backend is the backend's address without a trailing slash. Empty
	// means no backend is configured, which the page explains as such.
	Backend string
	// Sign signs a request to the backend (ADR 0006). Nil on a developer's
	// computer, where the backend is an ordinary local server.
	Sign func(r *http.Request, body []byte) error
	// Check says whether the person asking may use the Launcher, and gives
	// the Member's email when they may. An error means access could not be
	// checked, which is never treated as either answer.
	Check  func(w http.ResponseWriter, r *http.Request) (Access, string, error)
	Client *http.Client
	Log    *slog.Logger
}

func (t *Tool) Routes(r chi.Router) {
	r.Get("/api/minecraft/server", t.status)
	r.Post("/api/minecraft/server/start", t.start)
}

const maxResponseBody = 64 << 10

// GET /api/minecraft/server
func (t *Tool) status(w http.ResponseWriter, r *http.Request) {
	email, ok := t.allow(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodGet, "/server", email)
}

// POST /api/minecraft/server/start. Starting is idempotent at the backend:
// a second request while the server starts or runs changes nothing and gets
// the same status back, so repeated taps and several Members agree.
func (t *Tool) start(w http.ResponseWriter, r *http.Request) {
	// The Sign-in session cookie is SameSite=Lax, which already keeps it off
	// another site's POST; this refuses one outright where the browser says
	// where the request came from.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		writeError(w, http.StatusForbidden, "cross_site", "start the server from the Minecraft Launcher page")
		return
	}
	email, ok := t.allow(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodPost, "/server/start", email)
}

// allow answers the request itself unless the person may use the Launcher.
// Nothing about the server is said to anyone who may not.
func (t *Tool) allow(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil {
		t.Log.Error("minecraft: no access check configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return "", false
	}
	a, email, err := t.Check(w, r)
	switch {
	case err != nil:
		t.Log.Error("minecraft access check", "err", err)
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
	case a == SignedOut:
		writeError(w, http.StatusUnauthorized, "signed_out", "sign in to use the Minecraft Launcher")
	case a != Granted || email == "":
		writeError(w, http.StatusForbidden, "no_access", "you don't have access to this Tool")
	default:
		return email, true
	}
	return "", false
}

var errNotConnected = errors.New("MINECRAFT_BACKEND_URL is unset")

// forward passes the backend's status on. Only the answers the page knows
// how to read come through; anything else is the backend not answering.
func (t *Tool) forward(w http.ResponseWriter, r *http.Request, method, path, email string) {
	if t.Backend == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "the Minecraft Launcher has no server configured")
		return
	}
	res, err := t.do(r, method, path, email)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNotFound:
	default:
		t.unavailable(w, errors.New("the backend answered "+res.Status))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(res.Body, maxResponseBody))
}

func (t *Tool) do(r *http.Request, method, path, email string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), method, t.Backend+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(MemberHeader, email)
	if t.Sign != nil {
		if err := t.Sign(req, nil); err != nil {
			return nil, err
		}
	}
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, err
	}
	// AWS, not the backend, answers these when the signature is refused.
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		res.Body.Close()
		return nil, errors.New("the backend refused the site's request: " + res.Status)
	}
	return res, nil
}

// unavailable answers when the backend cannot be reached or answered
// something unreadable. The page then says it can't confirm the status; it
// never guesses one.
func (t *Tool) unavailable(w http.ResponseWriter, err error) {
	t.Log.Error("minecraft backend", "err", err)
	writeError(w, http.StatusServiceUnavailable, "tool_unavailable", "the Minecraft server's status can't be checked right now")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
