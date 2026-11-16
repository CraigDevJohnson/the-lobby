package soccer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/icloud"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// The daily iCloud update (ADR 0015): once a day, with nobody there, the
// site adds the upcoming games of a Member's Remembered teams to the iCloud
// calendar they picked, for each Member who turned it on. It only ever
// adds. The UIDs it has put in the calendar, or found already there, are
// kept with the connection (store.ICloud.Written) and never written again:
// a game the Member changed stays as they left it, one they deleted stays
// deleted, and a game the league moves keeps its old time until they press
// Add. On a day with nothing new, Apple isn't called at all.
const (
	// dailyBudget is how long one run may take. The Lambda has 30 seconds,
	// and Lambda runs a failed or timed-out scheduled run two more times, so
	// a run stops itself and ends successfully. Members it doesn't reach are
	// reached tomorrow.
	dailyBudget = 25 * time.Second
	// dailySave is the time saving one Member's list has to itself, so a run
	// that is out of time still keeps what it added.
	dailySave = 3 * time.Second
	// sessionOverUID starts the UID of the backend's notice that a Team's
	// Session is over. A game's starts "game-".
	sessionOverUID = "session-over-"

	backendDown = "the backend isn't answering"
	appleBusy   = "Apple said to slow down"
	outOfTime   = "out of time"
)

// dailyRun is one run of the daily update: what it has done, for the log,
// and why it stopped early, when it did.
type dailyRun struct {
	t   *Tool
	ctx context.Context
	// r carries ctx to the backend calls. do reads a request only for its
	// context and its app's name, and a run has no app, as with Ping.
	r   *http.Request
	key []byte

	added, already, failed, skipped, switchedOff int
	stopped                                      string
}

// DailyICloud runs the daily update for every Member who turned it on, one
// at a time. Whatever happens it answers nil, since a failed run would be
// run again minutes later; what went wrong is in the log, which carries
// counts and errors only: never a Member's address, an Apple Account, a
// calendar address, a UID or a link.
func (t *Tool) DailyICloud(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, dailyBudget)
	defer cancel()
	var key []byte
	ready := t.Store != nil && t.Backend != ""
	if ready {
		key, ready = t.icloudKey(ctx)
	}
	if !ready {
		t.Log.Info("soccer: the daily iCloud update isn't set up here")
		return nil
	}
	d := &dailyRun{t: t, ctx: ctx, r: new(http.Request).WithContext(ctx), key: key}
	all, err := t.Store.ListICloud(ctx)
	if err != nil {
		d.stop("the site's table isn't answering", err)
	}
	members := 0
	for _, c := range all {
		if !c.Daily {
			continue
		}
		members++
		if d.stopped == "" && ctx.Err() == nil {
			d.member(c)
		}
	}
	if d.stopped == "" && ctx.Err() != nil {
		d.stopped = outOfTime
	}
	t.Log.Info("soccer: daily iCloud update", "members", members, "added", d.added, "already", d.already, "failed", d.failed,
		"skipped", d.skipped, "switched_off", d.switchedOff, "stopped", d.stopped)
	return nil
}

// member brings one Member's calendar up to date, and saves their list and
// switch when either changed.
func (d *dailyRun) member(c store.ICloud) {
	t, ctx := d.t, d.ctx
	if m, err := t.Store.GetMember(ctx, c.Email); err != nil || !slices.Contains(m.Tools, Grant) {
		d.skip(err)
		return
	}
	s, err := t.Store.GetSoccer(ctx, c.Email)
	if err != nil || len(s.Teams) == 0 {
		d.skip(err)
		return
	}
	// The call a Member link fetch makes, so a Team is followed into its
	// next Session here too. The league being down or the backend not
	// answering is the same for every Member, so it stops the run.
	f, res, err := t.feed(d.r, s)
	if res != nil {
		res.Body.Close()
		err = errors.New("the backend answered " + res.Status)
	}
	var all, games []icloud.Event
	if err == nil {
		all, err = icloud.Events([]byte(f.Calendar))
	}
	if err == nil {
		games, err = d.upcoming(f.Teams)
	}
	if err != nil {
		d.stop(backendDown, err)
		return
	}
	// The feed's calendar holds every game, played and remaining, of every
	// Team ID the Member keeps. A game's UID it no longer holds is dropped
	// from the list, which so stays small. Its Session-over notices are
	// added too, whatever their day, and a notice's UID is kept: the backend
	// leaves a notice out on a day it can't check the Team's division, and
	// one the Member deleted mustn't come back when it returns.
	held := map[string]bool{}
	for _, ev := range all {
		held[ev.UID] = true
		if strings.HasPrefix(ev.UID, sessionOverUID) {
			games = append(games, ev)
		}
	}
	now := c
	now.Written = slices.DeleteFunc(slices.Clone(c.Written), func(uid string) bool {
		return !held[uid] && !strings.HasPrefix(uid, sessionOverUID)
	})
	var fresh []icloud.Event
	for _, ev := range games {
		if !slices.Contains(now.Written, ev.UID) {
			fresh = append(fresh, ev)
		}
	}
	if len(fresh) > 0 {
		d.write(&now, fresh)
	}
	if now.Daily != c.Daily || !slices.Equal(now.Written, c.Written) {
		d.save(now)
	}
}

