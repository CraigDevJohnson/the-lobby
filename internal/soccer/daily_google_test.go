package soccer

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

func googleDailySite(t *testing.T) (*memberSite, *fakeGoogle) {
	t.Helper()
	m, g := newGoogleSite(t)
	connect(t, m, same)
	m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)
	res, body := m.call(t, http.MethodPost, "/api/soccer/google/daily", `{"on":true,"calendar_id":"`+familyCalendar+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("daily setting: %s %s", res.Status, body)
	}
	m.mail = &outbox{}
	m.tool.Email = m.mail.send
	return m, g
}

func googleGames(t *testing.T, m *memberSite, games ...game) {
	t.Helper()
	m.lists(games...)
	m.league.events = nil
	for _, game := range games {
		at := game.at()
		start, err := time.ParseInLocation("20060102T150405", at.Start, time.FixedZone("MST", -7*3600))
		if err != nil {
			t.Fatal(err)
		}
		m.league.events = append(m.league.events, map[string]any{
			"id": game.id, "summary": "ROVERS vs OWLS - " + at.Field,
			"description": at.Field + "\nROVERS vs OWLS", "status": "confirmed",
			"start":              map[string]string{"dateTime": start.Format(time.RFC3339), "timeZone": "America/Denver"},
			"end":                map[string]string{"dateTime": start.Add(45 * time.Minute).Format(time.RFC3339), "timeZone": "America/Denver"},
			"extendedProperties": map[string]any{"private": map[string]string{"game_id": game.id}},
		})
	}
}

