// Package wish is Wish Lists, the family's gift ideas at /wish, kept in the
// site's own table behind the wish grant (ADR 0019).
//
//	GET    /api/wish        every list the family sees, and whether the
//	                        asker may manage lists
//	PUT    /api/wish/lists  {"id", "kind", "name", "revision", "wishes"}:
//	                        create a list at revision 0, or save over the
//	                        revision read. A new list's id is a draft seed.
//	                        Edits use the saved id that the request returns.
//	DELETE /api/wish/lists  {"id", "kind", "revision"}
//
// A Member with the grant reads every list and edits their own personal
// list. The Owner's lobby-admin grant also edits the managed lists, for
// people who do not sign in. Every request checks the Sign-in session, the
// membership's generation and the grants again, and each write holds only
// while that same membership still holds them (store.WishEditor). Nothing
// here logs a Member's address or anything a list holds.
package wish

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/members"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// Grant is Wish Lists' grant. ManageGrant, Lobby Admin's, also edits the
// managed lists.
const (
	Grant       = "wish"
	ManageGrant = members.Grant
)

// The routes.
const (
	FamilyPath = "/api/wish"
	ListsPath  = "/api/wish/lists"
)

// The limits on what a list holds, which the page holds to as well. Names,
// titles and notes count characters, and a link counts bytes.
const (
	maxName   = 80
	maxTitle  = 120
	maxNotes  = 500
	maxLink   = 2048
	maxWishes = 50
	// maxRevision is the largest whole number the page reads exactly.
	maxRevision = 1<<53 - 1
	maxBody     = 400 << 10
)

// Tool serves Wish Lists' routes.
type Tool struct {
	Store store.Store
	// Check says who is asking: their Member record as the access check read
	// it, ok false for a Visitor, or an error when access couldn't be
	// checked (web.Handler.CurrentMember).
	Check func(w http.ResponseWriter, r *http.Request) (m store.Member, ok bool, err error)
	Log   *slog.Logger
}

func (t *Tool) Routes(r chi.Router) {
	r.Get(FamilyPath, t.family)
	r.Put(ListsPath, t.save)
	r.Delete(ListsPath, t.remove)
}

type wishJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Notes string `json:"notes,omitempty"`
}

// listJSON is one list as the page shows it. It never carries the owner's
// address or generation: editable says what the page needs.
type listJSON struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Name     string     `json:"name"`
	Revision int64      `json:"revision"`
	Wishes   []wishJSON `json:"wishes"`
	Editable bool       `json:"editable"`
}

// GET /api/wish: every list, by name, with which the asker may edit.
func (t *Tool) family(w http.ResponseWriter, r *http.Request) {
	m, ok := t.allow(w, r)
	if !ok {
		return
	}
	lists, err := t.Store.ListWishLists(r.Context())
	if err != nil {
		t.Log.Error("wish lists: list", "err", err)
		writeError(w, http.StatusServiceUnavailable, "unavailable", "the wish lists couldn't be read")
		return
	}
	sort.Slice(lists, func(i, j int) bool {
		a, b := strings.ToLower(lists[i].Name), strings.ToLower(lists[j].Name)
		if a != b {
			return a < b
		}
		return lists[i].ID < lists[j].ID
	})
	out := struct {
		Lists     []listJSON `json:"lists"`
		CanManage bool       `json:"can_manage"`
	}{Lists: make([]listJSON, 0, len(lists)), CanManage: slices.Contains(m.Tools, ManageGrant)}
	for _, l := range lists {
		out.Lists = append(out.Lists, shown(l, m))
	}
	writeJSON(w, http.StatusOK, out)
}

// shown is a list as m sees it: a personal list is editable only by the
// membership that made it, and a managed list by whoever may manage lists.
func shown(l store.WishList, m store.Member) listJSON {
	editable := slices.Contains(m.Tools, ManageGrant)
	if l.Kind == store.WishPersonal {
		editable = l.Owner == store.NormalizeEmail(m.Email) && l.OwnerGeneration == m.Generation
	}
	out := listJSON{ID: l.ID, Kind: l.Kind, Name: l.Name, Revision: l.Revision, Wishes: make([]wishJSON, 0, len(l.Wishes)), Editable: editable}
	for _, x := range l.Wishes {
		out.Wishes = append(out.Wishes, wishJSON(x))
	}
	return out
}

