package wish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/members"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
	"github.com/CraigDevJohnson/the-lobby/internal/web"
)

// Synthetic Members and opaque ids, nothing from a real family.
const (
	kidEmail   = "kid@example.com"
	sibEmail   = "sib@example.com"
	ownerEmail = "owner@example.com"
	plainEmail = "plain@example.com"

	listA = "0123456789abcdef0123456789abcdef"
	listB = "fedcba9876543210fedcba9876543210"
	listC = "aaaaaaaaaaaaaaaabbbbbbbbbbbbbbbb"
	wish1 = "11111111111111111111111111111111"
	wish2 = "22222222222222222222222222222222"
)

type site struct {
	t        *testing.T
	srv      *httptest.Server
	store    *store.Memory
	sessions *session.Manager
	log      *bytes.Buffer
	wrap     *racing
}

// racing lets a test change the table between the access check and the
// write, as another request could.
type racing struct {
	store.Store
	mu     sync.Mutex
	before func()
}

func (r *racing) first() {
	r.mu.Lock()
	f := r.before
	r.before = nil
	r.mu.Unlock()
	if f != nil {
		f()
	}
}

func (r *racing) SaveWishList(ctx context.Context, by store.WishEditor, l store.WishList) (store.WishList, error) {
	r.first()
	return r.Store.SaveWishList(ctx, by, l)
}

func (r *racing) DeleteWishList(ctx context.Context, by store.WishEditor, l store.WishList) error {
	r.first()
	return r.Store.DeleteWishList(ctx, by, l)
}

func newSite(t *testing.T) *site {
	t.Helper()
	st := store.NewMemory()
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	sessions := &session.Manager{Store: st}
	h := &web.Handler{Store: st, Sessions: sessions, Log: log}
	wrap := &racing{Store: st}
	tool := &Tool{Store: wrap, Check: h.CurrentMember, Log: log}
	r := chi.NewRouter()
	tool.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	s := &site{t: t, srv: srv, store: st, sessions: sessions, log: &logged, wrap: wrap}
	s.member(kidEmail, "kid-1", Grant)
	s.member(sibEmail, "sib-1", Grant)
	s.member(ownerEmail, "owner-1", Grant, members.Grant)
	s.member(plainEmail, "plain-1", "soccer")
	return s
}

func (s *site) member(email, generation string, tools ...string) {
	s.t.Helper()
	if err := s.store.PutMember(context.Background(), store.Member{Email: email, Tools: tools, Generation: generation}); err != nil {
		s.t.Fatal(err)
	}
}

// grants replaces a Member's grants as Lobby Admin does, keeping their
// membership.
func (s *site) grants(email string, tools ...string) {
	s.t.Helper()
	m, err := s.store.GetMember(context.Background(), email)
	if err != nil {
		s.t.Fatal(err)
	}
	m.Tools = tools
	if err := s.store.ReplaceMember(context.Background(), m); err != nil {
		s.t.Fatal(err)
	}
}

func (s *site) signIn(email string) *http.Cookie {
	s.t.Helper()
	rec := httptest.NewRecorder()
	if _, err := s.sessions.Start(context.Background(), rec, email); err != nil {
		s.t.Fatal(err)
	}
	return rec.Result().Cookies()[0]
}

type answer struct {
	status int
	body   string
	header http.Header
}

func (a answer) code() string {
	var p struct{ Code string }
	_ = json.Unmarshal([]byte(a.body), &p)
	return p.Code
}

type family struct {
	Lists     []listJSON `json:"lists"`
	CanManage bool       `json:"can_manage"`
}

func (s *site) do(method, path string, cookie *http.Cookie, body string, header ...string) answer {
	s.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, s.srv.URL+path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := s.srv.Client().Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		s.t.Errorf("%s %s answered %d with Cache-Control %q", method, path, res.StatusCode, cc)
	}
	return answer{res.StatusCode, string(b), res.Header}
}

func (s *site) family(cookie *http.Cookie) family {
	s.t.Helper()
	a := s.do(http.MethodGet, FamilyPath, cookie, "")
	if a.status != http.StatusOK {
		s.t.Fatalf("GET %s: %d %s", FamilyPath, a.status, a.body)
	}
	var f family
	if err := json.Unmarshal([]byte(a.body), &f); err != nil {
		s.t.Fatal(err)
	}
	return f
}

func (s *site) save(cookie *http.Cookie, l any) answer {
	s.t.Helper()
	b, err := json.Marshal(l)
	if err != nil {
		s.t.Fatal(err)
	}
	return s.do(http.MethodPut, ListsPath, cookie, string(b))
}

