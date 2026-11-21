package icloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const file = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nMETHOD:PUBLISH\r\nCALSCALE:GREGORIAN\r\nX-WR-CALNAME:ROVERS\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:America/Denver\r\nBEGIN:STANDARD\r\nTZOFFSETFROM:-0600\r\nTZOFFSETTO:-0700\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\n" +
	"BEGIN:VEVENT\r\nUID:game-101@soccer.craigdevjohnson.com\r\nSUMMARY:ROVERS vs KICKERS\r\nDTSTART;TZID=America/Denver:20261121T191500\r\nDTEND;TZID=America/Denver:20261121T200500\r\nDESCRIPTION:Field 3\\nROVERS vs KICKERS\r\nBEGIN:VALARM\r\nUID:alarm-1\r\nACTION:DISPLAY\r\nTRIGGER:-PT40M\r\nDESCRIPTION:Soccer game starting soon\r\nEND:VALARM\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nSUMMARY:OWLS vs ROVERS\r\nUID:game-102@soccer.craigdevjohnson.\r\n com\r\nDTSTART;TZID=America/Denver:20261128T183000\r\nDTEND;TZID=America/Denver:20261128T192000\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

// Each checked game becomes a locally built calendar file of its own, with
// the fixed time zone and its own reminder but no backend METHOD or PRODID.
func TestEventsSplitsTheFile(t *testing.T) {
	evs, err := Events([]byte(file))
	if err != nil || len(evs) != 2 || evs[0].UID != "game-101@soccer.craigdevjohnson.com" || evs[1].UID != "game-102@soccer.craigdevjohnson.com" {
		t.Fatalf("events = %+v, %v", evs, err)
	}
	one := string(evs[0].data)
	for _, want := range []string{"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//craigdevjohnson.com//The Lobby//EN\r\nCALSCALE:GREGORIAN\r\nBEGIN:VTIMEZONE", "TZID:America/Denver", "SUMMARY:ROVERS vs KICKERS", "TRIGGER:-PT40M", "END:VEVENT\r\nEND:VCALENDAR\r\n"} {
		if !strings.Contains(one, want) {
			t.Fatalf("missing %q in:\n%s", want, one)
		}
	}
	for _, not := range []string{"METHOD", "X-WR-CALNAME", "-//test//", "OWLS"} {
		if strings.Contains(one, not) {
			t.Fatalf("%q in:\n%s", not, one)
		}
	}
	if strings.Count(one, "BEGIN:VEVENT") != 1 {
		t.Fatalf("more than one event in:\n%s", one)
	}
	if _, err := Events([]byte(`{"code":"x"}`)); err == nil {
		t.Fatal("JSON read as a calendar file")
	}
}

