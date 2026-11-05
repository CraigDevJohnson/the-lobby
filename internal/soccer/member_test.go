package soccer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

const memberEmail = "player@example.com"

// fakeLeague is the backend as My calendar uses it: Team names for GET
// /schedule, and for POST /feeds a calendar plus the Remembered teams it was
// sent, or follow's answer when set.
type fakeLeague struct {
	mu     sync.Mutex
	names  map[string]string
	failed string            // a reason every Team read fails with
	follow map[string]string // newest Team ID -> next Session's Team ID
	feeds  []string          // bodies POST /feeds received
	status int               // POST /feeds answers this instead, when set
	during func()            // runs while POST /feeds is answering, when set
	events []map[string]any  // what POST /events answers
	asked  []string          // bodies POST /events received
}

func (f *fakeLeague) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/schedule":
		var teams, failed []map[string]string
		for _, id := range r.URL.Query()["team"] {
			if name, ok := f.names[id]; ok && f.failed == "" {
				teams = append(teams, map[string]string{"id": id, "name": " " + name + " "})
				continue
			}
			reason := cmpOr(f.failed, "not_found")
			failed = append(failed, map[string]string{"team_id": id, "reason": reason})
		}
		if len(teams) == 0 && f.failed == "unavailable" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"league_unavailable","message":"down"}`))
			return
		}
		if len(teams) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"team_not_found","message":"no team has that team ID"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"teams": teams, "games": []any{}, "failed": failed})
	case r.Method == http.MethodPost && r.URL.Path == "/events":
		b, _ := io.ReadAll(r.Body)
		f.asked = append(f.asked, string(b))
		_ = json.NewEncoder(w).Encode(map[string]any{"events": f.events})
	case r.Method == http.MethodPost && r.URL.Path == "/feeds":
		b, _ := io.ReadAll(r.Body)
		f.feeds = append(f.feeds, string(b))
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"code":"league_unavailable","message":"down"}`))
			return
		}
		if f.during != nil {
			f.during()
		}
		var in struct{ Teams [][]string }
		_ = json.Unmarshal(b, &in)
		for i, ids := range in.Teams {
			if next, ok := f.follow[ids[0]]; ok {
				in.Teams[i] = []string{next, ids[0]}
			}
		}
		if in.Teams == nil {
			in.Teams = [][]string{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"calendar": "BEGIN:VCALENDAR\r\nX-TEAMS:" + flat(in.Teams) + "\r\nEND:VCALENDAR\r\n", "teams": in.Teams})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func flat(teams [][]string) string {
	var ids []string
	for _, t := range teams {
		ids = append(ids, t...)
	}
	return strings.Join(ids, ",")
}

// memberSite is the Tool in front of a fake backend, with a Member holding
// the grant signed in, and the store it keeps My calendar in.
type memberSite struct {
	url    string
	league *fakeLeague
	store  *store.Memory
	access Access
	logs   *bytes.Buffer
	tool   *Tool
}