func (s *site) del(cookie *http.Cookie, id, kind string, revision int64) answer {
	s.t.Helper()
	return s.do(http.MethodDelete, ListsPath, cookie, `{"id":"`+id+`","kind":"`+kind+`","revision":`+strconv.FormatInt(revision, 10)+`}`)
}

type input struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Name     string     `json:"name"`
	Revision int64      `json:"revision"`
	Wishes   []wishJSON `json:"wishes"`
}

func listOf(a answer) listJSON {
	var l listJSON
	_ = json.Unmarshal([]byte(a.body), &l)
	return l
}

func names(f family) []string {
	out := []string{}
	for _, l := range f.Lists {
		out = append(out, l.Name)
	}
	return out
}

// Every request checks the Sign-in session, the membership and the grant
// again. A Visitor is signed out, a Member without the grant has no
// access, a session from an earlier invitation of the address counts for
// nothing, and a check or a read that fails says so. None of them learns
// anything about the lists.
func TestEveryRequestChecksAccess(t *testing.T) {
	s := newSite(t)
	kid := s.signIn(kidEmail)
	if a := s.save(kid, input{ID: listA, Kind: store.WishPersonal, Name: "Kid", Wishes: []wishJSON{}}); a.status != http.StatusOK {
		t.Fatalf("create: %d %s", a.status, a.body)
	}
	plain := s.signIn(plainEmail)
	stale := s.signIn(sibEmail)
	s.member(sibEmail, "sib-2", Grant) // removed and invited again
	change := `{"id":"` + listA + `","kind":"personal","name":"X","revision":1,"wishes":[]}`
	for name, c := range map[string]struct {
		cookie *http.Cookie
		status int
		code   string
	}{
		"a Visitor":               {nil, http.StatusUnauthorized, "signed_out"},
		"an unknown session":      {&http.Cookie{Name: session.CookieName, Value: "nobody"}, http.StatusUnauthorized, "signed_out"},
		"a Member without access": {plain, http.StatusForbidden, "no_access"},
		"an earlier invitation's": {stale, http.StatusUnauthorized, "signed_out"},
	} {
		for _, a := range []answer{
			s.do(http.MethodGet, FamilyPath, c.cookie, ""),
			s.do(http.MethodPut, ListsPath, c.cookie, change),
			s.do(http.MethodDelete, ListsPath, c.cookie, `{"id":"`+listA+`","kind":"personal","revision":1}`),
		} {
			if a.status != c.status || a.code() != c.code || strings.Contains(a.body, "Kid") {
				t.Errorf("%s: %d %s, want %d %s", name, a.status, a.body, c.status, c.code)
			}
		}
	}

	// A check that fails is not a Visitor, and a read that fails is not an
	// empty family.
	broken := &Tool{Store: s.store, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Check: func(http.ResponseWriter, *http.Request) (store.Member, bool, error) {
		return store.Member{}, false, errors.New("the table stopped answering")
	}}
	rec := httptest.NewRecorder()
	broken.family(rec, httptest.NewRequest(http.MethodGet, FamilyPath, nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"access_unavailable"`) {
		t.Errorf("a failed check: %d %s", rec.Code, rec.Body.String())
	}
	failing := &Tool{Store: failingList{s.store}, Log: broken.Log, Check: func(http.ResponseWriter, *http.Request) (store.Member, bool, error) {
		return store.Member{Email: kidEmail, Tools: []string{Grant}, Generation: "kid-1"}, true, nil
	}}
	rec = httptest.NewRecorder()
	failing.family(rec, httptest.NewRequest(http.MethodGet, FamilyPath, nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"unavailable"`) {
		t.Errorf("a failed read: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	(&Tool{Log: broken.Log}).family(rec, httptest.NewRequest(http.MethodGet, FamilyPath, nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"access_unavailable"`) {
		t.Errorf("an unconfigured Tool: %d %s", rec.Code, rec.Body.String())
	}
}

type failingList struct{ store.Store }

func (failingList) ListWishLists(context.Context) ([]store.WishList, error) {
	return nil, errors.New("the table stopped answering")
}

// A Member with the grant sees every list and edits their own personal
// list, one per Member. The Owner also edits the managed lists, and can't
// edit anyone else's personal list: a personal change always goes to the
// asker's own. Lists come by name, then id, and never carry an address or a
// generation.
func TestMembersEditTheirOwnListAndTheOwnerTheManagedOnes(t *testing.T) {
	s := newSite(t)
	kid, sib, owner := s.signIn(kidEmail), s.signIn(sibEmail), s.signIn(ownerEmail)
	if f := s.family(kid); f.Lists == nil || len(f.Lists) != 0 || f.CanManage {
		t.Fatalf("an empty family: %+v", f)
	}
	a := s.save(kid, input{ID: listA, Kind: store.WishPersonal, Name: "  Kid  ", Wishes: []wishJSON{
		{ID: wish1, Title: " Kite ", URL: " https://example.com/kite ", Notes: "  "},
		{ID: wish2, Title: "Book", Notes: " the blue one\n"},
	}})
	if a.status != http.StatusOK {
		t.Fatalf("create: %d %s", a.status, a.body)
	}
	kidList := listOf(a)
	if l := kidList; !opaqueID(l.ID) || l.Kind != "personal" || l.Name != "Kid" || l.Revision != 1 || !l.Editable ||
		!slices.Equal(l.Wishes, []wishJSON{{ID: wish1, Title: "Kite", URL: "https://example.com/kite"}, {ID: wish2, Title: "Book", Notes: "the blue one"}}) {
		t.Fatalf("created %s", a.body)
	}
	if strings.Contains(a.body, `"notes":""`) || strings.Contains(a.body, `"url":""`) {
		t.Errorf("empty fields were sent: %s", a.body)
	}
	if a := s.save(kid, input{ID: listB, Kind: store.WishPersonal, Name: "Again", Wishes: []wishJSON{}}); a.status != http.StatusConflict || a.code() != "conflict" {
		t.Errorf("a second personal list: %d %s", a.status, a.body)
	}

	// Only the Owner makes managed lists.
	managed := input{ID: listB, Kind: store.WishManaged, Name: "baby", Wishes: []wishJSON{}}
	if a := s.save(kid, managed); a.status != http.StatusForbidden || a.code() != "not_editable" {
		t.Errorf("a Member's managed list: %d %s", a.status, a.body)
	}
	createdManaged := s.save(owner, managed)
	if createdManaged.status != http.StatusOK || !listOf(createdManaged).Editable {
		t.Fatalf("the Owner's managed list: %d %s", createdManaged.status, createdManaged.body)
	}
	managedID := listOf(createdManaged).ID
	if a := s.save(owner, input{ID: listC, Kind: store.WishPersonal, Name: "Owner", Wishes: []wishJSON{}}); a.status != http.StatusOK {
		t.Fatalf("the Owner's personal list: %d %s", a.status, a.body)
	}

	for _, c := range []struct {
		name   string
		cookie *http.Cookie
		manage bool
		edits  []string
	}{
		{"the kid", kid, false, []string{"Kid"}},
		{"the sibling", sib, false, nil},
		{"the Owner", owner, true, []string{"baby", "Owner"}},
	} {
		a := s.do(http.MethodGet, FamilyPath, c.cookie, "")
		for _, private := range []string{"@example.com", "kid-1", "owner-1", "owner\""} {
			if strings.Contains(a.body, private) {
				t.Errorf("%s was told %q: %s", c.name, private, a.body)
			}
		}
		var f family
		_ = json.Unmarshal([]byte(a.body), &f)
		if !slices.Equal(names(f), []string{"baby", "Kid", "Owner"}) || f.CanManage != c.manage {
			t.Errorf("%s sees %v, can manage %v", c.name, names(f), f.CanManage)
		}
		var edits []string
		for _, l := range f.Lists {
			if l.Editable {
				edits = append(edits, l.Name)
			}
		}
		if !slices.Equal(edits, c.edits) {
			t.Errorf("%s may edit %v, want %v", c.name, edits, c.edits)
		}
	}

	// The Owner's change to the kid's list goes to the Owner's own personal
	// list, which is another list, so it is refused, and so is a delete.
	if a := s.save(owner, input{ID: kidList.ID, Kind: store.WishPersonal, Name: "Changed", Revision: 1, Wishes: []wishJSON{}}); a.status != http.StatusConflict {
		t.Errorf("the Owner changing the kid's list: %d %s", a.status, a.body)
	}
	if a := s.del(owner, kidList.ID, store.WishPersonal, 1); a.status != http.StatusConflict {
		t.Errorf("the Owner deleting the kid's list: %d %s", a.status, a.body)
	}
	if a := s.del(sib, kidList.ID, store.WishPersonal, 1); a.status != http.StatusConflict {
		t.Errorf("a sibling deleting the kid's list: %d %s", a.status, a.body)
	}
	reused := []answer{
		s.save(sib, input{ID: listA, Kind: store.WishPersonal, Name: "Mine", Wishes: []wishJSON{}}),
		s.save(owner, input{ID: listA, Kind: store.WishManaged, Name: "Copy", Wishes: []wishJSON{}}),
	}
	seen := map[string]bool{kidList.ID: true}
	for _, a := range reused {
		l := listOf(a)
		if a.status != http.StatusOK || !opaqueID(l.ID) || seen[l.ID] {
			t.Fatalf("creating a distinct list with the same draft id: %d %s", a.status, a.body)
		}
		seen[l.ID] = true
		cookie := sib
		if l.Kind == store.WishManaged {
			cookie = owner
		}
		if a := s.del(cookie, l.ID, l.Kind, 1); a.status != http.StatusNoContent {
			t.Fatalf("remove the new list: %d %s", a.status, a.body)
		}
	}
	if a := s.del(kid, managedID, store.WishManaged, 1); a.status != http.StatusForbidden || a.code() != "not_editable" {
		t.Errorf("the kid deleting a managed list: %d %s", a.status, a.body)
	}
	if f := s.family(sib); f.Lists[1].Name != "Kid" || f.Lists[1].Revision != 1 || len(f.Lists[1].Wishes) != 2 {
		t.Fatalf("the kid's list changed: %+v", f.Lists[1])
	}

	// Names sort without regard to case, and the same name by id.
	second := s.save(owner, input{ID: wish2, Kind: store.WishManaged, Name: "Baby", Wishes: []wishJSON{}})
	if second.status != http.StatusOK {
		t.Fatal(second.body)
	}
	secondID := listOf(second).ID
	f := s.family(kid)
	if len(f.Lists) != 4 || f.Lists[0].ID != min(managedID, secondID) || f.Lists[1].ID != max(managedID, secondID) ||
		!slices.Equal(names(f)[2:], []string{"Kid", "Owner"}) {
		t.Errorf("the order: %v", names(f))
	}
	if a := s.del(owner, secondID, store.WishManaged, 1); a.status != http.StatusNoContent || a.body != "" {
		t.Errorf("the Owner deleting a managed list: %d %q", a.status, a.body)
	}
}

// A save names the revision it read, and a delete the same. A list that
// has moved on since, or is gone, is a conflict, and the list stays as it
// was saved. So a retried save or create never writes twice.
func TestStaleWritesAreConflicts(t *testing.T) {
	s := newSite(t)
	kid := s.signIn(kidEmail)
	first := input{ID: listA, Kind: store.WishPersonal, Name: "Kid", Wishes: []wishJSON{{ID: wish1, Title: "Kite"}}}
	created := s.save(kid, first)
	if created.status != http.StatusOK {
		t.Fatal(created.body)
	}
	listID := listOf(created).ID
	if a := s.save(kid, first); a.status != http.StatusConflict || a.code() != "conflict" {
		t.Errorf("the create retried: %d %s", a.status, a.body)
	}
	change := first
	change.ID, change.Revision, change.Name = listID, 1, "Kid's"
	if a := s.save(kid, change); a.status != http.StatusOK || listOf(a).Revision != 2 {
		t.Fatalf("save: %d %s", a.status, a.body)
	}
	for name, l := range map[string]input{
		"the save retried":    change,
		"an older revision":   {ID: listID, Kind: store.WishPersonal, Name: "Old", Revision: 1, Wishes: []wishJSON{}},
		"a revision not seen": {ID: listID, Kind: store.WishPersonal, Name: "Ahead", Revision: 7, Wishes: []wishJSON{}},
		"another id":          {ID: listB, Kind: store.WishPersonal, Name: "Other", Revision: 2, Wishes: []wishJSON{}},
	} {
		if a := s.save(kid, l); a.status != http.StatusConflict || a.code() != "conflict" {
			t.Errorf("%s: %d %s", name, a.status, a.body)
		}
	}
	if a := s.del(kid, listID, store.WishPersonal, 1); a.status != http.StatusConflict {
		t.Errorf("a delete at an older revision: %d %s", a.status, a.body)
	}
	if f := s.family(kid); len(f.Lists) != 1 || f.Lists[0].Name != "Kid's" || f.Lists[0].Revision != 2 {
		t.Fatalf("after the stale writes: %+v", f)
	}
	if a := s.del(kid, listID, store.WishPersonal, 2); a.status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", a.status, a.body)
	}
	if a := s.del(kid, listID, store.WishPersonal, 2); a.status != http.StatusConflict {
		t.Errorf("the delete retried: %d %s", a.status, a.body)
	}
	if a := s.save(kid, change); a.status != http.StatusConflict {
		t.Errorf("a save to a deleted list: %d %s", a.status, a.body)
	}
	if f := s.family(kid); len(f.Lists) != 0 {
		t.Fatalf("after the delete: %+v", f)
	}
}

// A change must come from The Lobby's own pages, as JSON, and be a whole,
// valid list. Anything else is refused before anything is written.
func TestMalformedAndCrossSiteChangesAreRefused(t *testing.T) {
	s := newSite(t)
	kid := s.signIn(kidEmail)
	valid := `{"id":"` + listA + `","kind":"personal","name":"Kid","revision":0,"wishes":[]}`
	for name, header := range map[string][]string{
		"another site":        {"Sec-Fetch-Site", "cross-site"},
		"a sibling subdomain": {"Sec-Fetch-Site", "same-site"},
		"a typed address":     {"Sec-Fetch-Site", "none"},
	} {
		for _, a := range []answer{
			s.do(http.MethodPut, ListsPath, kid, valid, header...),
			s.do(http.MethodDelete, ListsPath, kid, `{"id":"`+listA+`","kind":"personal","revision":1}`, header...),
		} {
			if a.status != http.StatusForbidden || a.code() != "cross_site" {
				t.Errorf("%s: %d %s", name, a.status, a.body)
			}
		}
	}
	for name, ct := range map[string]string{"a form": "application/x-www-form-urlencoded", "text": "text/plain", "none": ""} {
		req, _ := http.NewRequest(http.MethodPut, s.srv.URL+ListsPath, strings.NewReader(valid))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		req.AddCookie(kid)
		res, err := s.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("%s: %d", name, res.StatusCode)
		}
	}

	long := func(n int) string { return strings.Repeat("é", n) }
	link := func(n int) string { return "https://example.com/" + strings.Repeat("a", n-len("https://example.com/")) }
	many := func(n int) string {
		var out []string
		for i := range n {
			id := strconv.FormatInt(int64(i), 16)
			out = append(out, `{"id":"`+strings.Repeat("0", 32-len(id))+id+`","title":"t"}`)
		}
		return "[" + strings.Join(out, ",") + "]"
	}
	body := func(name, wishes string) string {
		return `{"id":"` + listA + `","kind":"personal","name":` + strconv.Quote(name) + `,"revision":0,"wishes":` + wishes + `}`
	}
	one := func(fields string) string { return body("Kid", `[{"id":"`+wish1+`",`+fields+`}]`) }
	for name, b := range map[string]string{
		"not JSON":              `{"id":`,
		"two objects":           valid + valid,
		"an array":              `[` + valid + `]`,
		"no id":                 `{"kind":"personal","name":"Kid","revision":0,"wishes":[]}`,
		"no kind":               `{"id":"` + listA + `","name":"Kid","revision":0,"wishes":[]}`,
		"no name":               `{"id":"` + listA + `","kind":"personal","revision":0,"wishes":[]}`,
		"no revision":           `{"id":"` + listA + `","kind":"personal","name":"Kid","wishes":[]}`,
		"no wishes":             `{"id":"` + listA + `","kind":"personal","name":"Kid","revision":0}`,
		"null wishes":           body("Kid", `null`),
		"an id in capitals":     strings.Replace(valid, listA, strings.ToUpper(listA), 1),
		"a short id":            strings.Replace(valid, listA, listA[:31], 1),
		"a uuid with hyphens":   strings.Replace(valid, listA, "01234567-89ab-cdef-0123-456789abcdef", 1),
		"another kind":          strings.Replace(valid, `"personal"`, `"shared"`, 1),
		"a negative revision":   strings.Replace(valid, `"revision":0`, `"revision":-1`, 1),
		"a revision past 2^53":  strings.Replace(valid, `"revision":0`, `"revision":9007199254740992`, 1),
		"a fractional revision": strings.Replace(valid, `"revision":0`, `"revision":1.5`, 1),
		"a revision as text":    strings.Replace(valid, `"revision":0`, `"revision":"1"`, 1),
		"a blank name":          body("   ", `[]`),
		"a name too long":       body(long(81), `[]`),
		"too many wishes":       body("Kid", many(51)),
		"a wish without an id":  body("Kid", `[{"title":"t"}]`),
		"the same wish twice":   body("Kid", `[{"id":"`+wish1+`","title":"a"},{"id":"`+wish1+`","title":"b"}]`),
		"a blank title":         one(`"title":"  "`),
		"no title":              one(`"notes":"n"`),
		"a title too long":      one(`"title":"` + long(121) + `"`),
		"notes too long":        one(`"title":"t","notes":"` + long(501) + `"`),
		"a link too long":       one(`"title":"t","url":"` + link(2049) + `"`),
		"a huge body":           body(strings.Repeat("x", maxBody), `[]`),
	} {
		if a := s.do(http.MethodPut, ListsPath, kid, b); a.status != http.StatusBadRequest || a.code() != "bad_request" {
			t.Errorf("%s: %d %s", name, a.status, a.body)
		}
	}
	for name, b := range map[string]string{
		"no revision":  `{"id":"` + listA + `","kind":"personal"}`,
		"revision 0":   `{"id":"` + listA + `","kind":"personal","revision":0}`,
		"a bad id":     `{"id":"x","kind":"personal","revision":1}`,
		"another kind": `{"id":"` + listA + `","kind":"shared","revision":1}`,
		"not JSON":     `{`,
	} {
		if a := s.do(http.MethodDelete, ListsPath, kid, b); a.status != http.StatusBadRequest || a.code() != "bad_request" {
			t.Errorf("delete, %s: %d %s", name, a.status, a.body)
		}
	}
	if f := s.family(kid); len(f.Lists) != 0 {
		t.Fatalf("a refused change wrote %+v", f)
	}

	// At the limits, and from the site's own page, a list is saved whole.
	at := body(long(80), many(50))
	created := s.do(http.MethodPut, ListsPath, kid, at, "Sec-Fetch-Site", "same-origin")
	if created.status != http.StatusOK || len(listOf(created).Wishes) != 50 {
		t.Fatalf("a list at the limits: %d %.200s", created.status, created.body)
	}
	full := strings.Replace(one(`"title":"`+long(120)+`","notes":"`+long(500)+`","url":"`+link(2048)+`","extra":true`), `"revision":0`, `"revision":1`, 1)
	full = strings.Replace(full, listA, listOf(created).ID, 1)
	if a := s.do(http.MethodPut, ListsPath, kid, full); a.status != http.StatusOK || listOf(a).Wishes[0].URL != link(2048) {
		t.Fatalf("a wish at the limits: %d %.200s", a.status, a.body)
	}
}

