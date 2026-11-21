package soccer

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// removedMeanwhile is a store where the Owner's removal of the Member begins
// just before one of their calendar records is written: the request passed
// its access check while they were a Member, and its write lands once the
// removal has marked them. The removal stops there, as one whose cleanup
// fails does, so a record the late write left behind would still be there.
// links is each Member link secret a request tried to keep.
type removedMeanwhile struct {
	*store.Memory
	links []string
}

func (s *removedMeanwhile) remove(ctx context.Context) {
	s.FailCleanup(errors.New("the table stopped answering"))
	_ = s.DeleteMember(ctx, memberEmail)
}

func (s *removedMeanwhile) PutGoogle(ctx context.Context, g store.Google) error {
	s.remove(ctx)
	return s.Memory.PutGoogle(ctx, g)
}

func (s *removedMeanwhile) PutICloud(ctx context.Context, c store.ICloud) error {
	s.remove(ctx)
	return s.Memory.PutICloud(ctx, c)
}

func (s *removedMeanwhile) PutSoccer(ctx context.Context, c store.Soccer) error {
	s.remove(ctx)
	return s.Memory.PutSoccer(ctx, c)
}

func (s *removedMeanwhile) PutSoccerLink(ctx context.Context, secret, email string) error {
	s.remove(ctx)
	s.links = append(s.links, secret)
	return s.Memory.PutSoccerLink(ctx, secret, email)
}

// finishRemoval is the Owner removing the Member again, which goes through,
// and then inviting the same address: the new Member must start with none of
// the old one's calendar records.
func finishRemoval(t *testing.T, m *memberSite) {
	t.Helper()
	ctx := context.Background()
	m.store.FailCleanup(nil)
	if err := m.store.DeleteMember(ctx, memberEmail); err != nil {
		t.Fatal(err)
	}
	if err := m.store.CreateMember(ctx, store.Member{Email: memberEmail, Tools: []string{Grant}, Generation: "second"}); err != nil {
		t.Fatalf("adding the address once the removal finished: %v", err)
	}
	_, soccerErr := m.store.GetSoccer(ctx, memberEmail)
	_, googleErr := m.store.GetGoogle(ctx, memberEmail)
	_, icloudErr := m.store.GetICloud(ctx, memberEmail)
	for name, err := range map[string]error{"Remembered teams": soccerErr, "Google connection": googleErr, "iCloud connection": icloudErr} {
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("the new Member has the old one's %s: %v", name, err)
		}
	}
}

// Google sends a Member back to the callback while the Owner is removing
// them. The callback had passed its access check, but the connection it then
// saves is refused, so the removal leaves no Google record behind and a later
// Member with the same address has no connection.
func TestAGoogleConnectionIsNotKeptOnceTheMembersRemovalHasBegun(t *testing.T) {
	m, _ := newGoogleSite(t)
	m.tool.Store = &removedMeanwhile{Memory: m.store}
	if to := connect(t, m, same); to != "/soccer?google=failed" {
		t.Fatalf("the callback of a Member being removed went to %s", to)
	}
	if _, err := m.store.GetGoogle(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the callback kept a Google connection: %v", err)
	}
	finishRemoval(t, m)
}

// The same for iCloud: Apple accepts the password while the Owner is
// removing the Member, and the connection is not kept.
func TestAnICloudConnectionIsNotKeptOnceTheMembersRemovalHasBegun(t *testing.T) {
	m, _ := newICloudSite(t)
	m.tool.Store = &removedMeanwhile{Memory: m.store}
	if code, a := m.icloudConnect(t, applePassword); code != http.StatusServiceUnavailable || a["code"] != "tool_unavailable" {
		t.Fatalf("the connect of a Member being removed: %d %v", code, a)
	}
	if _, err := m.store.GetICloud(context.Background(), memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the connect kept an iCloud connection: %v", err)
	}
	finishRemoval(t, m)
}

