package store

import (
	"context"
	"errors"
	"testing"
)

// A Member's Let's Plan Soccer calendar is its own record: writing the
// Member again, as `site member add` does, keeps it, and removing the
// Member deletes it, stops the link and forgets their Google access.
func TestMemberRecordAndSoccerCalendar(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	_ = m.PutMember(ctx, Member{Email: "Player@Example.com", Tools: []string{"soccer"}})
	teams := []RememberedTeam{{Name: "ROVERS", TeamIDs: []string{"311111", "111111"}}}
	if err := m.PutSoccer(ctx, Soccer{Email: "player@example.com", Teams: teams, Secret: "s1"}); err != nil {
		t.Fatal(err)
	}
	_ = m.PutSoccerLink(ctx, "s1", "player@example.com")
	teams[0].TeamIDs[0] = "changed"

	_ = m.PutMember(ctx, Member{Email: "player@example.com", Tools: []string{"soccer", "minecraft"}})
	s, err := m.GetSoccer(ctx, "PLAYER@example.com")
	if err != nil || s.Secret != "s1" || len(s.Teams) != 1 || s.Teams[0].TeamIDs[0] != "311111" {
		t.Fatalf("after member add: %+v, %v", s, err)
	}
	if email, err := m.SoccerLink(ctx, "s1"); err != nil || email != "player@example.com" {
		t.Fatalf("link: %q, %v", email, err)
	}

	_ = m.PutGoogle(ctx, Google{Email: "player@example.com", Account: "player@gmail.example", Token: "sealed", CalendarID: "primary"})
	if g, err := m.GetGoogle(ctx, "Player@example.com"); err != nil || g.Token != "sealed" {
		t.Fatalf("google: %+v, %v", g, err)
	}

	if err := m.DeleteMember(ctx, "player@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetSoccer(ctx, "player@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("calendar left: %v", err)
	}
	if _, err := m.SoccerLink(ctx, "s1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("link left: %v", err)
	}
	if _, err := m.GetGoogle(ctx, "player@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("google access left: %v", err)
	}
}

// The daily iCloud update saves its switch and list only onto the
// connection it read: once the Member has turned the switch off, connected
// again, picked another calendar or disconnected, nothing is changed.
func TestUpdateICloudDailyOnlyOnTheSameConnection(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	written := []string{"game-101@soccer.example"}
	read := ICloud{Email: "player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", Daily: true, Written: written}
	_ = m.PutICloud(ctx, read)
	_ = m.PutICloud(ctx, ICloud{Email: "other@example.com", Password: "sealed-2"})
	written[0] = "changed"

	all, err := m.ListICloud(ctx)
	if err != nil || len(all) != 2 || all[0].Email != "other@example.com" || !all[1].Daily || len(all[1].Written) != 1 || all[1].Written[0] != "game-101@soccer.example" {
		t.Fatalf("list: %+v, %v", all, err)
	}

	update := all[1]
	update.Written = append(update.Written, "game-102@soccer.example")
	update.Account, update.Home = "never saved", "never saved"
	if err := m.UpdateICloudDaily(ctx, update); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetICloud(ctx, "player@example.com")
	if len(got.Written) != 2 || !got.Daily || got.Account != "player@icloud.example" || got.Home != "home" {
		t.Fatalf("after the update: %+v", got)
	}

	// Turned off while the run was on them: the update doesn't turn it back
	// on, and the list stays as it was.
	off := got
	off.Daily = false
	_ = m.PutICloud(ctx, off)
	update.Written = append(update.Written, "game-103@soccer.example")
	if err := m.UpdateICloudDaily(ctx, update); !errors.Is(err, ErrNotFound) {
		t.Fatalf("turned off: %v, want ErrNotFound", err)
	}
	if got, _ := m.GetICloud(ctx, "player@example.com"); got.Daily || len(got.Written) != 2 {
		t.Fatalf("turned off: the update still wrote: %+v", got)
	}

	// Connected again, so a new sealed password; then another calendar.
	for _, now := range []ICloud{
		{Email: "player@example.com", Password: "sealed again", CalendarID: "family"},
		{Email: "player@example.com", Password: "sealed", CalendarID: "home"},
	} {
		_ = m.PutICloud(ctx, now)
		if err := m.UpdateICloudDaily(ctx, update); !errors.Is(err, ErrNotFound) {
			t.Fatalf("onto %+v: %v, want ErrNotFound", now, err)
		}
		if got, _ := m.GetICloud(ctx, "player@example.com"); got.Daily || got.Written != nil || got.Password != now.Password || got.CalendarID != now.CalendarID {
			t.Fatalf("onto %+v: the update still wrote: %+v", now, got)
		}
	}
	// Disconnected: the update doesn't bring the connection back.
	_ = m.DeleteICloud(ctx, "player@example.com")
	if err := m.UpdateICloudDaily(ctx, update); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disconnected: %v, want ErrNotFound", err)
	}
	if _, err := m.GetICloud(ctx, "player@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disconnected: the update brought the connection back: %v", err)
	}
}
