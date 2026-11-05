package soccer

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// Add to Google Calendar (#52): a Member with Grant connects their Google
// account and, each time they press Add, the games they ticked are written
// into a Google calendar they pick. Pressing again rewrites those games and
// adds any posted since; nothing runs in the background. Disconnect revokes
// the access at Google and forgets it; games already added stay.
const (
	googlePrefix   = "/soccer/google/"
	callbackPath   = googlePrefix + "callback"
	stateCookie    = "lobby_google_state"
	AddGooglePath  = "/api/soccer/google/add"
	googleWriters  = 4
	googleNotReady = "Add to Google Calendar isn't set up here"
)

// GoogleSettings reads the Google client's credentials and token key. Nil,
// or an error, means Add to Google Calendar isn't set up here.
type GoogleSettings func(ctx context.Context) (google.Settings, error)

func (t *Tool) googleRoutes(r chi.Router) {
	r.Get(googlePrefix+"connect", t.googleConnect)
	r.Get(callbackPath, t.googleCallback)
	r.Get("/api/soccer/google", t.googleStatus)
	r.Post(AddGooglePath, t.googleAdd)
	r.Post("/api/soccer/google/disconnect", t.googleDisconnect)
}

func (t *Tool) settings(ctx context.Context) (google.Settings, bool) {
	if t.Google == nil || t.GoogleSettings == nil {
		return google.Settings{}, false
	}
	s, err := t.GoogleSettings(ctx)
	if err != nil {
		t.Log.Error("soccer: Google settings", "err", err)
		return google.Settings{}, false
	}
	return s, true
}

// back sends a page visit to /soccer with how connecting went.
func back(w http.ResponseWriter, r *http.Request, outcome string) {
	http.Redirect(w, r, "/soccer?google="+outcome, http.StatusSeeOther)
}

func (t *Tool) redirectURL(r *http.Request) string {
	base := strings.TrimSuffix(t.BaseURL, "/")
	if base == "" {
		base = "https://" + r.Host
	}
	return base + callbackPath
}

func (t *Tool) granted(w http.ResponseWriter, r *http.Request) (string, bool) {
	if t.Check == nil {
		return "", false
	}
	a, email, err := t.Check(w, r)
	if err != nil {
		t.Log.Error("soccer access check", "err", err)
	}
	return email, err == nil && a == Granted && email != ""
}

// GET /soccer/google/connect: off to Google's consent screen. The state is
// a random value in a short-lived cookie that only the callback reads, kept
// beside a hash of the Member's address, so the callback also needs the
// same Member signed in.
func (t *Tool) googleConnect(w http.ResponseWriter, r *http.Request) {
	email, ok := t.granted(w, r)
	if !ok {
		back(w, r, "failed")
		return
	}
	s, ok := t.settings(r.Context())
	if !ok {
		back(w, r, "unavailable")
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		back(w, r, "failed")
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: state + "." + memberTag(email), Path: googlePrefix, MaxAge: 600,
		HttpOnly: true, Secure: !strings.HasPrefix(t.redirectURL(r), "http://"), SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, t.Google.AuthURL(s, t.redirectURL(r), state, email), http.StatusFound)
}

// memberTag names the Member in the state cookie without their address.
func memberTag(email string) string {
	sum := sha256.Sum256([]byte(email))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// GET /soccer/google/callback: Google sends the Member back here. The
// access is kept only with a refresh token, calendar.events granted and a
// verified Google address; the main calendar is the choice until they pick
// another.
func (t *Tool) googleCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: googlePrefix, MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	email, ok := t.granted(w, r)
	c, err := r.Cookie(stateCookie)
	q := r.URL.Query()
	if !ok || err != nil || q.Get("state") == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(q.Get("state")+"."+memberTag(email))) != 1 {
		back(w, r, "failed")
		return
	}
	if q.Get("error") != "" {
		back(w, r, "denied")
		return
	}
	s, ok := t.settings(r.Context())
	if !ok {
		back(w, r, "unavailable")
		return
	}
	ctx := r.Context()
	grant, err := t.Google.Exchange(ctx, s, t.redirectURL(r), q.Get("code"))
	if err != nil {
		t.Log.Error("soccer: Google connect", "err", err)
		back(w, r, "failed")
		return
	}
	if !grant.Has(google.ScopeEvents) {
		_ = t.Google.Revoke(ctx, cmp.Or(grant.Refresh, grant.Access))
		back(w, r, "denied")
		return
	}
	account, err := t.Google.Account(ctx, grant.Access)
	if err != nil || grant.Refresh == "" {
		t.Log.Error("soccer: Google connect", "err", err, "refresh_token", grant.Refresh != "")
		back(w, r, "failed")
		return
	}
	sealed, err := google.Seal(s.Key, email, grant.Refresh)
	if err != nil {
		t.Log.Error("soccer: seal the Google token", "err", err)
		back(w, r, "failed")
		return
	}
	g := store.Google{Email: email, Account: account, Token: sealed, CalendarID: "primary"}
	if was, err := t.Store.GetGoogle(ctx, email); err == nil && was.Account == account && was.CalendarID != "" {
		g.CalendarID = was.CalendarID
	}
	if err := t.Store.PutGoogle(ctx, g); err != nil {
		t.Log.Error("soccer: keep the Google connection", "err", err)
		back(w, r, "failed")
		return
	}
	back(w, r, "connected")
}

