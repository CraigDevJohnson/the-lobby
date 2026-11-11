// Package minecraft is the site's side of the Minecraft Launcher (ADR 0001).
// It checks that the person holds the Launcher's grant, then forwards the
// worlds' status and the requests to start or stop a world to the Tool's
// backend, naming the Member in a header (ADR 0006). Each world runs on its
// own server. Whether a world is running, and starting or stopping it, are
// the backend's; the site never reaches the game hosts, their Crafty
// Controller or AWS on a Member's behalf.
//
// The backend answers five requests, described in docs/minecraft-backend.md:
//
//	GET  /server        every world's current status
//	POST /server/start  start a world if it is stopped; answers its status
//	                    after
//	POST /server/stop   stop a world if it is ready; answers its status
//	                    after, or 409 with why it would not
//	GET  /mods          each world's mods and whether each is on
//	POST /mods          switch one mod on or off for one world, from its
//	                    next start
//
// The first three need the Launcher's grant, minecraft. The mods routes,
// for Minecraft Admin at /minecraft/admin, need minecraft-admin. Neither
// grant includes the other. The mods routes name no Member to the backend.
//
// A start or stop names its world in a body the site builds itself,
// {"world":"<id>"}, from the id the page sent; nothing else the browser
// sends reaches the backend. Without a world it sends no body, which the
// backend reads as its first world.
package minecraft

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"

	"github.com/go-chi/chi/v5"
)

// Grant is the access a Member needs to see, start and stop the worlds. The
// separate minecraft-admin grant does not include it.
const Grant = "minecraft"

// AdminGrant is the access a Member needs to see and switch the worlds' mods.
// The Launcher's grant does not include it.
const AdminGrant = "minecraft-admin"

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
	// Check says whether the person asking holds any of grants, and gives
	// the Member's email when they do. An error means access could not be
	// checked, which is never treated as either answer.
	Check func(w http.ResponseWriter, r *http.Request, grants ...string) (Access, string, error)
	// Client sends requests to the backend; nil means a plain client. Its
	// redirect policy is ignored: a redirect is never followed.
	Client *http.Client
	Log    *slog.Logger
}

func (t *Tool) Routes(r chi.Router) {
	r.Get("/api/minecraft/server", t.status)
	r.Post("/api/minecraft/server/start", t.start)
	r.Post("/api/minecraft/server/stop", t.stop)
	r.Get("/api/minecraft/mods", t.mods)
	r.Post("/api/minecraft/mods", t.switchMod)
}

const (
	maxResponseBody = 64 << 10
	// maxRequestBody bounds the page's start and stop bodies, which hold
	// only a world's id.
	maxRequestBody = 1 << 10
)

// worldID is the rule for a world's id, the backend's own.
var worldID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// GET /api/minecraft/server
func (t *Tool) status(w http.ResponseWriter, r *http.Request) {
	email, ok := t.allow(w, r, Grant)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodGet, "/server", email, nil)
}

// POST /api/minecraft/server/start. Starting is idempotent at the backend:
// a second request while the server starts or runs changes nothing and gets
// the same status back, so repeated taps and several Members agree.
func (t *Tool) start(w http.ResponseWriter, r *http.Request) {
	if !sameSite(w, r, "start the server from the Minecraft Launcher page") {
		return
	}
	email, ok := t.allow(w, r, Grant)
	if !ok {
		return
	}
	body, ok := world(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodPost, "/server/start", email, body)
}

// POST /api/minecraft/server/stop. The backend stops a ready server only
// when nobody is playing, and answers 409 with why not otherwise; it never
// kicks anyone, and the site never asks it to. As with starting, a second
// request while the server stops changes nothing and gets the status back.
func (t *Tool) stop(w http.ResponseWriter, r *http.Request) {
	if !sameSite(w, r, "stop the server from the Minecraft Launcher page") {
		return
	}
	email, ok := t.allow(w, r, Grant)
	if !ok {
		return
	}
	body, ok := world(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodPost, "/server/stop", email, body)
}

// world reads which world the page asked to start or stop and gives the body
// the site sends the backend: {"world":"<id>"}, or none when the page named
// no world. Anything else in the page's body is dropped, so a force or
// restore never reaches the backend from a browser. A body that isn't a
// JSON object of at most 1 KiB, or a world that isn't a world id, is
// answered 400 and the backend is never asked.
func world(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
	if err != nil || len(raw) > maxRequestBody {
		writeError(w, http.StatusBadRequest, "bad_request", "send at most a world's id")
		return nil, false
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send a JSON object naming the world")
		return nil, false
	}
	named, ok := fields["world"]
	if !ok {
		return nil, true
	}
	var id string
	if json.Unmarshal(named, &id) != nil || !worldID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request", "that isn't a world's id")
		return nil, false
	}
	body, _ := json.Marshal(map[string]string{"world": id})
	return body, true
}

