// Package foundry is the site's side of Foundry, the Tool for Craig's
// Foundry Virtual Tabletop server (ADR 0001). It checks that the person
// holds Foundry's grant, then asks the Tool's backend for the server's
// status, naming the Member in a header (ADR 0006). The status is the
// backend's; the site never reaches the Foundry host or AWS on a Member's
// behalf.
//
// The backend is the foundry repo's controller. It answers one request
// today, described in docs/foundry-backend.md:
//
//	GET /server  the server's current status
//
// Starting and stopping the server are not part of it yet.
package foundry

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Grant is the access a Member needs for every route here. The separate
// foundry-admin grant does not include it.
const Grant = "foundry"

// MemberHeader names the Member in each request to the backend. The backend
// believes it because only the site can reach the backend (ADR 0006).
const MemberHeader = "X-Lobby-Member"

// Access is what the site knows about the person asking.
type Access int

const (
	// SignedOut: no Sign-in session, or one that no longer counts.
	SignedOut Access = iota
	// NoAccess: a Member without Foundry's grant.
	NoAccess
	// Granted: a Member with Foundry's grant.
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
	// Check says whether the person asking may use Foundry, and gives the
	// Member's email when they may. An error means access could not be
	// checked, which is never treated as either answer.
	Check func(w http.ResponseWriter, r *http.Request) (Access, string, error)
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
}

const maxResponseBody = 64 << 10

// GET /api/foundry/server
func (t *Tool) status(w http.ResponseWriter, r *http.Request) {
	email, ok := t.allow(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodGet, "/server", email)
}

// allow answers the request itself unless the person may use Foundry.
// Nothing about the server is said to anyone who may not.
func (t *Tool) allow(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil {
		t.Log.Error("foundry: no access check configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return "", false
	}
	a, email, err := t.Check(w, r)
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

// forward passes the backend's answer on. Only the answers the page knows
// how to read come through: the status and no server. Anything else is the
// backend not answering.
func (t *Tool) forward(w http.ResponseWriter, r *http.Request, method, path, email string) {
	if t.Backend == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Foundry has no server configured")
		return
	}
	res, err := t.do(r, method, path, email)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK, http.StatusNotFound:
	default:
		t.unavailable(w, errors.New("the backend answered "+res.Status))
		return
	}
	// The whole answer is read before any of it is sent, so one that breaks
	// off, or runs longer than the page could need, is reported like any
	// other trouble rather than passed on half-written.
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBody+1))
	if err != nil {
		t.unavailable(w, err)
		return
	}
	if len(body) > maxResponseBody {
		t.unavailable(w, errors.New("the backend answered more than 64 KiB"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)
	_, _ = w.Write(body)
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

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
