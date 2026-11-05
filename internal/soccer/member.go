package soccer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// Grant is the Member grant for Let's Plan Soccer (`task member:add`). It
// opens My calendar: Remembered teams and their Member link (ADR 0011).
const Grant = "soccer"

// MaxRemembered is the most Remembered teams a Member keeps. Each holds at
// most two Team IDs, so a Member link's calendar stays within the 10 Teams
// the backend combines.
const MaxRemembered = 5

// MemberLinkRoute is the public address calendar apps fetch a Member link
// from. It carries the link's secret, so the site's request log records the
// route, not the path (internal/app).
const MemberLinkRoute = LinkPrefix + "member/{secret}"

// Access is what the site knows about the person asking.
type Access int

const (
	// SignedOut: no Sign-in session, or one that no longer counts.
	SignedOut Access = iota
	// NoAccess: a Member without Grant.
	NoAccess
	// Granted: a Member with Grant.
	Granted
)

// secretPattern is a Member link's secret: 32 random bytes as unpadded
// base64url.
var secretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// sixDigits is a Team ID as the backend accepts it.
var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// newest is a Remembered team's current Team ID.
func newest(rt store.RememberedTeam) string {
	if len(rt.TeamIDs) == 0 {
		return ""
	}
	return rt.TeamIDs[0]
}

func (t *Tool) memberRoutes(r chi.Router) {
	r.Get("/api/soccer/me", t.myCalendar)
	r.Post("/api/soccer/me/teams", t.addTeams)
	r.Delete("/api/soccer/me/teams/{teamID}", t.removeTeam)
	r.Post("/api/soccer/me/link", t.replaceLink)
	r.Get(MemberLinkRoute, t.serveMemberLink)
	r.Head(MemberLinkRoute, t.serveMemberLink)
}

// myAnswer is what every My calendar route answers: the Member link, once
// there is one, and the Remembered teams.
type myAnswer struct {
	URL   string                 `json:"url,omitempty"`
	Teams []store.RememberedTeam `json:"teams"`
}

// GET /api/soccer/me: the Member's link and Remembered teams.
func (t *Tool) myCalendar(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, false)
	if !ok {
		return
	}
	s, ok := t.load(w, r, email)
	if !ok {
		return
	}
	t.answer(w, r, s)
}

// POST /api/soccer/me/teams {team_ids, after?}: adds Teams to My calendar.
// They are read from the backend first, so only Teams the league lists are
// kept, under the names it gives. With after, the one new Team carries on
// the Remembered team whose newest Team ID is after, as a follow does: it
// goes in front and only the two newest are kept.
func (t *Tool) addTeams(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	var in struct {
		TeamIDs []string `json:"team_ids"`
		After   string   `json:"after"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || len(in.TeamIDs) == 0 || len(in.TeamIDs) > MaxRemembered ||
		(in.After != "" && (len(in.TeamIDs) != 1 || !sixDigits.MatchString(in.After))) {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with team_ids, and after only beside one Team")
		return
	}
	for _, id := range in.TeamIDs {
		if !sixDigits.MatchString(id) {
			writeError(w, http.StatusBadRequest, "invalid_team_id", "a Team ID has six digits")
			return
		}
	}
	names, ok := t.teamNames(w, r, in.TeamIDs)
	if !ok {
		return
	}
	s, ok := t.load(w, r, email)
	if !ok {
		return
	}
	for _, id := range in.TeamIDs {
		if i := slices.IndexFunc(s.Teams, func(rt store.RememberedTeam) bool { return in.After != "" && newest(rt) == in.After }); i >= 0 {
			s.Teams[i] = store.RememberedTeam{Name: names[id], TeamIDs: []string{id, in.After}}
			continue
		}
		if slices.ContainsFunc(s.Teams, func(rt store.RememberedTeam) bool { return newest(rt) == id }) {
			continue
		}
		if len(s.Teams) >= MaxRemembered {
			writeError(w, http.StatusConflict, "too_many_teams", "My calendar holds up to 5 Teams")
			return
		}
		s.Teams = append(s.Teams, store.RememberedTeam{Name: names[id], TeamIDs: []string{id}})
	}
	if s.Secret == "" {
		if !t.newLink(w, r, &s) {
			return
		}
	}
	if !t.save(w, r, s) {
		return
	}
	t.answer(w, r, s)
}

// teamNames reads the Teams in one backend call and returns their names.
// Any Team the backend could not read refuses the add: a Team ID the league
// does not list as team_not_found, the league failing as league_unavailable.
func (t *Tool) teamNames(w http.ResponseWriter, r *http.Request, ids []string) (map[string]string, bool) {
	res, err := t.do(r, http.MethodGet, "/schedule?"+url.Values{"team": ids}.Encode(), nil)
	if err != nil {
		t.unavailable(w, err)
		return nil, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.relay(w, res)
		return nil, false
	}
	answer, err := read(res)
	if err != nil {
		t.unavailable(w, err)
		return nil, false
	}
	var s struct {
		Teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"teams"`
		Failed []struct {
			Reason string `json:"reason"`
		} `json:"failed"`
	}
	if err := json.Unmarshal(answer, &s); err != nil {
		t.unavailable(w, errors.New("the backend's answer to GET /schedule could not be read"))
		return nil, false
	}
	for _, f := range s.Failed {
		if f.Reason == "unavailable" {
			writeError(w, http.StatusServiceUnavailable, "league_unavailable", "the league's schedule service is not answering")
			return nil, false
		}
	}
	names := map[string]string{}
	for _, team := range s.Teams {
		names[team.ID] = strings.TrimSpace(team.Name)
	}
	for _, id := range ids {
		if _, ok := names[id]; !ok {
			writeError(w, http.StatusNotFound, "team_not_found", "no team has that team ID")
			return nil, false
		}
	}
	return names, true
}

