// Package members is Lobby Admin, the Owner's screen at /lobby/admin for the
// Members and the Tools each may use (ADR 0004, ADR 0013). It changes the
// same Member records as `site member`, in the environment's own table, and
// it needs the lobby-admin grant.
//
//	GET  /api/lobby/members           every Member, with when each was last here
//	POST /api/lobby/members/add       {"email": ..., "tools": [...]}
//	POST /api/lobby/members/remove    {"email": ...}
//	POST /api/lobby/members/grant     {"email": ..., "tool": ..., "granted": true|false}
//	POST /api/lobby/members/sign-out  {"email": ...}
//
// Every answer that succeeds is the whole list as the table now has it, so
// the page shows what was stored rather than what was asked. Nothing here
// logs a Member's address.
package members

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// Grant is Lobby Admin's own grant. Only the Owner holds it.
const Grant = "lobby-admin"

// Grants are the Tools a Member record may hold, in the order the page shows
// them: the ids in web/src/tools.ts, which a test keeps in step.
var Grants = []string{"soccer", "minecraft", "foundry", "minecraft-admin", "foundry-admin", Grant}

// The routes. ListPath answers GET; the rest are changes.
const (
	ListPath    = "/api/lobby/members"
	AddPath     = "/api/lobby/members/add"
	RemovePath  = "/api/lobby/members/remove"
	GrantPath   = "/api/lobby/members/grant"
	SignOutPath = "/api/lobby/members/sign-out"
)

// maxBody bounds a change's body; the largest is an add naming every Tool.
const maxBody = 4 << 10

// Tool serves Lobby Admin's routes.
type Tool struct {
	Store store.Store
	// Check says who is asking: the Member's email and grants, an empty
	// email for a Visitor, or an error when access couldn't be checked
	// (web.Handler.Member).
	Check func(w http.ResponseWriter, r *http.Request) (email string, tools []string, err error)
	Log   *slog.Logger
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

func (t *Tool) Routes(r chi.Router) {
	r.Get(ListPath, t.list)
	r.Post(AddPath, t.add)
	r.Post(RemovePath, t.remove)
	r.Post(GrantPath, t.grant)
	r.Post(SignOutPath, t.signOut)
}

func (t *Tool) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// Listed is one Member as the page shows them. LastHere is when one of their
// Sign-in sessions was last renewed, which happens at most once a day, so it
// is right to the day; empty when they aren't signed in anywhere. Devices
// counts the browsers they're signed in on.
type Listed struct {
	Email    string   `json:"email"`
	Tools    []string `json:"tools"`
	AddedAt  string   `json:"added_at,omitempty"`
	LastHere string   `json:"last_here,omitempty"`
	Devices  int      `json:"devices"`
}

// List is every answer that succeeds: who is asking, so the page can mark
// them and keep them from locking themselves out, and every Member, A to Z.
type List struct {
	You     string   `json:"you"`
	Members []Listed `json:"members"`
}

func (t *Tool) list(w http.ResponseWriter, r *http.Request) {
	you, ok := t.allow(w, r)
	if !ok {
		return
	}
	t.answer(w, r.Context(), you)
}

// answer writes the list as the table has it now.
func (t *Tool) answer(w http.ResponseWriter, ctx context.Context, you string) {
	members, err := t.Store.ListMembers(ctx)
	if err != nil {
		t.Log.Error("lobby admin: list members", "err", err)
		writeError(w, http.StatusServiceUnavailable, "unavailable", "the Members couldn't be read")
		return
	}
	sessions, err := t.Store.ListSessions(ctx)
	if err != nil {
		t.Log.Error("lobby admin: list sessions", "err", err)
		writeError(w, http.StatusServiceUnavailable, "unavailable", "the Members couldn't be read")
		return
	}
	writeJSON(w, http.StatusOK, listOf(you, members, sessions, t.now()))
}

// listOf joins the Members to their live Sign-in sessions. A session's ID
// never leaves here.
func listOf(you string, members []store.Member, sessions []store.Session, now time.Time) List {
	type seen struct {
		last    time.Time
		devices int
	}
	by := map[string]seen{}
	for _, s := range sessions {
		if !s.ExpiresAt.After(now) {
			continue
		}
		email := store.NormalizeEmail(s.Email)
		// A session's expiry is pushed a whole Lifetime ahead each time it
		// is renewed, so the Lifetime before it is when it was last used.
		last := s.ExpiresAt.Add(-session.Lifetime)
		if last.After(now) {
			last = now
		}
		b := by[email]
		b.devices++
		if last.After(b.last) {
			b.last = last
		}
		by[email] = b
	}
	out := List{You: you, Members: make([]Listed, 0, len(members))}
	for _, m := range members {
		email := store.NormalizeEmail(m.Email)
		l := Listed{Email: email, Tools: ordered(m.Tools)}
		if !m.AddedAt.IsZero() {
			l.AddedAt = m.AddedAt.UTC().Format(time.RFC3339)
		}
		if b, ok := by[email]; ok {
			l.LastHere = b.last.UTC().Format(time.RFC3339)
			l.Devices = b.devices
		}
		out.Members = append(out.Members, l)
	}
	sort.Slice(out.Members, func(i, j int) bool { return out.Members[i].Email < out.Members[j].Email })
	return out
}

// ordered puts a Member's grants in the page's order, any the site no longer
// knows last, without duplicates. It never drops one: a grant the page has
// no box for stays on the record.
func ordered(tools []string) []string {
	out := []string{}
	for _, g := range Grants {
		if slices.Contains(tools, g) {
			out = append(out, g)
		}
	}
	for _, g := range tools {
		if !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	return out
}

// POST /api/lobby/members/add: a new Member with the Tools ticked. An
// address that already has a record is refused, so an add never quietly
// replaces someone's Tools.
func (t *Tool) add(w http.ResponseWriter, r *http.Request) {
	you, ok := t.change(w, r)
	if !ok {
		return
	}
	var body struct {
		Email *string  `json:"email"`
		Tools []string `json:"tools"`
	}
	if !decode(w, r, &body) || body.Email == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send the address and the Tools")
		return
	}
	email, valid := address(*body.Email)
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid_email", "that isn't an email address")
		return
	}
	tools := []string{}
	for _, g := range body.Tools {
		if !slices.Contains(Grants, g) {
			writeError(w, http.StatusBadRequest, "unknown_tool", "that isn't one of the site's Tools")
			return
		}
		if !slices.Contains(tools, g) {
			tools = append(tools, g)
		}
	}
	ctx := r.Context()
	switch _, err := t.Store.GetMember(ctx, email); {
	case err == nil:
		writeError(w, http.StatusConflict, "already_member", "that address is already a Member")
		return
	case !errors.Is(err, store.ErrNotFound):
		t.unavailable(w, "add", err)
		return
	}
	if err := t.Store.PutMember(ctx, store.Member{Email: email, Tools: ordered(tools), AddedAt: t.now().UTC()}); err != nil {
		t.unavailable(w, "add", err)
		return
	}
	t.Log.Info("lobby admin: member added", "tools", strings.Join(ordered(tools), ","))
	t.answer(w, ctx, you)
}