func newMemberSite(t *testing.T) *memberSite {
	t.Helper()
	m := &memberSite{
		league: &fakeLeague{names: map[string]string{"111111": "ROVERS", "222222": "KICKERS", "311111": "ROVERS", "411111": "KICKERS FC"}},
		store:  store.NewMemory(),
		access: Granted,
		logs:   &bytes.Buffer{},
	}
	be := httptest.NewServer(m.league)
	t.Cleanup(be.Close)
	_ = m.store.PutMember(context.Background(), store.Member{Email: memberEmail, Tools: []string{"minecraft", Grant}})
	tool := &Tool{
		Backend: be.URL,
		BaseURL: "https://lobby.example",
		Log:     slog.New(slog.NewJSONHandler(m.logs, nil)),
		Store:   m.store,
		Check: func(http.ResponseWriter, *http.Request) (Access, string, error) {
			if m.access == SignedOut {
				return SignedOut, "", nil
			}
			return m.access, memberEmail, nil
		},
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	m.url = site.URL
	m.tool = tool
	return m
}

func (m *memberSite) call(t *testing.T, method, path, body string) (*http.Response, string) {
	t.Helper()
	return do(t, method, m.url+path, body)
}

func (m *memberSite) mine(t *testing.T, method, path, body string) myAnswer {
	t.Helper()
	res, b := m.call(t, method, path, body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: %d %s", method, path, res.StatusCode, b)
	}
	var a myAnswer
	if err := json.Unmarshal([]byte(b), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

// fetch is a calendar app fetching the link: no cookie, no grant check of
// its own.
func (m *memberSite) fetch(t *testing.T, link string) (*http.Response, string) {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Host != "lobby.example" {
		t.Fatalf("link %q", link)
	}
	return do(t, http.MethodGet, m.url+u.Path, "")
}

func teams(a myAnswer) [][]string {
	var out [][]string
	for _, rt := range a.Teams {
		out = append(out, append([]string{rt.Name}, rt.TeamIDs...))
	}
	return out
}

func equalTeams(a, b [][]string) bool { return slices.EqualFunc(a, b, slices.Equal) }

func TestMyCalendarStartsEmpty(t *testing.T) {
	m := newMemberSite(t)
	a := m.mine(t, http.MethodGet, "/api/soccer/me", "")
	if a.URL != "" || a.Teams == nil || len(a.Teams) != 0 {
		t.Fatalf("%+v", a)
	}
}

func TestMyCalendarNeedsTheGrant(t *testing.T) {
	m := newMemberSite(t)
	for access, want := range map[Access]int{SignedOut: http.StatusUnauthorized, NoAccess: http.StatusForbidden} {
		m.access = access
		for _, c := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/soccer/me", ""},
			{http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`},
			{http.MethodDelete, "/api/soccer/me/teams/111111", ""},
			{http.MethodPost, "/api/soccer/me/link", ""},
		} {
			if res, body := m.call(t, c.method, c.path, c.body); res.StatusCode != want {
				t.Errorf("%v %s %s: %d %s", access, c.method, c.path, res.StatusCode, body)
			}
		}
	}
	if len(m.league.feeds) != 0 {
		t.Fatal("the backend was called")
	}
}

// Adding Teams reads them in one backend call, keeps the names it gives,
// and makes the Member link, which a calendar app then fetches.
func TestAddTeamsMakesTheMemberLink(t *testing.T) {
	m := newMemberSite(t)
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111","222222"]}`)
	if !equalTeams(teams(a), [][]string{{"ROVERS", "111111"}, {"KICKERS", "222222"}}) {
		t.Fatalf("teams = %v", teams(a))
	}
	if !strings.HasPrefix(a.URL, "https://lobby.example/soccer/link/member/") || !strings.HasSuffix(a.URL, ".ics") {
		t.Fatalf("url = %q", a.URL)
	}
	secret := strings.TrimSuffix(strings.TrimPrefix(a.URL, "https://lobby.example/soccer/link/member/"), ".ics")
	if !secretPattern.MatchString(secret) {
		t.Fatalf("secret %q", secret)
	}

	res, body := m.fetch(t, a.URL)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/calendar; charset=utf-8" ||
		res.Header.Get("Cache-Control") != "private, max-age=900" || !strings.Contains(body, "X-TEAMS:111111,222222") {
		t.Fatalf("%d %v %q", res.StatusCode, res.Header, body)
	}
	if got := m.league.feeds[0]; got != `{"teams":[["111111"],["222222"]]}` {
		t.Fatalf("feed request %s", got)
	}
	// Adding a Team already there, or again, changes nothing; the link stays.
	again := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["222222"]}`)
	if again.URL != a.URL || len(again.Teams) != 2 {
		t.Fatalf("%+v", again)
	}
	if mine := m.mine(t, http.MethodGet, "/api/soccer/me", ""); mine.URL != a.URL || len(mine.Teams) != 2 {
		t.Fatalf("%+v", mine)
	}
	if strings.Contains(m.logs.String(), secret) {
		t.Fatal("the secret reached the log")
	}
}

// When the backend moves a Team on to its next Session, the site keeps it,
// and the next fetch sends the new Team IDs.
func TestAFollowIsKept(t *testing.T) {
	m := newMemberSite(t)
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111","222222"]}`)
	m.league.follow = map[string]string{"111111": "311111"}
	if res, body := m.fetch(t, a.URL); res.StatusCode != http.StatusOK || !strings.Contains(body, "X-TEAMS:311111,111111,222222") {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	m.league.follow = nil
	m.fetch(t, a.URL)
	if got := m.league.feeds[1]; got != `{"teams":[["311111","111111"],["222222"]]}` {
		t.Fatalf("next fetch sent %s", got)
	}
	if mine := m.mine(t, http.MethodGet, "/api/soccer/me", ""); !equalTeams(teams(mine), [][]string{{"ROVERS", "311111", "111111"}, {"KICKERS", "222222"}}) {
		t.Fatalf("teams = %v", teams(mine))
	}
}

// A link replaced while a fetch is following isn't written over: the new
// link keeps working, and the follow waits for the next fetch.
func TestAChangeDuringAFetchWins(t *testing.T) {
	m := newMemberSite(t)
	ctx := context.Background()
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)
	m.league.follow = map[string]string{"111111": "311111"}
	fresh := strings.Repeat("n", 43)
	m.league.during = func() {
		s, _ := m.store.GetSoccer(ctx, memberEmail)
		s.Secret = fresh
		_ = m.store.PutSoccerLink(ctx, fresh, memberEmail)
		_ = m.store.PutSoccer(ctx, s)
	}
	if res, _ := m.fetch(t, a.URL); res.StatusCode != http.StatusOK {
		t.Fatalf("fetch: %d", res.StatusCode)
	}
	m.league.during = nil
	s, _ := m.store.GetSoccer(ctx, memberEmail)
	if s.Secret != fresh || !slices.Equal(s.Teams[0].TeamIDs, []string{"111111"}) {
		t.Fatalf("calendar = %+v", s)
	}
	if res, _ := m.fetch(t, "https://lobby.example/soccer/link/member/"+fresh+".ics"); res.StatusCode != http.StatusOK {
		t.Fatalf("new link: %d", res.StatusCode)
	}
}

// A pick from the next-Session page carries an ended Team on, as a follow
// does: the new Team ID goes in front, the old one stays, and the name is
// the new Team's.
func TestAPickAfterAnEndedSession(t *testing.T) {
	m := newMemberSite(t)
	m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111","222222"]}`)
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["411111"],"after":"222222"}`)
	if !equalTeams(teams(a), [][]string{{"ROVERS", "111111"}, {"KICKERS FC", "411111", "222222"}}) {
		t.Fatalf("teams = %v", teams(a))
	}
	// After a Team that isn't in My calendar, the pick is a new Team.
	a = m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["311111"],"after":"999999"}`)
	if len(a.Teams) != 3 || a.Teams[2].TeamIDs[0] != "311111" {
		t.Fatalf("teams = %v", teams(a))
	}
	for _, body := range []string{`{"team_ids":["311111","411111"],"after":"222222"}`, `{"team_ids":["311111"],"after":"22"}`, `{"team_ids":["31"]}`, `{"team_ids":[]}`, `{"team_ids":["311111"],"x":1}`} {
		if res, b := m.call(t, http.MethodPost, "/api/soccer/me/teams", body); res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d %s", body, res.StatusCode, b)
		}
	}
}