// DELETE /api/soccer/me/teams/{teamID}: takes the Remembered team holding
// that Team ID out of My calendar. Its link serves without it on the next
// fetch.
func (t *Tool) removeTeam(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	id := chi.URLParam(r, "teamID")
	if !sixDigits.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "a Team ID has six digits")
		return
	}
	s, ok := t.load(w, r, email)
	if !ok {
		return
	}
	s.Teams = slices.DeleteFunc(s.Teams, func(rt store.RememberedTeam) bool { return slices.Contains(rt.TeamIDs, id) })
	if !t.save(w, r, s) {
		return
	}
	t.answer(w, r, s)
}

// POST /api/soccer/me/link: a new Member link. The old address stops at once.
func (t *Tool) replaceLink(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	s, ok := t.load(w, r, email)
	if !ok {
		return
	}
	old := s.Secret
	if !t.newLink(w, r, &s) || !t.save(w, r, s) {
		return
	}
	// The served link must match the Member's own secret, so an old item
	// left behind here would answer 404 anyway.
	if old != "" {
		if err := t.Store.DeleteSoccerLink(r.Context(), old); err != nil {
			t.Log.Error("soccer: delete the replaced Member link", "err", err)
		}
	}
	t.answer(w, r, s)
}

// GET /soccer/link/member/{secret}.ics: what a calendar app fetches. It goes
// from the secret to the Member, who must still hold Grant, and asks the
// backend for the calendar of their Remembered teams. The backend moves a
// Team on to its next Session when it can, and the Remembered teams it
// answers are kept for the next fetch. Like a Session link, any answer but
// a 200 comes through unchanged, so a 503 keeps the calendar app's last
// copy; only a link that is gone answers 404.
func (t *Tool) serveMemberLink(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSuffix(chi.URLParam(r, "secret"), ".ics")
	if !secretPattern.MatchString(secret) {
		writeError(w, http.StatusNotFound, "invalid_link", "this calendar link is not valid")
		return
	}
	if t.Store == nil {
		t.unavailable(w, errors.New("soccer: no store for Member links"))
		return
	}
	s, err := t.memberLink(r.Context(), secret)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "invalid_link", "this calendar link is not valid")
		return
	case err != nil:
		t.unavailable(w, err)
		return
	}
	ids := make([][]string, len(s.Teams))
	for i, rt := range s.Teams {
		ids[i] = rt.TeamIDs
	}
	body, err := json.Marshal(map[string]any{"teams": ids})
	if err != nil {
		t.unavailable(w, err)
		return
	}
	res, err := t.do(r, http.MethodPost, "/feeds", body)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.relay(w, res)
		return
	}
	answer, err := read(res)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	var feed struct {
		Calendar string     `json:"calendar"`
		Teams    [][]string `json:"teams"`
	}
	if err := json.Unmarshal(answer, &feed); err != nil || !strings.HasPrefix(feed.Calendar, "BEGIN:VCALENDAR") {
		t.unavailable(w, errors.New("the backend's answer to POST /feeds could not be read"))
		return
	}
	if moved(s.Teams, feed.Teams) {
		for i := range s.Teams {
			s.Teams[i].TeamIDs = feed.Teams[i]
		}
		// The calendar is right either way; the next fetch follows again.
		if err := t.Store.PutSoccer(r.Context(), s); err != nil {
			t.Log.Error("soccer: keep the followed Teams", "err", err)
		}
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=900")
	_, _ = w.Write([]byte(feed.Calendar))
}

