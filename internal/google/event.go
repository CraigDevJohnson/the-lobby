package google

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// maxEvents bounds one press of Add before any work is started. Ten Teams
// fit comfortably inside it, while a backend answer cannot turn the site's
// four provider writers into an unbounded queue.
const maxEvents = 256

// Event is one backend event after its provider fields and identifier have
// been checked and its Google payload has been built here. data is private so
// callers cannot use Write to send an unchecked provider payload.
type Event struct {
	id   string
	data json.RawMessage
}

func (e Event) ID() string { return e.id }

type eventAnswer struct {
	Events []eventInput `json:"events"`
}

type eventInput struct {
	ID                 string          `json:"id"`
	Summary            string          `json:"summary,omitempty"`
	Description        string          `json:"description,omitempty"`
	Location           string          `json:"location,omitempty"`
	Start              *eventTime      `json:"start,omitempty"`
	End                *eventTime      `json:"end,omitempty"`
	Transparency       string          `json:"transparency,omitempty"`
	Reminders          *eventReminders `json:"reminders,omitempty"`
	ExtendedProperties *eventExtended  `json:"extendedProperties,omitempty"`
}

type eventTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type eventReminders struct {
	UseDefault *bool              `json:"useDefault"`
	Overrides  []reminderOverride `json:"overrides,omitempty"`
}

type reminderOverride struct {
	Method  string `json:"method"`
	Minutes int    `json:"minutes"`
}

type eventExtended struct {
	Private map[string]string `json:"private,omitempty"`
}

type eventPayload struct {
	ID                 string          `json:"id"`
	Summary            string          `json:"summary,omitempty"`
	Description        string          `json:"description,omitempty"`
	Location           string          `json:"location,omitempty"`
	Start              *eventTime      `json:"start,omitempty"`
	End                *eventTime      `json:"end,omitempty"`
	Transparency       string          `json:"transparency,omitempty"`
	Reminders          *eventReminders `json:"reminders,omitempty"`
	ExtendedProperties eventExtended   `json:"extendedProperties"`
}

// Events accepts the small event contract the soccer backend answers and
// constructs the only JSON Calendar.Write can send. Provider fields outside
// this contract, duplicate identifiers and oversized answers reject the whole
// answer before a Member's calendar is touched.
func Events(answer []byte) ([]Event, error) {
	var built eventAnswer
	dec := json.NewDecoder(bytes.NewReader(answer))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&built); err != nil {
		return nil, errors.New("google: invalid backend events")
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("google: invalid backend events")
	}
	if len(built.Events) > maxEvents {
		return nil, errors.New("google: too many backend events")
	}

	out := make([]Event, 0, len(built.Events))
	seen := make(map[string]bool, len(built.Events))
	for _, in := range built.Events {
		if !validEvent(in) || seen[in.ID] {
			return nil, errors.New("google: invalid backend event")
		}
		seen[in.ID] = true

		payload, err := json.Marshal(eventPayload{
			ID:           in.ID,
			Summary:      in.Summary,
			Description:  in.Description,
			Location:     in.Location,
			Start:        in.Start,
			End:          in.End,
			Transparency: in.Transparency,
			Reminders:    in.Reminders,
			ExtendedProperties: eventExtended{Private: map[string]string{
				"game_id": in.ID,
			}},
		})
		if err != nil {
			return nil, errors.New("google: build event")
		}
		out = append(out, Event{id: in.ID, data: payload})
	}
	return out, nil
}

func validEvent(e eventInput) bool {
	if len(e.ID) == 0 || len(e.ID) > 128 {
		return false
	}
	for _, r := range e.ID {
		if r < '0' || r > '9' && (r < 'a' || r > 'v') {
			return false
		}
	}
	if !validText(e.Summary, 512) || !validText(e.Description, 8<<10) || !validText(e.Location, 2<<10) {
		return false
	}
	if (e.Start == nil) != (e.End == nil) || e.Start != nil && !validPeriod(*e.Start, *e.End) {
		return false
	}
	if e.Transparency != "" && e.Transparency != "opaque" && e.Transparency != "transparent" {
		return false
	}
	if e.Reminders != nil && !e.Reminders.valid() {
		return false
	}
	if e.ExtendedProperties != nil {
		if len(e.ExtendedProperties.Private) != 1 || e.ExtendedProperties.Private["game_id"] != e.ID {
			return false
		}
	}
	return true
}

func (t eventTime) valid() bool {
	switch {
	case t.Date != "" && t.DateTime != "":
		return false
	case t.Date != "":
		if t.TimeZone != "" {
			return false
		}
		_, err := time.Parse("2006-01-02", t.Date)
		return err == nil
	case t.DateTime != "":
		if t.TimeZone != "" && t.TimeZone != "America/Denver" {
			return false
		}
		_, err := time.Parse(time.RFC3339, t.DateTime)
		return err == nil
	default:
		return false
	}
}

func validPeriod(start, end eventTime) bool {
	if !start.valid() || !end.valid() || (start.Date != "") != (end.Date != "") || start.TimeZone != end.TimeZone {
		return false
	}
	if start.Date != "" {
		s, _ := time.Parse("2006-01-02", start.Date)
		e, _ := time.Parse("2006-01-02", end.Date)
		return e.After(s)
	}
	s, _ := time.Parse(time.RFC3339, start.DateTime)
	e, _ := time.Parse(time.RFC3339, end.DateTime)
	return e.After(s)
}

func (r eventReminders) valid() bool {
	if r.UseDefault == nil || len(r.Overrides) > 4 || *r.UseDefault && len(r.Overrides) != 0 {
		return false
	}
	for _, reminder := range r.Overrides {
		if reminder.Method != "popup" || reminder.Minutes < 0 || reminder.Minutes > 7*24*60 {
			return false
		}
	}
	return true
}

func validText(s string, limit int) bool {
	if len(s) > limit || !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return false
	}
	for _, r := range s {
		if r < ' ' && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}
