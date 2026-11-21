package soccer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/oauth2"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

const (
	familyCalendar = "family123@group.calendar.google.com"
	googleAccount  = "family@example.org"
)

// fakeGoogle is the parts of Google the site calls: the token endpoint,
// UserInfo, the calendar list, event writes and revoke.
type fakeGoogle struct {
	mu       sync.Mutex
	scope    string                       // what a code grants
	refresh  map[string]bool              // refresh tokens Google still takes
	events   map[string]map[string]string // calendar ID -> event ID -> body
	readOnly map[string]bool              // calendars the account can only read
	gone     bool                         // the Family calendar was deleted
	limited  bool                         // Google answers writes with a rate limit
	revoked  []string
	onRevoke func() // runs as Google revokes a token, when set
}

func (g *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fail := func(code int, body string) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}
	bearer := r.Header.Get("Authorization")
	signedIn := bearer == "Bearer access-1" || bearer == "Bearer access-2"
	switch {
	case r.URL.Path == "/token":
		_ = r.ParseForm()
		if r.Form.Get("client_secret") != "secret" {
			fail(http.StatusUnauthorized, `{"error":"invalid_client"}`)
			return
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "good-code" {
				fail(http.StatusBadRequest, `{"error":"invalid_grant"}`)
				return
			}
			g.refresh["refresh-1"] = true
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 3600, "scope": g.scope})
		case "refresh_token":
			if !g.refresh[r.Form.Get("refresh_token")] {
				fail(http.StatusBadRequest, `{"error":"invalid_grant"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-2", "token_type": "Bearer", "expires_in": 3600})
		}
	case r.URL.Path == "/revoke":
		_ = r.ParseForm()
		g.revoked = append(g.revoked, r.Form.Get("token"))
		delete(g.refresh, r.Form.Get("token"))
		if g.onRevoke != nil {
			g.onRevoke()
		}
	case !signedIn:
		fail(http.StatusUnauthorized, `{}`)
	case r.URL.Path == "/userinfo":
		_ = json.NewEncoder(w).Encode(map[string]any{"email": googleAccount, "email_verified": true})
	case r.URL.Path == "/calendar/users/me/calendarList":
		if !strings.Contains(g.scope, google.ScopeCalendarList) {
			fail(http.StatusForbidden, `{}`)
			return
		}
		family := `{"id":"` + familyCalendar + `","summary":"Family"},`
		if g.gone {
			family = ""
		}
		_, _ = w.Write([]byte(`{"items":[` + family + `{"id":"` + googleAccount + `","summary":"` + googleAccount + `","primary":true}]}`))
	case strings.HasPrefix(r.URL.Path, "/calendar/calendars/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/calendar/calendars/"), "/")
		cal := parts[0]
		if g.readOnly[cal] {
			fail(http.StatusForbidden, `{}`)
			return
		}
		if g.limited {
			fail(http.StatusForbidden, `{"error":{"code":403,"errors":[{"reason":"rateLimitExceeded"}]}}`)
			return
		}
		if r.Method == http.MethodGet {
			if len(parts) != 3 {
				fail(http.StatusNotFound, `{}`)
				return
			}
			body, ok := g.events[cal][parts[2]]
			if !ok {
				fail(http.StatusNotFound, `{}`)
				return
			}
			_, _ = w.Write([]byte(body))
			return
		}
		b, _ := io.ReadAll(r.Body)
		var ev struct{ ID string }
		_ = json.Unmarshal(b, &ev)
		if g.events[cal] == nil {
			g.events[cal] = map[string]string{}
		}
		if r.Method == http.MethodPost {
			if _, ok := g.events[cal][ev.ID]; ok {
				fail(http.StatusConflict, `{}`)
				return
			}
			g.events[cal][ev.ID] = string(b)
		} else if r.Method == http.MethodPut {
			g.events[cal][parts[2]] = string(b)
		} else {
			fail(http.StatusMethodNotAllowed, `{}`)
			return
		}
		_, _ = w.Write(b)
	default:
		fail(http.StatusNotFound, `{}`)
	}
}

// newGoogleSite is the Member site with Add to Google Calendar set up
// against fakeGoogle, and a backend that builds two games' events.
func newGoogleSite(t *testing.T) (*memberSite, *fakeGoogle) {
	t.Helper()
	m := newMemberSite(t)
	g := &fakeGoogle{
		scope:    "openid email " + google.ScopeEvents + " " + google.ScopeCalendarList,
		refresh:  map[string]bool{},
		events:   map[string]map[string]string{},
		readOnly: map[string]bool{},
	}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	m.tool.Google = &google.Client{
		HTTP:        srv.Client(),
		Endpoint:    oauth2.Endpoint{AuthURL: srv.URL + "/auth", TokenURL: srv.URL + "/token", AuthStyle: oauth2.AuthStyleInParams},
		UserInfoURL: srv.URL + "/userinfo",
		CalendarURL: srv.URL + "/calendar",
		RevokeURL:   srv.URL + "/revoke",
	}
	key := bytes.Repeat([]byte{7}, 32)
	m.tool.GoogleSettings = func(context.Context) (google.Settings, error) {
		return google.Settings{ClientID: "client", ClientSecret: "secret", Key: key}, nil
	}
	m.league.events = []map[string]any{{"id": "101", "summary": "ROVERS vs KICKERS"}, {"id": "102", "summary": "OWLS vs ROVERS"}}
	return m, g
}

var noRedirects = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// connect goes through Connect and Google's return, answering where the
// callback sent the Member.
func connect(t *testing.T, m *memberSite, state func(string) string) string {
	t.Helper()
	res, err := noRedirects.Get(m.url + "/soccer/google/connect")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	to, _ := url.Parse(res.Header.Get("Location"))
	q := to.Query()
	if res.StatusCode != http.StatusFound || q.Get("login_hint") != memberEmail || q.Get("access_type") != "offline" ||
		q.Get("prompt") != "consent" || !strings.Contains(q.Get("scope"), google.ScopeEvents) ||
		q.Get("redirect_uri") != "https://lobby.example/soccer/google/callback" {
		t.Fatalf("connect: %d to %s", res.StatusCode, to)
	}
	req, _ := http.NewRequest(http.MethodGet, m.url+"/soccer/google/callback?"+url.Values{"state": {state(q.Get("state"))}, "code": {"good-code"}}.Encode(), nil)
	for _, c := range res.Cookies() {
		req.AddCookie(c)
	}
	back, err := noRedirects.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	back.Body.Close()
	return back.Header.Get("Location")
}

func same(s string) string { return s }

func (m *memberSite) googleStatus(t *testing.T) map[string]any {
	t.Helper()
	res, body := m.call(t, http.MethodGet, "/api/soccer/google", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d %s", res.StatusCode, body)
	}
	var a map[string]any
	_ = json.Unmarshal([]byte(body), &a)
	return a
}

func (m *memberSite) add(t *testing.T, calendar string) (int, string) {
	t.Helper()
	res, body := m.call(t, http.MethodPost, AddGooglePath, `{"teams":[{"id":"111111","name":"ROVERS"}],"calendar_id":"`+calendar+`"}`)
	return res.StatusCode, body
}

// A Member connects, adds their games, presses Add again once the league
// posts another game, and disconnects. Only a sealed refresh token is kept,
// and no token or Google address reaches the logs.
func TestGoogleConnectAddAndDisconnect(t *testing.T) {
	m, g := newGoogleSite(t)
	if to := connect(t, m, same); to != "/soccer?google=connected" {
		t.Fatalf("callback sent the Member to %q", to)
	}
	kept, err := m.store.GetGoogle(context.Background(), memberEmail)
	if err != nil || kept.Account != googleAccount || kept.CalendarID != "primary" || kept.Token == "" || kept.ConnectionID == "" || strings.Contains(kept.Token, "refresh-1") {
		t.Fatalf("kept %+v, %v", kept, err)
	}

	a := m.googleStatus(t)
	cals, _ := a["calendars"].([]any)
	if a["connected"] != true || a["account"] != googleAccount || a["calendar_id"] != googleAccount || len(cals) != 2 ||
		cals[0].(map[string]any)["id"] != googleAccount {
		t.Fatalf("status = %v", a)
	}

	if code, body := m.add(t, familyCalendar); code != http.StatusOK || body != `{"added":2,"failed":0,"updated":0}`+"\n" {
		t.Fatalf("add: %d %s", code, body)
	}
	if len(m.league.asked) != 1 || strings.Contains(m.league.asked[0], "calendar_id") || !strings.Contains(m.league.asked[0], `"teams"`) {
		t.Fatalf("backend was asked %q", m.league.asked)
	}
	// The league posts a third game; pressing Add again adds it and
	// rewrites the other two.
	m.league.events = append(m.league.events, map[string]any{"id": "103", "summary": "ROVERS vs COMETS"})
	if code, body := m.add(t, familyCalendar); code != http.StatusOK || body != `{"added":1,"failed":0,"updated":2}`+"\n" {
		t.Fatalf("second add: %d %s", code, body)
	}
	if len(g.events[familyCalendar]) != 3 {
		t.Fatalf("Google holds %v", g.events)
	}
	if a := m.googleStatus(t); a["calendar_id"] != familyCalendar {
		t.Fatalf("the chosen calendar wasn't kept: %v", a)
	}

	res, body := m.call(t, http.MethodPost, "/api/soccer/google/disconnect", "")
	if res.StatusCode != http.StatusOK || body != `{"available":true,"connected":false}`+"\n" {
		t.Fatalf("disconnect: %d %s", res.StatusCode, body)
	}
	if len(g.revoked) != 1 || g.revoked[0] != "refresh-1" {
		t.Fatalf("revoked %v", g.revoked)
	}
	if _, err := m.store.GetGoogle(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("still kept: %v", err)
	}
	for _, secret := range []string{"refresh-1", "access-1", "access-2", googleAccount, "good-code"} {
		if strings.Contains(m.logs.String(), secret) {
			t.Fatalf("logs hold %q:\n%s", secret, m.logs)
		}
	}
}

type disconnectingGoogleStore struct{ *store.Memory }

func (s disconnectingGoogleStore) UpdateGoogleCalendar(ctx context.Context, current store.Google, calendarID string) error {
	_ = s.DeleteGoogleConnection(ctx, current)
	return s.Memory.UpdateGoogleCalendar(ctx, current, calendarID)
}

// A disconnect after Add reads the credential but before it saves the
// calendar makes the conditional update fail. The request neither restores
// the credential record nor writes an event with its stale access token.
func TestGoogleAddCannotOutrunDisconnect(t *testing.T) {
	m, g := newGoogleSite(t)
	connect(t, m, same)
	m.tool.Store = disconnectingGoogleStore{m.store}
	if code, body := m.add(t, familyCalendar); code != http.StatusUnauthorized || errorCode(t, body) != "google_not_connected" {
		t.Fatalf("add after disconnect: %d %s", code, body)
	}
	if _, err := m.store.GetGoogle(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the stale add restored the connection: %v", err)
	}
	if len(g.events) != 0 {
		t.Fatalf("the stale add wrote events: %v", g.events)
	}
}

// A calendar saved by another request of the Member's, after Add read the
// connection, doesn't make Add answer that Google isn't connected: the
// connection is still theirs, and this press's choice is the one kept.
func TestGoogleAddOutlastsACalendarSavedMeanwhile(t *testing.T) {
	ctx := context.Background()
	m, g := newGoogleSite(t)
	connect(t, m, same)
	s := &meanwhile{Memory: m.store, before: func(update int) {
		if update == 1 {
			was, _ := m.store.GetGoogle(ctx, memberEmail)
			_ = m.store.UpdateGoogleCalendar(ctx, was, googleAccount)
		}
	}}
	m.tool.Store = s
	if code, body := m.add(t, familyCalendar); code != http.StatusOK || body != `{"added":2,"failed":0,"updated":0}`+"\n" {
		t.Fatalf("add, with a calendar saved meanwhile: %d %s", code, body)
	}
	if kept, _ := m.store.GetGoogle(ctx, memberEmail); kept.CalendarID != familyCalendar || len(g.events[familyCalendar]) != 2 || len(g.events) != 1 || s.updates != 2 {
		t.Fatalf("kept %+v after %d updates, Google holds %v", kept, s.updates, g.events)
	}

	// A calendar that changes again before each of the three tries is a
	// conflict with a code of its own, which the page doesn't take for a
	// connection that is gone. Nothing is written, and the connection stays.
	s.updates, s.before = 0, func(update int) {
		was, _ := m.store.GetGoogle(ctx, memberEmail)
		_ = m.store.UpdateGoogleCalendar(ctx, was, "calendar-"+strconv.Itoa(update))
	}
	if code, body := m.add(t, googleAccount); code != http.StatusConflict || errorCode(t, body) != "google_conflict" || s.updates != 3 {
		t.Fatalf("add, with the calendar changing every time: %d %s after %d updates", code, body, s.updates)
	}
	if kept, err := m.store.GetGoogle(ctx, memberEmail); err != nil || kept.CalendarID != "calendar-3" || len(g.events) != 1 {
		t.Fatalf("kept %+v, %v, Google holds %v", kept, err, g.events)
	}
}

// Disconnect forgets only the connection it read before it asked Google to
// revoke the token. A second Google account the Member connects while Google
// is answering stays connected, and the disconnect is still done.
func TestGoogleDisconnectLeavesAConnectionMadeMeanwhile(t *testing.T) {
	ctx := context.Background()
	m, g := newGoogleSite(t)
	connect(t, m, same)
	second := store.Google{Email: memberEmail, Account: "second@example.com", Token: "sealed-second", CalendarID: "primary", ConnectionID: "second"}
	g.onRevoke = func() { _ = m.store.PutGoogle(ctx, second) }
	res, body := m.call(t, http.MethodPost, "/api/soccer/google/disconnect", "")
	if res.StatusCode != http.StatusOK || body != `{"available":true,"connected":false}`+"\n" {
		t.Fatalf("disconnect: %d %s", res.StatusCode, body)
	}
	if len(g.revoked) != 1 || g.revoked[0] != "refresh-1" {
		t.Fatalf("revoked %v", g.revoked)
	}
	if kept, err := m.store.GetGoogle(ctx, memberEmail); err != nil || kept != second {
		t.Fatalf("the connection made meanwhile: %+v, %v", kept, err)
	}
}

// A backend-selected ID that collides with an event lacking the site's
// private game marker is reported as failed and never replaced.
func TestGoogleAddLeavesAnUnownedCollision(t *testing.T) {
	m, g := newGoogleSite(t)
	if to := connect(t, m, same); to != "/soccer?google=connected" {
		t.Fatalf("callback sent the Member to %q", to)
	}
	const existing = `{"id":"101","summary":"the Member's event"}`
	g.events[familyCalendar] = map[string]string{"101": existing}
	m.league.events = []map[string]any{{"id": "101", "summary": "ROVERS vs KICKERS"}}
	if code, body := m.add(t, familyCalendar); code != http.StatusOK || body != `{"added":0,"failed":1,"updated":0}`+"\n" {
		t.Fatalf("add: %d %s", code, body)
	}
	if got := g.events[familyCalendar]["101"]; got != existing {
		t.Fatalf("unowned event was replaced with %s", got)
	}
}

func TestGoogleAddUpdatesAnExistingFallbackGame(t *testing.T) {
	m, g := newGoogleSite(t)
	if to := connect(t, m, same); to != "/soccer?google=connected" {
		t.Fatalf("callback sent the Member to %q", to)
	}
	const gameID = "20261108T1830-North Court"
	const eventID = "lps68o34dhh64o3gl1h70pj0baedtp78q108dnnasjk"
	g.events[familyCalendar] = map[string]string{
		eventID: `{"id":"` + eventID + `","summary":"Game","extendedProperties":{"private":{"game_id":"` + gameID + `"}}}`,
	}
	m.league.events = []map[string]any{{
		"id": eventID, "summary": "ROVERS vs KICKERS", "status": "confirmed",
		"start":              map[string]string{"dateTime": "2026-11-08T18:30:00-07:00", "timeZone": "America/Denver"},
		"end":                map[string]string{"dateTime": "2026-11-08T19:20:00-07:00", "timeZone": "America/Denver"},
		"reminders":          map[string]any{"useDefault": false, "overrides": []map[string]any{{"method": "popup", "minutes": 40}}},
		"extendedProperties": map[string]any{"private": map[string]string{"game_id": gameID}},
	}}
	for range 2 {
		if code, body := m.add(t, familyCalendar); code != http.StatusOK || body != `{"added":0,"failed":0,"updated":1}`+"\n" {
			t.Fatalf("add: %d %s", code, body)
		}
	}
	if got := g.events[familyCalendar][eventID]; !strings.Contains(got, `"summary":"ROVERS vs KICKERS"`) || !strings.Contains(got, `"game_id":"`+gameID+`"`) {
		t.Fatalf("Google event = %s", got)
	}
}

func TestGoogleCallbackNeedsTheState(t *testing.T) {
	m, _ := newGoogleSite(t)
	if to := connect(t, m, func(string) string { return "forged" }); to != "/soccer?google=failed" {
		t.Fatalf("callback sent the Member to %q", to)
	}
	// Without Connect's cookie the callback refuses too.
	res, _ := noRedirects.Get(m.url + "/soccer/google/callback?state=x&code=good-code")
	if res.Header.Get("Location") != "/soccer?google=failed" {
		t.Fatalf("no cookie: %q", res.Header.Get("Location"))
	}
	if _, err := m.store.GetGoogle(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("kept: %v", err)
	}
}

// Google's return is taken only for the Member who started Connect.
func TestGoogleCallbackNeedsTheSameMember(t *testing.T) {
	m, _ := newGoogleSite(t)
	res, err := noRedirects.Get(m.url + "/soccer/google/connect")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	to, _ := url.Parse(res.Header.Get("Location"))
	m.tool.Check = func(http.ResponseWriter, *http.Request) (Access, string, error) {
		return Granted, "someone-else@example.com", nil
	}
	req, _ := http.NewRequest(http.MethodGet, m.url+"/soccer/google/callback?"+url.Values{"state": {to.Query().Get("state")}, "code": {"good-code"}}.Encode(), nil)
	for _, c := range res.Cookies() {
		req.AddCookie(c)
	}
	back, err := noRedirects.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	back.Body.Close()
	if back.Header.Get("Location") != "/soccer?google=failed" {
		t.Fatalf("callback sent the other Member to %q", back.Header.Get("Location"))
	}
	if _, err := m.store.GetGoogle(context.Background(), "someone-else@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("kept: %v", err)
	}
}

// A Member who unticks calendar access on Google's screen isn't connected,
// and what Google gave is handed back.
func TestGoogleWithoutCalendarAccess(t *testing.T) {
	m, g := newGoogleSite(t)
	g.scope = "openid email"
	if to := connect(t, m, same); to != "/soccer?google=denied" {
		t.Fatalf("callback sent the Member to %q", to)
	}
	if len(g.revoked) != 1 || g.revoked[0] != "refresh-1" {
		t.Fatalf("revoked %v", g.revoked)
	}
	if _, err := m.store.GetGoogle(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("kept: %v", err)
	}
}

// Without the calendar list, the main calendar is the only choice.
func TestGoogleWithoutTheCalendarList(t *testing.T) {
	m, g := newGoogleSite(t)
	g.scope = "openid email " + google.ScopeEvents
	connect(t, m, same)
	a := m.googleStatus(t)
	if cals, _ := a["calendars"].([]any); a["connected"] != true || len(cals) != 1 || a["calendar_id"] != "primary" {
		t.Fatalf("status = %v", a)
	}
	if code, body := m.add(t, "primary"); code != http.StatusOK || len(g.events["primary"]) != 2 {
		t.Fatalf("add: %d %s", code, body)
	}
}

// Access removed at Google drops the connection, and the Member is asked to
// connect again.
func TestGoogleAccessEnded(t *testing.T) {
	m, g := newGoogleSite(t)
	connect(t, m, same)
	delete(g.refresh, "refresh-1")
	if code, body := m.add(t, familyCalendar); code != http.StatusUnauthorized || errorCode(t, body) != "google_expired" {
		t.Fatalf("add: %d %s", code, body)
	}
	if a := m.googleStatus(t); a["available"] != true || a["connected"] != false {
		t.Fatalf("status = %v", a)
	}
	if code, body := m.add(t, familyCalendar); code != http.StatusUnauthorized || errorCode(t, body) != "google_not_connected" {
		t.Fatalf("add again: %d %s", code, body)
	}
}

func TestGoogleReadOnlyCalendar(t *testing.T) {
	m, g := newGoogleSite(t)
	connect(t, m, same)
	g.readOnly[familyCalendar] = true
	if code, body := m.add(t, familyCalendar); code != http.StatusConflict || errorCode(t, body) != "google_calendar" {
		t.Fatalf("add: %d %s", code, body)
	}
}

// A saved calendar that is gone isn't offered as the choice: the main one is.
func TestGoogleChosenCalendarGone(t *testing.T) {
	m, g := newGoogleSite(t)
	connect(t, m, same)
	if code, body := m.add(t, familyCalendar); code != http.StatusOK {
		t.Fatalf("add: %d %s", code, body)
	}
	g.gone = true
	if a := m.googleStatus(t); a["calendar_id"] != googleAccount {
		t.Fatalf("status = %v", a)
	}
}

// A rate limit isn't a calendar that can't be written to: the games count as
// not added, and the Member presses again.
func TestGoogleRateLimited(t *testing.T) {
	m, g := newGoogleSite(t)
	connect(t, m, same)
	g.limited = true
	if code, body := m.add(t, familyCalendar); code != http.StatusOK || body != `{"added":0,"failed":2,"updated":0}`+"\n" {
		t.Fatalf("add: %d %s", code, body)
	}
}

func TestGoogleNotSetUp(t *testing.T) {
	m := newMemberSite(t)
	if a := m.googleStatus(t); a["available"] != false || a["connected"] != false {
		t.Fatalf("status = %v", a)
	}
	if code, body := m.add(t, "primary"); code != http.StatusServiceUnavailable || errorCode(t, body) != "google_unavailable" {
		t.Fatalf("add: %d %s", code, body)
	}
	res, _ := noRedirects.Get(m.url + "/soccer/google/connect")
	if res.Header.Get("Location") != "/soccer?google=unavailable" {
		t.Fatalf("connect: %q", res.Header.Get("Location"))
	}
}

func TestGoogleNeedsTheGrant(t *testing.T) {
	m, _ := newGoogleSite(t)
	m.access = NoAccess
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/soccer/google"},
		{http.MethodPost, AddGooglePath},
		{http.MethodPost, "/api/soccer/google/disconnect"},
	} {
		if res, body := m.call(t, c.method, c.path, `{"calendar_id":"primary"}`); res.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s: %d %s", c.method, c.path, res.StatusCode, body)
		}
	}
	res, _ := noRedirects.Get(m.url + "/soccer/google/connect")
	if res.Header.Get("Location") != "/soccer?google=failed" {
		t.Fatalf("connect: %q", res.Header.Get("Location"))
	}
	if len(m.league.asked) != 0 {
		t.Fatal("the backend was called")
	}
}
