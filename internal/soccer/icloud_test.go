package soccer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/icloud"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

const (
	appleID       = "player@icloud.example"
	applePassword = "abcd-efgh-ijkl-mnop"
)

// fakeICloud is the parts of iCloud's CalDAV the site calls: finding the
// account's calendars, listing them and storing events.
type fakeICloud struct {
	mu       sync.Mutex
	url      string
	revoked  bool                         // the Member ended the password at Apple
	busy     bool                         // Apple says to slow down
	gone     bool                         // the Family calendar was deleted
	readOnly bool                         // Apple refuses writes to Family
	events   map[string]map[string]string // calendar -> file name -> calendar file
	refuses  map[string]int               // file name -> what Apple answers a PUT of it instead
	asks     map[string]int               // file name -> what Apple answers the question about it instead
	onRefuse func()                       // runs as Apple refuses a PUT, when set
	onList   func()                       // runs as Apple lists the calendars, when set
	onStore  func()                       // runs as Apple stores an event, when set
	calls    int                          // requests received
	replaced int                          // PUTs sent without If-None-Match: *, which replace
	// puts is each request about one event, as its kind and file name: a
	// PUT is "create" with If-None-Match: *, "replace" with If-Match: * and
	// "write" with neither, and "ask" is the question whether the calendar
	// holds the event.
	puts []string
	slow time.Duration // how long Apple takes to answer a PUT, when set
}

