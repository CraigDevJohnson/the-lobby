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
	answer := []byte(`{"events":[{"id":"101","summary":"ROVERS vs KICKERS","description":"Field 3","location":"Boise","start":{"dateTime":"2026-11-21T19:15:00-07:00","timeZone":"America/Denver"},"end":{"dateTime":"2026-11-21T20:05:00-07:00","timeZone":"America/Denver"},"reminders":{"useDefault":false,"overrides":[{"method":"popup","minutes":40}]},"extendedProperties":{"private":{"game_id":"101"}}}]}`)
	events, err := Events(answer)
	if err != nil || len(events) != 1 || events[0].ID() != "101" {
		t.Fatalf("events = %+v, %v", events, err)
	}
	var payload struct {
		ID                 string
		Summary            string
		ExtendedProperties struct {
			Private map[string]string
		}
	}
	if err := json.Unmarshal(events[0].data, &payload); err != nil || payload.ID != "101" || payload.Summary != "ROVERS vs KICKERS" || payload.ExtendedProperties.Private["game_id"] != "101" {
		t.Fatalf("payload = %s, %v", events[0].data, err)
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

// Provider behavior outside the structured contract, aliases for one event
// and excessive work reject the complete answer before Write can receive it.
func TestEventsRejectsBackendAuthority(t *testing.T) {
	for name, answer := range map[string]string{
		"attendee":       `{"events":[{"id":"101","attendees":[{"email":"guest@example.com"}]}]}`,
		"email reminder": `{"events":[{"id":"101","reminders":{"useDefault":false,"overrides":[{"method":"email","minutes":1}]}}]}`,
		"outside ID":     `{"events":[{"id":"101/x"}]}`,
		"duplicate":      `{"events":[{"id":"101"},{"id":"101"}]}`,
	} {
		if events, err := Events([]byte(answer)); err == nil || events != nil {
			t.Errorf("%s: events = %+v, err = %v", name, events, err)
		}
	}

	var many strings.Builder
	many.WriteString(`{"events":[`)
	for i := 0; i <= maxEvents; i++ {
		if i > 0 {
			many.WriteByte(',')
		}
		fmt.Fprintf(&many, `{"id":"%d"}`, i)
	}
	many.WriteString(`]}`)
	if events, err := Events([]byte(many.String())); err == nil || events != nil {
		t.Errorf("too many: events = %+v, err = %v", events, err)
	}
}
