package soccer

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/icloud"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// The daily iCloud update (ADR 0015, ADR 0017): once a day, with nobody
// there, the site brings the iCloud calendar a Member picked up to date with
// the upcoming games of their Remembered teams, for each Member who turned
// it on. It adds a game it hasn't added before, and it moves a game the
// league has moved since.
//
// The UIDs it has put in the calendar, or found already there, are kept with
// the connection (store.ICloud.Written) and never added again: a game the
// Member deleted stays deleted. Beside each game's UID it keeps the start
// and field the league last gave (store.ICloud.Times). A game whose start or
// field no longer matches has moved: it is replaced whole in the calendar,
// as a press of Add does, so anything the Member changed on it in iCloud is
// replaced too, and the Member is emailed what changed (moved.go). A moved
// game that isn't in the calendar is left out of it, and the email says so.
// A game that hasn't moved is never written again, and on a day with
// nothing new and nothing moved, Apple isn't called at all.
const (
	// dailyBudget is how long one run may take. The Lambda has 30 seconds,
	// and Lambda runs a failed or timed-out scheduled run two more times, so
	// a run stops itself and ends successfully. Members it doesn't reach are
	// reached tomorrow.
	dailyBudget = 25 * time.Second
	// dailySave is the time saving one Member's list has to itself, so a run
	// that is out of time still keeps what it added.
	dailySave = 3 * time.Second
	// dailyMoves is how much of the run must be left for a Member's moves to
	// start. The email about them is sent inside the run's own time, after
	// the moves and the save, and a move that is saved without its email is
	// never emailed. With less than this left, the Member's moved games wait
	// for tomorrow's run, which still sees them: their kept times are
	// unchanged.
	dailyMoves = 12 * time.Second
	// gameUID starts a game's UID, and sessionOverUID that of the backend's
	// notice that a Team's Session is over. Only a game has a time that is
	// kept and compared: a notice is all-day, has no field, and its day can
	// change.
	gameUID        = "game-"
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
	// moved is the games replaced in a calendar, missing the moved games that
	// weren't in one, and emailed the Members told about either.
	moved, missing, emailed int
	stopped                 string
}

// DailyICloud runs the daily update for every Member who turned it on, one
// at a time. Whatever happens it answers nil, since a failed run would be
// run again minutes later; what went wrong is in the log, which carries
// counts and errors only: never a Member's address, an Apple Account, a
// calendar address, a UID, a game's title or a link.
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
	// The keys up to stopped are read by name when the update is proved on
	// dev (infra/icloud/wizard.sh), so they keep their names.
	t.Log.Info("soccer: daily iCloud update", "members", members, "added", d.added, "already", d.already, "failed", d.failed,
		"skipped", d.skipped, "switched_off", d.switchedOff, "stopped", d.stopped,
		"moved", d.moved, "missing", d.missing, "emailed", d.emailed)
	return nil
}

// member brings one Member's calendar up to date, saves their list, times
// and switch when any changed, and emails them when a game moved.
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
	// from the list, which so stays small, and its kept time goes with it.
	// Its Session-over notices are added too, whatever their day, and a
	// notice's UID is kept: the backend leaves a notice out on a day it
	// can't check the Team's division, and one the Member deleted mustn't
	// come back when it returns.
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
	now.Times = map[string]store.GameTime{}
	for uid, at := range c.Times {
		if slices.Contains(now.Written, uid) {
			now.Times[uid] = at
		}
	}
	// A game not on the list is new. One on it has moved when the league's
	// start or field for it is no longer the kept one; nothing else counts,
	// not its title and not a score. One written before times were kept has
	// none to compare, so the league's are only noted: Apple isn't asked
	// about it, and nobody is emailed. A notice is never compared.
	var fresh, moved []icloud.Event
	for _, ev := range games {
		at, timed := now.Times[ev.UID]
		switch {
		case !slices.Contains(now.Written, ev.UID):
			fresh = append(fresh, ev)
		case !strings.HasPrefix(ev.UID, gameUID):
		case !timed:
			now.Times[ev.UID] = timeOf(ev)
		case at != timeOf(ev):
			moved = append(moved, ev)
		}
	}
	if !d.timeToMove() {
		moved = nil
	}
	var changes []movedGame
	if len(fresh) > 0 || len(moved) > 0 {
		changes = d.write(&now, fresh, moved)
	}
	// A change in the times alone is saved too, or the same move would be
	// made again every morning.
	kept := false
	if now.Daily != c.Daily || !slices.Equal(now.Written, c.Written) || !maps.Equal(now.Times, c.Times) {
		kept = d.save(now)
	}
	// The email goes only once the new times are kept, so a move is never
	// emailed twice, and only where the site sends email. It is sent inside
	// the run's own time and never tried again.
	if len(changes) == 0 || !kept || t.Email == nil {
		return
	}
	if err := t.Email(ctx, movedEmail(t.BaseURL, c.Email, changes)); err != nil {
		// The sender's error names the stage and the server's reply code,
		// never the address (invite.Error).
		t.Log.Warn("soccer: daily iCloud update: the email about moved games wasn't sent", "err", err)
		return
	}
	d.emailed++
}

