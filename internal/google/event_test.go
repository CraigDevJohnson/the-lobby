package google

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A normal backend game keeps its visible fields and reminder, while the
// provider ID is checked and the private ownership marker is built locally.
func TestEventsBuildsOwnedPayload(t *testing.T) {
	answer := []byte(`{"events":[{"id":"123456","summary":"ROVERS vs KICKERS","description":"Field 3","location":"Boise","start":{"dateTime":"2026-11-21T19:15:00-07:00","timeZone":"America/Denver"},"end":{"dateTime":"2026-11-21T20:05:00-07:00","timeZone":"America/Denver"},"status":"confirmed","reminders":{"useDefault":false,"overrides":[{"method":"popup","minutes":40}]},"extendedProperties":{"private":{"game_id":"123456"}}}]}`)
	events, err := Events(answer)
	if err != nil || len(events) != 1 || events[0].ID() != "123456" {
		t.Fatalf("events = %+v, %v", events, err)
	}
	var payload struct {
		ID                 string
		Summary            string
		Status             string
		ExtendedProperties struct {
			Private map[string]string
		}
	}
	if err := json.Unmarshal(events[0].data, &payload); err != nil || payload.ID != "123456" || payload.Summary != "ROVERS vs KICKERS" || payload.Status != "confirmed" || payload.ExtendedProperties.Private["game_id"] != "123456" {
		t.Fatalf("payload = %s, %v", events[0].data, err)
	}
}

func TestEventsKeepsEncodedShortGameID(t *testing.T) {
	events, err := Events([]byte(`{"events":[{"id":"lps64o32","extendedProperties":{"private":{"game_id":"101"}}}]}`))
	if err != nil || len(events) != 1 || events[0].ID() != "lps64o32" || !strings.Contains(string(events[0].data), `"game_id":"101"`) {
		t.Fatalf("events = %+v, %v", events, err)
	}
}

// All-day Session notices use the same checked contract, preserving their
// dates and transparent calendar behavior.
func TestEventsKeepsAllDayNotice(t *testing.T) {
	events, err := Events([]byte(`{"events":[{"id":"sessionover111111","summary":"ROVERS: this Session is over","start":{"date":"2026-12-20"},"end":{"date":"2026-12-21"},"transparency":"transparent"}]}`))
	if err != nil || len(events) != 1 || !strings.Contains(string(events[0].data), `"date":"2026-12-20"`) || !strings.Contains(string(events[0].data), `"transparency":"transparent"`) {
		t.Fatalf("events = %+v, %v", events, err)
	}
}

// Provider behavior outside the structured contract and aliases for one
// event reject the complete answer before Write can receive it.
func TestEventsRejectsBackendAuthority(t *testing.T) {
	for name, answer := range map[string]string{
		"attendee":       `{"events":[{"id":"101","attendees":[{"email":"guest@example.com"}]}]}`,
		"cancelled":      `{"events":[{"id":"101","status":"cancelled"}]}`,
		"email reminder": `{"events":[{"id":"101","reminders":{"useDefault":false,"overrides":[{"method":"email","minutes":1}]}}]}`,
		"outside ID":     `{"events":[{"id":"101/x"}]}`,
		"unrelated ID":   `{"events":[{"id":"lps64o32","extendedProperties":{"private":{"game_id":"102"}}}]}`,
		"duplicate":      `{"events":[{"id":"101"},{"id":"101"}]}`,
	} {
		if events, err := Events([]byte(answer)); err == nil || events != nil {
			t.Errorf("%s: events = %+v, err = %v", name, events, err)
		}
	}
}

func TestEventsStopsDecodingAtLimit(t *testing.T) {
	var many strings.Builder
	many.WriteString(`{"events":[`)
	for i := range maxEvents {
		if i > 0 {
			many.WriteByte(',')
		}
		fmt.Fprintf(&many, `{"id":"%05d"}`, i)
	}
	if events, err := Events([]byte(many.String() + `]}`)); err != nil || len(events) != maxEvents {
		t.Fatalf("at limit: got %d events, err = %v", len(events), err)
	}
	if events, err := Events([]byte(many.String() + `,42]}`)); err == nil || err.Error() != "google: too many backend events" || events != nil {
		t.Fatalf("over limit: got %d events, err = %v", len(events), err)
	}
}

func TestDailyGameAndSessionNotice(t *testing.T) {
	const fallback = "20261108T1830-North Court"
	input := `{"events":[{"id":"lps68o34dhh64o3gl1h70pj0baedtp78q108dnnasjk","summary":"ROVERS vs OWLS","description":"North Court\nROVERS vs OWLS","start":{"dateTime":"2026-11-08T18:30:00-07:00","timeZone":"America/Denver"},"end":{"dateTime":"2026-11-08T19:20:00-07:00","timeZone":"America/Denver"},"extendedProperties":{"private":{"game_id":"` + fallback + `"}}}]}`
	events, err := Events([]byte(input))
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %v, %v", events, err)
	}
	if game := events[0]; game.GameID() != fallback || game.Start != "20261108T183000" || game.Field != "North Court" || game.Title != "ROVERS vs OWLS" {
		t.Fatalf("daily metadata = %+v", game)
	}
	const uid = "session-over-123456@soccer.craigdevjohnson.com"
	notice, err := SessionNotice(uid, "20261231", "ROVERS: this Session is over", "Pick next Session's team at https://lobby.example/soccer?after=123456")
	if err != nil {
		t.Fatal(err)
	}
	var body eventPayload
	if err := json.Unmarshal(notice.data, &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != "sessionover123456" || body.Start.Date != "2026-12-31" || body.End.Date != "2027-01-01" ||
		body.Transparency != "transparent" || body.Reminders == nil || body.Reminders.UseDefault == nil || *body.Reminders.UseDefault || len(body.Reminders.Overrides) != 0 || body.ExtendedProperties.Private["game_id"] != uid ||
		!strings.Contains(body.Description, "after=123456") {
		t.Fatalf("notice = %s", notice.data)
	}
	for _, invalid := range []string{"session-over-oops@soccer.craigdevjohnson.com", "game-123456@soccer.craigdevjohnson.com", "session-over-123456@elsewhere"} {
		if _, err := SessionNotice(invalid, "20261231", "Notice", ""); err == nil {
			t.Fatalf("accepted uid %q", invalid)
		}
	}
}
