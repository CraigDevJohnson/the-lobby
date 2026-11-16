package soccer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/icloud"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// Add to iCloud Calendar (ADR 0012): a Member with Grant gives the site
// their Apple Account address and an app-specific password and, each time
// they press Add, the games they ticked are written into an iCloud calendar
// they pick. Pressing again rewrites those games and adds any posted since.
// Nothing else happens between presses unless the Member turns the daily
// update on (daily.go, ADR 0015): then, once a day, the site adds the games
// of their Remembered teams that it hasn't added yet, and rewrites none.
// Disconnect forgets the password; only the Member can end it at Apple, and
// games already added stay.
const (
	AddICloudPath  = "/api/soccer/icloud/add"
	icloudWriters  = 4
	icloudNotReady = "Add to iCloud Calendar isn't set up here"
)

// ICloudKey reads the key app-specific passwords are sealed with. Nil, or
// an error, means Add to iCloud Calendar isn't set up here.
type ICloudKey func(ctx context.Context) ([]byte, error)

func (t *Tool) icloudRoutes(r chi.Router) {
	r.Get("/api/soccer/icloud", t.icloudStatus)
	r.Post("/api/soccer/icloud/connect", t.icloudConnect)
	r.Post(AddICloudPath, t.icloudAdd)
	r.Post("/api/soccer/icloud/daily", t.icloudDaily)
	r.Post("/api/soccer/icloud/disconnect", t.icloudDisconnect)
}

func (t *Tool) icloudKey(ctx context.Context) ([]byte, bool) {
	if t.ICloud == nil || t.ICloudKey == nil {
		return nil, false
	}
	key, err := t.ICloudKey(ctx)
	if err != nil {
		t.Log.Error("soccer: iCloud key", "err", err)
		return nil, false
	}
	return key, true
}

// sealedFor binds a sealed password to its Member and to iCloud, so one
// copied to another Member's item, or to their Google item, won't open.
func sealedFor(email string) string { return "icloud:" + email }

type icloudAnswer struct {
	Available  bool              `json:"available"`
	Connected  bool              `json:"connected"`
	Expired    bool              `json:"expired,omitempty"`
	Account    string            `json:"account,omitempty"`
	Calendars  []icloud.Calendar `json:"calendars,omitempty"`
	CalendarID string            `json:"calendar_id,omitempty"`
	Daily      bool              `json:"daily,omitempty"`
}

func (t *Tool) icloudFailed(w http.ResponseWriter, err error) {
	t.Log.Error("soccer: iCloud", "err", err)
	writeError(w, http.StatusServiceUnavailable, "icloud_unavailable", "iCloud isn't answering")
}

// icloudConnection is the Member's iCloud access, opened, and the calendars
// it can add to. ok false means the answer is written already. No login
// means not connected; expired, that the connection was just dropped
// because Apple no longer takes the password or it no longer opens.
func (t *Tool) icloudConnection(ctx context.Context, w http.ResponseWriter, key []byte, email string) (c store.ICloud, l icloud.Login, cals []icloud.Calendar, expired, ok bool) {
	c, err := t.Store.GetICloud(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return c, l, nil, false, true
	}
	if err != nil {
		t.storeFailed(w, err)
		return c, l, nil, false, false
	}
	password, err := google.Open(key, sealedFor(email), c.Password)
	if err == nil {
		l = icloud.Login{AppleID: c.Account, Password: password}
		cals, err = t.ICloud.Calendars(ctx, l, c.Home)
	}
	switch {
	case errors.Is(err, icloud.ErrDenied), errors.Is(err, google.ErrSealed):
		if err := t.Store.DeleteICloud(ctx, email); err != nil {
			t.Log.Error("soccer: drop the iCloud connection", "err", err)
		}
		return store.ICloud{}, icloud.Login{}, nil, true, true
	case err != nil:
		t.icloudFailed(w, err)
		return c, l, nil, false, false
	}
	return c, l, cals, false, true
}

// chosenCalendar is the saved choice while the account can still add to
// it, or else the first calendar.
func chosenCalendar(cals []icloud.Calendar, saved string) string {
	for _, c := range cals {
		if c.ID == saved {
			return saved
		}
	}
	if len(cals) > 0 {
		return cals[0].ID
	}
	return ""
}