// And for My calendar. A first Team makes the Member link and then saves the
// Remembered teams: begun before either, the removal leaves neither. With a
// calendar already kept, a save and a new link change nothing, and the
// removal deletes what was there.
func TestRememberedTeamsAndAMemberLinkAreNotKeptOnceTheMembersRemovalHasBegun(t *testing.T) {
	ctx := context.Background()

	m := newMemberSite(t)
	late := &removedMeanwhile{Memory: m.store}
	m.tool.Store = late
	if res, body := m.call(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`); res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "tool_unavailable" {
		t.Fatalf("a first Team for a Member being removed: %d %s", res.StatusCode, body)
	}
	if _, err := m.store.GetSoccer(ctx, memberEmail); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the add kept Remembered teams: %v", err)
	}
	if len(late.links) != 1 {
		t.Fatalf("the add tried %d Member links", len(late.links))
	}
	if _, err := m.store.SoccerLink(ctx, late.links[0]); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the add kept a Member link: %v", err)
	}
	finishRemoval(t, m)

	for name, change := range map[string]struct{ method, path, body string }{
		"another Team": {http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["222222"]}`},
		"a new link":   {http.MethodPost, "/api/soccer/me/link", ""},
	} {
		m := newMemberSite(t)
		before := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`)
		kept, _ := m.store.GetSoccer(ctx, memberEmail)
		late := &removedMeanwhile{Memory: m.store}
		m.tool.Store = late
		if res, body := m.call(t, change.method, change.path, change.body); res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "tool_unavailable" {
			t.Fatalf("%s for a Member being removed: %d %s", name, res.StatusCode, body)
		}
		if now, err := m.store.GetSoccer(ctx, memberEmail); err != nil || now.Secret != kept.Secret || len(now.Teams) != 1 {
			t.Errorf("%s changed the calendar: %+v, %v", name, now, err)
		}
		for _, secret := range late.links {
			if _, err := m.store.SoccerLink(ctx, secret); !errors.Is(err, store.ErrNotFound) {
				t.Errorf("%s kept a Member link: %v", name, err)
			}
		}
		// The link the Member already had answers nothing for them now.
		if res, body := m.fetch(t, before.URL); res.StatusCode != http.StatusNotFound || errorCode(t, body) != "invalid_link" {
			t.Errorf("%s: the Member link of a Member being removed: %d %s", name, res.StatusCode, body)
		}
		finishRemoval(t, m)
		if _, err := m.store.SoccerLink(ctx, kept.Secret); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s: the old Member link is still kept: %v", name, err)
		}
	}
}

// None of that stops an ordinary connect or a first Team, for a Member
// invited since generations and for one made before them, whose record has
// none of the attributes added since.
func TestCalendarRecordsAreKeptForAMemberFromBeforeGenerations(t *testing.T) {
	ctx := context.Background()
	for name, member := range map[string]store.Member{
		"from before generations": {Email: memberEmail, Tools: []string{Grant}},
		"invited since":           {Email: memberEmail, Tools: []string{Grant}, Generation: "first"},
	} {
		m, _ := newGoogleSite(t)
		_ = m.store.PutMember(ctx, member)
		if to := connect(t, m, same); to != "/soccer?google=connected" {
			t.Errorf("%s: the Google callback went to %s", name, to)
		}
		if g, err := m.store.GetGoogle(ctx, memberEmail); err != nil || g.ConnectionID == "" {
			t.Errorf("%s: the Google connection: %+v, %v", name, g, err)
		}
		if a := m.mine(t, http.MethodPost, "/api/soccer/me/teams", `{"team_ids":["111111"]}`); a.URL == "" || len(a.Teams) != 1 {
			t.Errorf("%s: My calendar: %+v", name, a)
		}

		i, _ := newICloudSite(t)
		_ = i.store.PutMember(ctx, member)
		if code, a := i.icloudConnect(t, applePassword); code != http.StatusOK || a["connected"] != true {
			t.Errorf("%s: the iCloud connect: %d %v", name, code, a)
		}
		if c, err := i.store.GetICloud(ctx, memberEmail); err != nil || c.ConnectionID == "" {
			t.Errorf("%s: the iCloud connection: %+v, %v", name, c, err)
		}
	}
}
