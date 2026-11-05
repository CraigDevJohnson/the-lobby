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
