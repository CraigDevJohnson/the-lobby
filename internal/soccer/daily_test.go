package soccer

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/invite"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// newDailySite is the iCloud site with the Member connected, ROVERS in My
// calendar and the daily update on for the Family calendar. The league has
// posted game 101, Apple hasn't been called since the switch, and the site
// sends email, which m.mail keeps.
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
	m.mail = &outbox{}
	m.tool.Email = m.mail.send
	return m, f
}

// outbox is the site's email sender in a test: it keeps each email it is
// given, or fails with err, and counts both.
type outbox struct {
	sent   []invite.Email
	tried  int
	err    error
	during func(ctx context.Context) // runs as an email is sent, when set
}

func (o *outbox) send(ctx context.Context, e invite.Email) error {
	o.tried++
	if o.during != nil {
		o.during(ctx)
	}
	if o.err != nil {
		return o.err
	}
	o.sent = append(o.sent, e)
	return nil
}

// posts is the games the league lists, none of them played yet: the feed's
// calendar holds them all, and so does the file of upcoming games.
func (m *memberSite) posts(ids ...string) {
	m.league.feed, m.league.file = gameFile(ids...), gameFile(ids...)
}

// lists is the games the league lists, as posts is, each with a start and a
// field of its own.
func (m *memberSite) lists(games ...game) {
	m.league.feed, m.league.file = gamesFile(games...), gamesFile(games...)
}

// asked forgets what Apple has been asked so far.
func (f *fakeICloud) asked() {
	f.calls, f.puts = 0, nil
}

// uid is the UID of that game, and its file's name in an iCloud calendar.
func uid(id string) string { return "game-" + id + "@soccer.example" }

func icsOf(id string) string { return uid(id) + ".ics" }

// The game the league moves in these tests: a day later, on another field.
var (
	moved101  = game{id: "101", start: "20261122T183000", field: "Field 1"}
	usualTime = store.GameTime{Start: usualStart, Field: usualField}
)

const stopLine = `To stop these emails, turn off "Add new games every day": open https://lobby.example/soccer, sign in, find a Team, choose Apple Calendar, then Add to iCloud Calendar.` + "\n"

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
	c.Email, c.Password, c.Written, c.Times = email, sealed, nil, nil
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
// with its start and field, so a run with nothing new asks Apple nothing.
func TestDailyAddsANewGame(t *testing.T) {
	m, f := newDailySite(t)
	if c := m.run(t); !c.Daily || !slices.Equal(c.Written, uids("101")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): usualTime}) {
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
	if c := m.run(t); !slices.Equal(c.Written, uids("101", "102")) || len(c.Times) != 2 || len(f.events["family"]) != 2 || f.calls != 2 || f.replaced != 0 {
		t.Fatalf("a second game: kept %v, iCloud holds %q after %d calls", c.Written, f.events, f.calls)
	}
	f.calls = 0
	if m.run(t); f.calls != 0 {
		t.Fatalf("nothing new, and Apple was called %d times", f.calls)
	}
	if len(m.mail.sent) != 0 {
		t.Fatalf("no game moved, and %d emails were sent", len(m.mail.sent))
	}
}

