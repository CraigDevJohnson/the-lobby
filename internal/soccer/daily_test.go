package soccer

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// newDailySite is the iCloud site with the Member connected, ROVERS in My
// calendar and the daily update on for the Family calendar. The league has
// posted game 101, and Apple hasn't been called since the switch.
func newDailySite(t *testing.T) (*memberSite, *fakeICloud) {
	t.Helper()
	m, f := newICloudSite(t)
	_ = m.store.PutSoccer(context.Background(), store.Soccer{Email: memberEmail, Teams: []store.RememberedTeam{{Name: "ROVERS", TeamIDs: []string{"111111"}}}})
	m.icloudConnect(t, applePassword)
	if code, body := m.icloudDaily(t, `{"on":true,"calendar_id":"`+f.url+`/123/calendars/family/"}`); code != http.StatusOK {
		t.Fatalf("turn the daily update on: %d %s", code, body)
	}
	m.posts("101")
	f.calls = 0
	return m, f
}

// posts is the games the league lists, none of them played yet: the feed's
// calendar holds them all, and so does the file of upcoming games.
func (m *memberSite) posts(ids ...string) {
	m.league.feed, m.league.file = gameFile(ids...), gameFile(ids...)
}

// another gives a second Member, later in the run, the same Teams and the
// same iCloud connection, with the daily update on.
func (m *memberSite) another(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	const email = "zoe@example.com"
	c, _ := m.store.GetICloud(ctx, memberEmail)
	key, _ := m.tool.ICloudKey(ctx)
	sealed, err := google.Seal(key, sealedFor(email), applePassword)
	if err != nil {
		t.Fatal(err)
	}
	c.Email, c.Password, c.Written = email, sealed, nil
	_ = m.store.PutICloud(ctx, c)
	_ = m.store.PutMember(ctx, store.Member{Email: email, Tools: []string{Grant}})
	_ = m.store.PutSoccer(ctx, store.Soccer{Email: email, Teams: []store.RememberedTeam{{Name: "ROVERS", TeamIDs: []string{"111111"}}}})
}

// run runs the daily update, which never fails, and answers the Member's
// connection as it then is.
func (m *memberSite) run(t *testing.T) store.ICloud {
	t.Helper()
	if err := m.tool.DailyICloud(context.Background()); err != nil {
		t.Fatalf("the daily update failed: %v", err)
	}
	c, err := m.store.GetICloud(context.Background(), memberEmail)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// uids are the UIDs of those games.
func uids(ids ...string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = "game-" + id + "@soccer.example"
	}
	return out
}

// A game the league posts is added with a create-only PUT and its UID kept,
// so a run with nothing new asks Apple nothing.
func TestDailyAddsANewGame(t *testing.T) {
	m, f := newDailySite(t)
	if c := m.run(t); !c.Daily || !slices.Equal(c.Written, uids("101")) {
		t.Fatalf("kept %+v", c)
	}
	one := f.events["family"]["game-101@soccer.example.ics"]
	// Apple was asked for the calendars, then given the one game.
	if !strings.Contains(one, "SUMMARY:game 101") || !strings.Contains(one, "TZID:America/Denver") || f.calls != 2 || f.replaced != 0 {
		t.Fatalf("iCloud holds %q after %d calls, %d of them replacing", f.events, f.calls, f.replaced)
	}
	// The games asked for are those of the Remembered teams that haven't
	// started.
	if len(m.league.asked) != 1 || !strings.Contains(m.league.asked[0], `"team_ids":["111111"]`) || !strings.Contains(m.league.asked[0], `"since":"20`) {
		t.Fatalf("backend was asked %q", m.league.asked)
	}

	m.posts("101", "102")
	f.calls = 0
	if c := m.run(t); !slices.Equal(c.Written, uids("101", "102")) || len(f.events["family"]) != 2 || f.calls != 2 || f.replaced != 0 {
		t.Fatalf("a second game: kept %v, iCloud holds %q after %d calls", c.Written, f.events, f.calls)
	}
	f.calls = 0
	if m.run(t); f.calls != 0 {
		t.Fatalf("nothing new, and Apple was called %d times", f.calls)
	}
}

// A game the Member deleted from their calendar stays deleted: its UID is
// on the list, so Apple is never asked about it again.
func TestDailyLeavesADeletedGameDeleted(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	delete(f.events["family"], "game-101@soccer.example.ics")
	f.calls = 0
	if c := m.run(t); f.calls != 0 || len(f.events["family"]) != 0 || !slices.Equal(c.Written, uids("101")) {
		t.Fatalf("kept %v, iCloud holds %q after %d calls", c.Written, f.events, f.calls)
	}
}