// A wish link uses an absolute web address without embedded credentials,
// control characters or backslashes.
func TestOnlyWebLinksAreKept(t *testing.T) {
	for _, ok := range []string{
		"http://example.com",
		"https://example.com/gift?size=m#top",
		"HTTPS://Shop.Example.COM/a b",
		"https://example.com:8443/x",
		"https://example.com:/x",
		"https://sub_domain.example.com./x",
		"https://1.2.3.4/",
		"https://[2001:db8::1]:443/",
		"https://example.com/caf%C3%A9",
		"https://example.com/über",
		"https://bücher.example/geschenk",
		"https://xn--bcher-kva.example/geschenk",
	} {
		if !webLink(ok) {
			t.Errorf("refused %q", ok)
		}
	}
	for _, bad := range []string{
		"",
		"example.com",
		"//example.com/x",
		"/x",
		"javascript:alert(1)",
		"data:text/html,hi",
		"mailto:kid@example.com",
		"ftp://example.com/x",
		"https:example.com",
		"https:///x",
		"https://user@example.com/",
		"https://user:pw@example.com/",
		`https://example.com\@evil.example/`,
		"https://example.com/\x00",
		"https://exam\tple.com/",
		"https://example.com/\u0085",
		"https://a<b.example/",
		"https://exa mple.com/",
		"https://example.com:65536/",
		"https://example.com:99999/",
		"https://example.com/%zz",
		"https://" + strings.Repeat("a", 2040) + ".com/",
	} {
		if webLink(bad) {
			t.Errorf("kept %q", bad)
		}
	}
}

