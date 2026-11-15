package members

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/invite"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
	"github.com/CraigDevJohnson/the-lobby/internal/web"
)

const (
	owner = "owner@example.com"
	kid   = "kid@example.com"
)

type site struct {
	t        *testing.T
	store    *store.Memory
	sessions *session.Manager
	handler  http.Handler
	log      *bytes.Buffer
}

// newSite serves Lobby Admin's routes over a memory store, with the access
// check the real server uses (web.Handler.Member).
func newSite(t *testing.T) *site {
	t.Helper()
	st := store.NewMemory()
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	sessions := &session.Manager{Store: st}
	h := &web.Handler{Store: st, Sessions: sessions, Log: log}
	r := chi.NewRouter()
	(&Tool{Store: st, Check: h.Member, Log: log}).Routes(r)
	ctx := context.Background()
	_ = st.PutMember(ctx, store.Member{Email: owner, Tools: []string{"soccer", Grant}, AddedAt: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)})
	_ = st.PutMember(ctx, store.Member{Email: kid, Tools: []string{"minecraft"}, AddedAt: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)})
	return &site{t: t, store: st, sessions: sessions, handler: r, log: &logged}
}

// signIn starts a Sign-in session for email and returns its cookie.
func (s *site) signIn(email string) *http.Cookie {
	s.t.Helper()
	rec := httptest.NewRecorder()
	if _, err := s.sessions.Start(context.Background(), rec, email); err != nil {
		s.t.Fatal(err)
	}
	return rec.Result().Cookies()[0]
}