// A game already in the calendar, as after a press, is left as the Member
// has it: Apple's 412 is recorded, and the game is never PUT again.
func TestDailyNeverRewritesAGame(t *testing.T) {
	m, f := newDailySite(t)
	f.events["family"] = map[string]string{"game-101@soccer.example.ics": "the Member's own notes"}
	c := m.run(t)
	if !slices.Equal(c.Written, uids("101")) || f.events["family"]["game-101@soccer.example.ics"] != "the Member's own notes" || f.calls != 2 || f.replaced != 0 {
		t.Fatalf("kept %v, iCloud holds %q after %d calls", c.Written, f.events, f.calls)
	}
	if !strings.Contains(m.logs.String(), `"added":0,"already":1`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
	f.calls = 0
	if m.run(t); f.calls != 0 {
		t.Fatalf("Apple was asked about the game again: %d calls", f.calls)
	}
}

// The Session-over notice in the feed's calendar is written once, though no
// game is left to play. A played game is not added.
func TestDailyAddsTheSessionOverNoticeOnce(t *testing.T) {
	m, f := newDailySite(t)
	const notice = "session-over-111111@soccer.example"
	m.league.feed = strings.Replace(gameFile("101"), "END:VCALENDAR", "BEGIN:VEVENT\r\nUID:"+notice+"\r\nSUMMARY:ROVERS: this Session is over\r\nEND:VEVENT\r\nEND:VCALENDAR", 1)
	m.league.file = gameFile()
	c := m.run(t)
	if !slices.Equal(c.Written, []string{notice}) || len(f.events["family"]) != 1 || !strings.Contains(f.events["family"][notice+".ics"], "this Session is over") {
		t.Fatalf("kept %v, iCloud holds %q", c.Written, f.events)
	}
	f.calls = 0
	if m.run(t); f.calls != 0 {
		t.Fatalf("the notice was written again: %d calls", f.calls)
	}

	// The backend leaves the notice out on a day it can't check the Team's
	// division. One the Member deleted doesn't come back when it returns.
	with := m.league.feed
	delete(f.events["family"], notice+".ics")
	m.league.feed = gameFile("101")
	m.run(t)
	m.league.feed = with
	if c := m.run(t); f.calls != 0 || len(f.events["family"]) != 0 || !slices.Equal(c.Written, []string{notice}) {
		t.Fatalf("the deleted notice came back: kept %v, iCloud holds %d events after %d calls", c.Written, len(f.events["family"]), f.calls)
	}
}

// A Team the backend moves on to its next Session is saved, as on a Member
// link fetch, and the games asked for are the new Team ID's too.
func TestDailyFollowsATeam(t *testing.T) {
	m, _ := newDailySite(t)
	m.league.follow = map[string]string{"111111": "311111"}
	m.run(t)
	s, _ := m.store.GetSoccer(context.Background(), memberEmail)
	if len(s.Teams) != 1 || !slices.Equal(s.Teams[0].TeamIDs, []string{"311111", "111111"}) {
		t.Fatalf("Remembered teams = %+v", s.Teams)
	}
	if m.league.feeds[0] != `{"teams":[["111111"]]}` || !strings.Contains(m.league.asked[0], `"team_ids":["311111","111111"]`) {
		t.Fatalf("backend was sent %q, then asked %q", m.league.feeds, m.league.asked)
	}
}

// The switch off, the grant taken away, or no Teams in My calendar: the
// Member is passed over, and neither the backend nor Apple is called.
func TestDailySkipsWithoutCallingApple(t *testing.T) {
	ctx := context.Background()
	for name, change := range map[string]func(*memberSite){
		"switch off": func(m *memberSite) { m.icloudDaily(t, `{"on":false}`) },
		"no grant": func(m *memberSite) {
			_ = m.store.PutMember(ctx, store.Member{Email: memberEmail, Tools: []string{"minecraft"}})
		},
		"no Teams": func(m *memberSite) { _ = m.store.PutSoccer(ctx, store.Soccer{Email: memberEmail}) },
	} {
		m, f := newDailySite(t)
		change(m)
		if c := m.run(t); f.calls != 0 || len(m.league.feeds) != 0 || c.Written != nil {
			t.Errorf("%s: %d calls to Apple, %d to the backend, kept %v", name, f.calls, len(m.league.feeds), c.Written)
		}
	}
}

// A saved calendar Apple no longer lists is never written to. The Member is
// skipped, and the switch stays on.
func TestDailySkipsACalendarAppleNoLongerLists(t *testing.T) {
	m, f := newDailySite(t)
	f.gone = true
	if c := m.run(t); len(f.events) != 0 || c.Written != nil || !c.Daily || f.calls != 1 {
		t.Fatalf("kept %+v, iCloud holds %q after %d calls", c, f.events, f.calls)
	}
}

// Apple refusing the password, or a password that no longer opens, turns
// the switch off and changes nothing else: the page's own check drops the
// connection on the Member's next visit.
func TestDailyTurnsTheSwitchOffWhenThePasswordIsRefused(t *testing.T) {
	m, f := newDailySite(t)
	before := m.run(t)
	m.posts("101", "102")
	f.revoked = true
	c := m.run(t)
	if c.Daily || c.Password == "" || c.Password != before.Password || c.CalendarID != before.CalendarID || !slices.Equal(c.Written, uids("101")) || len(f.events["family"]) != 1 {
		t.Fatalf("kept %+v, iCloud holds %q", c, f.events)
	}
	if !strings.Contains(m.logs.String(), `"switched_off":1`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
	f.calls = 0
	if m.run(t); f.calls != 0 {
		t.Fatalf("switched off, and Apple was called %d times", f.calls)
	}

	f.revoked = false
	m.icloudDaily(t, `{"on":true,"calendar_id":"`+before.CalendarID+`"}`)
	m.tool.ICloudKey = func(context.Context) ([]byte, error) { return make([]byte, 32), nil }
	if c := m.run(t); c.Daily || c.Password != before.Password || f.calls != 0 {
		t.Fatalf("a password that no longer opens: kept %+v after %d calls", c, f.calls)
	}
}

// Apple refusing the password part way through, on a game, turns the switch
// off too, and the games added before it are kept.
func TestDailyTurnsTheSwitchOffWhenAGameIsRefusedThePassword(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102", "103")
	f.refuses = map[string]int{"game-102@soccer.example.ics": http.StatusUnauthorized}
	c := m.run(t)
	if c.Daily || !slices.Equal(c.Written, uids("101")) || len(f.events["family"]) != 1 {
		t.Fatalf("kept %+v, iCloud holds %q", c, f.events)
	}
	if !strings.Contains(m.logs.String(), `"added":1,"already":0,"failed":0,"skipped":0,"switched_off":1,"stopped":""`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
}

// Apple saying slow down stops the whole run until tomorrow: the games
// added before it are kept, and the next Member isn't started.
func TestDailyStopsWhenAppleSaysSlowDown(t *testing.T) {
	m, f := newDailySite(t)
	m.another(t)
	m.posts("101", "102", "103")
	f.refuses = map[string]int{"game-102@soccer.example.ics": http.StatusTooManyRequests}
	c := m.run(t)
	if !slices.Equal(c.Written, uids("101")) || len(f.events["family"]) != 1 || len(m.league.feeds) != 1 || !c.Daily {
		t.Fatalf("kept %+v, iCloud holds %q, %d Members started", c, f.events, len(m.league.feeds))
	}
	if !strings.Contains(m.logs.String(), `"members":2,"added":1,"already":0,"failed":0,"skipped":0,"switched_off":0,"stopped":"Apple said to slow down"`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
}

// Apple saying slow down as the calendars are listed stops the run before
// any game is sent, and the switch stays on.
func TestDailyStopsWhenAppleSaysSlowDownListingCalendars(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101")
	f.busy = true
	c := m.run(t)
	if !c.Daily || len(c.Written) != 0 || len(f.events["family"]) != 0 {
		t.Fatalf("kept %+v, iCloud holds %q", c, f.events)
	}
	if !strings.Contains(m.logs.String(), `"added":0,"already":0,"failed":0,"skipped":0,"switched_off":0,"stopped":"Apple said to slow down"`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
}

// The league down, or the backend not answering, stops the whole run with
// nothing written, and the run still ends successfully, so Lambda doesn't
// run it again.
func TestDailyStopsWhenTheBackendIsDown(t *testing.T) {
	m, f := newDailySite(t)
	m.another(t)
	m.league.status = http.StatusServiceUnavailable
	if c := m.run(t); f.calls != 0 || c.Written != nil || len(m.league.feeds) != 1 || len(m.league.asked) != 0 {
		t.Fatalf("kept %v after %d calls to Apple, %d Members started", c.Written, f.calls, len(m.league.feeds))
	}
	if !strings.Contains(m.logs.String(), `"stopped":"the backend isn't answering"`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
	// The file of upcoming games failing is the same.
	m.league.status, m.league.noFile = 0, http.StatusServiceUnavailable
	if c := m.run(t); f.calls != 0 || c.Written != nil || len(m.league.feeds) != 2 {
		t.Fatalf("kept %v after %d calls to Apple, %d Members started", c.Written, f.calls, len(m.league.feeds)-1)
	}
}

// The league knowing none of the Teams any more is no games, not a failure:
// the run goes on to the next Member.
func TestDailyTakesTeamsTheLeagueForgotAsNoGames(t *testing.T) {
	m, f := newDailySite(t)
	m.another(t)
	m.league.feed, m.league.noFile = gameFile(), http.StatusNotFound
	if m.run(t); f.calls != 0 || len(m.league.feeds) != 2 || !strings.Contains(m.logs.String(), `"skipped":0,"switched_off":0,"stopped":""`) {
		t.Fatalf("%d calls to Apple, %d Members started, logs:\n%s", f.calls, len(m.league.feeds), m.logs)
	}
}

// A run that is out of time stops, still keeps the games it added, and ends
// successfully.
func TestDailyOutOfTimeKeepsWhatItAdded(t *testing.T) {
	m, f := newDailySite(t)
	m.tool.Store = unhurried{m.store}
	m.posts("101", "102", "103")
	ctx, cancel := context.WithCancel(context.Background())
	f.refuses = map[string]int{"game-102@soccer.example.ics": http.StatusForbidden}
	f.onRefuse = cancel
	if err := m.tool.DailyICloud(ctx); err != nil {
		t.Fatalf("the daily update failed: %v", err)
	}
	c, _ := m.store.GetICloud(context.Background(), memberEmail)
	if !slices.Equal(c.Written, uids("101")) || len(f.events["family"]) != 1 || !strings.Contains(m.logs.String(), `"stopped":"out of time"`) {
		t.Fatalf("kept %v, iCloud holds %q, logs:\n%s", c.Written, f.events, m.logs)
	}
}

// unhurried is a store that, like DynamoDB, saves nothing once the context
// it is given is done.
type unhurried struct{ *store.Memory }

func (u unhurried) UpdateICloudDaily(ctx context.Context, c store.ICloud) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return u.Memory.UpdateICloudDaily(ctx, c)
}

// Where Add to iCloud Calendar isn't set up, a run does nothing and ends
// successfully.
func TestDailyWhereICloudIsNotSetUp(t *testing.T) {
	m := newMemberSite(t)
	if err := m.tool.DailyICloud(context.Background()); err != nil || len(m.league.feeds) != 0 {
		t.Fatalf("%v, %d calls to the backend", err, len(m.league.feeds))
	}
}

// The log carries counts and errors only: no Member's address, Apple
// Account, calendar address or UID, whatever happened.
func TestDailyLogsCountsOnly(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	// Apple refuses one game on its own: logged with its error, tried again
	// tomorrow.
	f.refuses = map[string]int{"game-102@soccer.example.ics": http.StatusForbidden}
	if c := m.run(t); !slices.Equal(c.Written, uids("101")) {
		t.Fatalf("kept %v", c.Written)
	}
	m.league.status = http.StatusServiceUnavailable
	m.run(t)
	m.league.status, f.revoked = 0, true
	m.run(t)
	logs := m.logs.String()
	for _, want := range []string{
		`"msg":"soccer: daily iCloud update","members":1,"added":1,"already":0,"failed":1,"skipped":0,"switched_off":0,"stopped":""`,
		`"err":"icloud: status 403"`,
		`"err":"the backend answered 503 Service Unavailable"`,
		`"switched_off":1`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %s", want)
		}
	}
	for _, never := range []string{"@", "/123/", "game-10", applePassword} {
		if strings.Contains(logs, never) {
			t.Errorf("logs hold %q", never)
		}
	}
	if t.Failed() {
		t.Logf("logs:\n%s", logs)
	}
}

// A game the feed's calendar no longer holds, as when its Team left My
// calendar, is dropped from the list, which so stays small. That alone
// needs no call to Apple.
func TestDailyDropsGamesTheFeedNoLongerHolds(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	m.run(t)
	m.posts("102")
	f.calls = 0
	if c := m.run(t); !slices.Equal(c.Written, uids("102")) || f.calls != 0 || len(f.events["family"]) != 2 {
		t.Fatalf("kept %v, iCloud holds %q after %d calls", c.Written, f.events, f.calls)
	}
}