// upcoming asks the backend's POST /calendar for the games of these Team
// IDs that haven't started, each as a calendar file of its own. A 404 means
// the league knows none of the Teams any more, which is no games, not a
// failure.
func (d *dailyRun) upcoming(teams [][]string) ([]icloud.Event, error) {
	body, err := json.Marshal(map[string]any{"team_ids": slices.Concat(teams...), "since": time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	res, err := d.t.do(d.r, http.MethodPost, "/calendar", body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNotFound:
		return nil, nil
	case http.StatusOK:
		file, err := read(res)
		if err != nil {
			return nil, err
		}
		return icloud.Events(file)
	}
	return nil, errors.New("the backend answered " + res.Status)
}

// write adds the new events to the Member's calendar, one at a time, and
// notes on c each UID Apple took or already had. The Member's calendars are
// listed first, as on a press, so the password goes only to a calendar
// Apple just listed. Apple refusing the password, or a password that no
// longer opens, turns the Member's switch off and changes nothing else: the
// page's own check drops the connection and says so on their next visit.
func (d *dailyRun) write(c *store.ICloud, fresh []icloud.Event) {
	t, ctx := d.t, d.ctx
	password, err := google.Open(d.key, sealedFor(c.Email), c.Password)
	l := icloud.Login{AppleID: c.Account, Password: password}
	var cals []icloud.Calendar
	if err == nil {
		cals, err = t.ICloud.Calendars(ctx, l, c.Home)
	}
	switch {
	case errors.Is(err, icloud.ErrDenied), errors.Is(err, google.ErrSealed):
		c.Daily = false
		d.switchedOff++
		return
	case errors.Is(err, icloud.ErrBusy):
		d.stop(appleBusy, err)
		return
	case err != nil:
		d.skip(err)
		return
	case !slices.ContainsFunc(cals, func(cal icloud.Calendar) bool { return cal.ID == c.CalendarID }):
		d.skip(nil)
		return
	}
	for _, ev := range fresh {
		created, err := t.ICloud.Create(ctx, l, c.CalendarID, ev)
		switch {
		case errors.Is(err, icloud.ErrDenied):
			c.Daily = false
			d.switchedOff++
			return
		case errors.Is(err, icloud.ErrBusy):
			// The limit may not be this account's alone.
			d.stop(appleBusy, err)
			return
		case err != nil && ctx.Err() != nil:
			d.stop(outOfTime, err)
			return
		case err != nil:
			// Apple refused this one game. Tomorrow's run tries it again.
			d.failed++
			t.Log.Warn("soccer: daily iCloud update: add a game", "err", err)
			continue
		case created:
			d.added++
		default:
			d.already++
		}
		c.Written = append(c.Written, ev.UID)
	}
}

// save keeps the Member's list and switch, with time of its own. The store
// answering ErrNotFound means the Member disconnected, connected again,
// picked another calendar or turned the switch off meanwhile, and nothing
// was changed.
func (d *dailyRun) save(c store.ICloud) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(d.ctx), dailySave)
	defer cancel()
	if err := d.t.Store.UpdateICloudDaily(ctx, c); err != nil && !errors.Is(err, store.ErrNotFound) {
		d.t.Log.Error("soccer: daily iCloud update: keep the games added", "err", err)
	}
}

// skip passes over a Member until tomorrow's run. A record that is gone is
// an ordinary reason; anything else is logged.
func (d *dailyRun) skip(err error) {
	d.skipped++
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		d.t.Log.Warn("soccer: daily iCloud update: skipped a Member", "err", err)
	}
}

// stop ends the whole run: what went wrong would go wrong for the next
// Member too, and tomorrow's run tries again. A run that is out of time
// says only that.
func (d *dailyRun) stop(why string, err error) {
	if d.ctx.Err() != nil {
		d.stopped = outOfTime
		return
	}
	d.stopped = why
	d.t.Log.Error("soccer: daily iCloud update stopped", "err", err)
}