// A Team the backend can't read is never remembered.
func TestAddRefusesTeamsTheLeagueDoesNotList(t *testing.T) {
	m := newMemberSite(t)
	res, body := m.call(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111","555555"]}`)
	if res.StatusCode != http.StatusNotFound || errorCode(t, body) != "team_not_found" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	m.league.failed = "unavailable"
	res, body = m.call(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)
	if res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "league_unavailable" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	if _, err := m.store.GetSoccer(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("something was kept: %v", err)
	}
}

func TestMyCalendarHoldsFiveTeams(t *testing.T) {
	m := newMemberSite(t)
	for _, id := range []string{"100001", "100002", "100003", "100004", "100005", "100006"} {
		m.league.names[id] = "TEAM " + id
	}
	m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["100001","100002","100003","100004","100005"]}`)
	res, body := m.call(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["100006"]}`)
	if res.StatusCode != http.StatusConflict || errorCode(t, body) != "too_many_teams" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	// Six at once is the same answer.
	m.mine(t, http.MethodDelete, "/api/soccer/me/teams/100001", "")
	res, body = m.call(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["100001","100002","100003","100004","100005","100006"]}`)
	if res.StatusCode != http.StatusConflict || errorCode(t, body) != "too_many_teams" {
		t.Fatalf("six at once: %d %s", res.StatusCode, body)
	}
}

func TestRemoveATeam(t *testing.T) {
	m := newMemberSite(t)
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111","222222"]}`)
	a = m.mine(t, http.MethodDelete, "/api/soccer/me/teams/111111", "")
	if !equalTeams(teams(a), [][]string{{"KICKERS", "222222"}}) {
		t.Fatalf("teams = %v", teams(a))
	}
	m.fetch(t, a.URL)
	if got := m.league.feeds[0]; got != `{"teams":[["222222"]]}` {
		t.Fatalf("feed request %s", got)
	}
}

