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

// Calendar settings and provider-driven deletes are compare-and-set
// operations on the connection that was read. A deleted item stays absent,
// and reconnecting with the same sealed credential still has a new identity
// that an older request cannot update or delete.
func TestConnectionMutationsOnlyChangeTheConnectionTheyRead(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	g := Google{Email: "player@example.com", Account: "player@gmail.example", Token: "sealed", CalendarID: "primary", ConnectionID: "google-1"}
	_ = m.PutGoogle(ctx, g)
	if err := m.UpdateGoogleCalendar(ctx, g, "family"); err != nil {
		t.Fatal(err)
	}
	liveGoogle, _ := m.GetGoogle(ctx, g.Email)
	if liveGoogle.CalendarID != "family" || liveGoogle.Token != g.Token || liveGoogle.ConnectionID != g.ConnectionID {
		t.Fatalf("updated Google connection: %+v", liveGoogle)
	}
	_ = m.DeleteGoogle(ctx, g.Email)
	if err := m.UpdateGoogleCalendar(ctx, g, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Google update after disconnect: %v, want ErrNotFound", err)
	}
	if _, err := m.GetGoogle(ctx, g.Email); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Google update recreated the connection: %v", err)
	}
	newGoogle := g
	newGoogle.ConnectionID, newGoogle.CalendarID = "google-2", "new"
	_ = m.PutGoogle(ctx, newGoogle)
	if err := m.UpdateGoogleCalendar(ctx, g, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Google update after reconnect: %v, want ErrNotFound", err)
	}
	if err := m.DeleteGoogleConnection(ctx, g); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Google delete after reconnect: %v, want ErrNotFound", err)
	}
	if liveGoogle, _ = m.GetGoogle(ctx, g.Email); liveGoogle.ConnectionID != "google-2" || liveGoogle.CalendarID != "new" {
		t.Fatalf("older Google request changed the new connection: %+v", liveGoogle)
	}

	c := ICloud{Email: "player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", ConnectionID: "icloud-1", Daily: true,
		Written: []string{"game-101@soccer.example"}, Times: map[string]GameTime{"game-101@soccer.example": {Start: "20261121T191500", Field: "Field 3"}}}
	_ = m.PutICloud(ctx, c)
	if err := m.UpdateICloudCalendar(ctx, c, "home"); err != nil {
		t.Fatal(err)
	}
	liveICloud, _ := m.GetICloud(ctx, c.Email)
	if liveICloud.CalendarID != "home" || !liveICloud.Daily || liveICloud.Written != nil || liveICloud.Times != nil || liveICloud.Password != c.Password || liveICloud.ConnectionID != c.ConnectionID {
		t.Fatalf("updated iCloud connection: %+v", liveICloud)
	}
	if err := m.UpdateICloudDailySwitch(ctx, liveICloud, false, liveICloud.CalendarID); err != nil {
		t.Fatal(err)
	}
	if liveICloud, _ = m.GetICloud(ctx, c.Email); liveICloud.Daily {
		t.Fatalf("daily switch stayed on: %+v", liveICloud)
	}
	_ = m.DeleteICloud(ctx, c.Email)
	if err := m.UpdateICloudCalendar(ctx, c, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("iCloud update after disconnect: %v, want ErrNotFound", err)
	}
	if err := m.UpdateICloudDailySwitch(ctx, c, true, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("iCloud switch after disconnect: %v, want ErrNotFound", err)
	}
	if _, err := m.GetICloud(ctx, c.Email); !errors.Is(err, ErrNotFound) {
		t.Fatalf("iCloud update recreated the connection: %v", err)
	}
	newICloud := c
	newICloud.ConnectionID, newICloud.CalendarID, newICloud.Daily, newICloud.Written, newICloud.Times = "icloud-2", "new", false, nil, nil
	_ = m.PutICloud(ctx, newICloud)
	if err := m.UpdateICloudCalendar(ctx, c, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("iCloud update after reconnect: %v, want ErrNotFound", err)
	}
	if err := m.UpdateICloudDailySwitch(ctx, c, true, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("iCloud switch after reconnect: %v, want ErrNotFound", err)
	}
	if err := m.DeleteICloudConnection(ctx, c); !errors.Is(err, ErrNotFound) {
		t.Fatalf("iCloud delete after reconnect: %v, want ErrNotFound", err)
	}
	if liveICloud, _ = m.GetICloud(ctx, c.Email); liveICloud.ConnectionID != "icloud-2" || liveICloud.CalendarID != "new" || liveICloud.Daily {
		t.Fatalf("older iCloud request changed the new connection: %+v", liveICloud)
	}
}