// POST /api/lobby/members/remove: the Member record goes, with their Let's
// Plan Soccer calendar and calendar access (store.DeleteMember), and so do
// their Sign-in sessions. Nobody removes themselves here.
func (t *Tool) remove(w http.ResponseWriter, r *http.Request) {
	you, ok := t.change(w, r)
	if !ok {
		return
	}
	email, ok := t.named(w, r)
	if !ok {
		return
	}
	if email == you {
		writeError(w, http.StatusConflict, "self", "you can't remove yourself")
		return
	}
	ctx := r.Context()
	if !t.exists(w, ctx, "remove", email) {
		return
	}
	// Their sessions go first, while the record still lets Remove be tried
	// again: a session left behind would count again if the address were
	// added back within its 90 days.
	if _, err := t.endSessions(ctx, email, ""); err != nil {
		t.unavailable(w, "remove", err)
		return
	}
	if err := t.Store.DeleteMember(ctx, email); err != nil {
		t.unavailable(w, "remove", err)
		return
	}
	t.Log.Info("lobby admin: member removed")
	t.answer(w, ctx, you)
}

// POST /api/lobby/members/grant: one Tool on or off for one Member. The
// record is read and written whole, so their other grants and the day they
// were added stay as they were. Nobody takes their own lobby-admin here.
func (t *Tool) grant(w http.ResponseWriter, r *http.Request) {
	you, ok := t.change(w, r)
	if !ok {
		return
	}
	var body struct {
		Email   *string `json:"email"`
		Tool    *string `json:"tool"`
		Granted *bool   `json:"granted"`
	}
	if !decode(w, r, &body) || body.Email == nil || body.Tool == nil || body.Granted == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send the address, the Tool and whether it is granted")
		return
	}
	email := store.NormalizeEmail(*body.Email)
	tool := *body.Tool
	if !slices.Contains(Grants, tool) {
		writeError(w, http.StatusBadRequest, "unknown_tool", "that isn't one of the site's Tools")
		return
	}
	if email == you && tool == Grant && !*body.Granted {
		writeError(w, http.StatusConflict, "self", "you can't take away your own Lobby Admin")
		return
	}
	ctx := r.Context()
	m, err := t.Store.GetMember(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_member", "that address isn't a Member")
		return
	}
	if err != nil {
		t.unavailable(w, "grant", err)
		return
	}
	tools := slices.DeleteFunc(slices.Clone(m.Tools), func(g string) bool { return g == tool })
	if *body.Granted {
		tools = append(tools, tool)
	}
	m.Tools = ordered(tools)
	if err := t.Store.PutMember(ctx, m); err != nil {
		t.unavailable(w, "grant", err)
		return
	}
	t.Log.Info("lobby admin: grant changed", "tool", tool, "granted", *body.Granted)
	t.answer(w, ctx, you)
}