func (f *fakeICloud) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if id, pw, ok := r.BasicAuth(); !ok || id != appleID || pw != applePassword || f.revoked {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if f.busy {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	multi := func(responses string) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">` + responses + `</multistatus>`))
	}
	found := func(href, prop string) string {
		return `<response><href>` + href + `</href><propstat><prop>` + prop + `</prop><status>HTTP/1.1 200 OK</status></propstat></response>`
	}
	calendar := func(href, name, comp, privilege string) string {
		return found(href, `<displayname>`+name+`</displayname><resourcetype><collection/><C:calendar/></resourcetype>`+
			`<C:supported-calendar-component-set><C:comp name="`+comp+`"/></C:supported-calendar-component-set>`+
			`<current-user-privilege-set><privilege><read/></privilege>`+privilege+`</current-user-privilege-set>`)
	}
	const write = `<privilege><write/></privilege>`
	switch {
	case r.Method == "PROPFIND" && r.URL.Path == "/":
		multi(found("/", `<current-user-principal><href>/123/principal/</href></current-user-principal>`))
	case r.Method == "PROPFIND" && r.URL.Path == "/123/principal/":
		// Apple answers with a full address, on another host of its own.
		multi(found("/123/principal/", `<C:calendar-home-set><href>`+f.url+`/123/calendars/</href></C:calendar-home-set>`))
	case r.Method == "PROPFIND" && r.URL.Path == "/123/calendars/" && r.Header.Get("Depth") == "1":
		if f.onList != nil {
			f.onList()
		}
		family := calendar("/123/calendars/family/", "Family", "VEVENT", write)
		if f.gone {
			family = ""
		}
		multi(found("/123/calendars/", `<resourcetype><collection/></resourcetype>`) +
			calendar("/123/calendars/home/", "Home", "VEVENT", write) + family +
			calendar("/123/calendars/reminders/", "Reminders", "VTODO", write) +
			calendar("/123/calendars/school/", "School", "VEVENT", "") +
			found("/123/calendars/inbox/", `<displayname>Inbox</displayname><resourcetype><collection/></resourcetype>`))
	case r.Method == "PROPFIND" && strings.HasSuffix(r.URL.Path, ".ics") && r.Header.Get("Depth") == "0":
		// Whether the calendar holds one event: its ETag, or 404.
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/123/calendars/"), "/")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.puts = append(f.puts, "ask "+parts[1])
		if code := f.asks[parts[1]]; code != 0 {
			w.WriteHeader(code)
			return
		}
		if _, had := f.events[parts[0]][parts[1]]; !had {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		multi(found(r.URL.Path, `<getetag>"C=1@U=2"</getetag>`))
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/123/calendars/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/123/calendars/"), "/")
		if len(parts) != 2 || f.readOnly {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		cal, name := parts[0], parts[1]
		onlyNew, onlyThere := r.Header.Get("If-None-Match") == "*", r.Header.Get("If-Match") == "*"
		switch {
		case onlyNew:
			f.puts = append(f.puts, "create "+name)
		case onlyThere:
			f.puts = append(f.puts, "replace "+name)
		default:
			f.puts = append(f.puts, "write "+name)
		}
		time.Sleep(f.slow)
		if code := f.refuses[name]; code != 0 {
			if f.onRefuse != nil {
				f.onRefuse()
			}
			w.WriteHeader(code)
			return
		}
		if !onlyNew {
			f.replaced++
		}
		if f.events[cal] == nil {
			f.events[cal] = map[string]string{}
		}
		_, had := f.events[cal][name]
		// A PUT that may only add finds the file there: Apple changes
		// nothing. Apple doesn't keep If-Match: * the same way. On dev, on
		// 2026-11-19, it refused that PUT for an event that was there, and
		// stored one that had been deleted (ADR 0017), so the site never
		// sends it.
		if had && (onlyNew || onlyThere) {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.events[cal][name] = string(b)
		if f.onStore != nil {
			f.onStore()
		}
		if had {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// game is one game as the league lists it: its ID, and its start and field
// when they aren't the usual ones.
type game struct{ id, start, field string }

// The start and field of a game that names neither.
const (
	usualStart = "20261121T191500"
	usualField = "Field 3"
)

// at is where the league has the game: what the daily update keeps for it.
func (g game) at() store.GameTime {
	at := store.GameTime{Start: usualStart, Field: usualField}
	if g.start != "" {
		at.Start = g.start
	}
	if g.field != "" {
		at.Field = g.field
	}
	return at
}

// gameFile is the backend's calendar file for these games, each at the usual
// start and field.
func gameFile(ids ...string) string {
	games := make([]game, len(ids))
	for i, id := range ids {
		games[i] = game{id: id}
	}
	return gamesFile(games...)
}

// gamesFile is the backend's calendar file for these games, in the shape
// the backend writes: the time zone with DTSTART lines of its own, and each
// game with its start in Boise wall-clock digits, its field as the first
// line of its description and at the end of its title, and a reminder with
// a description of its own.
func gamesFile(games ...game) string {
	file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nMETHOD:PUBLISH\r\n" +
		"BEGIN:VTIMEZONE\r\nTZID:America/Denver\r\nBEGIN:STANDARD\r\nDTSTART:19701101T020000\r\nTZOFFSETFROM:-0600\r\nTZOFFSETTO:-0700\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\n"
	for _, g := range games {
		at := g.at()
		file += "BEGIN:VEVENT\r\nUID:game-" + g.id + "@soccer.example\r\nSUMMARY:game " + g.id + " - " + at.Field + "\r\n" +
			"DTSTART;TZID=America/Denver:" + at.Start + "\r\n" +
			"DESCRIPTION:" + at.Field + "\\ngame " + g.id + "\\nLet's Play Soccer\\, Boise\r\n" +
			"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT40M\r\nDESCRIPTION:Soccer game starting soon\r\nEND:VALARM\r\n" +
			"END:VEVENT\r\n"
	}
	return file + "END:VCALENDAR\r\n"
}

// newICloudSite is the Member site with Add to iCloud Calendar set up
// against fakeICloud, and a backend whose calendar file holds two games.
func newICloudSite(t *testing.T) (*memberSite, *fakeICloud) {
	t.Helper()
	m := newMemberSite(t)
	f := &fakeICloud{events: map[string]map[string]string{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	f.url = srv.URL
	m.tool.ICloud = &icloud.Client{HTTP: srv.Client(), URL: srv.URL}
	key := bytes.Repeat([]byte{7}, 32)
	m.tool.ICloudKey = func(context.Context) ([]byte, error) { return key, nil }
	m.league.file = gameFile("101", "102")
	return m, f
}

func (m *memberSite) icloudConnect(t *testing.T, password string) (int, map[string]any) {
	t.Helper()
	res, body := m.call(t, http.MethodPost, "/api/soccer/icloud/connect", `{"apple_id":" `+appleID+` ","password":"`+password+`"}`)
	var a map[string]any
	_ = json.Unmarshal([]byte(body), &a)
	return res.StatusCode, a
}

func (m *memberSite) icloudStatus(t *testing.T) map[string]any {
	t.Helper()
	res, body := m.call(t, http.MethodGet, "/api/soccer/icloud", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d %s", res.StatusCode, body)
	}
	var a map[string]any
	_ = json.Unmarshal([]byte(body), &a)
	return a
}

func (m *memberSite) icloudAdd(t *testing.T, calendar string) (int, string) {
	t.Helper()
	res, body := m.call(t, http.MethodPost, AddICloudPath, `{"teams":[{"id":"111111","name":"ROVERS"}],"calendar_id":"`+calendar+`"}`)
	return res.StatusCode, body
}

func (m *memberSite) icloudDaily(t *testing.T, body string) (int, string) {
	t.Helper()
	res, b := m.call(t, http.MethodPost, "/api/soccer/icloud/daily", body)
	return res.StatusCode, b
}

// A Member connects, adds their games, presses Add again once the league
// posts another game, and disconnects. Only a sealed password is kept, and
// neither it nor the Apple Account address reaches the logs.
func TestICloudConnectAddAndDisconnect(t *testing.T) {
	m, f := newICloudSite(t)
	family, home := f.url+"/123/calendars/family/", f.url+"/123/calendars/home/"
	code, a := m.icloudConnect(t, applePassword)
	cals, _ := a["calendars"].([]any)
	// Reminders, the view-only School calendar and the inbox aren't offered.
	if code != http.StatusOK || a["connected"] != true || a["account"] != appleID || len(cals) != 2 ||
		cals[0].(map[string]any)["id"] != family || cals[1].(map[string]any)["name"] != "Home" || a["calendar_id"] != family {
		t.Fatalf("connect: %d %v", code, a)
	}
	kept, err := m.store.GetICloud(context.Background(), memberEmail)
	if err != nil || kept.Account != appleID || kept.Home != f.url+"/123/calendars/" || kept.Password == "" || kept.ConnectionID == "" || strings.Contains(kept.Password, applePassword) {
		t.Fatalf("kept %+v, %v", kept, err)
	}

	if code, body := m.icloudAdd(t, home); code != http.StatusOK || body != `{"added":2,"failed":0,"updated":0}`+"\n" {
		t.Fatalf("add: %d %s", code, body)
	}
	if len(m.league.asked) != 1 || strings.Contains(m.league.asked[0], "calendar_id") || !strings.Contains(m.league.asked[0], `"teams"`) {
		t.Fatalf("backend was asked %q", m.league.asked)
	}
	one := f.events["home"]["game-101@soccer.example.ics"]
	if !strings.Contains(one, "SUMMARY:game 101") || !strings.Contains(one, "TZID:America/Denver") || strings.Contains(one, "METHOD") || strings.Contains(one, "game 102") {
		t.Fatalf("iCloud holds %q", f.events)
	}
	// The league posts a third game; pressing Add again adds it and
	// rewrites the other two.
	m.league.file = gameFile("101", "102", "103")
	if code, body := m.icloudAdd(t, home); code != http.StatusOK || body != `{"added":1,"failed":0,"updated":2}`+"\n" {
		t.Fatalf("second add: %d %s", code, body)
	}
	if len(f.events["home"]) != 3 || len(f.events) != 1 {
		t.Fatalf("iCloud holds %v", f.events)
	}
	if a := m.icloudStatus(t); a["connected"] != true || a["calendar_id"] != home {
		t.Fatalf("the chosen calendar wasn't kept: %v", a)
	}

	res, body := m.call(t, http.MethodPost, "/api/soccer/icloud/disconnect", "")
	if res.StatusCode != http.StatusOK || body != `{"available":true,"connected":false}`+"\n" {
		t.Fatalf("disconnect: %d %s", res.StatusCode, body)
	}
	if _, err := m.store.GetICloud(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("still kept: %v", err)
	}
	if len(f.events["home"]) != 3 {
		t.Fatalf("disconnecting took games out: %v", f.events)
	}
	for _, secret := range []string{applePassword, appleID, "/123/"} {
		if strings.Contains(m.logs.String(), secret) {
			t.Fatalf("logs hold %q:\n%s", secret, m.logs)
		}
	}
}

type disconnectingICloudStore struct{ *store.Memory }

func (s disconnectingICloudStore) UpdateICloudCalendar(ctx context.Context, current store.ICloud, calendarID string) error {
	_ = s.DeleteICloud(ctx, current.Email)
	return s.Memory.UpdateICloudCalendar(ctx, current, calendarID)
}

// A disconnect after Add reads the credential but before it saves the
// calendar makes the conditional update fail. The request neither restores
// the credential record nor writes an event with its stale password.
func TestICloudAddCannotOutrunDisconnect(t *testing.T) {
	m, f := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	m.tool.Store = disconnectingICloudStore{m.store}
	home := f.url + "/123/calendars/home/"
	if code, body := m.icloudAdd(t, home); code != http.StatusUnauthorized || errorCode(t, body) != "icloud_not_connected" {
		t.Fatalf("add after disconnect: %d %s", code, body)
	}
	if _, err := m.store.GetICloud(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the stale add restored the connection: %v", err)
	}
	if len(f.events) != 0 {
		t.Fatalf("the stale add wrote events: %v", f.events)
	}
}

func TestICloudWrongPassword(t *testing.T) {
	m, _ := newICloudSite(t)
	if code, a := m.icloudConnect(t, "wrong-pass-word-here"); code != http.StatusUnauthorized || a["code"] != "icloud_denied" {
		t.Fatalf("connect: %d %v", code, a)
	}
	if _, err := m.store.GetICloud(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("kept: %v", err)
	}
	if a := m.icloudStatus(t); a["available"] != true || a["connected"] != false {
		t.Fatalf("status = %v", a)
	}
	for _, body := range []string{`{}`, `{"apple_id":"a:b@example.com","password":"x"}`, `not json`} {
		if res, _ := m.call(t, http.MethodPost, "/api/soccer/icloud/connect", body); res.StatusCode != http.StatusBadRequest {
			t.Fatalf("connect %s: %d", body, res.StatusCode)
		}
	}
}

// The Member ends the password at Apple: the connection is dropped, and
// they connect again.
func TestICloudPasswordEnded(t *testing.T) {
	m, f := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	f.revoked = true
	if code, body := m.icloudAdd(t, f.url+"/123/calendars/home/"); code != http.StatusUnauthorized || !strings.Contains(body, "icloud_expired") {
		t.Fatalf("add: %d %s", code, body)
	}
	if _, err := m.store.GetICloud(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("still kept: %v", err)
	}
	if a := m.icloudStatus(t); a["connected"] != false {
		t.Fatalf("status = %v", a)
	}
	if code, body := m.icloudAdd(t, f.url+"/123/calendars/home/"); code != http.StatusUnauthorized || !strings.Contains(body, "icloud_not_connected") {
		t.Fatalf("add when not connected: %d %s", code, body)
	}
}

// Only a calendar Apple just listed is written to: not one that's gone,
// one shared to view only, a reminder list, or an address of someone
// else's.
func TestICloudOnlyListedCalendars(t *testing.T) {
	m, f := newICloudSite(t)
	var elsewhere int
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere++ }))
	t.Cleanup(other.Close)
	m.icloudConnect(t, applePassword)
	f.gone = true
	for _, cal := range []string{f.url + "/123/calendars/family/", f.url + "/123/calendars/school/", f.url + "/123/calendars/reminders/", other.URL + "/123/calendars/home/"} {
		if code, body := m.icloudAdd(t, cal); code != http.StatusConflict || !strings.Contains(body, "icloud_calendar") {
			t.Fatalf("add to %s: %d %s", cal, code, body)
		}
	}
	if len(f.events) != 0 || elsewhere != 0 || len(m.league.asked) != 0 {
		t.Fatalf("written %v, %d requests elsewhere, backend asked %v", f.events, elsewhere, m.league.asked)
	}
	// The saved choice is gone, so the first calendar left is offered.
	if a := m.icloudStatus(t); a["calendar_id"] != f.url+"/123/calendars/home/" {
		t.Fatalf("status = %v", a)
	}
	// Apple refusing the write itself asks for another calendar too.
	f.readOnly = true
	if code, body := m.icloudAdd(t, f.url+"/123/calendars/home/"); code != http.StatusConflict || !strings.Contains(body, "icloud_calendar") {
		t.Fatalf("add to a calendar Apple refuses: %d %s", code, body)
	}
}

func TestICloudNotSetUp(t *testing.T) {
	m := newMemberSite(t)
	if a := m.icloudStatus(t); a["available"] != false {
		t.Fatalf("status = %v", a)
	}
	if code, a := m.icloudConnect(t, applePassword); code != http.StatusServiceUnavailable || a["code"] != "icloud_unavailable" {
		t.Fatalf("connect: %d %v", code, a)
	}
	if code, _ := m.icloudAdd(t, "x"); code != http.StatusServiceUnavailable {
		t.Fatalf("add: %d", code)
	}
}

func TestICloudNeedsTheGrant(t *testing.T) {
	m, f := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	for _, a := range []Access{SignedOut, NoAccess} {
		m.access = a
		for _, c := range [][2]string{{http.MethodGet, "/api/soccer/icloud"}, {http.MethodPost, "/api/soccer/icloud/connect"}, {http.MethodPost, AddICloudPath}, {http.MethodPost, "/api/soccer/icloud/daily"}, {http.MethodPost, "/api/soccer/icloud/disconnect"}} {
			if res, _ := m.call(t, c[0], c[1], `{"calendar_id":"x"}`); res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
				t.Fatalf("%v %s %s: %d", a, c[0], c[1], res.StatusCode)
			}
		}
	}
	if len(f.events) != 0 {
		t.Fatalf("written without the grant: %v", f.events)
	}
	// Removing the Member forgets the connection.
	_ = m.store.DeleteMember(context.Background(), memberEmail)
	if _, err := m.store.GetICloud(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("still kept: %v", err)
	}
}

// The daily update's switch. Turning it on saves the calendar it writes to,
// and a calendar other than the saved one starts the list of games added,
// and their kept times, afresh, as a press on another calendar does. Turning
// it off keeps them, and connecting again starts with it off, no list and
// no times. Apple isn't asked.
func TestICloudDailySwitch(t *testing.T) {
	m, f := newICloudSite(t)
	ctx := context.Background()
	family, home := f.url+"/123/calendars/family/", f.url+"/123/calendars/home/"
	on := func(calendar string) (int, string) {
		return m.icloudDaily(t, `{"on":true,"calendar_id":"`+calendar+`"}`)
	}
	kept := func() store.ICloud {
		c, _ := m.store.GetICloud(ctx, memberEmail)
		return c
	}
	// added stands in for a run of the daily update having added a game and
	// kept its time.
	added := func() {
		c := kept()
		c.Written = []string{"game-101@soccer.example"}
		c.Times = map[string]store.GameTime{"game-101@soccer.example": {Start: usualStart, Field: usualField}}
		_ = m.store.PutICloud(ctx, c)
	}

	if code, body := on(family); code != http.StatusUnauthorized || errorCode(t, body) != "icloud_not_connected" {
		t.Fatalf("not connected: %d %s", code, body)
	}
	m.icloudConnect(t, applePassword)
	if a := m.icloudStatus(t); a["daily"] != nil {
		t.Fatalf("a new connection's status = %v", a)
	}
	for _, body := range []string{`{"on":true}`, `{"on":true,"calendar_id":""}`, `not json`} {
		if code, _ := m.icloudDaily(t, body); code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, code)
		}
	}

	f.calls = 0
	if code, body := on(family); code != http.StatusOK || body != `{"daily":true}`+"\n" {
		t.Fatalf("on: %d %s", code, body)
	}
	added()
	if on(family); f.calls != 0 || !kept().Daily || kept().CalendarID != family || len(kept().Written) != 1 || len(kept().Times) != 1 {
		t.Fatalf("on, the same calendar: kept %+v after %d calls to Apple", kept(), f.calls)
	}
	if a := m.icloudStatus(t); a["daily"] != true || a["calendar_id"] != family {
		t.Fatalf("status = %v", a)
	}
	if on(home); !kept().Daily || kept().CalendarID != home || kept().Written != nil || kept().Times != nil {
		t.Fatalf("on, another calendar: kept %+v", kept())
	}

	added()
	if code, body := m.icloudDaily(t, `{"on":false}`); code != http.StatusOK || body != `{"daily":false}`+"\n" {
		t.Fatalf("off: %d %s", code, body)
	}
	if kept().Daily || kept().CalendarID != home || len(kept().Written) != 1 || len(kept().Times) != 1 {
		t.Fatalf("off: kept %+v", kept())
	}
	if a := m.icloudStatus(t); a["daily"] != nil {
		t.Fatalf("status = %v", a)
	}
	// On again doesn't bring back a game the Member deleted meanwhile, and
	// the game's kept time is still there to tell a move by.
	if on(home); len(kept().Written) != 1 || len(kept().Times) != 1 {
		t.Fatalf("on again: kept %+v", kept())
	}

	if code, body := m.icloudAdd(t, family); code != http.StatusOK {
		t.Fatalf("add: %d %s", code, body)
	}
	if !kept().Daily || kept().CalendarID != family || kept().Written != nil || kept().Times != nil {
		t.Fatalf("a press on another calendar: kept %+v", kept())
	}
	// A press on the saved calendar keeps the list and the times.
	added()
	if code, body := m.icloudAdd(t, family); code != http.StatusOK || len(kept().Written) != 1 || len(kept().Times) != 1 {
		t.Fatalf("a press on the same calendar: %d %s, kept %+v", code, body, kept())
	}

	if code, a := m.icloudConnect(t, applePassword); code != http.StatusOK || a["daily"] != nil || kept().Daily || kept().Written != nil || kept().Times != nil || kept().CalendarID != family {
		t.Fatalf("connected again: %d %v, kept %+v", code, a, kept())
	}
}

type reconnectingICloudStore struct {
	*store.Memory
	next store.ICloud
}

func (s reconnectingICloudStore) UpdateICloudDailySwitch(ctx context.Context, current store.ICloud, on bool, calendarID string) error {
	_ = s.PutICloud(ctx, s.next)
	return s.Memory.UpdateICloudDailySwitch(ctx, current, on, calendarID)
}

// Even if reconnecting produces the same sealed password, its immutable
// connection ID makes an older switch request fail instead of overwriting
// the new connection and turning its daily update on.
func TestICloudDailySwitchCannotOverwriteReconnect(t *testing.T) {
	m, f := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	next, _ := m.store.GetICloud(context.Background(), memberEmail)
	next.ConnectionID, next.CalendarID, next.Daily = "new-connection", "new-calendar", false
	m.tool.Store = reconnectingICloudStore{Memory: m.store, next: next}
	if code, body := m.icloudDaily(t, `{"on":true,"calendar_id":"`+f.url+`/123/calendars/family/"}`); code != http.StatusUnauthorized || errorCode(t, body) != "icloud_not_connected" {
		t.Fatalf("switch after reconnect: %d %s", code, body)
	}
	kept, _ := m.store.GetICloud(context.Background(), memberEmail)
	if kept.ConnectionID != "new-connection" || kept.CalendarID != "new-calendar" || kept.Daily {
		t.Fatalf("the stale switch changed the new connection: %+v", kept)
	}
}

// meanwhile is a store where something else happens to the Member's
// connection just before each conditional update of its calendar or switch
// reaches the store, as when another request of theirs gets there first.
type meanwhile struct {
	*store.Memory
	updates int              // the conditional updates tried so far
	before  func(update int) // runs before each, with its number from 1
}

func (s *meanwhile) happen() {
	s.updates++
	s.before(s.updates)
}

func (s *meanwhile) UpdateICloudCalendar(ctx context.Context, current store.ICloud, calendarID string) error {
	s.happen()
	return s.Memory.UpdateICloudCalendar(ctx, current, calendarID)
}

func (s *meanwhile) UpdateICloudDailySwitch(ctx context.Context, current store.ICloud, on bool, calendarID string) error {
	s.happen()
	return s.Memory.UpdateICloudDailySwitch(ctx, current, on, calendarID)
}

func (s *meanwhile) UpdateGoogleCalendar(ctx context.Context, current store.Google, calendarID string) error {
	s.happen()
	return s.Memory.UpdateGoogleCalendar(ctx, current, calendarID)
}

// Another request of the Member's that saves a calendar after the switch
// read the connection doesn't make the switch answer that iCloud isn't
// connected: the connection is still theirs, so the switch is set on what is
// there now, and turning the daily update off takes effect. That holds for a
// connection made before connection IDs too. A disconnect still answers
// icloud_not_connected.
func TestICloudDailySwitchOutlastsACalendarSavedMeanwhile(t *testing.T) {
	ctx := context.Background()
	for name, legacy := range map[string]bool{"a connection with an ID": false, "a connection from before connection IDs": true} {
		m, f := newDailySite(t)
		home := f.url + "/123/calendars/home/"
		if legacy {
			c, _ := m.store.GetICloud(ctx, memberEmail)
			c.ConnectionID = ""
			_ = m.store.PutICloud(ctx, c)
		}
		s := &meanwhile{Memory: m.store, before: func(update int) {
			if update == 1 {
				c, _ := m.store.GetICloud(ctx, memberEmail)
				_ = m.store.UpdateICloudCalendar(ctx, c, home)
			}
		}}
		m.tool.Store = s
		if code, body := m.icloudDaily(t, `{"on":false}`); code != http.StatusOK || body != `{"daily":false}`+"\n" {
			t.Fatalf("%s: off, with a calendar saved meanwhile: %d %s", name, code, body)
		}
		if c, err := m.store.GetICloud(ctx, memberEmail); err != nil || c.Daily || c.CalendarID != home || s.updates != 2 {
			t.Fatalf("%s: kept %+v, %v after %d updates", name, c, err, s.updates)
		}

		s.updates, s.before = 0, func(int) { _ = m.store.DeleteICloud(ctx, memberEmail) }
		if code, body := m.icloudDaily(t, `{"on":true,"calendar_id":"`+home+`"}`); code != http.StatusUnauthorized || errorCode(t, body) != "icloud_not_connected" || s.updates != 1 {
			t.Fatalf("%s: on, disconnected meanwhile: %d %s after %d updates", name, code, body, s.updates)
		}
		if _, err := m.store.GetICloud(ctx, memberEmail); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%s: the stale switch restored the connection: %v", name, err)
		}
	}
}

// A press of Add is the same: a calendar saved meanwhile on the same
// connection doesn't stop it, and its own choice is the one kept.
func TestICloudAddOutlastsACalendarSavedMeanwhile(t *testing.T) {
	ctx := context.Background()
	m, f := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	family, home := f.url+"/123/calendars/family/", f.url+"/123/calendars/home/"
	s := &meanwhile{Memory: m.store, before: func(update int) {
		if update == 1 {
			c, _ := m.store.GetICloud(ctx, memberEmail)
			_ = m.store.UpdateICloudDailySwitch(ctx, c, true, family)
		}
	}}
	m.tool.Store = s
	if code, body := m.icloudAdd(t, home); code != http.StatusOK || body != `{"added":2,"failed":0,"updated":0}`+"\n" {
		t.Fatalf("add, with a calendar saved meanwhile: %d %s", code, body)
	}
	if c, _ := m.store.GetICloud(ctx, memberEmail); c.CalendarID != home || !c.Daily || len(f.events["home"]) != 2 || len(f.events) != 1 || s.updates != 2 {
		t.Fatalf("kept %+v after %d updates, iCloud holds %q", c, s.updates, f.events)
	}
}

// Settings that change again before each of the three tries are a conflict
// with a code of its own, which the page doesn't take for a connection that
// is gone. Nothing is written, and the connection stays.
func TestICloudSettingsThatKeepChangingAreAConflict(t *testing.T) {
	ctx := context.Background()
	m, f := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	s := &meanwhile{Memory: m.store, before: func(update int) {
		c, _ := m.store.GetICloud(ctx, memberEmail)
		_ = m.store.UpdateICloudCalendar(ctx, c, "calendar-"+strconv.Itoa(update))
	}}
	m.tool.Store = s
	home := f.url + "/123/calendars/home/"
	if code, body := m.icloudDaily(t, `{"on":true,"calendar_id":"`+home+`"}`); code != http.StatusConflict || errorCode(t, body) != "icloud_conflict" || s.updates != 3 {
		t.Fatalf("switch: %d %s after %d updates", code, body, s.updates)
	}
	if code, body := m.icloudAdd(t, home); code != http.StatusConflict || errorCode(t, body) != "icloud_conflict" || s.updates != 6 {
		t.Fatalf("add: %d %s after %d updates", code, body, s.updates)
	}
	if c, err := m.store.GetICloud(ctx, memberEmail); err != nil || c.Daily || c.CalendarID != "calendar-6" || len(f.events) != 0 {
		t.Fatalf("kept %+v, %v, iCloud holds %q", c, err, f.events)
	}
}

// The switch is changed only from The Lobby's own page, like My calendar.
func TestICloudDailySwitchIsSameSite(t *testing.T) {
	m, _ := newICloudSite(t)
	m.icloudConnect(t, applePassword)
	req, _ := http.NewRequest(http.MethodPost, m.url+"/api/soccer/icloud/daily", strings.NewReader(`{"on":true,"calendar_id":"x"}`))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if c, _ := m.store.GetICloud(context.Background(), memberEmail); res.StatusCode != http.StatusForbidden || c.Daily {
		t.Fatalf("%d, kept %+v", res.StatusCode, c)
	}
}
