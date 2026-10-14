// Package foundry is the site's side of Foundry, the Tool for Craig's
// Foundry Virtual Tabletop server (ADR 0001). It checks that the person
// holds the route's grant, then asks the Tool's backend, naming the Member in
// a header (ADR 0006). The status is the backend's; the site never reaches
// the Foundry host or AWS on a Member's behalf.
//
// The backend is the foundry repo's controller. It answers three requests,
// described in docs/foundry-backend.md:
//
//	GET  /server        the server's current status
//	POST /server/start  start the host
//	POST /server/stop   shut Foundry down and stop the host
//
// Each route needs the foundry or foundry-admin grant.
package foundry

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Grant is the player Tool at /foundry: it lets a Member see the server's
// status, and start and stop it (Craig's "Every player" choice). The player
// page shows only the status; a later page offers starting and stopping.
const Grant = "foundry"

// AdminGrant is the Foundry Admin Tool. It allows everything Grant does.
const AdminGrant = "foundry-admin"

// MemberHeader names the Member in each request to the backend. The backend
// believes it because only the site can reach the backend (ADR 0006).
const MemberHeader = "X-Lobby-Member"

// Access is what the site knows about the person asking.
type Access int

const (
	// SignedOut: no Sign-in session, or one that no longer counts.
	SignedOut Access = iota
	// NoAccess: a Member without the grant asked for.
	NoAccess
	// Granted: a Member with the grant asked for.
	Granted
)

// Tool forwards Foundry requests to its backend.
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
	// JoinURL is the page players open to join, FOUNDRY_JOIN_URL: the same
	// address as the backend's own join_url. When the backend can't be
	// asked, a Member with the grant is still given it, so a failed status
	// check never keeps them from joining. Empty offers none.
	JoinURL string
	// Client sends requests to the backend; nil means a plain client. Its
	// redirect policy is ignored: a redirect is never followed.
	Client *http.Client
	Log    *slog.Logger
}

// ValidJoinURL says whether raw is exactly https://host[:port]/join, with no
// credentials, query or fragment: what the controller requires of its own
// join page setting.
func ValidJoinURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "/join" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}

func (t *Tool) Routes(r chi.Router) {
	r.Get("/api/foundry/server", t.status)
	r.Post("/api/foundry/server/start", t.start)
	r.Post("/api/foundry/server/stop", t.stop)
}

const (
	maxResponseBody = 64 << 10
	// maxConfirmation bounds the stop's request body.
	maxConfirmation = 1 << 10
)

// confirmation is the only body the site sends with a stop: the Member's
// word that nobody is playing, which the backend requires because it can't
// see who is.
var confirmation = []byte(`{"nobody_playing":true}`)

// GET /api/foundry/server: the player page reads it, and so does Foundry
// Admin, to follow a start or stop.
func (t *Tool) status(w http.ResponseWriter, r *http.Request) {
	email, ok := t.allow(w, r, Grant, AdminGrant)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodGet, "/server", email, nil)
}

// POST /api/foundry/server/start
func (t *Tool) start(w http.ResponseWriter, r *http.Request) {
	if !sameSite(w, r, "start") {
		return
	}
	email, ok := t.allow(w, r, Grant, AdminGrant)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodPost, "/server/start", email, nil)
}

// POST /api/foundry/server/stop: the page sends {"nobody_playing": true}
// once the Member confirms nobody is playing, and nothing is sent to the
// backend without it.
func (t *Tool) stop(w http.ResponseWriter, r *http.Request) {
	if !sameSite(w, r, "stop") {
		return
	}
	email, ok := t.allow(w, r, Grant, AdminGrant)
	if !ok {
		return
	}
	if !confirmed(w, r) {
		writeError(w, http.StatusBadRequest, "confirmation_required", "confirm that nobody is playing before stopping the server")
		return
	}
	t.forward(w, r, http.MethodPost, "/server/stop", email, confirmation)
}