// Replace link stops the old address at once.
func TestReplaceLink(t *testing.T) {
	m := newMemberSite(t)
	old := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)
	fresh := m.mine(t, http.MethodPost, "/api/soccer/me/link", "")
	if fresh.URL == old.URL || fresh.URL == "" || len(fresh.Teams) != 1 {
		t.Fatalf("%+v", fresh)
	}
	if res, body := m.fetch(t, old.URL); res.StatusCode != http.StatusNotFound || errorCode(t, body) != "invalid_link" {
		t.Fatalf("old link: %d %s", res.StatusCode, body)
	}
	if res, _ := m.fetch(t, fresh.URL); res.StatusCode != http.StatusOK {
		t.Fatalf("new link: %d", res.StatusCode)
	}
}

// A link answers 404 once its Member loses the grant or is removed. Adding
// the Member again keeps their Teams and link.
func TestTheLinkStopsWithTheGrant(t *testing.T) {
	m := newMemberSite(t)
	ctx := context.Background()
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)

	_ = m.store.PutMember(ctx, store.Member{Email: memberEmail, Tools: []string{"minecraft"}})
	if res, _ := m.fetch(t, a.URL); res.StatusCode != http.StatusNotFound {
		t.Fatalf("without the grant: %d", res.StatusCode)
	}
	_ = m.store.PutMember(ctx, store.Member{Email: memberEmail, Tools: []string{Grant}})
	if res, _ := m.fetch(t, a.URL); res.StatusCode != http.StatusOK {
		t.Fatalf("grant given again: %d", res.StatusCode)
	}

	_ = m.store.DeleteMember(ctx, memberEmail)
	if res, _ := m.fetch(t, a.URL); res.StatusCode != http.StatusNotFound {
		t.Fatalf("Member removed: %d", res.StatusCode)
	}
	if _, err := m.store.GetSoccer(ctx, memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Remembered teams left behind: %v", err)
	}
	for _, bad := range []string{"/soccer/link/member/short.ics", "/soccer/link/member/" + strings.Repeat("a", 43) + ".ics"} {
		if res, body := m.call(t, http.MethodGet, bad, ""); res.StatusCode != http.StatusNotFound || errorCode(t, body) != "invalid_link" {
			t.Errorf("%s: %d %s", bad, res.StatusCode, body)
		}
	}
}

// The backend's failure comes through unchanged, so a calendar app keeps
// its last copy; the site's own table failing is the Tool being down.
func TestMemberLinkFailures(t *testing.T) {
	m := newMemberSite(t)
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)
	m.league.status = http.StatusServiceUnavailable
	if res, body := m.fetch(t, a.URL); res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "league_unavailable" {
		t.Fatalf("league down: %d %s", res.StatusCode, body)
	}

	m.league.status = 0
	broken := &brokenStore{Memory: m.store}
	tool := &Tool{Backend: "http://127.0.0.1:1", Log: slog.New(slog.NewJSONHandler(m.logs, nil)), Store: broken}
	r := chi.NewRouter()
	tool.Routes(r)
	u, _ := url.Parse(a.URL)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u.Path, nil))
	if rec.Code != http.StatusServiceUnavailable || errorCode(t, rec.Body.String()) != "tool_unavailable" {
		t.Fatalf("store down: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(m.logs.String(), strings.TrimSuffix(u.Path[len("/soccer/link/member/"):], ".ics")) {
		t.Fatal("the secret reached the log")
	}
}

type brokenStore struct{ *store.Memory }

func (brokenStore) SoccerLink(context.Context, string) (string, error) {
	return "", errors.New("get soccerlink: throttled")
}

// A change must come from The Lobby's own page.
func TestMyCalendarChangesAreSameSite(t *testing.T) {
	m := newMemberSite(t)
	req, _ := http.NewRequest(http.MethodPost, m.url+"/api/soccer/me/link", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("%d", res.StatusCode)
	}
}