// A game the Member deleted from their calendar stays deleted: its UID is
// on the list, so while the league leaves the game where it is, Apple isn't
// asked about it.
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
// has it: Apple's 412 is recorded with where the league has the game, and
// the game isn't PUT again while the league leaves it there.
func TestDailyLeavesAGameThatHasNotMoved(t *testing.T) {
	m, f := newDailySite(t)
	f.events["family"] = map[string]string{"game-101@soccer.example.ics": "the Member's own notes"}
	c := m.run(t)
	if !slices.Equal(c.Written, uids("101")) || c.Times[uid("101")] != usualTime || f.events["family"]["game-101@soccer.example.ics"] != "the Member's own notes" || f.calls != 2 || f.replaced != 0 {
		t.Fatalf("kept %v, iCloud holds %q after %d calls", c.Written, f.events, f.calls)
	}
	if !strings.Contains(m.logs.String(), `"added":0,"already":1`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
	f.calls = 0
	if m.run(t); f.calls != 0 || len(m.mail.sent) != 0 {
		t.Fatalf("Apple was asked about the game again: %d calls, %d emails", f.calls, len(m.mail.sent))
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
// skipped, and the kept switch stays on.
func TestDailySkipsACalendarAppleNoLongerLists(t *testing.T) {
	m, f := newDailySite(t)
	f.gone = true
	if c := m.run(t); len(f.events) != 0 || c.Written != nil || !c.Daily || f.calls != 1 {
		t.Fatalf("kept %+v, iCloud holds %q after %d calls", c, f.events, f.calls)
	}
	// The page doesn't show the switch on beside another calendar: it reads
	// off, and turning it on saves the calendar then showing.
	home := f.url + "/123/calendars/home/"
	if a := m.icloudStatus(t); a["daily"] != nil || a["calendar_id"] != home {
		t.Fatalf("status with the saved calendar gone: %v", a)
	}
	m.icloudDaily(t, `{"on":true,"calendar_id":"`+home+`"}`)
	if a := m.icloudStatus(t); a["daily"] != true || a["calendar_id"] != home {
		t.Fatalf("status after turning it on again: %v", a)
	}
	if c := m.run(t); c.CalendarID != home || len(f.events["home"]) != 1 {
		t.Fatalf("kept %+v, iCloud holds %q", c, f.events)
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
		`"msg":"soccer: daily iCloud update","members":1,"added":1,"already":0,"failed":1,"skipped":0,"switched_off":0,"stopped":"","moved":0,"missing":0,"emailed":0}`,
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

// counting is a store that counts the daily update's saves.
type counting struct {
	*store.Memory
	saves int
}

func (c *counting) UpdateICloudDaily(ctx context.Context, i store.ICloud) error {
	c.saves++
	return c.Memory.UpdateICloudDaily(ctx, i)
}

// moves is what Apple gets for each moved game that is in the calendar: the
// question whether it holds the game, then the game.
func moves(ids ...string) []string {
	var out []string
	for _, id := range ids {
		out = append(out, "ask "+icsOf(id), "write "+icsOf(id))
	}
	return out
}

// A game the league moves is replaced whole in the calendar, once Apple has
// said the calendar holds it, its new start and field are kept, and the
// Member is emailed what changed, once. The next run does nothing.
func TestDailyMovesAGameTheLeagueMoved(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	m.run(t)
	// What the Member changed on the game in iCloud is replaced with it.
	f.events["family"][icsOf("101")] += "the Member's own notes"
	m.lists(moved101, game{id: "102"})
	saves := &counting{Memory: m.store}
	m.tool.Store = saves
	f.asked()

	c := m.run(t)
	// Apple was asked for the calendars and whether it holds the game, then
	// sent the one game.
	if f.calls != 3 || !slices.Equal(f.puts, moves("101")) {
		t.Fatalf("Apple got %d calls, PUTs %q", f.calls, f.puts)
	}
	if one := f.events["family"][icsOf("101")]; !strings.Contains(one, "DTSTART;TZID=America/Denver:20261122T183000") || !strings.Contains(one, "SUMMARY:game 101 - Field 1") || strings.Contains(one, "the Member's own notes") || len(f.events["family"]) != 2 {
		t.Fatalf("iCloud holds %q", f.events)
	}
	if !c.Daily || !slices.Equal(c.Written, uids("101", "102")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): moved101.at(), uid("102"): usualTime}) || saves.saves != 1 {
		t.Fatalf("kept %+v after %d saves", c, saves.saves)
	}
	want := invite.Email{To: memberEmail, Subject: "A soccer game moved", Body: "The league moved a game that is in your iCloud calendar, so The Lobby moved it there too.\n" +
		"\n" +
		"game 101\n" +
		"  Was: Sat, Nov 21 at 7:15 PM, Field 3\n" +
		"  Now: Sun, Nov 22 at 6:30 PM, Field 1\n" +
		"\n" +
		"Anything you changed on a moved game in iCloud, such as notes or alerts, was replaced.\n" +
		"\n" + stopLine}
	if len(m.mail.sent) != 1 || m.mail.sent[0] != want {
		t.Fatalf("emails sent: %+v\nwant one: %+v", m.mail.sent, want)
	}
	if !strings.Contains(m.logs.String(), `"added":0,"already":0,"failed":0,"skipped":0,"switched_off":0,"stopped":"","moved":1,"missing":0,"emailed":1}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}

	f.asked()
	if c := m.run(t); f.calls != 0 || len(m.mail.sent) != 1 || saves.saves != 1 || c.Times[uid("101")] != moved101.at() {
		t.Fatalf("the next run: %d calls to Apple, %d emails, %d saves, kept %+v", f.calls, len(m.mail.sent), saves.saves, c)
	}
}

// A game moved to another field at the same time has moved. A game whose
// title alone changed has not.
func TestDailyCountsAChangeOfFieldAlone(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	m.lists(game{id: "101", field: "Field 1"})
	f.asked()
	c := m.run(t)
	if !slices.Equal(f.puts, moves("101")) || c.Times[uid("101")] != (store.GameTime{Start: usualStart, Field: "Field 1"}) {
		t.Fatalf("PUTs %q, kept %+v", f.puts, c.Times)
	}
	if len(m.mail.sent) != 1 || !strings.Contains(m.mail.sent[0].Body, "game 101\n  Was: Sat, Nov 21 at 7:15 PM, Field 3\n  Now: Sat, Nov 21 at 7:15 PM, Field 1\n") {
		t.Fatalf("emails sent: %+v", m.mail.sent)
	}

	renamed := strings.Replace(gamesFile(game{id: "101", field: "Field 1"}), "SUMMARY:game 101", "SUMMARY:OWLS vs ROVERS", 1)
	m.league.feed, m.league.file = renamed, renamed
	f.asked()
	if m.run(t); f.calls != 0 || len(m.mail.sent) != 1 {
		t.Fatalf("a new title alone: %d calls to Apple, %d emails", f.calls, len(m.mail.sent))
	}
}

// A game written before times were kept has none to compare: the league's
// start and field are noted and saved, Apple isn't asked about the game and
// nobody is emailed, even if the league had moved it meanwhile. A later move
// is acted on.
func TestDailyOnlyNotesAGameWrittenBeforeTimesWereKept(t *testing.T) {
	m, f := newDailySite(t)
	ctx := context.Background()
	m.posts("101", "102")
	m.run(t)
	// The connection as it was kept before: the list, and no times.
	before, _ := m.store.GetICloud(ctx, memberEmail)
	before.Times = nil
	_ = m.store.PutICloud(ctx, before)
	m.lists(moved101, game{id: "102"})
	f.asked()

	c := m.run(t)
	if f.calls != 0 || len(m.mail.sent) != 0 {
		t.Fatalf("the first run: %d calls to Apple, %d emails", f.calls, len(m.mail.sent))
	}
	if !slices.Equal(c.Written, uids("101", "102")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): moved101.at(), uid("102"): usualTime}) {
		t.Fatalf("the first run kept %+v", c)
	}
	if !strings.Contains(f.events["family"][icsOf("101")], usualStart) {
		t.Fatalf("the game was rewritten: %q", f.events["family"][icsOf("101")])
	}

	again := game{id: "101", start: "20261123T200000", field: "Field 2"}
	m.lists(again, game{id: "102"})
	c = m.run(t)
	if !slices.Equal(f.puts, moves("101")) || c.Times[uid("101")] != again.at() || !strings.Contains(f.events["family"][icsOf("101")], "20261123T200000") {
		t.Fatalf("a later move: PUTs %q, kept %+v", f.puts, c.Times)
	}
	if len(m.mail.sent) != 1 || !strings.Contains(m.mail.sent[0].Body, "game 101\n  Was: Sun, Nov 22 at 6:30 PM, Field 1\n  Now: Mon, Nov 23 at 8:00 PM, Field 2\n") {
		t.Fatalf("emails sent: %+v", m.mail.sent)
	}
}

// A moved game that isn't in the calendar, because the Member deleted it,
// is not put back: Apple says the calendar doesn't hold it, so it is never
// sent. The email says the game wasn't found, and the new time is kept, so
// the move isn't acted on again.
func TestDailyLeavesOutAMovedGameThatIsNotInTheCalendar(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	delete(f.events["family"], icsOf("101"))
	m.lists(moved101)
	f.asked()

	c := m.run(t)
	if f.calls != 2 || !slices.Equal(f.puts, []string{"ask " + icsOf("101")}) || len(f.events["family"]) != 0 {
		t.Fatalf("Apple got %d calls, PUTs %q, and holds %q", f.calls, f.puts, f.events)
	}
	if !slices.Equal(c.Written, uids("101")) || c.Times[uid("101")] != moved101.at() {
		t.Fatalf("kept %+v", c)
	}
	want := invite.Email{To: memberEmail, Subject: "A soccer game moved", Body: "The league moved a game, but it wasn't found in your iCloud calendar, so nothing was changed there:\n" +
		"\n" +
		"game 101\n" +
		"  Was: Sat, Nov 21 at 7:15 PM, Field 3\n" +
		"  Now: Sun, Nov 22 at 6:30 PM, Field 1\n" +
		"\n" + stopLine}
	if len(m.mail.sent) != 1 || m.mail.sent[0] != want {
		t.Fatalf("emails sent: %+v\nwant one: %+v", m.mail.sent, want)
	}
	// The log says how Apple said the game isn't there, and nothing of the
	// game.
	if logs := m.logs.String(); !strings.Contains(logs, `"stopped":"","moved":0,"missing":1,"emailed":1}`) ||
		!strings.Contains(logs, `"msg":"soccer: daily iCloud update: a moved game isn't in the calendar","apple":"404"}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}

	f.asked()
	if m.run(t); f.calls != 0 || len(m.mail.sent) != 1 || len(f.events["family"]) != 0 {
		t.Fatalf("the next run: %d calls to Apple, %d emails, iCloud holds %q", f.calls, len(m.mail.sent), f.events)
	}
}

// One email carries both: the game that was moved, and the one that wasn't
// found.
func TestDailySendsOneEmailForAMembersMoves(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	m.run(t)
	delete(f.events["family"], icsOf("102"))
	m.lists(moved101, game{id: "102", start: "20261128T181500"})
	m.run(t)
	if len(m.mail.sent) != 1 {
		t.Fatalf("emails sent: %+v", m.mail.sent)
	}
	want := "The league moved a game that is in your iCloud calendar, so The Lobby moved it there too.\n" +
		"\n" +
		"game 101\n" +
		"  Was: Sat, Nov 21 at 7:15 PM, Field 3\n" +
		"  Now: Sun, Nov 22 at 6:30 PM, Field 1\n" +
		"\n" +
		"This game moved too, but it wasn't found in your iCloud calendar, so nothing was changed there:\n" +
		"\n" +
		"game 102\n" +
		"  Was: Sat, Nov 21 at 7:15 PM, Field 3\n" +
		"  Now: Sat, Nov 28 at 6:15 PM, Field 3\n" +
		"\n" +
		"Anything you changed on a moved game in iCloud, such as notes or alerts, was replaced.\n" +
		"\n" + stopLine
	if got := m.mail.sent[0].Body; got != want {
		t.Fatalf("body:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(m.logs.String(), `"moved":1,"missing":1,"emailed":1}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
}

// A Session-over notice is never compared: when its day and its sentence
// change, Apple isn't asked, nobody is emailed and no time is kept for it.
func TestDailyNeverComparesASessionOverNotice(t *testing.T) {
	m, f := newDailySite(t)
	const notice = "session-over-111111@soccer.example"
	with := func(day, sentence string) string {
		return strings.Replace(gameFile("101"), "END:VCALENDAR", "BEGIN:VEVENT\r\nUID:"+notice+"\r\nSUMMARY:ROVERS: this Session is over\r\n"+
			"DTSTART;VALUE=DATE:"+day+"\r\nDESCRIPTION:"+sentence+"\r\nEND:VEVENT\r\nEND:VCALENDAR", 1)
	}
	m.league.feed = with("20261220", "ROVERS has played its last game of this Session.")
	c := m.run(t)
	if !slices.Equal(c.Written, append(uids("101"), notice)) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): usualTime}) || !strings.Contains(f.events["family"][notice+".ics"], "DTSTART;VALUE=DATE:20261220") {
		t.Fatalf("kept %+v, iCloud holds %q", c, f.events)
	}

	m.league.feed = with("20261227", "ROVERS plays no more games this Session.")
	f.asked()
	c = m.run(t)
	if f.calls != 0 || len(m.mail.sent) != 0 {
		t.Fatalf("the notice's day changed: %d calls to Apple, %d emails", f.calls, len(m.mail.sent))
	}
	if !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): usualTime}) || !strings.Contains(f.events["family"][notice+".ics"], "DTSTART;VALUE=DATE:20261220") {
		t.Fatalf("kept %+v, iCloud holds %q", c.Times, f.events)
	}
}