// confirmed reads the page's body: a JSON object of at most 1 KiB whose
// nobody_playing is true.
func confirmed(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		NobodyPlaying *bool `json:"nobody_playing"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxConfirmation))
	if err := decoder.Decode(&body); err != nil || body.NobodyPlaying == nil || !*body.NobodyPlaying {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

// sameSite refuses a request the browser says came from another site, so no
// other site can start or stop the server through a Member's browser, as the
// Minecraft Launcher's routes do.
func sameSite(w http.ResponseWriter, r *http.Request, verb string) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		writeError(w, http.StatusForbidden, "cross_site", verb+" the server from The Lobby")
		return false
	}
	return true
}

// allow answers the request itself unless the person holds one of grants.
// Nothing about the server is said to anyone who doesn't.
func (t *Tool) allow(w http.ResponseWriter, r *http.Request, grants ...string) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil {
		t.Log.Error("foundry: no access check configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return "", false
	}
	a, email, err := t.Check(w, r, grants...)
	switch {
	case err != nil:
		t.Log.Error("foundry access check", "err", err)
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
	case a == SignedOut:
		writeError(w, http.StatusUnauthorized, "signed_out", "sign in to use Foundry")
	case a != Granted || email == "":
		writeError(w, http.StatusForbidden, "no_access", "you don't have access to this Tool")
	default:
		return email, true
	}
	return "", false
}

// refusals are the reasons the backend gives for refusing a change, by route:
// a start while the host is stopping, and a stop while it is starting or its
// agent is offline.
var refusals = map[string][]string{
	"/server/start": {"stopping"},
	"/server/stop":  {"starting", "agent_offline"},
}

// answered are the states a start or stop answers in, by route and by
// whether it began the change (202) or found nothing to change (200): a start
// begins in starting and has nothing to do once the host is starting or
// running, and a stop begins in stopping and has nothing to do once the host
// is stopped or stopping (docs/foundry-backend.md).
var answered = map[string]map[int][]string{
	"/server/start": {
		http.StatusAccepted: {"starting"},
		http.StatusOK:       {"starting", "ready", "unresponsive"},
	},
	"/server/stop": {
		http.StatusAccepted: {"stopping"},
		http.StatusOK:       {"stopped", "stopping"},
	},
}

// forward passes the backend's answer on. Only the answers the page knows
// how to read come through: the status, no server, and for a start or stop,
// the status once it began and the reason it was refused. Anything else,
// including a 404 that isn't the backend saying it has no server and a
// refusal for a reason the route doesn't give, is the backend not answering.
func (t *Tool) forward(w http.ResponseWriter, r *http.Request, method, path, email string, body []byte) {
	trouble := t.unavailable
	if method != http.MethodGet {
		trouble = t.changeUnavailable
	}
	if t.Backend == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Foundry has no server configured")
		return
	}
	res, err := t.do(r, method, path, email, body)
	if err != nil {
		trouble(w, err)
		return
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusOK, res.StatusCode == http.StatusNotFound:
	case method != http.MethodGet && (res.StatusCode == http.StatusAccepted || res.StatusCode == http.StatusConflict):
	default:
		trouble(w, errors.New("the backend answered "+res.Status))
		return
	}
	// The whole answer is read and checked before any of it is sent or
	// believed, so one that breaks off, runs longer than the page could need
	// or carries more than one JSON value is reported like any other trouble
	// rather than passed on half-written.
	answer, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBody+1))
	if err != nil {
		trouble(w, err)
		return
	}
	if len(answer) > maxResponseBody {
		trouble(w, errors.New("the backend answered more than 64 KiB"))
		return
	}
	if !json.Valid(answer) {
		trouble(w, errors.New("the backend answered "+res.Status+" without JSON"))
		return
	}
	switch res.StatusCode {
	case http.StatusNotFound:
		// Only the backend saying it has no server; a 404 for a route it
		// doesn't know must not tell the page there is no server.
		if reason(answer) != "no_server" {
			trouble(w, errors.New("the backend answered "+res.Status+" without no_server"))
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "no_server"})
	case http.StatusConflict:
		// Only the refusal's reason, and only one this route gives.
		code := reason(answer)
		if !slices.Contains(refusals[path], code) {
			trouble(w, errors.New("the backend refused without a reason this route gives"))
			return
		}
		writeJSON(w, http.StatusConflict, map[string]string{"code": code})
	default:
		// Only a status the page can show: one of its states, with the join
		// page, so a status that came through never hides the link.
		if !isStatus(answer) {
			trouble(w, errors.New("the backend answered "+res.Status+" without a status"))
			return
		}
		// A change answers in a state its route gives for that answer, so
		// a start never reports a stopped host as nothing to do, nor a stop
		// a running one.
		if method != http.MethodGet && !slices.Contains(answered[path][res.StatusCode], text(answer, "state")) {
			trouble(w, errors.New("the backend answered "+res.Status+" in a state this change doesn't answer in"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(answer)
	}
}

// states are the status's states (docs/foundry-backend.md).
var states = []string{"stopped", "starting", "ready", "stopping", "unresponsive", "unknown"}

// isStatus reports whether a JSON answer is a status: an object with one of
// the states and a join page held to the same rule as FOUNDRY_JOIN_URL, so
// the page never offers a link that isn't Foundry's join page.
func isStatus(answer []byte) bool {
	return slices.Contains(states, text(answer, "state")) && ValidJoinURL(text(answer, "join_url"))
}

// reason is the code in a JSON answer, or "" when it has none.
func reason(answer []byte) string {
	return text(answer, "code")
}

// text is the string a JSON object holds under exactly this key, or "" when
// the answer is not an object or holds no string there. Decoding into a struct
// would match a key in any case, but the page reads only the lowercase one.
func text(answer []byte, key string) string {
	var object map[string]json.RawMessage
	if json.Unmarshal(answer, &object) != nil {
		return ""
	}
	var value string
	if json.Unmarshal(object[key], &value) != nil {
		return ""
	}
	return value
}

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
	req.Header.Set(MemberHeader, email)
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
// never guesses one. It is only ever reached after the access check, so the
// join page it offers goes to Members with the grant alone.
func (t *Tool) unavailable(w http.ResponseWriter, err error) {
	t.Log.Error("foundry backend", "err", err)
	body := map[string]string{"code": "tool_unavailable", "message": "Foundry's status can't be checked right now"}
	if t.JoinURL != "" {
		body["join_url"] = t.JoinURL
	}
	writeJSON(w, http.StatusServiceUnavailable, body)
}

// changeUnavailable answers a start or stop the backend did not answer
// readably. The page then can't tell whether it happened, and asks for the
// status rather than sending it again.
func (t *Tool) changeUnavailable(w http.ResponseWriter, err error) {
	t.Log.Error("foundry backend", "err", err)
	writeError(w, http.StatusServiceUnavailable, "tool_unavailable", "Foundry's server can't be changed right now")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