// timeOf is the start and field the backend's calendar file gives a game.
func timeOf(ev icloud.Event) store.GameTime {
	return store.GameTime{Start: ev.Start, Field: ev.Field}
}

// timeToMove reports whether enough of the run is left to start a Member's
// moves and still send the email about them (dailyMoves).
func (d *dailyRun) timeToMove() bool {
	end, ok := d.ctx.Deadline()
	return !ok || time.Until(end) >= dailyMoves
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

// write adds the new events to the Member's calendar and then replaces the
// moved games in it, one at a time. It notes on c each UID Apple took or
// already had, with the game's start and field, and the new start and field
// of each moved game Apple replaced or didn't have, and answers those moved
// games for the email. The Member's calendars are listed first, once, as on
// a press, so the password goes only to a calendar Apple just listed. Apple
// refusing the password, or a password that no longer opens, turns the
// Member's switch off and changes nothing else: the page's own check drops
// the connection and says so on their next visit. The Member's connection
// is read again before Apple is first called and before each game is sent
// (current), and a Member who disconnected, connected again, picked another
// calendar or turned the switch off meanwhile gets nothing more written.
func (d *dailyRun) write(c *store.ICloud, fresh, moved []icloud.Event) []movedGame {
	t, ctx := d.t, d.ctx
	// ListICloud is eventually consistent and the backend calls above may
	// take time, so the first look is before the password is used at all.
	if !d.current(c) {
		return nil
	}
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
		return nil
	case errors.Is(err, icloud.ErrBusy):
		d.stop(appleBusy, err)
		return nil
	case err != nil:
		d.skip(err)
		return nil
	case !slices.ContainsFunc(cals, func(cal icloud.Calendar) bool { return cal.ID == c.CalendarID }):
		d.skip(nil)
		return nil
	}
	for _, ev := range fresh {
		// Apple took time to list the calendars, and to take each game before
		// this one.
		if !d.current(c) {
			return nil
		}
		created, err := t.ICloud.Create(ctx, l, c.CalendarID, ev)
		switch {
		case d.ends(c, err):
			return nil
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
		if strings.HasPrefix(ev.UID, gameUID) {
			c.Times[ev.UID] = timeOf(ev)
		}
	}
	// Adding took some of the run's time, so the moves are checked against
	// what is left now.
	if len(moved) == 0 || !d.timeToMove() {
		return nil
	}
	var changes []movedGame
	for _, ev := range moved {
		if !d.current(c) {
			return changes
		}
		replaced, said, err := t.ICloud.Replace(ctx, l, c.CalendarID, ev)
		switch {
		case d.ends(c, err):
			return changes
		case err != nil:
			// Apple refused this one game. Its kept time stays as it was, so
			// tomorrow's run tries it again.
			d.failed++
			t.Log.Warn("soccer: daily iCloud update: move a game", "err", err)
			continue
		case replaced:
			d.moved++
		default:
			// The Member deleted it, or it is there under another file name.
			// It is left alone, and the new time is kept all the same, so the
			// same move is acted on once. The log line says only how Apple
			// said the game isn't there.
			d.missing++
			t.Log.Info("soccer: daily iCloud update: a moved game isn't in the calendar", "apple", said)
		}
		changes = append(changes, movedGame{Title: ev.Title, Was: c.Times[ev.UID], Now: timeOf(ev), Found: replaced})
		c.Times[ev.UID] = timeOf(ev)
	}
	return changes
}

// current reads the Member's connection again, strongly consistently, and
// reports whether the run may still write to Apple with c: the connection is
// still the one c was read from, with the same calendar and the daily update
// still on. Otherwise, or when the read fails, the Member is skipped. What
// the run noted on c before that is saved only if the connection is still
// c's (UpdateICloudDaily).
func (d *dailyRun) current(c *store.ICloud) bool {
	live, err := d.t.Store.GetICloud(d.ctx, c.Email)
	if err == nil && sameICloud(*c, live) && live.CalendarID == c.CalendarID && live.Daily {
		return true
	}
	d.skip(err)
	return false
}

// ends reports whether err, from adding or moving one game, ends this
// Member's writes, and acts on it: Apple refusing the password turns their
// switch off, and Apple saying slow down, or the run being out of time,
// stops the run.
func (d *dailyRun) ends(c *store.ICloud, err error) bool {
	switch {
	case errors.Is(err, icloud.ErrDenied):
		c.Daily = false
		d.switchedOff++
	case errors.Is(err, icloud.ErrBusy):
		// The limit may not be this account's alone.
		d.stop(appleBusy, err)
	case err != nil && d.ctx.Err() != nil:
		d.stop(outOfTime, err)
	default:
		return false
	}
	return true
}

// save keeps the Member's list, times and switch, with time of its own, and
// reports whether they were kept. The store answering ErrNotFound means the
// Member disconnected, connected again, picked another calendar or turned
// the switch off meanwhile, and nothing was changed.
func (d *dailyRun) save(c store.ICloud) bool {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(d.ctx), dailySave)
	defer cancel()
	err := d.t.Store.UpdateICloudDaily(ctx, c)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		d.t.Log.Error("soccer: daily iCloud update: keep the games added and moved", "err", err)
	}
	return err == nil
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
