package soccer

import (
	"bytes"
	"context"
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
	r.Delete("/api/soccer/me/link", t.revokeLink)
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
	if err := dec.Decode(&in); err != nil || len(in.TeamIDs) == 0 ||
		(in.After != "" && (len(in.TeamIDs) != 1 || !sixDigits.MatchString(in.After))) {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with team_ids, and after only beside one Team")
		return
	}
	if len(in.TeamIDs) > MaxRemembered {
		writeError(w, http.StatusConflict, "too_many_teams", "My calendar holds up to 5 Teams")
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
	was := s
	s.Teams = slices.Clone(s.Teams)
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
	if !t.saveTeams(w, r, was, s.Teams) {
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
	was := s
	s.Teams = slices.DeleteFunc(slices.Clone(s.Teams), func(rt store.RememberedTeam) bool { return slices.Contains(rt.TeamIDs, id) })
	if !t.saveTeams(w, r, was, s.Teams) {
		return
	}
	t.answer(w, r, s)
}

// Existing subscribers can retire their public address without forgetting Teams.
func (t *Tool) revokeLink(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	s, ok := t.load(w, r, email)
	if !ok {
		return
	}
	if s.Secret != "" {
		if err := t.Store.RevokeSoccerLink(r.Context(), s); err != nil {
			t.storeFailed(w, err)
			return
		}
		s.Secret = ""
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
	f, res, err := t.feed(r, s)
	switch {
	case err != nil:
		t.unavailable(w, err)
		return
	case res != nil:
		defer res.Body.Close()
		t.relay(w, res)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=900")
	_, _ = w.Write([]byte(f.Calendar))
}

// feedAnswer is what the backend's POST /feeds answers: the calendar of a
// Member's Remembered teams, holding every game of theirs, played and
// remaining, and those Remembered teams' Team IDs as they now are.
type feedAnswer struct {
	Calendar string     `json:"calendar"`
	Teams    [][]string `json:"teams"`
}

// feed asks the backend's POST /feeds for the calendar of s's Remembered
// teams and keeps the Teams the backend moved on, for a Member link fetch
// and for the daily iCloud update (daily.go). Any answer but a 200 comes
// back as the response, unread, for the caller to pass on or give up on,
// and to close.
func (t *Tool) feed(r *http.Request, s store.Soccer) (feedAnswer, *http.Response, error) {
	ids := make([][]string, len(s.Teams))
	for i, rt := range s.Teams {
		ids[i] = rt.TeamIDs
	}
	body, err := json.Marshal(map[string]any{"teams": ids})
	if err != nil {
		return feedAnswer{}, nil, err
	}
	res, err := t.do(r, http.MethodPost, "/feeds", body)
	if err != nil {
		return feedAnswer{}, nil, err
	}
	if res.StatusCode != http.StatusOK {
		return feedAnswer{}, res, nil
	}
	defer res.Body.Close()
	answer, err := read(res)
	if err != nil {
		return feedAnswer{}, nil, err
	}
	var f feedAnswer
	if err := json.Unmarshal(answer, &f); err != nil || !strings.HasPrefix(f.Calendar, "BEGIN:VCALENDAR") {
		return feedAnswer{}, nil, errors.New("the backend's answer to POST /feeds could not be read")
	}
	teams := slices.Clone(s.Teams)
	if len(f.Teams) != len(teams) {
		return feedAnswer{}, nil, errors.New("the backend returned a different number of Remembered teams")
	}
	changed := false
	for i, ids := range f.Teams {
		if len(ids) == 0 || len(ids) > 2 || slices.ContainsFunc(ids, func(id string) bool { return !sixDigits.MatchString(id) }) {
			return feedAnswer{}, nil, errors.New("the backend returned invalid Team IDs")
		}
		if !slices.Equal(ids, teams[i].TeamIDs) {
			if len(ids) != 2 || ids[1] != newest(teams[i]) {
				return feedAnswer{}, nil, errors.New("the backend returned an unrelated Team")
			}
			teams[i].TeamIDs = ids
			changed = true
		}
	}
	if changed {
		// A concurrent edit wins. The next fetch will follow its Teams.
		if err := t.Store.UpdateSoccerTeams(r.Context(), s, teams); err != nil && !errors.Is(err, store.ErrNotFound) {
			return feedAnswer{}, nil, err
		}
	}
	return f, nil, nil
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

func (t *Tool) saveTeams(w http.ResponseWriter, r *http.Request, was store.Soccer, teams []store.RememberedTeam) bool {
	if err := t.Store.UpdateSoccerTeams(r.Context(), was, teams); err != nil {
		t.storeFailed(w, err)
		return false
	}
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