// A day with nothing new and nothing moved: Apple isn't called, nothing is
// saved and nothing is sent.
func TestDailyDoesNothingOnAQuietDay(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	before := m.run(t)
	saves := &counting{Memory: m.store}
	m.tool.Store = saves
	f.asked()
	c := m.run(t)
	if f.calls != 0 || saves.saves != 0 || m.mail.tried != 0 {
		t.Fatalf("%d calls to Apple, %d saves, %d emails", f.calls, saves.saves, m.mail.tried)
	}
	if !slices.Equal(c.Written, before.Written) || !maps.Equal(c.Times, before.Times) || len(c.Times) != 2 {
		t.Fatalf("kept %+v, before %+v", c, before)
	}
	if !strings.Contains(m.logs.String(), `"members":1,"added":0,"already":0,"failed":0,"skipped":0,"switched_off":0,"stopped":"","moved":0,"missing":0,"emailed":0}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
}

// New games and a moved game for one Member: their calendars are listed
// once, the new game is added and the moved one replaced.
func TestDailyListsTheCalendarsOnceForNewAndMovedGames(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	m.lists(moved101, game{id: "103"})
	f.asked()
	c := m.run(t)
	if f.calls != 4 || !slices.Equal(f.puts, append([]string{"create " + icsOf("103")}, moves("101")...)) {
		t.Fatalf("Apple got %d calls, PUTs %q", f.calls, f.puts)
	}
	if !slices.Equal(c.Written, uids("101", "103")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): moved101.at(), uid("103"): usualTime}) || len(m.mail.sent) != 1 {
		t.Fatalf("kept %+v, %d emails", c, len(m.mail.sent))
	}
	if !strings.Contains(m.logs.String(), `"added":1,"already":0,"failed":0,"skipped":0,"switched_off":0,"stopped":"","moved":1,"missing":0,"emailed":1}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
}