// GET /api/soccer/icloud: whether the Member is connected, as which Apple
// Account, the calendars they can add to, the one they picked and whether
// the daily update is on. It answers on only while the account can still
// add to the saved calendar: the daily update skips a Member whose saved
// calendar is gone, so the card shows the switch off, and turning it on
// saves the calendar then showing.
func (t *Tool) icloudStatus(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, false)
	if !ok {
		return
	}
	key, ok := t.icloudKey(r.Context())
	if !ok {
		writeJSON(w, http.StatusOK, icloudAnswer{})
		return
	}
	c, l, cals, expired, ok := t.icloudConnection(r.Context(), w, key, email)
	if !ok {
		return
	}
	if l.Password == "" {
		writeJSON(w, http.StatusOK, icloudAnswer{Available: true, Expired: expired})
		return
	}
	chosen := chosenCalendar(cals, c.CalendarID)
	writeJSON(w, http.StatusOK, icloudAnswer{Available: true, Connected: true, Account: c.Account, Calendars: cals, CalendarID: chosen, Daily: c.Daily && chosen == c.CalendarID})
}

// POST /api/soccer/icloud/connect {apple_id, password}: Apple is asked
// where the account's calendars are, which proves the pair, and only then
// is the password kept, sealed. Neither is logged. A new connection starts
// with the daily update off and no list of games it has added.
func (t *Tool) icloudConnect(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	var in struct {
		AppleID  string `json:"apple_id"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with apple_id and password")
		return
	}
	l := icloud.Login{AppleID: strings.TrimSpace(in.AppleID), Password: strings.TrimSpace(in.Password)}
	if l.AppleID == "" || l.Password == "" || len(l.AppleID) > 254 || len(l.Password) > 128 || strings.ContainsAny(l.AppleID, ": \t\r\n") {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with apple_id and password")
		return
	}
	key, ok := t.icloudKey(r.Context())
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "icloud_unavailable", icloudNotReady)
		return
	}
	ctx := r.Context()
	home, err := t.ICloud.Home(ctx, l)
	var cals []icloud.Calendar
	if err == nil {
		cals, err = t.ICloud.Calendars(ctx, l, home)
	}
	switch {
	case errors.Is(err, icloud.ErrDenied):
		writeError(w, http.StatusUnauthorized, "icloud_denied", "Apple didn't accept that address and password")
		return
	case err != nil:
		t.icloudFailed(w, err)
		return
	}
	sealed, err := google.Seal(key, sealedFor(email), l.Password)
	if err != nil {
		t.Log.Error("soccer: seal the iCloud password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	c := store.ICloud{Email: email, Account: l.AppleID, Password: sealed, Home: home}
	if was, err := t.Store.GetICloud(ctx, email); err == nil && strings.EqualFold(was.Account, c.Account) {
		c.CalendarID = was.CalendarID
	}
	if err := t.Store.PutICloud(ctx, c); err != nil {
		t.storeFailed(w, err)
		return
	}
	writeJSON(w, http.StatusOK, icloudAnswer{Available: true, Connected: true, Account: c.Account, Calendars: cals, CalendarID: chosenCalendar(cals, c.CalendarID)})
}

// POST /api/soccer/icloud/add: the download's body plus calendar_id. The
// choice of calendar is kept for next time; the backend writes the calendar
// file from the rest, and each of its games is put into that calendar, a
// few at a time. It has its own, longer deadline (internal/app), as adding
// to Google Calendar does.
func (t *Tool) icloudAdd(w http.ResponseWriter, r *http.Request) {
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
	key, ok := t.icloudKey(r.Context())
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "icloud_unavailable", icloudNotReady)
		return
	}
	ctx := r.Context()
	c, l, cals, expired, ok := t.icloudConnection(ctx, w, key, email)
	switch {
	case !ok:
		return
	case l.Password == "":
		code := "icloud_not_connected"
		if expired {
			code = "icloud_expired"
		}
		writeError(w, http.StatusUnauthorized, code, "connect iCloud Calendar first")
		return
	case chosenCalendar(cals, calendarID) != calendarID:
		// Only a calendar Apple just listed is written to, so the password
		// goes nowhere the request names.
		writeError(w, http.StatusConflict, "icloud_calendar", "that calendar can't be written to; pick another")
		return
	}

	res, err := t.do(r, http.MethodPost, "/calendar", choice)
	if err != nil {
		t.unavailable(w, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.relay(w, res)
		return
	}
	file, err := read(res)
	var events []icloud.Event
	if err == nil {
		events, err = icloud.Events(file)
	}
	if err != nil {
		t.unavailable(w, err)
		return
	}

	if c.CalendarID != calendarID {
		// The daily update's list of games it has added belongs to one
		// calendar.
		c.CalendarID, c.Written = calendarID, nil
		if err := t.Store.PutICloud(ctx, c); err != nil {
			t.Log.Error("soccer: keep the chosen iCloud calendar", "err", err)
		}
	}

	var (
		mu             sync.Mutex
		added, updated int
		stop           error
		wg             sync.WaitGroup
	)
	// A few writes at a time; the first refused-password or calendar error
	// stops the rest.
	ectx, cancel := context.WithCancel(ctx)
	defer cancel()
	slots := make(chan struct{}, icloudWriters)
	for _, ev := range events {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			if ectx.Err() != nil {
				return
			}
			isNew, err := t.ICloud.Write(ectx, l, calendarID, ev)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case stop != nil:
			case errors.Is(err, icloud.ErrDenied), errors.Is(err, icloud.ErrCalendar):
				stop = err
				cancel()
			case err != nil:
				t.Log.Warn("soccer: write an iCloud event", "err", err)
			case isNew:
				added++
			default:
				updated++
			}
		})
	}
	wg.Wait()
	switch {
	case errors.Is(stop, icloud.ErrDenied):
		_ = t.Store.DeleteICloud(ctx, email)
		writeError(w, http.StatusUnauthorized, "icloud_expired", "Apple no longer takes the password; connect again")
		return
	case errors.Is(stop, icloud.ErrCalendar):
		writeError(w, http.StatusConflict, "icloud_calendar", "that calendar can't be written to; pick another")
		return
	}
	// Games never tried before the deadline count as failed too.
	failed := len(events) - added - updated
	t.Log.Info("soccer: games added to iCloud Calendar", "added", added, "updated", updated, "failed", failed)
	writeJSON(w, http.StatusOK, map[string]int{"added": added, "updated": updated, "failed": failed})
}

// POST /api/soccer/icloud/daily {on, calendar_id}: the daily update's
// switch. Turning it on keeps the calendar it writes to, and a calendar
// other than the saved one starts the list of games added afresh. Turning
// it off keeps both, so turning it on again doesn't bring back games the
// Member deleted. Apple isn't asked here: the daily update lists the
// Member's calendars before every write and skips a saved calendar Apple no
// longer lists, and icloud.Client refuses any address that isn't iCloud's.
func (t *Tool) icloudDaily(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	var in struct {
		On         bool   `json:"on"`
		CalendarID string `json:"calendar_id"`
	}
	if err := json.Unmarshal(body, &in); err != nil || (in.On && in.CalendarID == "") {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON with on, and calendar_id to turn it on")
		return
	}
	c, err := t.Store.GetICloud(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusUnauthorized, "icloud_not_connected", "connect iCloud Calendar first")
		return
	case err != nil:
		t.storeFailed(w, err)
		return
	}
	c.Daily = in.On
	if in.On && c.CalendarID != in.CalendarID {
		c.CalendarID, c.Written = in.CalendarID, nil
	}
	if err := t.Store.PutICloud(r.Context(), c); err != nil {
		t.storeFailed(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"daily": c.Daily})
}

// POST /api/soccer/icloud/disconnect: forget the connection. Apple gives a
// site no way to end an app-specific password, so the Member does that at
// Apple. Added games stay.
func (t *Tool) icloudDisconnect(w http.ResponseWriter, r *http.Request) {
	email, ok := t.member(w, r, true)
	if !ok {
		return
	}
	if err := t.Store.DeleteICloud(r.Context(), email); err != nil {
		t.storeFailed(w, err)
		return
	}
	_, ready := t.icloudKey(r.Context())
	writeJSON(w, http.StatusOK, icloudAnswer{Available: ready})
}