// testdata/backend.ics is a calendar file as the soccer backend writes one
// (its internal/calendar, rendered for two made-up games and a Session-over
// notice, CRLF line ends and all). Each event's start, field and title are
// read from its own lines: not from the time zone's DTSTART lines, which
// come first in every event's file, nor from the reminder's DESCRIPTION. A
// folded line is put back together and its escaping undone.
func TestEventsReadsTheBackendsOwnLines(t *testing.T) {
	file, err := os.ReadFile("testdata/backend.ics")
	if err != nil {
		t.Fatal(err)
	}
	// The traps are in the file: the zone's own DTSTART lines, a folded
	// DESCRIPTION, an escaped comma in a Team name and the reminder's
	// DESCRIPTION.
	for _, trap := range []string{
		"BEGIN:STANDARD\r\nDTSTART:19701101T020000\r\n",
		"BEGIN:DAYLIGHT\r\nDTSTART:19700308T020000\r\n",
		"DESCRIPTION:Field 3\\nROVERS vs KICKERS\\, UNITED FC\\nLet's Play Soccer\\,\r\n  Boise\r\n",
		"SUMMARY:ROVERS vs KICKERS\\, UNITED FC - Field 3\r\n",
		"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT40M\r\nDESCRIPTION:Soccer game starting soon\r\nEND:VALARM\r\n",
	} {
		if !strings.Contains(string(file), trap) {
			t.Fatalf("the fixture lacks %q", trap)
		}
	}
	evs, err := Events(file)
	if err != nil || len(evs) != 3 {
		t.Fatalf("events = %+v, %v", evs, err)
	}
	for i, want := range []Event{
		{UID: "game-101@soccer.craigdevjohnson.com", Start: "20261121T191500", Field: "Field 3", Title: "ROVERS vs KICKERS, UNITED FC - Field 3"},
		{UID: "game-102@soccer.craigdevjohnson.com", Start: "20261128T183000", Field: "North Court", Title: "ROVERS vs OWLS - North Court"},
		// The Session-over notice is all-day and its description is one
		// sentence: it gets what is there, and nothing fails.
		{UID: "session-over-111111@soccer.craigdevjohnson.com", Start: "20261220", Title: "ROVERS: this Session is over",
			Field: "ROVERS has played its last game of this Session. Pick next Session's Team on Let's Plan Soccer, and its games join this same calendar: https://lobby.example/soccer?after=111111"},
	} {
		got := evs[i]
		if got.UID != want.UID || got.Start != want.Start || got.Field != want.Field || got.Title != want.Title {
			t.Errorf("event %d:\n got %q, %q, %q, %q\nwant %q, %q, %q, %q", i, got.UID, got.Start, got.Field, got.Title, want.UID, want.Start, want.Field, want.Title)
		}
	}
	// Each event's own file starts with the zone, whose DTSTART lines so come
	// before the event's, and still carries the reminder.
	one := string(evs[0].data)
	zone, event := strings.Index(one, "DTSTART:19701101T020000"), strings.Index(one, "DTSTART;TZID=America/Denver:20261121T191500")
	if zone < 0 || event < zone || !strings.Contains(one, "DESCRIPTION:Soccer game starting soon") || strings.Contains(one, "game-102") {
		t.Errorf("the first event's file:\n%s", one)
	}
}

// Escapes are undone, folded values are put back together, and a reminder's
// description is never mistaken for the event's field.
func TestEventsReadsFoldedSafeFields(t *testing.T) {
	const odd = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:game-7@soccer.craigdevjohnson.com\r\n" +
		"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT40M\r\nDESCRIPTION:Soccer game starting soon\r\nEND:VALARM\r\n" +
		"DTSTART;TZID=America/Denver:20261121T191500\r\nDTEND;TZID=America/Denver:20261121T200500\r\n" +
		"SUMMARY:A\\; B\\\\C vs D\\, E - Fie\r\n ld 3\r\n" +
		"DESCRIPTION;LANGUAGE=en:Fie\r\n\tld 3\\NA vs B\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	evs, err := Events([]byte(odd))
	if err != nil || len(evs) != 1 {
		t.Fatalf("events = %+v, %v", evs, err)
	}
	if e := evs[0]; e.Start != "20261121T191500" || e.Field != "Field 3" || e.Title != `A; B\C vs D, E - Field 3` {
		t.Errorf("read %q, %q, %q", e.Start, e.Field, e.Title)
	}
}

// Backend-selected attendee behavior, UIDs outside the site's namespace,
// duplicates and excessive work are all rejected before a CalDAV write can
// receive an Event.
func TestEventsRejectsBackendAuthority(t *testing.T) {
	outside := strings.Replace(file, "game-101@soccer.craigdevjohnson.com", "someone-else@example.com", 1)
	attendee := strings.Replace(file, "SUMMARY:ROVERS vs KICKERS", "ATTENDEE:mailto:guest@example.com\r\nSUMMARY:ROVERS vs KICKERS", 1)
	duplicate := strings.Replace(file, "game-102@soccer.craigdevjohnson.com", "game-101@soccer.craigdevjohnson.com", 1)
	for name, input := range map[string]string{"outside UID": outside, "attendee": attendee, "duplicate": duplicate} {
		if events, err := Events([]byte(input)); err == nil || events != nil {
			t.Errorf("%s: events = %+v, err = %v", name, events, err)
		}
	}

	var many strings.Builder
	many.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\n")
	for i := 0; i <= maxEvents; i++ {
		many.WriteString("BEGIN:VEVENT\r\nUID:game-")
		many.WriteString(fmt.Sprint(i))
		many.WriteString("@soccer.craigdevjohnson.com\r\nSUMMARY:game\r\nDTSTART;TZID=America/Denver:20261121T191500\r\nDTEND;TZID=America/Denver:20261121T200500\r\nEND:VEVENT\r\n")
	}
	many.WriteString("END:VCALENDAR\r\n")
	if events, err := Events([]byte(many.String())); err == nil || events != nil {
		t.Errorf("too many: events = %+v, err = %v", events, err)
	}
}