func (s *site) do(method, path string, cookie *http.Cookie, body string) (int, map[string]any) {
	s.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (s *site) member(email string) store.Member {
	s.t.Helper()
	m, err := s.store.GetMember(context.Background(), email)
	if err != nil {
		s.t.Fatalf("%s: %v", email, err)
	}
	return m
}

func listed(body map[string]any, email string) map[string]any {
	members, _ := body["members"].([]any)
	for _, m := range members {
		if e, _ := m.(map[string]any); e["email"] == email {
			return e
		}
	}
	return nil
}

// Every route needs lobby-admin: a Visitor is told to sign in, a Member
// without it is refused, and nothing about the Members reaches either.
func TestEveryRouteNeedsLobbyAdmin(t *testing.T) {
	s := newSite(t)
	kidCookie := s.signIn(kid)
	routes := []struct{ method, path, body string }{
		{http.MethodGet, ListPath, ""},
		{http.MethodPost, AddPath, `{"email":"new@example.com","tools":[]}`},
		{http.MethodPost, RemovePath, `{"email":"owner@example.com"}`},
		{http.MethodPost, GrantPath, `{"email":"kid@example.com","tool":"lobby-admin","granted":true}`},
		{http.MethodPost, SignOutPath, `{"email":"owner@example.com"}`},
	}
	for _, r := range routes {
		code, body := s.do(r.method, r.path, nil, r.body)
		if code != http.StatusUnauthorized || body["code"] != "signed_out" {
			t.Errorf("%s %s signed out: %d %v", r.method, r.path, code, body)
		}
		code, body = s.do(r.method, r.path, kidCookie, r.body)
		if code != http.StatusForbidden || body["code"] != "no_access" {
			t.Errorf("%s %s without the grant: %d %v", r.method, r.path, code, body)
		}
		if _, ok := body["members"]; ok {
			t.Errorf("%s %s without the grant listed the Members", r.method, r.path)
		}
	}
	if m := s.member(kid); slices.Contains(m.Tools, Grant) {
		t.Error("a Member without lobby-admin gave themselves it")
	}
	if _, err := s.store.GetMember(context.Background(), "new@example.com"); err == nil {
		t.Error("a Member without lobby-admin added someone")
	}
}

// The list names every Member A to Z with their Tools in the page's order,
// when they were added, when they were last here and on how many browsers,
// and who is asking. A session's ID, the cookie's value, never appears.
func TestTheListShowsWhenEachMemberWasLastHere(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	kidPhone := s.signIn(kid)
	s.signIn(kid)
	// A session last renewed three days ago.
	ctx := context.Background()
	_ = s.store.PutSession(ctx, store.Session{ID: "old", Email: kid, CreatedAt: time.Now().Add(-10 * 24 * time.Hour), ExpiresAt: time.Now().Add(session.Lifetime - 72*time.Hour)})
	_ = s.store.PutMember(ctx, store.Member{Email: "never@example.com", Tools: []string{"foundry-admin", "soccer"}})

	code, body := s.do(http.MethodGet, ListPath, cookie, "")
	if code != http.StatusOK {
		t.Fatalf("list: %d %v", code, body)
	}
	if body["you"] != owner {
		t.Errorf("you: %v", body["you"])
	}
	var order []string
	for _, m := range body["members"].([]any) {
		order = append(order, m.(map[string]any)["email"].(string))
	}
	if !slices.Equal(order, []string{"kid@example.com", "never@example.com", owner}) {
		t.Errorf("order: %v", order)
	}
	k := listed(body, kid)
	if k["devices"] != float64(3) || k["added_at"] != "2026-11-06T00:00:00Z" {
		t.Errorf("kid: %v", k)
	}
	last, err := time.Parse(time.RFC3339, k["last_here"].(string))
	if err != nil || time.Since(last) > time.Minute {
		t.Errorf("kid last here %v (%v), want just now", k["last_here"], err)
	}
	n := listed(body, "never@example.com")
	if n["devices"] != float64(0) || n["last_here"] != nil {
		t.Errorf("never signed in: %v", n)
	}
	if tools := n["tools"].([]any); len(tools) != 2 || tools[0] != "soccer" || tools[1] != "foundry-admin" {
		t.Errorf("tools in the page's order: %v", tools)
	}
	raw, _ := json.Marshal(body)
	for _, id := range []string{cookie.Value, kidPhone.Value, `"old"`} {
		if strings.Contains(string(raw), id) {
			t.Errorf("the list carries a session ID %q: %s", id, raw)
		}
	}
}

// An add writes a new record with the Tools ticked, and refuses an address
// that is already a Member, so it never replaces anyone's Tools, as well as
// anything that isn't an address or a Tool.
func TestAddInvitesANewMemberOnly(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	code, body := s.do(http.MethodPost, AddPath, cookie, `{"email":"  New@Example.com ","tools":["foundry","soccer","foundry"]}`)
	if code != http.StatusOK || listed(body, "new@example.com") == nil {
		t.Fatalf("add: %d %v", code, body)
	}
	if _, ok := body["invite"]; ok {
		t.Errorf("a site that sends no email answered about one: %v", body["invite"])
	}
	m := s.member("new@example.com")
	if !slices.Equal(m.Tools, []string{"soccer", "foundry"}) || m.AddedAt.IsZero() {
		t.Errorf("added: %+v", m)
	}

	code, body = s.do(http.MethodPost, AddPath, cookie, `{"email":"kid@example.com","tools":[]}`)
	if code != http.StatusConflict || body["code"] != "already_member" {
		t.Errorf("add an existing Member: %d %v", code, body)
	}
	if !slices.Equal(s.member(kid).Tools, []string{"minecraft"}) {
		t.Errorf("an add replaced a Member's Tools: %v", s.member(kid).Tools)
	}

	for _, c := range []struct{ body, code string }{
		{`{"email":"not-an-address","tools":[]}`, "invalid_email"},
		{`{"email":"a@b","tools":[]}`, "invalid_email"},
		{`{"email":"two words@example.com","tools":[]}`, "invalid_email"},
		{`{"email":"a@b@example.com","tools":[]}`, "invalid_email"},
		{`{"email":"x@example.com","tools":["admin"]}`, "unknown_tool"},
		{`{"tools":[]}`, "bad_request"},
		{`{"email":"x@example.com","tools":[]} {}`, "bad_request"},
	} {
		code, body := s.do(http.MethodPost, AddPath, cookie, c.body)
		if code/100 != 4 || body["code"] != c.code {
			t.Errorf("%s: %d %v, want %s", c.body, code, body, c.code)
		}
	}
	if _, err := s.store.GetMember(context.Background(), "x@example.com"); err == nil {
		t.Error("a refused add wrote a record")
	}
}

// outbox stands in for the invite sender: it keeps each email it is given
// and answers with err.
type outbox struct {
	emails []invite.Email
	err    error
}

// withInvites serves s's routes again with o as the invite sender and
// worlds as the Minecraft worlds' reader.
func (s *site) withInvites(o *outbox, worlds func(context.Context) ([]invite.World, error)) {
	log := slog.New(slog.NewTextHandler(s.log, nil))
	h := &web.Handler{Store: s.store, Sessions: s.sessions, Log: log}
	r := chi.NewRouter()
	(&Tool{
		Store:   s.store,
		Check:   h.Member,
		Log:     log,
		BaseURL: "https://next.example.com",
		Invite: func(_ context.Context, e invite.Email) error {
			o.emails = append(o.emails, e)
			return o.err
		},
		Worlds: worlds,
	}).Routes(r)
	s.handler = r
}

// An add emails the new Member their invite, with the worlds' addresses for
// the Launcher's grant, and says so. A later change to their Tools sends
// nothing, and the log never names them.
func TestAddEmailsTheNewMemberTheirInvite(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	o := &outbox{}
	reads := 0
	s.withInvites(o, func(context.Context) ([]invite.World, error) {
		reads++
		return []invite.World{{Name: "Survival", JavaAddress: "survival.mc.example.net", BedrockAddress: "survival.mc.example.net", BedrockPort: 19132}}, nil
	})

	code, body := s.do(http.MethodPost, AddPath, cookie, `{"email":"New@example.com","tools":["minecraft"]}`)
	if code != http.StatusOK || body["invite"] != "sent" || listed(body, "new@example.com") == nil {
		t.Fatalf("add: %d %v", code, body)
	}
	if len(o.emails) != 1 || reads != 1 {
		t.Fatalf("%d emails, worlds read %d times", len(o.emails), reads)
	}
	e := o.emails[0]
	for _, want := range []string{"Enter new@example.com.", "- Minecraft Launcher: https://next.example.com/minecraft", "- Survival: survival.mc.example.net (Bedrock: port 19132)"} {
		if e.To != "new@example.com" || !strings.Contains(e.Body, want) {
			t.Errorf("to %q, missing %q in:\n%s", e.To, want, e.Body)
		}
	}

	code, body = s.do(http.MethodPost, GrantPath, cookie, `{"email":"new@example.com","tool":"foundry","granted":true}`)
	if code != http.StatusOK || len(o.emails) != 1 || body["invite"] != nil {
		t.Errorf("a grant change: %d, %d emails, invite %v", code, len(o.emails), body["invite"])
	}
	if !strings.Contains(s.log.String(), "invite email sent") || strings.Contains(s.log.String(), "new@example.com") {
		t.Errorf("log: %s", s.log.String())
	}
}

// The add stands when the email doesn't go, and the answer says so. The log
// says at which stage and with which reply, not to whom. Without the
// Launcher's grant the worlds aren't read.
func TestAFailedInviteKeepsTheAdd(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	o := &outbox{err: &invite.Error{Stage: "rcpt", Code: 550}}
	s.withInvites(o, func(context.Context) ([]invite.World, error) {
		t.Error("the worlds were read for a Member without the Launcher")
		return nil, nil
	})

	code, body := s.do(http.MethodPost, AddPath, cookie, `{"email":"new@example.com","tools":["foundry"]}`)
	if code != http.StatusOK || body["invite"] != "failed" || listed(body, "new@example.com") == nil {
		t.Fatalf("add: %d %v", code, body)
	}
	s.member("new@example.com")
	if log := s.log.String(); !strings.Contains(log, "invite email not sent") || !strings.Contains(log, "rcpt: 550") || strings.Contains(log, "new@example.com") {
		t.Errorf("log: %s", log)
	}
}

// When the worlds can't be read the invite still goes, linking to the
// Launcher instead.
func TestAnInviteWithoutTheWorldsLinksToTheLauncher(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	o := &outbox{}
	s.withInvites(o, func(context.Context) ([]invite.World, error) { return nil, errors.New("the backend answered 503") })

	code, body := s.do(http.MethodPost, AddPath, cookie, `{"email":"new@example.com","tools":["minecraft"]}`)
	if code != http.StatusOK || body["invite"] != "sent" || len(o.emails) != 1 {
		t.Fatalf("add: %d %v, %d emails", code, body, len(o.emails))
	}
	if !strings.Contains(o.emails[0].Body, "The page shows each world’s address:\nhttps://next.example.com/minecraft\n") {
		t.Errorf("body:\n%s", o.emails[0].Body)
	}
}

// A change must come from The Lobby's own pages as JSON: another site's
// request, or a form, changes nothing.
func TestAChangeComesFromTheLobbyAsJSON(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	req := httptest.NewRequest(http.MethodPost, AddPath, strings.NewReader(`{"email":"x@example.com","tools":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cross_site") {
		t.Errorf("cross-site: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, AddPath, strings.NewReader("email=x%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("a form: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := s.store.GetMember(context.Background(), "x@example.com"); err == nil {
		t.Error("a refused change wrote a record")
	}
}

// A grant goes on or off by itself: the Member's other Tools, any the page
// has no box for, and the day they were added stay. Nobody takes their own
// lobby-admin, and an address that isn't a Member is said so.
func TestAGrantChangesOneToolAndKeepsTheRest(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	ctx := context.Background()
	added := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	_ = s.store.PutMember(ctx, store.Member{Email: kid, Tools: []string{"minecraft", "retired-tool"}, AddedAt: added})

	code, body := s.do(http.MethodPost, GrantPath, cookie, `{"email":"Kid@example.com","tool":"foundry","granted":true}`)
	if code != http.StatusOK {
		t.Fatalf("grant: %d %v", code, body)
	}
	if m := s.member(kid); !slices.Equal(m.Tools, []string{"minecraft", "foundry", "retired-tool"}) || !m.AddedAt.Equal(added) {
		t.Errorf("after granting foundry: %+v", m)
	}
	if tools := listed(body, kid)["tools"].([]any); len(tools) != 3 {
		t.Errorf("the answer's list: %v", tools)
	}
	code, _ = s.do(http.MethodPost, GrantPath, cookie, `{"email":"kid@example.com","tool":"minecraft","granted":false}`)
	if m := s.member(kid); code != http.StatusOK || !slices.Equal(m.Tools, []string{"foundry", "retired-tool"}) {
		t.Errorf("after taking minecraft: %d %v", code, m.Tools)
	}
	// Granting what they hold, or taking what they don't, changes nothing.
	s.do(http.MethodPost, GrantPath, cookie, `{"email":"kid@example.com","tool":"foundry","granted":true}`)
	s.do(http.MethodPost, GrantPath, cookie, `{"email":"kid@example.com","tool":"soccer","granted":false}`)
	if m := s.member(kid); !slices.Equal(m.Tools, []string{"foundry", "retired-tool"}) {
		t.Errorf("a repeated change changed something: %v", m.Tools)
	}

	code, body = s.do(http.MethodPost, GrantPath, cookie, `{"email":"owner@example.com","tool":"lobby-admin","granted":false}`)
	if code != http.StatusConflict || body["code"] != "self" || !slices.Contains(s.member(owner).Tools, Grant) {
		t.Errorf("taking your own lobby-admin: %d %v", code, body)
	}
	// Your other Tools are yours to change.
	if code, _ := s.do(http.MethodPost, GrantPath, cookie, `{"email":"owner@example.com","tool":"soccer","granted":false}`); code != http.StatusOK {
		t.Errorf("taking your own soccer: %d", code)
	}

	for _, c := range []struct{ body, code string }{
		{`{"email":"nobody@example.com","tool":"soccer","granted":true}`, "not_member"},
		{`{"email":"kid@example.com","tool":"retired-tool","granted":true}`, "unknown_tool"},
		{`{"email":"kid@example.com","tool":"soccer"}`, "bad_request"},
	} {
		code, body := s.do(http.MethodPost, GrantPath, cookie, c.body)
		if code/100 != 4 || body["code"] != c.code {
			t.Errorf("%s: %d %v, want %s", c.body, code, body, c.code)
		}
	}
}

// Removing a Member deletes their record, their calendar and their Sign-in
// sessions. Nobody removes themselves.
func TestRemoveEndsTheMemberAndTheirSessions(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	kidCookie := s.signIn(kid)
	ctx := context.Background()
	_ = s.store.PutSoccer(ctx, store.Soccer{Email: kid, Secret: "s1"})
	_ = s.store.PutSoccerLink(ctx, "s1", kid)

	code, body := s.do(http.MethodPost, RemovePath, cookie, `{"email":"kid@example.com"}`)
	if code != http.StatusOK || listed(body, kid) != nil {
		t.Fatalf("remove: %d %v", code, body)
	}
	if _, err := s.store.GetMember(ctx, kid); err == nil {
		t.Error("the record stayed")
	}
	if _, err := s.store.GetSession(ctx, kidCookie.Value); err == nil {
		t.Error("their session stayed")
	}
	if _, err := s.store.SoccerLink(ctx, "s1"); err == nil {
		t.Error("their Member link still works")
	}

	code, body = s.do(http.MethodPost, RemovePath, cookie, `{"email":"Owner@Example.com"}`)
	if code != http.StatusConflict || body["code"] != "self" {
		t.Errorf("removing yourself: %d %v", code, body)
	}
	code, body = s.do(http.MethodPost, RemovePath, cookie, `{"email":"kid@example.com"}`)
	if code != http.StatusNotFound || body["code"] != "not_member" {
		t.Errorf("removing someone who isn't a Member: %d %v", code, body)
	}
	if strings.Contains(s.log.String(), kid) {
		t.Errorf("the log names a Member: %s", s.log.String())
	}
}

// noSessions is a store whose Sign-in sessions can't be listed.
type noSessions struct{ store.Store }

func (noSessions) ListSessions(context.Context) ([]store.Session, error) {
	return nil, errors.New("scan failed")
}

// A Member whose sessions can't be ended stays a Member, so Remove can be
// tried again: removing the record first would leave their old sessions to
// come back if the address were added again.
func TestRemoveKeepsTheMemberWhenTheirSessionsCanNotBeEnded(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	kidCookie := s.signIn(kid)
	h := &web.Handler{Store: s.store, Sessions: s.sessions, Log: slog.New(slog.NewTextHandler(s.log, nil))}
	r := chi.NewRouter()
	(&Tool{Store: noSessions{s.store}, Check: h.Member, Log: slog.New(slog.NewTextHandler(s.log, nil))}).Routes(r)
	s.handler = r

	code, body := s.do(http.MethodPost, RemovePath, cookie, `{"email":"kid@example.com"}`)
	if code != http.StatusServiceUnavailable || body["code"] != "unavailable" {
		t.Fatalf("remove: %d %v", code, body)
	}
	s.member(kid)
	if _, err := s.store.GetSession(context.Background(), kidCookie.Value); err != nil {
		t.Errorf("their session: %v", err)
	}
}

// Sign out everywhere ends every one of a Member's sessions and keeps their
// record; for the person asking, it keeps the browser they asked from.
func TestSignOutEndsSessionsButTheOneAsking(t *testing.T) {
	s := newSite(t)
	cookie := s.signIn(owner)
	otherOwner := s.signIn(owner)
	kidA, kidB := s.signIn(kid), s.signIn(kid)
	ctx := context.Background()

	code, body := s.do(http.MethodPost, SignOutPath, cookie, `{"email":"kid@example.com"}`)
	if code != http.StatusOK || listed(body, kid)["devices"] != float64(0) {
		t.Fatalf("sign out: %d %v", code, body)
	}
	for _, c := range []*http.Cookie{kidA, kidB} {
		if _, err := s.store.GetSession(ctx, c.Value); err == nil {
			t.Error("a session stayed")
		}
	}
	if !slices.Equal(s.member(kid).Tools, []string{"minecraft"}) {
		t.Error("signing out changed the record")
	}

	code, body = s.do(http.MethodPost, SignOutPath, cookie, `{"email":"owner@example.com"}`)
	if code != http.StatusOK || listed(body, owner)["devices"] != float64(1) {
		t.Fatalf("sign out your other browsers: %d %v", code, body)
	}
	if _, err := s.store.GetSession(ctx, cookie.Value); err != nil {
		t.Error("the browser asking was signed out")
	}
	if _, err := s.store.GetSession(ctx, otherOwner.Value); err == nil {
		t.Error("your other browser stayed signed in")
	}
}

// The grants the server accepts are the Tools the page offers: the ids in
// web/src/tools.ts, in the same order.
func TestGrantsMatchThePagesTools(t *testing.T) {
	src, err := os.ReadFile("../../web/src/tools.ts")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range regexp.MustCompile(`\{ id: "([a-z-]+)"`).FindAllStringSubmatch(string(src), -1) {
		ids = append(ids, m[1])
	}
	if !slices.Equal(ids, Grants) {
		t.Errorf("web/src/tools.ts has %v; the server accepts %v", ids, Grants)
	}
}