// PUT /api/wish/lists: one list, saved whole. A personal list is always the
// asker's own, and each membership has one. A managed list needs
// ManageGrant.
func (t *Tool) save(w http.ResponseWriter, r *http.Request) {
	m, ok := t.change(w, r)
	if !ok {
		return
	}
	var body struct {
		ID       *string     `json:"id"`
		Kind     *string     `json:"kind"`
		Name     *string     `json:"name"`
		Revision *int64      `json:"revision"`
		Wishes   *[]wishJSON `json:"wishes"`
	}
	if !decode(w, r, &body) || body.ID == nil || body.Kind == nil || body.Name == nil || body.Revision == nil || body.Wishes == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send the list's id, kind, name, revision and wishes")
		return
	}
	l, problem := listFrom(*body.ID, *body.Kind, *body.Name, *body.Revision, *body.Wishes)
	if problem != "" {
		writeError(w, http.StatusBadRequest, "bad_request", problem)
		return
	}
	by, ok := t.editor(w, m, l.Kind)
	if !ok {
		return
	}
	if l.Revision == 0 {
		l.ID = createdListID(l, m)
	}
	saved, err := t.Store.SaveWishList(r.Context(), by, l)
	if err != nil {
		t.refused(w, r, by, "save", err)
		return
	}
	t.Log.Info("wish lists: list saved", "kind", saved.Kind)
	writeJSON(w, http.StatusOK, shown(saved, m))
}

func createdListID(l store.WishList, m store.Member) string {
	scope := l.Kind
	if l.Kind == store.WishPersonal {
		scope += "#" + m.Generation + "#" + store.NormalizeEmail(m.Email)
	}
	id := sha256.Sum256([]byte(scope + "\x00" + l.ID))
	return hex.EncodeToString(id[:16])
}

// DELETE /api/wish/lists: one list, at the revision read.
func (t *Tool) remove(w http.ResponseWriter, r *http.Request) {
	m, ok := t.change(w, r)
	if !ok {
		return
	}
	var body struct {
		ID       *string `json:"id"`
		Kind     *string `json:"kind"`
		Revision *int64  `json:"revision"`
	}
	if !decode(w, r, &body) || body.ID == nil || body.Kind == nil || body.Revision == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "send the list's id, kind and revision")
		return
	}
	if !opaqueID(*body.ID) || !kind(*body.Kind) || *body.Revision < 1 || *body.Revision > maxRevision {
		writeError(w, http.StatusBadRequest, "bad_request", "that isn't a saved list")
		return
	}
	by, ok := t.editor(w, m, *body.Kind)
	if !ok {
		return
	}
	if err := t.Store.DeleteWishList(r.Context(), by, store.WishList{ID: *body.ID, Kind: *body.Kind, Revision: *body.Revision}); err != nil {
		t.refused(w, r, by, "delete", err)
		return
	}
	t.Log.Info("wish lists: list deleted", "kind", *body.Kind)
	w.WriteHeader(http.StatusNoContent)
}

// editor is m changing a list of this kind, with the grants that needs, or
// the request answered when m doesn't hold them.
func (t *Tool) editor(w http.ResponseWriter, m store.Member, kind string) (store.WishEditor, bool) {
	need := []string{Grant}
	if kind == store.WishManaged {
		need = append(need, ManageGrant)
	}
	if !holds(m.Tools, need) {
		writeError(w, http.StatusForbidden, "not_editable", "you can't change this list")
		return store.WishEditor{}, false
	}
	return store.WishEditor{Email: m.Email, Generation: m.Generation, Need: need}, true
}

// refused answers a write the store didn't make. A list changed since it
// was read is a conflict. When the asker's access changed between the check
// and the write, the check is made again to say what changed, and if
// nothing has by then, the page refreshes as for a conflict.
func (t *Tool) refused(w http.ResponseWriter, r *http.Request, by store.WishEditor, verb string, err error) {
	switch {
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "that list has changed; refresh and try again")
		return
	case !errors.Is(err, store.ErrAccessChanged):
		t.unavailable(w, verb, err)
		return
	}
	m, ok, err := t.Check(w, r)
	switch {
	case err != nil:
		t.Log.Error("wish lists access check", "err", err)
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
	case !ok:
		writeError(w, http.StatusUnauthorized, "signed_out", "sign in to use Wish Lists")
	case !slices.Contains(m.Tools, Grant):
		writeError(w, http.StatusForbidden, "no_access", "you don't have access to this Tool")
	case !holds(m.Tools, by.Need):
		writeError(w, http.StatusForbidden, "not_editable", "you can't change this list")
	default:
		writeError(w, http.StatusConflict, "conflict", "that list has changed; refresh and try again")
	}
}

