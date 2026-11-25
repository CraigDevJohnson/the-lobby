package soccer

import (
	"context"
	"slices"

	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/icloud"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

func (t *Tool) DailyICloud(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, dailyBudget)
	defer cancel()
	if t.Store == nil || t.Backend == "" {
		t.Log.Info("soccer: the daily iCloud update isn't set up here")
		return nil
	}
	key, ok := t.icloudKey(ctx)
	if !ok {
		t.Log.Info("soccer: the daily iCloud update isn't set up here")
		return nil
	}
	d := &dailyRun{ctx: ctx, t: t, provider: "iCloud"}
	all, err := t.Store.ListICloud(ctx)
	if err != nil {
		d.stop("the site's table isn't answering", err)
	}
	for _, c := range all {
		updateDaily(d, dailyCalendar[icloud.Event]{
			email:    c.Email,
			history:  dailyHistory{c.Daily, c.Written, c.Times},
			upcoming: d.upcomingICloud,
			current: func() (bool, error) {
				live, err := t.Store.GetICloud(ctx, c.Email)
				return sameICloud(c, live) && live.CalendarID == c.CalendarID && live.Daily, err
			},
			open: func() (dailyWriter[icloud.Event], error) { return d.openICloud(c, key) },
			save: func(ctx context.Context, h dailyHistory) error {
				c.Daily, c.Written, c.Times = h.on, h.written, h.times
				return t.Store.UpdateICloudDaily(ctx, c)
			},
		})
	}
	d.report()
	return nil
}

func (d *dailyRun) upcomingICloud(selection dailySelection, notices []icloud.Event) ([]dailyEvent[icloud.Event], error) {
	file, err := d.upcoming("/calendar", selection)
	if err != nil {
		return nil, err
	}
	var games []icloud.Event
	if file != nil {
		games, err = icloud.Events(file)
		if err != nil {
			return nil, err
		}
	}
	out := make([]dailyEvent[icloud.Event], 0, len(games)+len(notices))
	for _, ev := range append(games, notices...) {
		out = append(out, dailyEvent[icloud.Event]{ev.UID, ev.Title, store.GameTime{Start: ev.Start, Field: ev.Field}, ev})
	}
	return out, nil
}

func (d *dailyRun) openICloud(c store.ICloud, key []byte) (w dailyWriter[icloud.Event], err error) {
	password, err := google.Open(key, sealedFor(c.Email), c.Password)
	if err != nil {
		return w, err
	}
	client, ctx := d.t.ICloud, d.ctx
	login := icloud.Login{AppleID: c.Account, Password: password}
	cals, err := client.Calendars(ctx, login, c.Home)
	if err != nil {
		return w, err
	}
	if !slices.ContainsFunc(cals, func(cal icloud.Calendar) bool { return cal.ID == c.CalendarID }) {
		return w, store.ErrNotFound
	}
	w.create = func(ev icloud.Event) (bool, error) { return client.Create(ctx, login, c.CalendarID, ev) }
	w.replace = func(ev icloud.Event) (bool, error) {
		replaced, said, err := client.Replace(ctx, login, c.CalendarID, ev)
		if !replaced && err == nil {
			d.t.Log.Info(d.label()+": a moved game isn't in the calendar", "apple", said)
		}
		return replaced, err
	}
	return w, nil
}