// The password is sent only to the Client's own host and to icloud.com
// hosts, whatever Apple's answer or a request names.
func TestThePasswordGoesNowhereElse(t *testing.T) {
	var elsewhere int
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere++ }))
	defer other.Close()
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<multistatus xmlns="DAV:"><response><href>/</href><propstat><prop><current-user-principal><href>` + other.URL + `/principal/</href></current-user-principal></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`))
	}))
	defer apple.Close()
	c := &Client{HTTP: apple.Client(), URL: apple.URL}
	l := Login{AppleID: "a@example.com", Password: "pw"}
	ctx := context.Background()
	if _, err := c.Home(ctx, l); !errors.Is(err, errUntrusted) {
		t.Fatalf("home: %v", err)
	}
	if _, err := c.Calendars(ctx, l, other.URL+"/calendars/"); !errors.Is(err, errUntrusted) {
		t.Fatalf("calendars: %v", err)
	}
	if _, err := c.Write(ctx, l, other.URL+"/calendars/home/", Event{UID: "u", data: []byte("x")}); !errors.Is(err, errUntrusted) {
		t.Fatalf("write: %v", err)
	}
	if _, err := c.Create(ctx, l, other.URL+"/calendars/home/", Event{UID: "u", data: []byte("x")}); !errors.Is(err, errUntrusted) {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := c.Replace(ctx, l, other.URL+"/calendars/home/", Event{UID: "u", data: []byte("x")}); !errors.Is(err, errUntrusted) {
		t.Fatalf("replace: %v", err)
	}
	if elsewhere != 0 {
		t.Fatalf("%d requests went elsewhere", elsewhere)
	}
	for at, want := range map[string]bool{
		"https://p42-caldav.icloud.com:443/1/calendars/": true,
		"https://caldav.icloud.com/1/":                   true,
		"http://p42-caldav.icloud.com/1/":                false,
		"https://icloud.com.evil.example/1/":             false,
		"https://evilicloud.com/1/":                      false,
		"https://p42-caldav.icloud.com:8443/1/":          false,
	} {
		if _, err := c.resolve(at, ""); (err == nil) != want {
			t.Errorf("resolve(%q): %v, want trusted %v", at, err, want)
		}
	}
}

// Create sends the create-only PUT alone, whatever Apple answers: an event
// already there is left as it is, never replaced.
func TestCreateNeverReplaces(t *testing.T) {
	var (
		status int
		puts   []string
	)
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		puts = append(puts, r.Method+" "+r.URL.Path+" If-None-Match: "+r.Header.Get("If-None-Match"))
		w.WriteHeader(status)
	}))
	defer apple.Close()
	c := &Client{HTTP: apple.Client(), URL: apple.URL}
	l := Login{AppleID: "a@example.com", Password: "pw"}
	for _, want := range []struct {
		status  int
		created bool
		err     error
	}{
		{http.StatusCreated, true, nil},
		{http.StatusNoContent, true, nil},
		{http.StatusPreconditionFailed, false, nil},
		{http.StatusUnauthorized, false, ErrDenied},
		{http.StatusTooManyRequests, false, ErrBusy},
		{http.StatusServiceUnavailable, false, ErrBusy},
	} {
		status, puts = want.status, nil
		created, err := c.Create(context.Background(), l, apple.URL+"/1/calendars/home/", Event{UID: "game-101@soccer.example", data: []byte("x")})
		if created != want.created || !errors.Is(err, want.err) {
			t.Errorf("%d: created %v, %v; want %v, %v", want.status, created, err, want.created, want.err)
		}
		if len(puts) != 1 || puts[0] != "PUT /1/calendars/home/game-101@soccer.example.ics If-None-Match: *" {
			t.Errorf("%d: sent %q, want the create-only PUT alone", want.status, puts)
		}
	}
	// Apple refusing this one event is an error of its own, not a calendar
	// that can't be written to.
	status = http.StatusForbidden
	if _, err := c.Create(context.Background(), l, apple.URL+"/1/calendars/home/", Event{UID: "u", data: []byte("x")}); err == nil || errors.Is(err, ErrCalendar) || errors.Is(err, ErrBusy) || err.Error() != "icloud: status 403" {
		t.Errorf("403: %v", err)
	}
	// Listing calendars is told to slow down the same way, and the status
	// still shows in the log.
	status = http.StatusTooManyRequests
	if _, err := c.Calendars(context.Background(), l, apple.URL+"/1/calendars/"); !errors.Is(err, ErrBusy) || !strings.Contains(err.Error(), "429") {
		t.Errorf("calendars: %v", err)
	}
}

// Replace asks Apple whether the calendar holds the event and sends it only
// when it does, with no condition on the PUT: an event that isn't there is
// never sent, so it is never added. It never sends If-Match: *, which iCloud
// doesn't keep (dev proof, 2026-11-19: refused for an event that was there,
// stored for one that had been deleted).
func TestReplaceNeverAdds(t *testing.T) {
	const there = `<?xml version="1.0"?><multistatus xmlns="DAV:"><response><href>/1/calendars/home/game-101@soccer.example.ics</href>` +
		`<propstat><prop><getetag>"C=7@U=abc"</getetag></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`
	// An answer that names the address and gives no ETag for it.
	const noETag = `<?xml version="1.0"?><multistatus xmlns="DAV:"><response><href>/1/calendars/home/game-101@soccer.example.ics</href>` +
		`<propstat><prop><getetag/></prop><status>HTTP/1.1 404 Not Found</status></propstat></response></multistatus>`
	// An answer that says 200 for the address and gives no ETag in it.
	const emptyETag = `<?xml version="1.0"?><multistatus xmlns="DAV:"><response><href>/1/calendars/home/game-101@soccer.example.ics</href>` +
		`<propstat><prop><getetag/></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`
	// An answer that gives an ETag under a status that isn't 200.
	const refusedETag = `<?xml version="1.0"?><multistatus xmlns="DAV:"><response><href>/1/calendars/home/game-101@soccer.example.ics</href>` +
		`<propstat><prop><getetag>"C=7@U=abc"</getetag></prop><status>HTTP/1.1 404 Not Found</status></propstat></response></multistatus>`
	var (
		ask, put int
		holds    string
		sent     []string
	)
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = append(sent, r.Method+" "+r.URL.Path+" Depth: "+r.Header.Get("Depth")+" If-Match: "+r.Header.Get("If-Match")+" If-None-Match: "+r.Header.Get("If-None-Match"))
		if r.Method == "PROPFIND" {
			w.WriteHeader(ask)
			_, _ = w.Write([]byte(holds))
			return
		}
		w.WriteHeader(put)
	}))
	defer apple.Close()
	c := &Client{HTTP: apple.Client(), URL: apple.URL}
	l := Login{AppleID: "a@example.com", Password: "pw"}
	const asked = "PROPFIND /1/calendars/home/game-101@soccer.example.ics Depth: 0 If-Match:  If-None-Match: "
	const wrote = "PUT /1/calendars/home/game-101@soccer.example.ics Depth:  If-Match:  If-None-Match: "
	for name, want := range map[string]struct {
		ask      int
		holds    string
		put      int
		replaced bool
		said     string
		err      error
		message  string
		sent     []string
	}{
		"there, replaced":              {ask: http.StatusMultiStatus, holds: there, put: http.StatusNoContent, replaced: true, sent: []string{asked, wrote}},
		"there, stored as new":         {ask: http.StatusMultiStatus, holds: there, put: http.StatusCreated, replaced: true, sent: []string{asked, wrote}},
		"not there":                    {ask: http.StatusNotFound, said: NotFound, sent: []string{asked}},
		"an answer without an ETag":    {ask: http.StatusMultiStatus, holds: noETag, said: NoETag, sent: []string{asked}},
		"a 200 with an empty ETag":     {ask: http.StatusMultiStatus, holds: emptyETag, said: NoETag, sent: []string{asked}},
		"an ETag under another status": {ask: http.StatusMultiStatus, holds: refusedETag, said: NoETag, sent: []string{asked}},
		"asked, password refused":      {ask: http.StatusUnauthorized, err: ErrDenied, sent: []string{asked}},
		"asked, slow down":             {ask: http.StatusTooManyRequests, err: ErrBusy, sent: []string{asked}},
		"asked, not available":         {ask: http.StatusServiceUnavailable, err: ErrBusy, sent: []string{asked}},
		"asked, Apple failed":          {ask: http.StatusInternalServerError, message: "icloud: status 500", sent: []string{asked}},
		"sent, password refused":       {ask: http.StatusMultiStatus, holds: there, put: http.StatusUnauthorized, err: ErrDenied, sent: []string{asked, wrote}},
		"sent, slow down":              {ask: http.StatusMultiStatus, holds: there, put: http.StatusTooManyRequests, err: ErrBusy, sent: []string{asked, wrote}},
		"sent, refused on its own":     {ask: http.StatusMultiStatus, holds: there, put: http.StatusForbidden, message: "icloud: status 403", sent: []string{asked, wrote}},
		"sent, a condition Apple made": {ask: http.StatusMultiStatus, holds: there, put: http.StatusPreconditionFailed, message: "icloud: status 412", sent: []string{asked, wrote}},
		"sent, Apple says not found":   {ask: http.StatusMultiStatus, holds: there, put: http.StatusNotFound, message: "icloud: status 404", sent: []string{asked, wrote}},
	} {
		ask, holds, put, sent = want.ask, want.holds, want.put, nil
		replaced, said, err := c.Replace(context.Background(), l, apple.URL+"/1/calendars/home/", Event{UID: "game-101@soccer.example", data: []byte("x")})
		failed := want.err != nil || want.message != ""
		if replaced != want.replaced || said != want.said || (err != nil) != failed || (want.err != nil && !errors.Is(err, want.err)) {
			t.Errorf("%s: replaced %v, Apple said %q, %v; want %v, %q, %v %s", name, replaced, said, err, want.replaced, want.said, want.err, want.message)
		}
		// Apple refusing this one event is an error of its own: not a
		// calendar that can't be written to, and not an event that isn't
		// there.
		if want.message != "" && (err == nil || err.Error() != want.message || errors.Is(err, ErrCalendar) || errors.Is(err, ErrBusy)) {
			t.Errorf("%s: %v, want %s", name, err, want.message)
		}
		if strings.Join(sent, "|") != strings.Join(want.sent, "|") {
			t.Errorf("%s: sent %q, want %q", name, sent, want.sent)
		}
	}
}

// Write and Create send what they always did: Create its create-only PUT,
// and Write that PUT and then, when the event is there, one without a
// condition. Neither sends If-Match.
func TestWriteAndCreateSendNoIfMatch(t *testing.T) {
	var puts []string
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		puts = append(puts, "If-Match: "+r.Header.Get("If-Match")+" If-None-Match: "+r.Header.Get("If-None-Match"))
		if r.Header.Get("If-None-Match") == "*" {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer apple.Close()
	c := &Client{HTTP: apple.Client(), URL: apple.URL}
	l := Login{AppleID: "a@example.com", Password: "pw"}
	e := Event{UID: "game-101@soccer.example", data: []byte("x")}
	if isNew, err := c.Write(context.Background(), l, apple.URL+"/1/calendars/home/", e); isNew || err != nil {
		t.Fatalf("write: %v, %v", isNew, err)
	}
	if created, err := c.Create(context.Background(), l, apple.URL+"/1/calendars/home/", e); created || err != nil {
		t.Fatalf("create: %v, %v", created, err)
	}
	if want := []string{"If-Match:  If-None-Match: *", "If-Match:  If-None-Match: ", "If-Match:  If-None-Match: *"}; strings.Join(puts, "|") != strings.Join(want, "|") {
		t.Errorf("sent %q, want %q", puts, want)
	}
}
