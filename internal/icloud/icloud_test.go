package icloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const file = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nMETHOD:PUBLISH\r\nCALSCALE:GREGORIAN\r\nX-WR-CALNAME:ROVERS\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:America/Denver\r\nBEGIN:STANDARD\r\nTZOFFSETFROM:-0600\r\nTZOFFSETTO:-0700\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\n" +
	"BEGIN:VEVENT\r\nUID:game-101@soccer.example\r\nSUMMARY:ROVERS vs KICKERS\r\nBEGIN:VALARM\r\nUID:alarm-1\r\nTRIGGER:-PT40M\r\nEND:VALARM\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nSUMMARY:OWLS vs ROVERS\r\nUID:game-102@soccer.exa\r\n mple\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nSUMMARY:no UID\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

// Each game becomes a calendar file of its own, with the time zone, its own
// reminder and no METHOD; an event without a UID is left out.
func TestEventsSplitsTheFile(t *testing.T) {
	evs, err := Events([]byte(file))
	if err != nil || len(evs) != 2 || evs[0].UID != "game-101@soccer.example" || evs[1].UID != "game-102@soccer.example" {
		t.Fatalf("events = %+v, %v", evs, err)
	}
	one := string(evs[0].Data)
	for _, want := range []string{"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nCALSCALE:GREGORIAN\r\nBEGIN:VTIMEZONE", "TZID:America/Denver", "SUMMARY:ROVERS vs KICKERS", "TRIGGER:-PT40M", "END:VEVENT\r\nEND:VCALENDAR\r\n"} {
		if !strings.Contains(one, want) {
			t.Fatalf("missing %q in:\n%s", want, one)
		}
	}
	for _, not := range []string{"METHOD", "X-WR-CALNAME", "OWLS"} {
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
	if _, err := c.Write(ctx, l, other.URL+"/calendars/home/", Event{UID: "u", Data: []byte("x")}); !errors.Is(err, errUntrusted) {
		t.Fatalf("write: %v", err)
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