// The daily iCloud update saves its switch, its list and each game's time
// and field only onto the connection it read: once the Member has turned the
// switch off, connected again, picked another calendar or disconnected,
// nothing is changed.
func TestUpdateICloudDailyOnlyOnTheSameConnection(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	const one, two = "game-101@soccer.example", "game-102@soccer.example"
	first := GameTime{Start: "20261121T191500", Field: "Field 3"}
	written, times := []string{one}, map[string]GameTime{one: first}
	read := ICloud{Email: "player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", Daily: true, Written: written, Times: times}
	_ = m.PutICloud(ctx, read)
	_ = m.PutICloud(ctx, ICloud{Email: "other@example.com", Password: "sealed-2"})
	// What was given to the store is its own copy, as with DynamoDB.
	written[0], times[one] = "changed", GameTime{Start: "changed"}

	all, err := m.ListICloud(ctx)
	if err != nil || len(all) != 2 || all[0].Email != "other@example.com" || all[0].Times != nil || !all[1].Daily || len(all[1].Written) != 1 || all[1].Written[0] != one ||
		len(all[1].Times) != 1 || all[1].Times[one] != first {
		t.Fatalf("list: %+v, %v", all, err)
	}

	second := GameTime{Start: "20261128T181500", Field: "Field 1"}
	update := all[1]
	update.Written = append(update.Written, two)
	update.Times[two] = second
	update.Account, update.Home = "never saved", "never saved"
	// What a list answered is a copy too: changing it changed nothing kept.
	if got, _ := m.GetICloud(ctx, "player@example.com"); len(got.Written) != 1 || len(got.Times) != 1 {
		t.Fatalf("changing what the list answered changed what is kept: %+v", got)
	}
	if err := m.UpdateICloudDaily(ctx, update); err != nil {
		t.Fatal(err)
	}
	// Nor does the update keep the caller's own map.
	update.Times[two] = GameTime{Start: "changed"}
	got, _ := m.GetICloud(ctx, "player@example.com")
	if len(got.Written) != 2 || !got.Daily || got.Account != "player@icloud.example" || got.Home != "home" || len(got.Times) != 2 || got.Times[one] != first || got.Times[two] != second {
		t.Fatalf("after the update: %+v", got)
	}
	// And what a read answered can be changed without changing what is kept.
	got.Times[one] = GameTime{Start: "changed"}
	if again, _ := m.GetICloud(ctx, "player@example.com"); again.Times[one] != first {
		t.Fatalf("changing what a read answered changed what is kept: %+v", again)
	}
	got.Times[one] = first

	// Turned off while the run was on them: the update doesn't turn it back
	// on, and the list stays as it was.
	off := got
	off.Daily = false
	_ = m.PutICloud(ctx, off)
	update.Written = append(update.Written, "game-103@soccer.example")
	if err := m.UpdateICloudDaily(ctx, update); !errors.Is(err, ErrNotFound) {
		t.Fatalf("turned off: %v, want ErrNotFound", err)
	}
	if got, _ := m.GetICloud(ctx, "player@example.com"); got.Daily || len(got.Written) != 2 || got.Times[two] != second {
		t.Fatalf("turned off: the update still wrote: %+v", got)
	}

	// Connected again, so a new sealed password; then another calendar.
	for _, now := range []ICloud{
		{Email: "player@example.com", Password: "sealed again", CalendarID: "family"},
		{Email: "player@example.com", Password: "sealed", CalendarID: "family", ConnectionID: "another connection"},
		{Email: "player@example.com", Password: "sealed", CalendarID: "home"},
	} {
		_ = m.PutICloud(ctx, now)
		if err := m.UpdateICloudDaily(ctx, update); !errors.Is(err, ErrNotFound) {
			t.Fatalf("onto %+v: %v, want ErrNotFound", now, err)
		}
		if got, _ := m.GetICloud(ctx, "player@example.com"); got.Daily || got.Written != nil || got.Times != nil || got.Password != now.Password || got.CalendarID != now.CalendarID {
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
