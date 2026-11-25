package soccer

import (
	"context"
	"errors"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/icloud"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

func (t *Tool) DailyGoogle(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, dailyBudget)
	defer cancel()
	if t.Store == nil || t.Backend == "" {
		return nil
	}
	settings, ok := t.settings(ctx)
	if !ok {
		t.Log.Info("soccer: the daily Google update isn't set up here")
		return nil
	}
	d := &dailyRun{ctx: ctx, t: t, provider: "Google"}
	all, err := t.Store.ListGoogle(ctx)
	if err != nil {
		d.stop("the site's table isn't answering", err)
	}
	for _, c := range all {
		updateDaily(d, dailyCalendar[google.Event]{
			email:    c.Email,
			history:  dailyHistory{c.Daily, c.Written, c.Times},
			upcoming: d.upcomingGoogle,
			current: func() (bool, error) {
				live, err := t.Store.GetGoogle(ctx, c.Email)
				return sameGoogle(c, live) && live.CalendarID == c.CalendarID && live.Daily && live.DailyRevision == c.DailyRevision, err
			},
			open: func() (dailyWriter[google.Event], error) { return d.openGoogle(c, settings) },
			save: func(ctx context.Context, h dailyHistory) error {
				c.Daily, c.Written, c.Times = h.on, h.written, h.times
				return t.Store.UpdateGoogleDaily(ctx, c)
			},
		})
	}
	d.report()
	return nil
}

func (d *dailyRun) upcomingGoogle(selection dailySelection, notices []icloud.Event) ([]dailyEvent[google.Event], error) {
	body, err := d.upcoming("/events", selection)
	if err != nil {
		return nil, err
	}
	var games []google.Event
	if body != nil {
		games, err = google.Events(body)
		if err != nil {
			return nil, err
		}
	}
	out := make([]dailyEvent[google.Event], 0, len(games)+len(notices))
	for _, ev := range games {
		if ev.Start == "" {
			return nil, errors.New("google: daily game has no scheduled start")
		}
		at := store.GameTime{Start: ev.Start, Field: ev.Field}
		out = append(out, dailyEvent[google.Event]{icloud.GameUID(ev.GameID()), ev.Title, at, ev})
	}
	for _, notice := range notices {
		ev, err := google.SessionNotice(notice.UID, notice.Start, notice.Title, notice.Description)
		if err != nil {
			return nil, err
		}
		out = append(out, dailyEvent[google.Event]{uid: notice.UID, title: notice.Title, event: ev})
	}
	return out, nil
}

func (d *dailyRun) openGoogle(c store.Google, s google.Settings) (w dailyWriter[google.Event], err error) {
	refresh, err := google.Open(s.Key, c.Email, c.Token)
	if err != nil {
		return w, err
	}
	t, ctx := d.t, d.ctx
	access, err := t.Google.Access(ctx, s, refresh)
	if err != nil {
		return w, err
	}
	calendars, err := t.googleCalendars(ctx, access)
	if err != nil {
		return w, err
	}
	if !googleCalendarAvailable(calendars, c.CalendarID) {
		return w, store.ErrNotFound
	}
	w.create = func(ev google.Event) (bool, error) { return t.Google.Create(ctx, access, c.CalendarID, ev) }
	w.replace = func(ev google.Event) (bool, error) { return t.Google.Replace(ctx, access, c.CalendarID, ev) }
	return w, nil
}