// refusing is a store whose save for the daily update answers err.
type refusing struct {
	*store.Memory
	err error
}

func (r refusing) UpdateICloudDaily(context.Context, store.ICloud) error { return r.err }

// A save the store refuses, because the Member changed their connection
// meanwhile or the table failed, sends no email: the old time is still
// kept, so the next run sees the same move, and an email now would be sent
// again then.
func TestDailySendsNoEmailWhenTheSaveIsRefused(t *testing.T) {
	for name, refusal := range map[string]error{"the connection changed": store.ErrNotFound, "the table failed": errors.New("update icloud: throttled")} {
		m, f := newDailySite(t)
		m.run(t)
		m.lists(moved101)
		m.tool.Store = refusing{m.store, refusal}
		f.asked()
		c := m.run(t)
		if !slices.Equal(f.puts, moves("101")) || m.mail.tried != 0 || c.Times[uid("101")] != usualTime {
			t.Fatalf("%s: PUTs %q, %d emails, kept %+v", name, f.puts, m.mail.tried, c.Times)
		}
		if !strings.Contains(m.logs.String(), `"moved":1,"missing":0,"emailed":0}`) {
			t.Fatalf("%s: logs:\n%s", name, m.logs)
		}
		// The next run makes the move again, keeps it and emails once.
		m.tool.Store = m.store
		if c := m.run(t); len(m.mail.sent) != 1 || c.Times[uid("101")] != moved101.at() {
			t.Fatalf("%s: the next run sent %d emails and kept %+v", name, len(m.mail.sent), c.Times)
		}
	}
}