type googleAnswer struct {
	Available  bool              `json:"available"`
	Connected  bool              `json:"connected"`
	Expired    bool              `json:"expired,omitempty"`
	Account    string            `json:"account,omitempty"`
	Calendars  []google.Calendar `json:"calendars,omitempty"`
	CalendarID string            `json:"calendar_id,omitempty"`
}

// connection is the Member's Google access, opened, and a fresh access
// token. ok false means the answer is written already; a nil error with ok
// false never happens. expired means the connection was dropped.
func (t *Tool) connection(ctx context.Context, w http.ResponseWriter, s google.Settings, email string) (g store.Google, access string, expired, ok bool) {
	g, err := t.Store.GetGoogle(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return g, "", false, true
	}
	if err != nil {
		t.storeFailed(w, err)
		return g, "", false, false
	}
	refresh, err := google.Open(s.Key, email, g.Token)
	if err == nil {
		access, err = t.Google.Access(ctx, s, refresh)
	}
	switch {
	case errors.Is(err, google.ErrExpired), errors.Is(err, google.ErrSealed):
		// Google no longer takes it, or it was sealed with another key:
		// either way the Member connects again.
		if err := t.Store.DeleteGoogle(ctx, email); err != nil {
			t.Log.Error("soccer: drop the Google connection", "err", err)
		}
		return store.Google{}, "", true, true
	case err != nil:
		t.googleFailed(w, err)
		return g, "", false, false
	}
	return g, access, false, true
}

func (t *Tool) googleFailed(w http.ResponseWriter, err error) {
	t.Log.Error("soccer: Google", "err", err)
	writeError(w, http.StatusServiceUnavailable, "google_unavailable", "Google isn't answering")
}

// GET /api/soccer/google: whether the Member is connected, as which Google
// account, the calendars they can write to and the one they picked.
func (t *Tool) googleStatus(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, false)
	if !ok {
		return
	}
	s, ok := t.settings(r.Context())
	if !ok {
		writeJSON(w, http.StatusOK, googleAnswer{})
		return
	}
	g, access, expired, ok := t.connection(r.Context(), w, s, email)
	if !ok {
		return
	}
	if access == "" {
		writeJSON(w, http.StatusOK, googleAnswer{Available: true, Expired: expired})
		return
	}
	cals, err := t.Google.Calendars(r.Context(), access)
	switch {
	case errors.Is(err, google.ErrScope):
		cals = []google.Calendar{{ID: "primary", Name: "Main calendar", Primary: true}}
	case errors.Is(err, google.ErrExpired):
		_ = t.Store.DeleteGoogle(r.Context(), email)
		writeJSON(w, http.StatusOK, googleAnswer{Available: true, Expired: true})
		return
	case err != nil:
		t.googleFailed(w, err)
		return
	}
	// The saved choice, or the main calendar when it was the main one or
	// can no longer be written to (gone, or now shared read-only).
	chosen := ""
	for _, c := range cals {
		if c.ID == g.CalendarID || chosen == "" && c.Primary {
			chosen = c.ID
		}
	}
	if chosen == "" && len(cals) > 0 {
		chosen = cals[0].ID
	}
	writeJSON(w, http.StatusOK, googleAnswer{Available: true, Connected: true, Account: g.Account, Calendars: cals, CalendarID: chosen})
}