// POST /api/lobby/members/sign-out: ends the Member's Sign-in sessions on
// every browser, except, for the person asking, the one they're asking
// from. Their record stays, so they can sign in again.
func (t *Tool) signOut(w http.ResponseWriter, r *http.Request) {
	you, ok := t.change(w, r)
	if !ok {
		return
	}
	email, ok := t.named(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if !t.exists(w, ctx, "sign out", email) {
		return
	}
	keep := ""
	if email == you {
		if c, err := r.Cookie(session.CookieName); err == nil {
			keep = c.Value
		}
	}
	n, err := t.endSessions(ctx, email, keep)
	if err != nil {
		t.unavailable(w, "sign out", err)
		return
	}
	t.Log.Info("lobby admin: member signed out", "sessions", n)
	t.answer(w, ctx, you)
}

// endSessions deletes email's Sign-in sessions but keep, and says how many.
func (t *Tool) endSessions(ctx context.Context, email, keep string) (int, error) {
	sessions, err := t.Store.ListSessions(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range sessions {
		if store.NormalizeEmail(s.Email) != email || (keep != "" && s.ID == keep) {
			continue
		}
		if err := t.Store.DeleteSession(ctx, s.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			return n, err
		}
		n++
	}
	return n, nil
}

// exists answers the request itself unless email has a Member record.
func (t *Tool) exists(w http.ResponseWriter, ctx context.Context, verb, email string) bool {
	_, err := t.Store.GetMember(ctx, email)
	switch {
	case err == nil:
		return true
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_member", "that address isn't a Member")
	default:
		t.unavailable(w, verb, err)
	}
	return false
}

// named reads a body that names one Member: {"email": ...}.
func (t *Tool) named(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Email *string `json:"email"`
	}
	if !decode(w, r, &body) || body.Email == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send the Member's address")
		return "", false
	}
	return store.NormalizeEmail(*body.Email), true
}

func (t *Tool) unavailable(w http.ResponseWriter, verb string, err error) {
	t.Log.Error("lobby admin: "+verb, "err", err)
	writeError(w, http.StatusServiceUnavailable, "unavailable", "the change couldn't be made")
}

// change checks a change before its body is read: it must come from The
// Lobby's own pages as JSON, which another site's form can't send, and from
// someone with lobby-admin.
func (t *Tool) change(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		writeError(w, http.StatusForbidden, "cross_site", "change Members from The Lobby")
		return "", false
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "bad_request", "send JSON")
		return "", false
	}
	return t.allow(w, r)
}

// allow answers the request itself unless the person holds lobby-admin.
// Nothing about the Members is said to anyone who doesn't.
func (t *Tool) allow(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil || t.Store == nil {
		t.Log.Error("lobby admin: not configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return "", false
	}
	email, tools, err := t.Check(w, r)
	switch {
	case err != nil:
		t.Log.Error("lobby admin access check", "err", err)
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
	case email == "":
		writeError(w, http.StatusUnauthorized, "signed_out", "sign in to use Lobby Admin")
	case !slices.Contains(tools, Grant):
		writeError(w, http.StatusForbidden, "no_access", "you don't have access to this Tool")
	default:
		return store.NormalizeEmail(email), true
	}
	return "", false
}

// decode reads exactly one JSON object of at most maxBody bytes into v.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	if err := d.Decode(v); err != nil {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}

// address is the lookup form of an email address and whether it could be
// one: a single @ with something either side, no spaces, a dot in the
// domain, and no longer than an address may be.
func address(raw string) (string, bool) {
	email := store.NormalizeEmail(raw)
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" || len(email) > 254 || strings.Contains(domain, "@") ||
		strings.ContainsFunc(email, func(r rune) bool { return r <= ' ' || r == 0x7f }) ||
		!strings.Contains(strings.Trim(domain, "."), ".") {
		return "", false
	}
	return email, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