// Where the site sends no email, a moved game is still moved and its new
// time kept.
func TestDailyMovesGamesWhereTheSiteSendsNoEmail(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	m.tool.Email = nil
	m.lists(moved101)
	f.asked()
	c := m.run(t)
	if !slices.Equal(f.puts, moves("101")) || !strings.Contains(f.events["family"][icsOf("101")], "20261122T183000") || c.Times[uid("101")] != moved101.at() {
		t.Fatalf("PUTs %q, iCloud holds %q, kept %+v", f.puts, f.events, c.Times)
	}
	if m.mail.tried != 0 || !strings.Contains(m.logs.String(), `"moved":1,"missing":0,"emailed":0}`) {
		t.Fatalf("%d emails tried, logs:\n%s", m.mail.tried, m.logs)
	}
	f.asked()
	if m.run(t); f.calls != 0 {
		t.Fatalf("the next run called Apple %d times", f.calls)
	}
}

// An email that doesn't send is logged by the sender's own error, its stage
// and the server's reply code, with no address, UID, title or link, and is
// not tried again: the move is kept.
func TestDailyLogsAFailedEmailWithoutTheMember(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	m.mail.err = &invite.Error{Stage: "rcpt", Code: 550}
	m.lists(moved101)
	f.asked()
	c := m.run(t)
	if !slices.Equal(f.puts, moves("101")) || c.Times[uid("101")] != moved101.at() || m.mail.tried != 1 || len(m.mail.sent) != 0 {
		t.Fatalf("PUTs %q, kept %+v, %d emails tried", f.puts, c.Times, m.mail.tried)
	}
	logs := m.logs.String()
	for _, want := range []string{`"msg":"soccer: daily iCloud update: the email about moved games wasn't sent","err":"email: rcpt: 550"`, `"moved":1,"missing":0,"emailed":0}`} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %s", want)
		}
	}
	for _, never := range []string{"@", "game-10", "game 101", "/123/", "lobby.example", applePassword} {
		if strings.Contains(logs, never) {
			t.Errorf("logs hold %q", never)
		}
	}
	if t.Failed() {
		t.Logf("logs:\n%s", logs)
	}
	m.mail.err = nil
	f.asked()
	if m.run(t); f.calls != 0 || m.mail.tried != 1 {
		t.Fatalf("the next run: %d calls to Apple, %d emails tried", f.calls, m.mail.tried)
	}
}

// The email is sent inside the run's own time, not time of its own: once
// the run's context ends, so does the one the sender was given.
func TestDailySendsTheEmailInsideTheRunsTime(t *testing.T) {
	m, _ := newDailySite(t)
	m.run(t)
	m.lists(moved101)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var left time.Duration
	var ended error
	m.mail.during = func(sending context.Context) {
		if end, ok := sending.Deadline(); ok {
			left = time.Until(end)
		}
		cancel()
		ended = sending.Err()
	}
	if err := m.tool.DailyICloud(ctx); err != nil {
		t.Fatalf("the daily update failed: %v", err)
	}
	if m.mail.tried != 1 || left <= dailyMoves || left > dailyBudget || ended == nil {
		t.Fatalf("%d emails tried with %v left; after the run's context ended the sender's read %v", m.mail.tried, left, ended)
	}
}