// POST /api/soccer/google/add: the download's body plus calendar_id. The
// choice of calendar is kept for next time; the backend builds the events
// from the rest, and each is written into that calendar, a few at a time.
// It has its own, longer deadline (internal/app), since ten Teams' games are
// many calls to Google.
func (t *Tool) googleAdd(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	var calendarID string
	if err := json.Unmarshal(body, &fields); err != nil || json.Unmarshal(fields["calendar_id"], &calendarID) != nil || calendarID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with calendar_id")
		return
	}
	delete(fields, "calendar_id")
	choice, _ := json.Marshal(fields)
	s, ok := t.settings(r.Context())
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "google_unavailable", googleNotReady)
		return
	}
	ctx := r.Context()
	g, access, expired, ok := t.connection(ctx, w, s, email)
	switch {
	case !ok:
		return
	case access == "":
		code := "google_not_connected"
		if expired {
			code = "google_expired"
		}
		writeError(w, http.StatusUnauthorized, code, "connect Google Calendar first")
		return
	}

	res, err := t.do(r, http.MethodPost, "/events", choice)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.relay(w, res)
		return
	}
	answer, err := read(res)
	var built struct {
		Events []json.RawMessage `json:"events"`
	}
	if err == nil {
		err = json.Unmarshal(answer, &built)
	}
	if err != nil {
		t.unavailable(w, err)
		return
	}

	if g.CalendarID != calendarID {
		g.CalendarID = calendarID
		if err := t.Store.PutGoogle(ctx, g); err != nil {
			t.Log.Error("soccer: keep the chosen Google calendar", "err", err)
		}
	}

	var (
		mu                        sync.Mutex
		attempted, added, updated int
		stop                      error
		wg                        sync.WaitGroup
	)
	// A few writes at a time; the first expired-access or calendar error
	// stops the rest.
	ectx, cancel := context.WithCancel(ctx)
	defer cancel()
	slots := make(chan struct{}, googleWriters)
	for _, ev := range built.Events {
		var id struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(ev, &id) != nil || id.ID == "" {
			continue
		}
		attempted++
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			if ectx.Err() != nil {
				return
			}
			isNew, err := t.Google.Write(ectx, access, calendarID, id.ID, ev)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case stop != nil:
			case errors.Is(err, google.ErrExpired), errors.Is(err, google.ErrCalendar):
				stop = err
				cancel()
			case err != nil:
				t.Log.Warn("soccer: write a Google event", "err", err)
			case isNew:
				added++
			default:
				updated++
			}
		})
	}
	wg.Wait()
	switch {
	case errors.Is(stop, google.ErrExpired):
		_ = t.Store.DeleteGoogle(ctx, email)
		writeError(w, http.StatusUnauthorized, "google_expired", "Google access ended; connect again")
		return
	case errors.Is(stop, google.ErrCalendar):
		writeError(w, http.StatusConflict, "google_calendar", "that calendar can't be written to; pick another")
		return
	}
	// Games never tried before the deadline count as failed too.
	failed := attempted - added - updated
	t.Log.Info("soccer: games added to Google Calendar", "added", added, "updated", updated, "failed", failed)
	writeJSON(w, http.StatusOK, map[string]int{"added": added, "updated": updated, "failed": failed})
}

// POST /api/soccer/google/disconnect: revoke at Google, then forget the
// connection, whatever Google answered. Added games stay.
func (t *Tool) googleDisconnect(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	ctx := r.Context()
	g, err := t.Store.GetGoogle(ctx, email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		t.storeFailed(w, err)
		return
	}
	if err == nil {
		if s, ok := t.settings(ctx); ok {
			if refresh, err := google.Open(s.Key, email, g.Token); err == nil {
				rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if err := t.Google.Revoke(rctx, refresh); err != nil {
					t.Log.Warn("soccer: revoke Google access", "err", err)
				}
				cancel()
			}
		}
		if err := t.Store.DeleteGoogle(ctx, email); err != nil {
			t.storeFailed(w, err)
			return
		}
	}
	_, ready := t.settings(ctx)
	writeJSON(w, http.StatusOK, googleAnswer{Available: ready})
}