// memberLink is the calendar a Member link's secret belongs to, or
// store.ErrNotFound when the link is gone: replaced, its Member removed, or
// their grant taken away.
func (t *Tool) memberLink(ctx context.Context, secret string) (store.Soccer, error) {
	email, err := t.Store.SoccerLink(ctx, secret)
	if err != nil {
		return store.Soccer{}, err
	}
	m, err := t.Store.GetMember(ctx, email)
	if err != nil {
		return store.Soccer{}, err
	}
	if !slices.Contains(m.Tools, Grant) {
		return store.Soccer{}, store.ErrNotFound
	}
	s, err := t.Store.GetSoccer(ctx, email)
	if err != nil {
		return store.Soccer{}, err
	}
	if s.Secret != secret {
		return store.Soccer{}, store.ErrNotFound
	}
	return s, nil
}

// moved reports whether the backend moved any Remembered team on. It must
// answer the same teams in the same order, each one or two Team IDs.
func moved(teams []store.RememberedTeam, answered [][]string) bool {
	if len(answered) != len(teams) {
		return false
	}
	changed := false
	for i, ids := range answered {
		if len(ids) < 1 || len(ids) > 2 || slices.ContainsFunc(ids, func(id string) bool { return !sixDigits.MatchString(id) }) {
			return false
		}
		if !slices.Equal(ids, teams[i].TeamIDs) {
			changed = true
		}
	}
	return changed
}

// member answers the request itself unless a Member with Grant is asking.
// A change must come from The Lobby's own page.
func (t *Tool) member(w http.ResponseWriter, r *http.Request, change bool) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil || t.Store == nil {
		t.Log.Error("soccer: My calendar is not configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return "", false
	}
	if change {
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			writeError(w, http.StatusForbidden, "cross_site", "change My calendar from The Lobby")
			return "", false
		}
	}
	a, email, err := t.Check(w, r)
	switch {
	case err != nil:
		t.Log.Error("soccer access check", "err", err)
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
	case a == SignedOut:
		writeError(w, http.StatusUnauthorized, "signed_out", "sign in to use My calendar")
	case a != Granted || email == "":
		writeError(w, http.StatusForbidden, "no_access", "you don't have access to My calendar")
	default:
		return email, true
	}
	return "", false
}

// load reads the Member's calendar; a Member with none yet has no Teams.
func (t *Tool) load(w http.ResponseWriter, r *http.Request, email string) (store.Soccer, bool) {
	s, err := t.Store.GetSoccer(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return store.Soccer{Email: email}, true
	case err != nil:
		t.storeFailed(w, err)
		return store.Soccer{}, false
	}
	return s, true
}

func (t *Tool) save(w http.ResponseWriter, r *http.Request, s store.Soccer) bool {
	if err := t.Store.PutSoccer(r.Context(), s); err != nil {
		t.storeFailed(w, err)
		return false
	}
	return true
}

// newLink gives s a new secret and points it at the Member. The calendar
// still has to be saved with it.
func (t *Tool) newLink(w http.ResponseWriter, r *http.Request, s *store.Soccer) bool {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.storeFailed(w, err)
		return false
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	if err := t.Store.PutSoccerLink(r.Context(), secret, s.Email); err != nil {
		t.storeFailed(w, err)
		return false
	}
	s.Secret = secret
	return true
}

func (t *Tool) answer(w http.ResponseWriter, r *http.Request, s store.Soccer) {
	a := myAnswer{Teams: s.Teams}
	if a.Teams == nil {
		a.Teams = []store.RememberedTeam{}
	}
	if s.Secret != "" {
		a.URL = t.base(r) + LinkPrefix + "member/" + s.Secret + ".ics"
	}
	writeJSON(w, http.StatusOK, a)
}

// storeFailed answers when the site's own table could not be read or
// written. The error names only the kind of record, never a key.
func (t *Tool) storeFailed(w http.ResponseWriter, err error) {
	t.Log.Error("soccer: My calendar store", "err", err)
	writeError(w, http.StatusServiceUnavailable, "tool_unavailable", "Let's Plan Soccer is not answering")
}