// Taking the grant away ends the Member's access and leaves their list for
// the family. Giving it back gives them their list again. Removing them
// hides their list at once, and deletes it before the removal finishes,
// retried until it does. Invited again, they start afresh, and the earlier
// invitation's session changes nothing.
func TestRevokeAndRemove(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	kid, sib := s.signIn(kidEmail), s.signIn(sibEmail)
	created := s.save(kid, input{ID: listA, Kind: store.WishPersonal, Name: "Kid", Wishes: []wishJSON{{ID: wish1, Title: "Kite"}}})
	if created.status != http.StatusOK {
		t.Fatal(created.body)
	}
	listID := listOf(created).ID
	s.grants(kidEmail, "soccer")
	if a := s.do(http.MethodGet, FamilyPath, kid, ""); a.status != http.StatusForbidden || a.code() != "no_access" {
		t.Errorf("revoked, the list: %d %s", a.status, a.body)
	}
	if a := s.save(kid, input{ID: listID, Kind: store.WishPersonal, Name: "Still", Revision: 1, Wishes: []wishJSON{}}); a.status != http.StatusForbidden || a.code() != "no_access" {
		t.Errorf("revoked, a save: %d %s", a.status, a.body)
	}
	if a := s.del(kid, listID, store.WishPersonal, 1); a.status != http.StatusForbidden || a.code() != "no_access" {
		t.Errorf("revoked, a delete: %d %s", a.status, a.body)
	}
	if f := s.family(sib); !slices.Equal(names(f), []string{"Kid"}) || f.Lists[0].Editable || len(f.Lists[0].Wishes) != 1 {
		t.Fatalf("the family, after the revoke: %+v", f)
	}
	s.grants(kidEmail, "soccer", Grant)
	if f := s.family(kid); len(f.Lists) != 1 || !f.Lists[0].Editable {
		t.Fatalf("granted again: %+v", f)
	}

	s.store.FailCleanup(errors.New("the table stopped answering"))
	if err := members.Remove(ctx, s.store, kidEmail); err == nil {
		t.Fatal("the removal didn't stop")
	}
	if f := s.family(sib); len(f.Lists) != 0 {
		t.Errorf("an unfinished removal still shows the list: %+v", f)
	}
	if !s.store.HasWishList(store.WishPersonal, listID, kidEmail, "kid-1") {
		t.Fatal("the stopped removal deleted the list")
	}
	s.store.FailCleanup(nil)
	if err := members.Remove(ctx, s.store, kidEmail); err != nil {
		t.Fatal(err)
	}
	if s.store.HasWishList(store.WishPersonal, listID, kidEmail, "kid-1") {
		t.Fatal("the removal left the list")
	}
	if a := s.do(http.MethodGet, FamilyPath, kid, ""); a.status != http.StatusUnauthorized {
		t.Errorf("removed: %d %s", a.status, a.body)
	}

	m, _ := store.NewMember(kidEmail, []string{Grant}, time.Now().UTC())
	if err := s.store.CreateMember(ctx, m); err != nil {
		t.Fatal(err)
	}
	if a := s.save(kid, input{ID: listA, Kind: store.WishPersonal, Name: "Old", Wishes: []wishJSON{}}); a.status != http.StatusUnauthorized {
		t.Errorf("the earlier invitation's session: %d %s", a.status, a.body)
	}
	again := s.signIn(kidEmail)
	if f := s.family(again); len(f.Lists) != 0 {
		t.Fatalf("invited again, the family: %+v", f)
	}
	if a := s.save(again, input{ID: listB, Kind: store.WishPersonal, Name: "Kid again", Wishes: []wishJSON{}}); a.status != http.StatusOK || listOf(a).Revision != 1 {
		t.Fatalf("invited again, a new list: %d %s", a.status, a.body)
	}
}