// GET /api/minecraft/mods: each world's mods, for Minecraft Admin.
func (t *Tool) mods(w http.ResponseWriter, r *http.Request) {
	if _, ok := t.allow(w, r, AdminGrant); !ok {
		return
	}
	t.forward(w, r, http.MethodGet, "/mods", "", nil)
}

// POST /api/minecraft/mods: switch one mod on or off for one world. It
// applies at that world's next start. The backend answers with every
// world's mods, or 409 unknown_world or unknown_mod.
func (t *Tool) switchMod(w http.ResponseWriter, r *http.Request) {
	if !sameSite(w, r, "switch mods from the Minecraft Admin page") {
		return
	}
	if _, ok := t.allow(w, r, AdminGrant); !ok {
		return
	}
	body, ok := modSwitch(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodPost, "/mods", "", body)
}

// modSwitch reads a switch from the page and gives the body the site sends
// the backend, {"world":"<id>","mod":"<id>","enabled":<bool>}, built by the
// site from checked values; anything else in the page's body is dropped. A
// body that isn't that, or is over 1 KiB, is answered 400 and the backend
// is never asked.
func modSwitch(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
	var asked struct {
		World   *string `json:"world"`
		Mod     *string `json:"mod"`
		Enabled *bool   `json:"enabled"`
	}
	if err != nil || len(raw) > maxRequestBody || json.Unmarshal(raw, &asked) != nil ||
		asked.World == nil || !worldID.MatchString(*asked.World) ||
		asked.Mod == nil || !worldID.MatchString(*asked.Mod) || asked.Enabled == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send a world, a mod and whether it is on")
		return nil, false
	}
	body, _ := json.Marshal(struct {
		World   string `json:"world"`
		Mod     string `json:"mod"`
		Enabled bool   `json:"enabled"`
	}{*asked.World, *asked.Mod, *asked.Enabled})
	return body, true
}

// sameSite refuses a request the browser says came from another site, so
// no other site can start or stop a world, or switch a mod, through a
// Member's browser.
// The Sign-in session cookie is SameSite=Lax, which already keeps it off
// another site's POST; this refuses one outright where the browser says
// where the request came from.
func sameSite(w http.ResponseWriter, r *http.Request, from string) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		writeError(w, http.StatusForbidden, "cross_site", from)
		return false
	}
	return true
}

// allow answers the request itself unless the person holds one of grants.
// Nothing about the worlds is said to anyone who doesn't.
func (t *Tool) allow(w http.ResponseWriter, r *http.Request, grants ...string) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil {
		t.Log.Error("minecraft: no access check configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return "", false
	}
	a, email, err := t.Check(w, r, grants...)
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

// forward passes the backend's answer on. Only the answers the page knows
// how to read come through: the status, no server, and a refusal to change
// it (409 with its reason, such as unknown_world); anything else is the
// backend not answering.
func (t *Tool) forward(w http.ResponseWriter, r *http.Request, method, path, email string, body []byte) {
	if t.Backend == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "the Minecraft Launcher has no server configured")
		return
	}
	res, err := t.do(r, method, path, email, body)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNotFound, http.StatusConflict:
	default:
		t.unavailable(w, errors.New("the backend answered "+res.Status))
		return
	}
	// The whole answer is read and checked before any of it is sent, so one
	// that breaks off, runs longer than the page could need or isn't JSON is
	// reported like any other trouble rather than passed on half-written.
	answer, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBody+1))
	if err != nil {
		t.unavailable(w, err)
		return
	}
	if len(answer) > maxResponseBody {
		t.unavailable(w, errors.New("the backend answered more than 64 KiB"))
		return
	}
	if !json.Valid(answer) {
		t.unavailable(w, errors.New("the backend answered "+res.Status+" without JSON"))
		return
	}
	if res.StatusCode == http.StatusConflict {
		t.refused(w, answer)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)
	_, _ = w.Write(answer)
}

// refused passes on only a refusal's reason. The backend also says how many
// players are online, which the Launcher leaves out, so the count never
// reaches the browser.
func (t *Tool) refused(w http.ResponseWriter, answer []byte) {
	var reason struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(answer, &reason); err != nil || reason.Code == "" {
		t.unavailable(w, errors.New("the backend refused without a reason"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": reason.Code})
}

// do sends one request to the backend, with the site-built body when there
// is one, signed together with it. It names the Member when given one; the
// mods routes give none.
func (t *Tool) do(r *http.Request, method, path, email string, body []byte) (*http.Response, error) {
	var content io.Reader
	if body != nil {
		content = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, t.Backend+path, content)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if email != "" {
		req.Header.Set(MemberHeader, email)
	}
	if t.Sign != nil {
		if err := t.Sign(req, body); err != nil {
			return nil, err
		}
	}
	var client http.Client
	if t.Client != nil {
		client = *t.Client
	}
	// A redirect would carry the Member's email wherever it pointed, and
	// only the backend's own answer counts, so the 3xx itself comes back
	// and is reported as trouble.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
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
