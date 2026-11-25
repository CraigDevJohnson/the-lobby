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

// Both providers follow Remembered teams and reconcile the games in one destination calendar.
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

type dailyRun struct {
	t        *Tool
	ctx      context.Context
	provider string

	members, added, already, failed, skipped, switchedOff int
	moved, missing, emailed                               int
	stopped                                               string
}

func (d *dailyRun) report() {
	if d.stopped == "" && d.ctx.Err() != nil {
		d.stopped = outOfTime
	}
	// The iCloud verification wizard reads these keys.
	d.t.Log.Info(d.label(), "members", d.members, "added", d.added, "already", d.already, "failed", d.failed,
		"skipped", d.skipped, "switched_off", d.switchedOff, "stopped", d.stopped,
		"moved", d.moved, "missing", d.missing, "emailed", d.emailed)
}

func (d *dailyRun) label() string { return "soccer: daily " + d.provider + " update" }

type dailyHistory struct {
	on      bool
	written []string
	times   map[string]store.GameTime
}

// The UID is shared with the feed, even when the provider uses another event ID.
type dailyEvent[E any] struct {
	uid, title string
	at         store.GameTime
	event      E
}

type dailyWriter[E any] struct {
	create  func(E) (bool, error)
	replace func(E) (bool, error)
}

type dailyCalendar[E any] struct {
	email    string
	history  dailyHistory
	upcoming func(dailySelection, []icloud.Event) ([]dailyEvent[E], error)
	current  func() (bool, error)
	open     func() (dailyWriter[E], error)
	save     func(context.Context, dailyHistory) error
}

func updateDaily[E any](d *dailyRun, cal dailyCalendar[E]) {
	if !cal.history.on {
		return
	}
	d.members++
	if d.stopped != "" || d.ctx.Err() != nil {
		return
	}
	t, ctx := d.t, d.ctx
	if m, err := t.Store.GetMember(ctx, cal.email); err != nil || !slices.Contains(m.Tools, Grant) {
		d.skip(err)
		return
	}
	s, err := t.Store.GetSoccer(ctx, cal.email)
	if err != nil || len(s.Teams) == 0 {
		d.skip(err)
		return
	}
	f, res, err := t.feed(new(http.Request).WithContext(ctx), s)
	if res != nil {
		res.Body.Close()
		err = errors.New("the backend answered " + res.Status)
	}
	if err != nil {
		d.stop(backendDown, err)
		return
	}
	all, err := icloud.Events([]byte(f.Calendar))
	if err != nil {
		d.stop(backendDown, err)
		return
	}
	selection := dailySelection{TeamIDs: slices.Concat(f.Teams...), Since: time.Now().UTC()}
	var notices []icloud.Event
	held := make(map[string]bool, len(all))
	for _, ev := range all {
		held[ev.UID] = true
		if strings.HasPrefix(ev.UID, sessionOverUID) {
			notices = append(notices, ev)
		} else if at, tracked := cal.history.times[ev.UID]; tracked && at != (store.GameTime{Start: ev.Start, Field: ev.Field}) {
			// A game moved earlier may already have started. Include its ID so
			// the copy still at the old time is corrected on this run.
			selection.Included = append(selection.Included, ev.GameID())
		}
	}
	games, err := cal.upcoming(selection, notices)
	if err != nil {
		d.stop(backendDown, err)
		return
	}
	// The two provider tasks can follow the same Team concurrently. Use the
	// persisted selection after both league reads so either task can proceed
	// with an identical follow, and a Member's edit wins over a stale feed.
	live, err := t.Store.GetSoccer(ctx, cal.email)
	if err != nil || !sameTeamIDs(live.Teams, f.Teams) {
		d.skip(err)
		return
	}
	// Drop history outside the two retained Sessions. Keep notice IDs even
	// on days the league cannot check a Team's division.
	before := cal.history
	now := dailyHistory{on: true, times: make(map[string]store.GameTime)}
	for _, uid := range before.written {
		if held[uid] || strings.HasPrefix(uid, sessionOverUID) {
			now.written = append(now.written, uid)
			if at, ok := before.times[uid]; ok {
				now.times[uid] = at
			}
		}
	}
	var fresh, moved []dailyEvent[E]
	for _, ev := range games {
		at, timed := now.times[ev.uid]
		switch {
		case !slices.Contains(now.written, ev.uid):
			fresh = append(fresh, ev)
		case !strings.HasPrefix(ev.uid, gameUID):
		case !timed:
			now.times[ev.uid] = ev.at
		case at != ev.at && d.timeToMove():
			moved = append(moved, ev)
		}
	}

	var changes []movedGame
	if len(fresh)+len(moved) > 0 {
		changes = writeDaily(d, cal, &now, fresh, moved)
	}
	if now.on == before.on && slices.Equal(now.written, before.written) && maps.Equal(now.times, before.times) {
		return
	}
	// A run that ran out of time still checkpoints successful writes. Email
	// follows the conditional save so a stale run never announces a move.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dailySave)
	defer cancel()
	if err := cal.save(saveCtx, now); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			d.t.Log.Error(d.label()+": keep the games added and moved", "err", err)
		}
		return
	}
	if len(changes) > 0 && t.Email != nil {
		if err := t.Email(ctx, movedEmail(t.BaseURL, cal.email, d.provider, changes)); err != nil {
			d.t.Log.Warn(d.label()+": the email about moved games wasn't sent", "err", err)
			return
		}
		d.emailed++
	}
}