// The grants are checked again as the list is written, in the same write:
// one taken away, or the Member removed or invited again, after the
// request's check and before the write leaves the list as it was, and the
// answer says what changed.
func TestAccessThatChangesBeforeTheWriteIsRefused(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	kid, owner := s.signIn(kidEmail), s.signIn(ownerEmail)
	personal := s.save(kid, input{ID: listA, Kind: store.WishPersonal, Name: "Kid", Wishes: []wishJSON{}})
	managed := s.save(owner, input{ID: listB, Kind: store.WishManaged, Name: "Baby", Wishes: []wishJSON{}})
	if personal.status != http.StatusOK || managed.status != http.StatusOK {
		t.Fatalf("create: %s %s", personal.body, managed.body)
	}
	kidID, managedID := listOf(personal).ID, listOf(managed).ID
	for _, c := range []struct {
		name   string
		cookie *http.Cookie
		change func()
		write  func(*http.Cookie) answer
		status int
		code   string
	}{
		{"the grant taken away", kid, func() { s.grants(kidEmail, "soccer") },
			func(c *http.Cookie) answer {
				return s.save(c, input{ID: kidID, Kind: store.WishPersonal, Name: "Late", Revision: 1, Wishes: []wishJSON{}})
			}, http.StatusForbidden, "no_access"},
		{"Lobby Admin taken away", owner, func() { s.grants(ownerEmail, Grant) },
			func(c *http.Cookie) answer { return s.del(c, managedID, store.WishManaged, 1) }, http.StatusForbidden, "not_editable"},
		{"removed", kid, func() { _ = members.Remove(ctx, s.store, kidEmail) },
			func(c *http.Cookie) answer { return s.del(c, kidID, store.WishPersonal, 1) }, http.StatusUnauthorized, "signed_out"},
		{"a grant taken away and given back", owner, func() { s.grants(ownerEmail, Grant); s.grants(ownerEmail, Grant, members.Grant) },
			func(c *http.Cookie) answer {
				return s.save(c, input{ID: managedID, Kind: store.WishManaged, Name: "Late", Revision: 1, Wishes: []wishJSON{}})
			},
			http.StatusOK, ""},
	} {
		s.wrap.before = c.change
		a := c.write(c.cookie)
		if a.status != c.status || a.code() != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, a.status, a.body, c.status, c.code)
		}
		// Back as they were for the next case, unless removed.
		s.grants(ownerEmail, Grant, members.Grant)
		if _, err := s.store.GetMember(ctx, kidEmail); err == nil {
			s.grants(kidEmail, Grant)
		}
	}
	if s.store.HasWishList(store.WishPersonal, listA, kidEmail, "kid-1") {
		t.Error("the removal left the kid's list")
	}
	lists, _ := s.store.ListWishLists(ctx)
	if len(lists) != 1 || lists[0].Name != "Late" || lists[0].Revision != 2 {
		t.Fatalf("the lists after the races: %+v", lists)
	}
	// A re-invitation between the check and the write: the write names the
	// membership checked, which is gone.
	s.member(sibEmail, "sib-1", Grant)
	sib := s.signIn(sibEmail)
	s.wrap.before = func() { _ = s.store.DeleteMember(ctx, sibEmail); s.member(sibEmail, "sib-2", Grant) }
	if a := s.save(sib, input{ID: listC, Kind: store.WishPersonal, Name: "Sib", Wishes: []wishJSON{}}); a.status != http.StatusUnauthorized || a.code() != "signed_out" {
		t.Errorf("invited again before the write: %d %s", a.status, a.body)
	}
	if s.store.HasWishList(store.WishPersonal, listC, sibEmail, "sib-2") || s.store.HasWishList(store.WishPersonal, listC, sibEmail, "sib-1") {
		t.Error("the earlier invitation's create was written")
	}
}