// With less of the run left than a Member's moves and their email need, the
// moves wait for the next run, their kept times unchanged. New games are
// still added.
func TestDailyLeavesMovesForTheNextRunWhenTimeIsShort(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	m.lists(moved101, game{id: "103"})
	f.asked()
	short, cancel := context.WithTimeout(context.Background(), dailyMoves-time.Second)
	defer cancel()
	if err := m.tool.DailyICloud(short); err != nil {
		t.Fatalf("the daily update failed: %v", err)
	}
	c, _ := m.store.GetICloud(context.Background(), memberEmail)
	if !slices.Equal(f.puts, []string{"create " + icsOf("103")}) || !strings.Contains(f.events["family"][icsOf("101")], usualStart) {
		t.Fatalf("PUTs %q, iCloud holds %q", f.puts, f.events)
	}
	if !slices.Equal(c.Written, uids("101", "103")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): usualTime, uid("103"): usualTime}) || m.mail.tried != 0 {
		t.Fatalf("kept %+v, %d emails tried", c, m.mail.tried)
	}
	if !strings.Contains(m.logs.String(), `"added":1,"already":0,"failed":0,"skipped":0,"switched_off":0,"stopped":"","moved":0,"missing":0,"emailed":0}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}

	// With only a moved game and too little time, Apple isn't called at all.
	f.asked()
	short, cancel = context.WithTimeout(context.Background(), dailyMoves-time.Second)
	defer cancel()
	if _ = m.tool.DailyICloud(short); f.calls != 0 {
		t.Fatalf("time is short and nothing is new: %d calls to Apple", f.calls)
	}

	// The next run, with its whole time, makes the move.
	if c := m.run(t); !slices.Equal(f.puts, moves("101")) || c.Times[uid("101")] != moved101.at() || len(m.mail.sent) != 1 {
		t.Fatalf("the next run: PUTs %q, kept %+v, %d emails", f.puts, c.Times, len(m.mail.sent))
	}
}

// The time left is looked at again once a Member's new games are added,
// since adding them takes some of it: here adding one game leaves less than
// the moves need, so the move waits for the next run.
func TestDailyLooksAtTheTimeLeftAgainAfterAdding(t *testing.T) {
	m, f := newDailySite(t)
	m.run(t)
	m.lists(moved101, game{id: "103"})
	f.asked()
	f.slow = 400 * time.Millisecond
	// Enough time for the moves as the Member is started, and too little
	// once the new game is added.
	ctx, cancel := context.WithTimeout(context.Background(), dailyMoves+250*time.Millisecond)
	defer cancel()
	if err := m.tool.DailyICloud(ctx); err != nil {
		t.Fatalf("the daily update failed: %v", err)
	}
	c, _ := m.store.GetICloud(context.Background(), memberEmail)
	if !slices.Equal(f.puts, []string{"create " + icsOf("103")}) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): usualTime, uid("103"): usualTime}) || m.mail.tried != 0 {
		t.Fatalf("PUTs %q, kept %+v, %d emails tried", f.puts, c.Times, m.mail.tried)
	}
}

// Apple refusing the password during a move turns the switch off, as on an
// add. The game moved before it is kept and emailed; the one refused keeps
// its old time.
func TestDailyTurnsTheSwitchOffWhenAMoveIsRefusedThePassword(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	before := m.run(t)
	m.lists(moved101, game{id: "102", field: "Field 2"})
	f.refuses = map[string]int{icsOf("102"): http.StatusUnauthorized}
	f.asked()
	c := m.run(t)
	if c.Daily || c.Password != before.Password || c.CalendarID != before.CalendarID || !slices.Equal(c.Written, uids("101", "102")) {
		t.Fatalf("kept %+v", c)
	}
	if !slices.Equal(f.puts, moves("101", "102")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): moved101.at(), uid("102"): usualTime}) {
		t.Fatalf("PUTs %q, kept %+v", f.puts, c.Times)
	}
	if len(m.mail.sent) != 1 || !strings.Contains(m.mail.sent[0].Body, "game 101\n") || strings.Contains(m.mail.sent[0].Body, "game 102") {
		t.Fatalf("emails sent: %+v", m.mail.sent)
	}
	if !strings.Contains(m.logs.String(), `"switched_off":1,"stopped":"","moved":1,"missing":0,"emailed":1}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
	f.asked()
	if m.run(t); f.calls != 0 {
		t.Fatalf("switched off, and Apple was called %d times", f.calls)
	}

	// Refused on the only moved game: the switch goes off and nobody is
	// emailed, since nothing moved.
	m, f = newDailySite(t)
	m.run(t)
	m.lists(moved101)
	f.refuses = map[string]int{icsOf("101"): http.StatusUnauthorized}
	if c := m.run(t); c.Daily || c.Times[uid("101")] != usualTime || m.mail.tried != 0 {
		t.Fatalf("the only move refused: kept %+v, %d emails tried", c, m.mail.tried)
	}
}