func writeDaily[E any](d *dailyRun, cal dailyCalendar[E], now *dailyHistory, fresh, moved []dailyEvent[E]) []movedGame {
	current := func() bool {
		if d.ctx.Err() != nil {
			d.stopped = outOfTime
			return false
		}
		ok, err := cal.current()
		if !ok || err != nil {
			d.skip(err)
			return false
		}
		return true
	}
	if !current() {
		return nil
	}
	writer, err := cal.open()
	if err != nil {
		if !d.ends(now, err) {
			d.skip(err)
		}
		return nil
	}
	for _, ev := range fresh {
		if !current() {
			return nil
		}
		created, err := writer.create(ev.event)
		switch {
		case d.ends(now, err):
			return nil
		case err != nil:
			d.failed++
			d.t.Log.Warn(d.label()+": add a game", "err", err)
			continue
		case created:
			d.added++
		default:
			d.already++
		}
		now.written = append(now.written, ev.uid)
		if strings.HasPrefix(ev.uid, gameUID) {
			now.times[ev.uid] = ev.at
		}
	}
	if !d.timeToMove() {
		return nil
	}
	var changes []movedGame
	for _, ev := range moved {
		if !current() {
			break
		}
		found, err := writer.replace(ev.event)
		switch {
		case d.ends(now, err):
			return changes
		case err != nil:
			d.failed++
			d.t.Log.Warn(d.label()+": move a game", "err", err)
			continue
		case found:
			d.moved++
		default:
			d.missing++
		}
		changes = append(changes, movedGame{Title: ev.title, Was: now.times[ev.uid], Now: ev.at, Found: found})
		now.times[ev.uid] = ev.at
	}
	return changes
}

func (d *dailyRun) ends(now *dailyHistory, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, icloud.ErrDenied), errors.Is(err, google.ErrExpired), errors.Is(err, google.ErrSealed):
		now.on = false
		d.switchedOff++
	case errors.Is(err, icloud.ErrBusy):
		d.stop(appleBusy, err)
	case errors.Is(err, google.ErrBusy):
		d.stop("Google said to slow down", err)
	case d.ctx.Err() != nil:
		d.stop(outOfTime, err)
	default:
		return false
	}
	return true
}

func (d *dailyRun) timeToMove() bool {
	end, ok := d.ctx.Deadline()
	return !ok || time.Until(end) >= dailyMoves
}

func sameTeamIDs(teams []store.RememberedTeam, ids [][]string) bool {
	if len(teams) != len(ids) {
		return false
	}
	for i, team := range teams {
		if !slices.Equal(team.TeamIDs, ids[i]) {
			return false
		}
	}
	return true
}

// Both backend formats accept the same cutoff, followed Teams and exceptions.
type dailySelection struct {
	TeamIDs  []string  `json:"team_ids"`
	Since    time.Time `json:"since"`
	Included []string  `json:"included,omitempty"`
}

func (d *dailyRun) upcoming(path string, selection dailySelection) ([]byte, error) {
	body, err := json.Marshal(selection)
	if err != nil {
		return nil, err
	}
	res, err := d.t.do(new(http.Request).WithContext(d.ctx), http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNotFound:
		return nil, nil
	case http.StatusOK:
		return read(res)
	default:
		return nil, errors.New("the backend answered " + res.Status)
	}
}

func (d *dailyRun) skip(err error) {
	d.skipped++
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		d.t.Log.Warn(d.label()+": skipped a Member", "err", err)
	}
}
func (d *dailyRun) stop(why string, err error) {
	if d.ctx.Err() != nil {
		d.stopped = outOfTime
		return
	}
	d.stopped = why
	d.t.Log.Error(d.label()+" stopped", "err", err)
}