// Nothing a list holds, no id from a request and no address reaches the
// log, whatever the outcome.
func TestTheLogKeepsListsOut(t *testing.T) {
	s := newSite(t)
	kid, owner := s.signIn(kidEmail), s.signIn(ownerEmail)
	secret := input{ID: listA, Kind: store.WishPersonal, Name: "Secret Name", Wishes: []wishJSON{{ID: wish1, Title: "Secret Title", URL: "https://secret.example/link", Notes: "Secret notes"}}}
	personal := s.save(kid, secret)
	s.save(kid, secret)
	managed := s.save(owner, input{ID: listB, Kind: store.WishManaged, Name: "Secret Managed", Wishes: []wishJSON{}})
	personalID, managedID := listOf(personal).ID, listOf(managed).ID
	s.del(kid, personalID, store.WishPersonal, 1)
	s.del(kid, managedID, store.WishManaged, 1)
	s.do(http.MethodPut, ListsPath, kid, `{"id":"`+listC+`","kind":"personal","name":"Secret Bad","revision":0,"wishes":[{"id":"x","title":"Secret"}]}`)
	s.wrap.before = func() { s.grants(kidEmail, "soccer") }
	s.save(kid, input{ID: listC, Kind: store.WishPersonal, Name: "Secret Race", Wishes: []wishJSON{}})
	logged := s.log.String()
	if !strings.Contains(logged, "wish lists: list saved") || !strings.Contains(logged, "kind=personal") {
		t.Fatalf("the saves weren't logged: %s", logged)
	}
	for _, private := range []string{"Secret", "secret.example", "@example.com", listA, listB, listC, wish1, "kid-1", personalID, managedID} {
		if strings.Contains(logged, private) {
			t.Errorf("the log carries %q: %s", private, logged)
		}
	}
}