// Apple refusing one moved game on its own counts as failed and keeps the
// game's old time, so tomorrow's run tries again. Apple saying slow down
// during a move stops the run, and the next Member isn't started.
func TestDailyTriesAMoveAppleRefusedAgain(t *testing.T) {
	m, f := newDailySite(t)
	m.another(t)
	m.posts("101", "102")
	m.run(t)
	m.lists(moved101, game{id: "102", field: "Field 2"})
	f.refuses = map[string]int{icsOf("101"): http.StatusForbidden}
	feeds := len(m.league.feeds)
	f.asked()
	c := m.run(t)
	if !maps.Equal(c.Times, map[string]store.GameTime{uid("101"): usualTime, uid("102"): {Start: usualStart, Field: "Field 2"}}) || len(m.league.feeds) != feeds+2 {
		t.Fatalf("kept %+v, %d Members started", c.Times, len(m.league.feeds)-feeds)
	}
	// Each Member's game 102 moved; game 101 failed for both.
	if !strings.Contains(m.logs.String(), `"members":2,"added":0,"already":0,"failed":2,"skipped":0,"switched_off":0,"stopped":"","moved":2,"missing":0,"emailed":2}`) || !strings.Contains(m.logs.String(), `"msg":"soccer: daily iCloud update: move a game","err":"icloud: status 403"`) {
		t.Fatalf("logs:\n%s", m.logs)
	}
	if len(m.mail.sent) != 2 || strings.Contains(m.mail.sent[0].Body, "game 101") || !strings.Contains(m.mail.sent[0].Body, "game 102\n") {
		t.Fatalf("emails sent: %+v", m.mail.sent)
	}

	f.refuses = map[string]int{icsOf("101"): http.StatusTooManyRequests}
	feeds = len(m.league.feeds)
	f.asked()
	c = m.run(t)
	if !slices.Equal(f.puts, moves("101")) || c.Times[uid("101")] != usualTime || !c.Daily || len(m.league.feeds) != feeds+1 || len(m.mail.sent) != 2 {
		t.Fatalf("slow down: PUTs %q, kept %+v, %d Members started, %d emails", f.puts, c, len(m.league.feeds)-feeds, len(m.mail.sent))
	}
	if !strings.Contains(m.logs.String(), `"stopped":"Apple said to slow down","moved":0,"missing":0,"emailed":0}`) {
		t.Fatalf("logs:\n%s", m.logs)
	}

	f.refuses = nil
	if c := m.run(t); c.Times[uid("101")] != moved101.at() || len(m.mail.sent) != 4 {
		t.Fatalf("the next run kept %+v and sent %d emails in all", c.Times, len(m.mail.sent))
	}
}

// Apple refusing at the question, before the game is sent, is acted on as a
// refusal of the game itself is: the password refused turns the switch off,
// slow down stops the run, and any other answer counts as failed. In none of
// them is the game sent, taken for one the Member deleted, or emailed about,
// and its old time is kept, so a later run makes the move.
func TestDailyActsOnARefusalOfTheQuestionAboutAMovedGame(t *testing.T) {
	for name, want := range map[string]struct {
		code  int
		daily bool
		logs  string
	}{
		"the password refused": {http.StatusUnauthorized, false, `"failed":0,"skipped":0,"switched_off":1,"stopped":"","moved":0,"missing":0,"emailed":0}`},
		"slow down":            {http.StatusTooManyRequests, true, `"failed":0,"skipped":0,"switched_off":0,"stopped":"Apple said to slow down","moved":0,"missing":0,"emailed":0}`},
		"Apple failed":         {http.StatusInternalServerError, true, `"failed":1,"skipped":0,"switched_off":0,"stopped":"","moved":0,"missing":0,"emailed":0}`},
	} {
		m, f := newDailySite(t)
		m.run(t)
		before := f.events["family"][icsOf("101")]
		m.lists(moved101)
		f.asks = map[string]int{icsOf("101"): want.code}
		f.asked()
		c := m.run(t)
		if f.calls != 2 || !slices.Equal(f.puts, []string{"ask " + icsOf("101")}) || f.events["family"][icsOf("101")] != before {
			t.Fatalf("%s: Apple got %d calls, %q, and holds %q", name, f.calls, f.puts, f.events)
		}
		if c.Daily != want.daily || c.Times[uid("101")] != usualTime || m.mail.tried != 0 {
			t.Fatalf("%s: kept %+v, %d emails tried", name, c, m.mail.tried)
		}
		if !strings.Contains(m.logs.String(), want.logs) {
			t.Fatalf("%s: logs:\n%s", name, m.logs)
		}
		// Once Apple answers again, a run that still has the switch on makes
		// the move.
		f.asks = nil
		f.asked()
		c = m.run(t)
		if want.daily && (!slices.Equal(f.puts, moves("101")) || c.Times[uid("101")] != moved101.at() || len(m.mail.sent) != 1) {
			t.Fatalf("%s: the next run: %q, kept %+v, %d emails", name, f.puts, c.Times, len(m.mail.sent))
		}
		if !want.daily && (f.calls != 0 || m.mail.tried != 0) {
			t.Fatalf("%s: switched off, and Apple was called %d times", name, f.calls)
		}
	}
}

