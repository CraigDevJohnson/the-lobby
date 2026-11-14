package soccer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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
	gone     bool                         // the Family calendar was deleted
	readOnly bool                         // Apple refuses writes to Family
	events   map[string]map[string]string // calendar -> file name -> calendar file
}

func (f *fakeICloud) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, pw, ok := r.BasicAuth(); !ok || id != appleID || pw != applePassword || f.revoked {
		w.WriteHeader(http.StatusUnauthorized)
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
		family := calendar("/123/calendars/family/", "Family", "VEVENT", write)
		if f.gone {
			family = ""
		}
		multi(found("/123/calendars/", `<resourcetype><collection/></resourcetype>`) +
			calendar("/123/calendars/home/", "Home", "VEVENT", write) + family +
			calendar("/123/calendars/reminders/", "Reminders", "VTODO", write) +
			calendar("/123/calendars/school/", "School", "VEVENT", "") +
			found("/123/calendars/inbox/", `<displayname>Inbox</displayname><resourcetype><collection/></resourcetype>`))
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/123/calendars/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/123/calendars/"), "/")
		if len(parts) != 2 || f.readOnly {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		cal, name := parts[0], parts[1]
		if f.events[cal] == nil {
			f.events[cal] = map[string]string{}
		}
		_, had := f.events[cal][name]
		if had && r.Header.Get("If-None-Match") == "*" {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.events[cal][name] = string(b)
		if had {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func gameFile(ids ...string) string {
	file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nMETHOD:PUBLISH\r\nBEGIN:VTIMEZONE\r\nTZID:America/Denver\r\nEND:VTIMEZONE\r\n"
	for _, id := range ids {
		file += "BEGIN:VEVENT\r\nUID:game-" + id + "@soccer.example\r\nSUMMARY:game " + id + "\r\nEND:VEVENT\r\n"
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
	if err != nil || kept.Account != appleID || kept.Home != f.url+"/123/calendars/" || kept.Password == "" || strings.Contains(kept.Password, applePassword) {
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
		for _, c := range [][2]string{{http.MethodGet, "/api/soccer/icloud"}, {http.MethodPost, "/api/soccer/icloud/connect"}, {http.MethodPost, AddICloudPath}, {http.MethodPost, "/api/soccer/icloud/disconnect"}} {
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
