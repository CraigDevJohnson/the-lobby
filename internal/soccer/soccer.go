// Package soccer is the site's side of the Schedule Downloader (ADR 0001). It
// forwards the public Visitor flow to the Tool's backend and hands Session
// links to calendar apps. League access, schedules and calendar files stay in
// the backend (the soccer repo); nothing here reads the league or writes an
// event. None of these routes looks at the Sign-in session: the Visitor flow
// is public.
package soccer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// LinkRoute is the public address calendar apps fetch a Session link from.
// It carries the link's token, so the request log records the route, not the
// path.
const LinkRoute = "/soccer/link/{token}"

// Tool forwards requests to the Schedule Downloader's backend.
type Tool struct {
	// Backend is the backend's address without a trailing slash, such as
	// "http://127.0.0.1:8081". Empty means the Tool is not connected.
	Backend string
	// Sign signs a request to the backend (ADR 0006). Nil on a developer's
	// computer, where the backend is an ordinary local server.
	Sign func(r *http.Request, body []byte) error
	// BaseURL is the site's public address, which Session links are built
	// on. Empty means the address the request came to.
	BaseURL string
	Client  *http.Client
	Log     *slog.Logger
}

func (t *Tool) Routes(r chi.Router) {
	r.Get("/api/soccer/teams", t.searchTeams)
	r.Get("/api/soccer/teams/{teamID}", t.team)
	r.Get("/api/soccer/divisions", t.divisions)
	r.Get("/api/soccer/divisions/{divisionID}/teams", t.divisionTeams)
	r.Post("/api/soccer/calendar", t.download)
	r.Post("/api/soccer/links", t.createLink)
	r.Get(LinkRoute, t.serveLink)
	r.Head(LinkRoute, t.serveLink)
}

var (
	teamIDPattern = regexp.MustCompile(`^[0-9]{1,12}$`)
	tokenPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.ics)?$`)
)

// validToken reports whether s could be a Session link token, with or
// without the .ics ending calendar apps expect. The length is the backend's.
func validToken(s string) bool {
	return len(s) <= 4096+len(".ics") && tokenPattern.MatchString(s)
}

const (
	maxRequestBody  = 64 << 10
	maxResponseBody = 4 << 20
)

// GET /api/soccer/teams?q=rovers
func (t *Tool) searchTeams(w http.ResponseWriter, r *http.Request) {
	q := url.Values{"q": {r.URL.Query().Get("q")}}
	t.forward(w, r, http.MethodGet, "/teams?"+q.Encode(), nil)
}

// GET /api/soccer/teams/{teamID}: one Team and its games.
func (t *Tool) team(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "teamID")
	if !teamIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "a Team ID is a number")
		return
	}
	t.forward(w, r, http.MethodGet, "/teams/"+id, nil)
}

// GET /api/soccer/divisions
func (t *Tool) divisions(w http.ResponseWriter, r *http.Request) {
	t.forward(w, r, http.MethodGet, "/divisions", nil)
}

// GET /api/soccer/divisions/{divisionID}/teams
func (t *Tool) divisionTeams(w http.ResponseWriter, r *http.Request) {
	t.forward(w, r, http.MethodGet, "/divisions/"+url.PathEscape(chi.URLParam(r, "divisionID"))+"/teams", nil)
}

// POST /api/soccer/calendar: the checked games as a one-time .ics file. The
// backend decides what the body means and writes the file.
func (t *Tool) download(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	t.forward(w, r, http.MethodPost, "/calendar", body)
}

// linkSince is sent as the backend's "since" for every Session link. The
// backend puts every game starting at or after it in the link, so a moment
// before any Session makes the link carry all of the chosen Teams' games,
// including ones the league posts later.
var linkSince = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// POST /api/soccer/links: a Session link for the chosen Teams. Only the
// Teams are read from the request: the games someone unchecked for a
// download never narrow a Session link.
func (t *Tool) createLink(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	var in struct {
		TeamIDs []string `json:"team_ids"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || len(in.TeamIDs) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with team_ids")
		return
	}
	out, err := json.Marshal(map[string]any{"team_ids": in.TeamIDs, "since": linkSince})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	res, err := t.do(r, http.MethodPost, "/links", out)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.relay(w, res)
		return
	}
	var made struct {
		Token string          `json:"token"`
		Teams json.RawMessage `json:"teams"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBody)).Decode(&made); err != nil || !validToken(made.Token) {
		t.unavailable(w, errors.New("the backend's answer to POST /links could not be read"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{
		"url":   t.base(r) + "/soccer/link/" + strings.TrimSuffix(made.Token, ".ics") + ".ics",
		"teams": made.Teams,
	})
}

// GET /soccer/link/{token}.ics: what a calendar app fetches. The backend's
// status comes through unchanged: a calendar app keeps its last full copy on
// a 503, and would drop games on a 200 that is missing them.
func (t *Tool) serveLink(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if !validToken(token) {
		writeError(w, http.StatusNotFound, "invalid_link", "this calendar link is not valid")
		return
	}
	res, err := t.do(r, r.Method, "/links/"+token, nil)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	t.relay(w, res)
}

func (t *Tool) forward(w http.ResponseWriter, r *http.Request, method, path string, body []byte) {
	res, err := t.do(r, method, path, body)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	t.relay(w, res)
}

var errNotConnected = errors.New("SOCCER_BACKEND_URL is unset")

func (t *Tool) do(r *http.Request, method, path string, body []byte) (*http.Response, error) {
	if t.Backend == "" {
		return nil, errNotConnected
	}
	req, err := http.NewRequestWithContext(r.Context(), method, t.Backend+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/calendar")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// The calendar app's own name goes along, so the backend's log can say
	// which app fetched a link.
	if ua := r.UserAgent(); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if t.Sign != nil {
		if err := t.Sign(req, body); err != nil {
			return nil, err
		}
	}
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	// AWS, not the backend, answers these when the signature is refused.
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		res.Body.Close()
		return nil, errors.New("the backend refused the site's request: " + res.Status)
	}
	return res, nil
}

// relay passes the backend's answer on as it is.
func (t *Tool) relay(w http.ResponseWriter, res *http.Response) {
	for _, name := range []string{"Content-Type", "Content-Disposition", "Cache-Control"} {
		if v := res.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(res.Body, maxResponseBody))
}

// unavailable answers when the backend itself cannot be reached, which is
// different from the league being down (the backend's own league_unavailable).
func (t *Tool) unavailable(w http.ResponseWriter, err error) {
	t.Log.Error("soccer backend", "err", err)
	w.Header().Set("Cache-Control", "no-store")
	writeError(w, http.StatusServiceUnavailable, "tool_unavailable", "the Schedule Downloader is not answering")
}

func (t *Tool) base(r *http.Request) string {
	if t.BaseURL != "" {
		return strings.TrimSuffix(t.BaseURL, "/")
	}
	return "http://" + r.Host
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "body is too large")
		return nil, false
	}
	return body, true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