func (t *Tool) unavailable(w http.ResponseWriter, verb string, err error) {
	t.Log.Error("wish lists: "+verb, "err", err)
	writeError(w, http.StatusServiceUnavailable, "unavailable", "the change couldn't be made")
}

// change checks a change before its body is read: it must come from The
// Lobby's own pages as JSON, which another site's form can't send, and from
// someone with the grant.
func (t *Tool) change(w http.ResponseWriter, r *http.Request) (store.Member, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		writeError(w, http.StatusForbidden, "cross_site", "change wish lists from The Lobby")
		return store.Member{}, false
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "bad_request", "send JSON")
		return store.Member{}, false
	}
	return t.allow(w, r)
}

// allow answers the request itself unless the asker holds the grant.
// Nothing about the lists is said to anyone who doesn't.
func (t *Tool) allow(w http.ResponseWriter, r *http.Request) (store.Member, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if t.Check == nil || t.Store == nil {
		t.Log.Error("wish lists: not configured")
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
		return store.Member{}, false
	}
	m, ok, err := t.Check(w, r)
	switch {
	case err != nil:
		t.Log.Error("wish lists access check", "err", err)
		writeError(w, http.StatusServiceUnavailable, "access_unavailable", "access could not be checked")
	case !ok:
		writeError(w, http.StatusUnauthorized, "signed_out", "sign in to use Wish Lists")
	case !slices.Contains(m.Tools, Grant):
		writeError(w, http.StatusForbidden, "no_access", "you don't have access to this Tool")
	default:
		return m, true
	}
	return store.Member{}, false
}

// listFrom is the list a save asks for, trimmed, or what is wrong with it.
func listFrom(id, k, name string, revision int64, wishes []wishJSON) (store.WishList, string) {
	switch {
	case !opaqueID(id):
		return store.WishList{}, "the list's id isn't valid"
	case !kind(k):
		return store.WishList{}, "a list is personal or managed"
	case revision < 0 || revision >= maxRevision:
		return store.WishList{}, "the list's revision isn't valid"
	case len(wishes) > maxWishes:
		return store.WishList{}, "a list holds at most " + strconv.Itoa(maxWishes) + " wishes"
	}
	l := store.WishList{ID: id, Kind: k, Name: strings.TrimSpace(name), Revision: revision, Wishes: make([]store.Wish, 0, len(wishes))}
	if l.Name == "" || utf8.RuneCountInString(l.Name) > maxName {
		return store.WishList{}, "give the list a name of at most " + strconv.Itoa(maxName) + " characters"
	}
	seen := map[string]bool{}
	for _, x := range wishes {
		wish := store.Wish{ID: x.ID, Title: strings.TrimSpace(x.Title), URL: strings.TrimSpace(x.URL), Notes: strings.TrimSpace(x.Notes)}
		switch {
		case !opaqueID(wish.ID) || seen[wish.ID]:
			return store.WishList{}, "each wish needs its own id"
		case wish.Title == "" || utf8.RuneCountInString(wish.Title) > maxTitle:
			return store.WishList{}, "give each wish a title of at most " + strconv.Itoa(maxTitle) + " characters"
		case utf8.RuneCountInString(wish.Notes) > maxNotes:
			return store.WishList{}, "a wish's notes hold at most " + strconv.Itoa(maxNotes) + " characters"
		case wish.URL != "" && !webLink(wish.URL):
			return store.WishList{}, "a wish's link must be a web address starting with http:// or https://"
		}
		seen[wish.ID] = true
		l.Wishes = append(l.Wishes, wish)
	}
	return l, ""
}

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// opaqueID is an id the page makes: 32 lowercase hex digits.
func opaqueID(s string) bool { return idPattern.MatchString(s) }

func kind(s string) bool { return s == store.WishPersonal || s == store.WishManaged }

func holds(tools, need []string) bool {
	for _, g := range need {
		if !slices.Contains(tools, g) {
			return false
		}
	}
	return true
}

func webLink(s string) bool {
	if len(s) > maxLink || strings.ContainsRune(s, '\\') || strings.ContainsFunc(s, unicode.IsControl) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "" || u.User != nil {
		return false
	}
	host := u.Hostname()
	if host == "" || strings.ContainsAny(host, "<>^|") || strings.ContainsFunc(host, unicode.IsSpace) {
		return false
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		return err == nil && n <= 65535
	}
	return true
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