func runGoogle(t *testing.T, m *memberSite) store.Google {
	t.Helper()
	if err := m.tool.DailyGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err := m.store.GetGoogle(t.Context(), memberEmail)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDailyGoogleFollowsRememberedTeamAndMovesItsCopiedGame(t *testing.T) {
	m, g := googleDailySite(t)
	const id = "123456"
	googleGames(t, m, game{id: id})
	m.league.follow = map[string]string{"111111": "311111"}
	c := runGoogle(t, m)
	remembered, err := m.store.GetSoccer(t.Context(), memberEmail)
	if err != nil || remembered.Secret != "" || len(remembered.Teams) != 1 ||
		!slices.Equal(remembered.Teams[0].TeamIDs, []string{"311111", "111111"}) {
		t.Fatalf("saved team across sessions = %+v, %v", remembered.Teams, err)
	}
	if !slices.Equal(c.Written, uids(id)) || c.Times[uid(id)] != usualTime || !strings.Contains(g.events[familyCalendar][id], "ROVERS vs OWLS") {
		t.Fatalf("first copy: recorded %v at %v, Google has %v", c.Written, c.Times, g.events)
	}
	calls := g.calls
	runGoogle(t, m)
	if g.calls != calls || len(m.mail.sent) != 0 || len(m.league.feeds) != 2 || m.league.feeds[1] != `{"teams":[["311111","111111"]]}` {
		t.Fatalf("unchanged run: %d Google calls, %d emails, feeds %v", g.calls-calls, len(m.mail.sent), m.league.feeds)
	}
	for _, request := range m.league.asked {
		if !strings.Contains(request, `"team_ids":["311111","111111"]`) || !strings.Contains(request, `"since":`) {
			t.Fatalf("requested games with %s", request)
		}
	}

	moved := game{id: id, start: "20261122T183000", field: "Field 1"}
	googleGames(t, m, moved)
	c = runGoogle(t, m)
	if len(g.events[familyCalendar]) != 1 || !strings.Contains(g.events[familyCalendar][id], "2026-11-22T18:30:00-07:00") ||
		c.Times[uid(id)] != moved.at() || !slices.Equal(c.Written, uids(id)) || len(m.mail.sent) != 1 {
		t.Fatalf("move: recorded %v, Google has %v, %d emails", c.Times, g.events, len(m.mail.sent))
	}
	var selected struct{ Included []string }
	if err := json.Unmarshal([]byte(m.league.asked[len(m.league.asked)-1]), &selected); err != nil || !slices.Equal(selected.Included, []string{id}) {
		t.Fatalf("changed game must also be returned if it moved before today's cutoff: %v, %v", selected.Included, err)
	}
	for _, want := range []string{"your Google calendar", "Was: Sat, Nov 21 at 7:15 PM, Field 3", "Now: Sun, Nov 22 at 6:30 PM, Field 1"} {
		if !strings.Contains(m.mail.sent[0].Body, want) {
			t.Errorf("move email omitted %q", want)
		}
	}

	delete(g.events[familyCalendar], id)
	googleGames(t, m, game{id: id, start: "20261123T180000", field: "Field 2"})
	runGoogle(t, m)
	if len(g.events[familyCalendar]) != 0 || len(m.mail.sent) != 2 || !strings.Contains(m.mail.sent[1].Body, "wasn't found in your Google calendar") {
		t.Fatalf("member deletion: Google has %v and %d emails", g.events, len(m.mail.sent))
	}
	calls = g.calls
	runGoogle(t, m)
	if calls != g.calls || len(m.mail.sent) != 2 || len(g.events[familyCalendar]) != 0 {
		t.Fatal("another run rewrote or announced the deleted game")
	}
}

func TestDailyGoogleCarriesManualNextSessionChoice(t *testing.T) {
	m, g := googleDailySite(t)
	googleGames(t, m)
	const noticeUID = "session-over-111111@soccer.craigdevjohnson.com"
	const nextURL = "https://lobby.example/soccer?after=111111"
	m.league.feed = strings.Replace(m.league.feed, "END:VCALENDAR", "BEGIN:VEVENT\r\nUID:"+noticeUID+"\r\nSUMMARY:ROVERS: this Session is over\r\nDESCRIPTION:Pick next Session's Team: "+nextURL+"\r\nURL:"+nextURL+"\r\nDTSTART;VALUE=DATE:20261220\r\nDTEND;VALUE=DATE:20261221\r\nEND:VEVENT\r\nEND:VCALENDAR", 1)
	c := runGoogle(t, m)
	var notice struct {
		Description string
		Start       struct{ Date string }
	}
	body := g.events[familyCalendar]["sessionover111111"]
	if err := json.Unmarshal([]byte(body), &notice); err != nil || !strings.Contains(notice.Description, nextURL) || notice.Start.Date != "2026-12-20" ||
		!slices.Equal(c.Written, []string{noticeUID}) {
		t.Fatalf("notice = %s, history = %v", body, c.Written)
	}
	a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["311111"],"after":"111111"}`)
	if a.URL != "" || !equalTeams(teams(a), [][]string{{"ROVERS", "311111", "111111"}}) {
		t.Fatalf("manual choice = %+v", a)
	}
	googleGames(t, m, game{id: "456789"})
	c = runGoogle(t, m)
	if !slices.Equal(c.Written, []string{noticeUID, uid("456789")}) || g.events[familyCalendar]["456789"] == "" || len(g.events[familyCalendar]) != 2 ||
		!strings.Contains(m.league.asked[len(m.league.asked)-1], `"team_ids":["311111","111111"]`) {
		t.Fatalf("after manual choice: calendar = %v, history = %v", g.events, c.Written)
	}
}

func TestDailyGoogleRetriesAfterProviderLimitAndHonorsSwitchOff(t *testing.T) {
	m, g := googleDailySite(t)
	googleGames(t, m, game{id: "123456"}, game{id: "456789"})
	g.afterInsert = func(id string) { g.limited = true }
	c := runGoogle(t, m)
	if !c.Daily || !slices.Equal(c.Written, uids("123456")) || len(g.events[familyCalendar]) != 1 {
		t.Fatalf("after rate limit: history %v, calendar %v", c.Written, g.events)
	}
	g.afterInsert, g.limited = nil, false
	c = runGoogle(t, m)
	if !slices.Equal(c.Written, uids("123456", "456789")) || len(g.events[familyCalendar]) != 2 {
		t.Fatalf("retry: history %v, calendar %v", c.Written, g.events)
	}

	m.league.during = func() {
		c, _ := m.store.GetGoogle(t.Context(), memberEmail)
		if err := m.store.UpdateGoogleDailySwitch(t.Context(), c, false, c.CalendarID); err != nil {
			t.Errorf("switch off: %v", err)
		}
	}
	googleGames(t, m, game{id: "567890"})
	calls := g.calls
	c = runGoogle(t, m)
	if c.Daily || c.Written == nil || g.calls != calls || len(g.events[familyCalendar]) != 2 {
		t.Fatal("switching off during the update allowed a stale provider write or save")
	}
	feeds := len(m.league.feeds)
	runGoogle(t, m)
	if len(m.league.feeds) != feeds || g.calls != calls {
		t.Fatal("disabled update called the backend or Google")
	}
}

func TestDailySkipsTeamsForgottenWhileFollowing(t *testing.T) {
	m, g := googleDailySite(t)
	googleGames(t, m, game{id: "123456"})
	m.league.follow = map[string]string{"111111": "311111"}
	m.league.during = func() {
		was, err := m.store.GetSoccer(t.Context(), memberEmail)
		if err == nil {
			err = m.store.UpdateSoccerTeams(t.Context(), was, nil)
		}
		if err != nil {
			t.Error(err)
		}
	}
	calls := g.calls
	if c := runGoogle(t, m); g.calls != calls || len(c.Written) != 0 || len(g.events[familyCalendar]) != 0 {
		t.Fatal("a stale Team selection wrote to Google")
	}
}