// A moved game its Team's feed no longer holds is dropped from the list
// with its time, not moved.
func TestDailyDropsAKeptTimeWithItsGame(t *testing.T) {
	m, f := newDailySite(t)
	m.posts("101", "102")
	m.run(t)
	m.posts("102")
	f.asked()
	if c := m.run(t); f.calls != 0 || !slices.Equal(c.Written, uids("102")) || !maps.Equal(c.Times, map[string]store.GameTime{uid("102"): usualTime}) || m.mail.tried != 0 {
		t.Fatalf("kept %+v after %d calls to Apple", c, f.calls)
	}
}

// The run reads a calendar file as the backend itself writes one
// (internal/icloud/testdata/backend.ics): each game's start and field come
// from its own lines, not the time zone's or the reminder's, and the email
// names the game by its title, unfolded and unescaped, without its field.
func TestDailyReadsTheBackendsOwnFile(t *testing.T) {
	m, f := newDailySite(t)
	raw, err := os.ReadFile("../icloud/testdata/backend.ics")
	if err != nil {
		t.Fatal(err)
	}
	feed := string(raw)
	// The file of upcoming games holds the games alone, without the notice.
	notice := strings.Index(feed, "BEGIN:VEVENT\r\nUID:session-over-")
	if notice < 0 {
		t.Fatal("the fixture has no Session-over notice")
	}
	upcoming := feed[:notice] + "END:VCALENDAR\r\n"
	const one, two, over = "game-101@soccer.craigdevjohnson.com", "game-102@soccer.craigdevjohnson.com", "session-over-111111@soccer.craigdevjohnson.com"
	m.league.feed, m.league.file = feed, upcoming
	c := m.run(t)
	want := map[string]store.GameTime{one: {Start: "20261121T191500", Field: "Field 3"}, two: {Start: "20261128T183000", Field: "North Court"}}
	if !slices.Equal(c.Written, []string{one, two, over}) || !maps.Equal(c.Times, want) || len(f.events["family"]) != 3 {
		t.Fatalf("kept %+v, iCloud holds %d events", c, len(f.events["family"]))
	}

	// The league moves the first game a day on, to Field 1.
	move := strings.NewReplacer(
		"DTSTART;TZID=America/Denver:20261121T191500", "DTSTART;TZID=America/Denver:20261122T183000",
		"DTEND;TZID=America/Denver:20261121T200500", "DTEND;TZID=America/Denver:20261122T192000",
		"UNITED FC - Field 3", "UNITED FC - Field 1",
		`DESCRIPTION:Field 3\n`, `DESCRIPTION:Field 1\n`,
	)
	m.league.feed, m.league.file = move.Replace(feed), move.Replace(upcoming)
	f.asked()
	c = m.run(t)
	want[one] = store.GameTime{Start: "20261122T183000", Field: "Field 1"}
	if !slices.Equal(f.puts, []string{"ask " + one + ".ics", "write " + one + ".ics"}) || !maps.Equal(c.Times, want) || !strings.Contains(f.events["family"][one+".ics"], "DTSTART;TZID=America/Denver:20261122T183000") {
		t.Fatalf("PUTs %q, kept %+v", f.puts, c.Times)
	}
	if len(m.mail.sent) != 1 || !strings.Contains(m.mail.sent[0].Body, "\nROVERS vs KICKERS, UNITED FC\n  Was: Sat, Nov 21 at 7:15 PM, Field 3\n  Now: Sun, Nov 22 at 6:30 PM, Field 1\n") {
		t.Fatalf("emails sent: %+v", m.mail.sent)
	}
}
