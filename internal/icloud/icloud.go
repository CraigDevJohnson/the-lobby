// Package icloud is the site's side of Add to iCloud Calendar (ADR 0012): a
// Member gives the site their Apple Account address and an app-specific
// password, and when they press Add the site writes the games they ticked
// into an iCloud calendar they pick. For a Member who turns the daily update
// on, the site also adds their new games once a day (ADR 0015) and replaces
// a game the league has moved (ADR 0017). Apple has no consent screen or web
// API for this, so the calls are CalDAV (RFC 4791) with that password. The
// soccer backend only writes the calendar file and never sees the password.
package icloud

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Address is where iCloud's CalDAV discovery starts.
const Address = "https://caldav.icloud.com"

var (
	// ErrDenied means Apple doesn't take the address and password: mistyped,
	// or the Member revoked the password at Apple.
	ErrDenied = errors.New("icloud: Apple refused the address and password")
	// ErrCalendar means the chosen calendar can't be written to: gone, or
	// shared read-only.
	ErrCalendar = errors.New("icloud: calendar can't be written to")
	// ErrBusy means Apple said to slow down (429 or 503).
	ErrBusy = errors.New("icloud: Apple said to slow down")
)

// Client makes the iCloud calls. URL is Apple's own unless a test points it
// elsewhere. Apple answers discovery with addresses on other icloud.com
// hosts; the password is sent only to URL's own host and to those.
type Client struct {
	HTTP *http.Client
	URL  string
}

// New is a Client for iCloud itself. A redirect is never followed, since it
// would carry the password wherever it pointed.
func New(hc *http.Client) *Client {
	own := *hc
	own.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{HTTP: &own, URL: Address}
}

// Login is what Apple checks on every call.
type Login struct {
	AppleID  string
	Password string
}

// Calendar is one calendar the Member can add events to. ID is its address.
type Calendar struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type multistatus struct {
	Responses []struct {
		Href      string `xml:"href"`
		Propstats []struct {
			Status string `xml:"status"`
			Prop   struct {
				Principal *struct {
					Href string `xml:"href"`
				} `xml:"current-user-principal"`
				Home *struct {
					Href string `xml:"href"`
				} `xml:"calendar-home-set"`
				Name string `xml:"displayname"`
				ETag string `xml:"getetag"`
				Type struct {
					Calendar *struct{} `xml:"calendar"`
				} `xml:"resourcetype"`
				Comps *struct {
					Comp []struct {
						Name string `xml:"name,attr"`
					} `xml:"comp"`
				} `xml:"supported-calendar-component-set"`
				Privileges *struct {
					Privilege []struct {
						All          *struct{} `xml:"all"`
						Write        *struct{} `xml:"write"`
						WriteContent *struct{} `xml:"write-content"`
						Bind         *struct{} `xml:"bind"`
					} `xml:"privilege"`
				} `xml:"current-user-privilege-set"`
			} `xml:"prop"`
		} `xml:"propstat"`
	} `xml:"response"`
}

const (
	askPrincipal = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`
	askHome      = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-home-set/></d:prop></d:propfind>`
	askCalendars = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:displayname/><d:resourcetype/><c:supported-calendar-component-set/><d:current-user-privilege-set/></d:prop></d:propfind>`
	askEvent     = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:getetag/></d:prop></d:propfind>`
)

// Home finds where the account's calendars live, which also proves Apple
// takes the address and password. The answer is kept with the connection.
func (c *Client) Home(ctx context.Context, l Login) (string, error) {
	var ms multistatus
	if err := c.propfind(ctx, l, c.URL+"/", "0", askPrincipal, &ms); err != nil {
		return "", err
	}
	principal := ""
	for _, r := range ms.Responses {
		for _, ps := range r.Propstats {
			if ps.Prop.Principal != nil && principal == "" {
				principal = ps.Prop.Principal.Href
			}
		}
	}
	if principal == "" {
		return "", errNoAnswer
	}
	at, err := c.resolve(c.URL+"/", principal)
	if err != nil {
		return "", err
	}
	ms = multistatus{}
	if err := c.propfind(ctx, l, at, "0", askHome, &ms); err != nil {
		return "", err
	}
	home := ""
	for _, r := range ms.Responses {
		for _, ps := range r.Propstats {
			if ps.Prop.Home != nil && home == "" {
				home = ps.Prop.Home.Href
			}
		}
	}
	if home == "" {
		return "", errNoAnswer
	}
	return c.resolve(at, home)
}

// Calendars lists the calendars under home that hold events and that the
// account can add to, by name. Reminder lists, calendars subscribed to from
// elsewhere and calendars shared to view only are left out.
func (c *Client) Calendars(ctx context.Context, l Login, home string) ([]Calendar, error) {
	if _, err := c.resolve(home, ""); err != nil {
		return nil, err
	}
	var ms multistatus
	if err := c.propfind(ctx, l, home, "1", askCalendars, &ms); err != nil {
		return nil, err
	}
	cals := []Calendar{}
	for _, r := range ms.Responses {
		isCalendar, events, writable, name := false, false, false, ""
		for _, ps := range r.Propstats {
			if !strings.Contains(ps.Status, " 200") {
				continue
			}
			p := ps.Prop
			isCalendar = isCalendar || p.Type.Calendar != nil
			if p.Name != "" {
				name = p.Name
			}
			if p.Comps != nil {
				for _, comp := range p.Comps.Comp {
					events = events || strings.EqualFold(comp.Name, "VEVENT")
				}
			}
			if p.Privileges != nil {
				for _, pr := range p.Privileges.Privilege {
					writable = writable || pr.All != nil || pr.Write != nil || pr.WriteContent != nil || pr.Bind != nil
				}
			}
		}
		if !isCalendar || !events || !writable {
			continue
		}
		at, err := c.resolve(home, r.Href)
		if err != nil {
			continue
		}
		if !strings.HasSuffix(at, "/") {
			at += "/"
		}
		cals = append(cals, Calendar{ID: at, Name: strings.TrimSpace(name)})
	}
	sort.SliceStable(cals, func(i, j int) bool { return strings.ToLower(cals[i].Name) < strings.ToLower(cals[j].Name) })
	return cals, nil
}

// Write puts one event into the calendar under its UID: added, or, when the
// calendar already holds it, replaced whole. It reports whether it was new.
// calendarID must be one Calendars answered.
func (c *Client) Write(ctx context.Context, l Login, calendarID string, e Event) (bool, error) {
	at, err := c.eventAddress(calendarID, e)
	if err != nil {
		return false, err
	}
	code, err := c.put(ctx, l, at, e.data, onlyNew)
	if err == nil && code == http.StatusPreconditionFailed {
		code, err = c.put(ctx, l, at, e.data, either)
		if err == nil && code/100 == 2 {
			return false, nil
		}
	}
	switch {
	case err != nil:
		return false, err
	case code/100 == 2:
		return true, nil
	case code == http.StatusForbidden || code == http.StatusNotFound:
		return false, fmt.Errorf("%w (%w)", ErrCalendar, &statusError{code})
	}
	return false, &statusError{code}
}

// Create puts one event into the calendar only when the calendar doesn't
// hold it yet, and reports whether it did. Apple answering that the event is
// already there, under this file name or another, is no error, and nothing
// is ever replaced. calendarID must be one Calendars answered.
func (c *Client) Create(ctx context.Context, l Login, calendarID string, e Event) (bool, error) {
	at, err := c.eventAddress(calendarID, e)
	if err != nil {
		return false, err
	}
	code, err := c.put(ctx, l, at, e.data, onlyNew)
	switch {
	case err != nil:
		return false, err
	case code/100 == 2:
		return true, nil
	case code == http.StatusPreconditionFailed:
		return false, nil
	}
	return false, &statusError{code}
}

// Replace puts one event into the calendar only when the calendar already
// holds it under its UID's file name, replacing it whole, and reports
// whether it did. Apple is asked first whether the event is there, and the
// event is sent only when it is, so an event Apple says isn't there is never
// sent: a game the Member deleted stays deleted. The two can't be one
// request, because iCloud doesn't keep If-Match: * as the standard has it:
// on dev, on 2026-11-19, it refused that PUT for an event that was there and
// stored one that had been deleted (ADR 0017). An event deleted in the
// moment between the two requests comes back, and so does one deleted on a
// device that hasn't told iCloud yet. calendarID must be one Calendars
// answered.
//
// For an event that isn't there, said is how Apple said so, NotFound or
// NoETag, for a log line: it names nothing of the event.
func (c *Client) Replace(ctx context.Context, l Login, calendarID string, e Event) (replaced bool, said string, err error) {
	at, err := c.eventAddress(calendarID, e)
	if err != nil {
		return false, "", err
	}
	said, err = c.holds(ctx, l, at)
	if err != nil || said != "" {
		return false, said, err
	}
	code, err := c.put(ctx, l, at, e.data, either)
	switch {
	case err != nil:
		return false, "", err
	case code/100 == 2:
		return true, "", nil
	}
	return false, "", &statusError{code}
}

// How Apple says no event is stored at an address: 404, or an answer that
// gives no ETag for it.
const (
	NotFound = "404"
	NoETag   = "no ETag"
)

// holds asks Apple whether an event is stored at this address, and answers
// "" when one is, and how Apple said so (NotFound or NoETag) when none is.
// Apple's answer carries the event's ETag and nothing the event says.
func (c *Client) holds(ctx context.Context, l Login, at string) (string, error) {
	var ms multistatus
	err := c.propfind(ctx, l, at, "0", askEvent, &ms)
	var status *statusError
	if errors.As(err, &status) && status.code == http.StatusNotFound {
		return NotFound, nil
	}
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		for _, ps := range r.Propstats {
			if strings.Contains(ps.Status, " 200") && strings.TrimSpace(ps.Prop.ETag) != "" {
				return "", nil
			}
		}
	}
	return NoETag, nil
}

// eventAddress is where the calendar stores the event: a file named for its
// UID. The calendar must be at iCloud.
func (c *Client) eventAddress(calendarID string, e Event) (string, error) {
	if _, err := c.resolve(calendarID, ""); err != nil {
		return "", err
	}
	return strings.TrimSuffix(calendarID, "/") + "/" + url.PathEscape(e.UID) + ".ics", nil
}

// when says which events a PUT may write: either one, or only one the
// calendar doesn't hold yet (If-None-Match: *). There is no "only one it
// already holds": iCloud doesn't keep If-Match: * (Replace).
type when int

const (
	either when = iota
	onlyNew
)

func (c *Client) put(ctx context.Context, l Login, at string, data []byte, w when) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, at, bytes.NewReader(data))
	if err != nil {
		return 0, errors.New("icloud: bad calendar address")
	}
	req.SetBasicAuth(l.AppleID, l.Password)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	if w == onlyNew {
		req.Header.Set("If-None-Match", "*")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("icloud: PUT: %w", withoutURL(err))
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return 0, ErrDenied
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return 0, fmt.Errorf("%w (%w)", ErrBusy, &statusError{res.StatusCode})
	}
	return res.StatusCode, nil
}

func (c *Client) propfind(ctx context.Context, l Login, at, depth, body string, out *multistatus) error {
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", at, strings.NewReader(body))
	if err != nil {
		return errors.New("icloud: bad address")
	}
	req.SetBasicAuth(l.AppleID, l.Password)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", depth)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("icloud: PROPFIND: %w", withoutURL(err))
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("icloud: read the answer: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return ErrDenied
	case res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusServiceUnavailable:
		return fmt.Errorf("%w (%w)", ErrBusy, &statusError{res.StatusCode})
	case res.StatusCode != http.StatusMultiStatus:
		return &statusError{res.StatusCode}
	}
	if err := xml.Unmarshal(data, out); err != nil {
		return errors.New("icloud: the answer could not be read")
	}
	return nil
}

// resolve is href as an address, read against base, refused unless it is on
// URL's own host or, over https, on an icloud.com host: the password goes
// nowhere else, whatever an answer or a request names.
func (c *Client) resolve(base, href string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", errUntrusted
	}
	u, err := b.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", errUntrusted
	}
	own, err := url.Parse(c.URL)
	if err != nil {
		return "", errUntrusted
	}
	apple := u.Scheme == "https" && strings.HasSuffix(u.Hostname(), ".icloud.com") && (u.Port() == "" || u.Port() == "443")
	if !(u.Scheme == own.Scheme && u.Host == own.Host) && !apple {
		return "", errUntrusted
	}
	u.User, u.Fragment, u.RawQuery = nil, "", ""
	return u.String(), nil
}

var (
	errUntrusted = errors.New("icloud: an address that isn't iCloud's")
	errNoAnswer  = errors.New("icloud: Apple didn't say where the calendars are")
)

// withoutURL drops the address from a transport error: it carries the
// account's number at Apple, and errors are logged.
func withoutURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("icloud: status %d", e.code) }

// Event is one checked soccer game or Session-over notice as a calendar file
// built by this package. data is private so the CalDAV writers cannot be used
// to send a backend-provided calendar payload directly. Start is the Boise
// wall-clock DTSTART as written, Field is the first description line, and
// Title is the summary; the daily update uses those three values.
type Event struct {
	UID         string
	Start       string
	Field       string
	Title       string
	Description string
	data        []byte
}

const (
	maxEvents           = 256
	maxSummaryBytes     = 512
	maxDescriptionBytes = 8 << 10
	// An escaped description is the longest permitted property. Each byte
	// of its text can take two bytes on the wire.
	maxCalendarLineBytes = len("DESCRIPTION;LANGUAGE=en:") + 2*maxDescriptionBytes
	soccerUIDSuffix      = "@soccer.craigdevjohnson.com"
)

type calendarEvent struct {
	uid, summary, description, location string
	start, end, stamp, transparency     string
	startParams, endParams              string
	url                                 string
	allDay                              bool
	alarm                               string
}

// Events reads the bounded event contract the soccer backend answers and
// constructs one safe calendar file per event. Unknown properties,
// components, duplicate UIDs and events outside the site's UID namespace
// reject the whole answer before a Member's calendar is touched.
func Events(file []byte) ([]Event, error) {
	lines, err := calendarLines(file)
	if err != nil || len(lines) == 0 || !strings.EqualFold(lines[0], "BEGIN:VCALENDAR") {
		return nil, errors.New("icloud: not a calendar file")
	}
	seenTop := map[string]bool{}
	seenUID := map[string]bool{}
	out := make([]Event, 0)
	ended := false
	for i := 1; i < len(lines); {
		line := lines[i]
		if line == "" {
			i++
			continue
		}
		if strings.EqualFold(line, "END:VCALENDAR") {
			ended = true
			for _, rest := range lines[i+1:] {
				if rest != "" {
					return nil, errors.New("icloud: invalid calendar file")
				}
			}
			break
		}
		if strings.EqualFold(line, "BEGIN:VTIMEZONE") {
			_, i, err = calendarComponent(lines, i, "VTIMEZONE")
			if err != nil {
				return nil, err
			}
			continue
		}
		if strings.EqualFold(line, "BEGIN:VEVENT") {
			if len(out) == maxEvents {
				return nil, errors.New("icloud: too many backend events")
			}
			var body []string
			body, i, err = calendarComponent(lines, i, "VEVENT")
			if err != nil {
				return nil, err
			}
			e, err := parseCalendarEvent(body)
			if err != nil || seenUID[e.uid] {
				return nil, errors.New("icloud: invalid backend event")
			}
			seenUID[e.uid] = true
			field, _, _ := strings.Cut(e.description, "\n")
			out = append(out, Event{
				UID:         e.uid,
				Start:       e.start,
				Field:       strings.TrimSpace(field),
				Title:       strings.TrimSpace(e.summary),
				Description: e.description,
				data:        renderCalendarEvent(e),
			})
			continue
		}
		name, params, value, ok := calendarProperty(line)
		if !ok || seenTop[name] || !validTopProperty(name, params, value) {
			return nil, errors.New("icloud: invalid calendar file")
		}
		seenTop[name] = true
		i++
	}
	if !ended || !seenTop["VERSION"] {
		return nil, errors.New("icloud: invalid calendar file")
	}
	return out, nil
}

func calendarLines(file []byte) ([]string, error) {
	if !utf8.Valid(file) || bytes.IndexByte(file, 0) >= 0 {
		return nil, errors.New("icloud: invalid calendar file")
	}
	raw := strings.ReplaceAll(string(file), "\r\n", "\n")
	if strings.ContainsRune(raw, '\r') || continuationLine(raw) {
		return nil, errors.New("icloud: invalid calendar file")
	}
	var lines []string
	for raw != "" {
		line, rest, _ := strings.Cut(raw, "\n")
		raw = rest
		if len(line) > maxCalendarLineBytes {
			return nil, errors.New("icloud: calendar property is too long")
		}
		if continuationLine(raw) {
			var unfolded strings.Builder
			unfolded.WriteString(line)
			for continuationLine(raw) {
				part, rest, _ := strings.Cut(raw, "\n")
				raw = rest
				part = part[1:]
				if len(part) > maxCalendarLineBytes-unfolded.Len() {
					return nil, errors.New("icloud: calendar property is too long")
				}
				unfolded.WriteString(part)
			}
			line = unfolded.String()
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func continuationLine(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

// calendarComponent returns the lines inside one well-nested component and
// the first line after it.
func calendarComponent(lines []string, at int, want string) ([]string, int, error) {
	stack := []string{want}
	for i := at + 1; i < len(lines); i++ {
		upper := strings.ToUpper(lines[i])
		switch {
		case strings.HasPrefix(upper, "BEGIN:"):
			stack = append(stack, strings.TrimSpace(upper[len("BEGIN:"):]))
		case strings.HasPrefix(upper, "END:"):
			name := strings.TrimSpace(upper[len("END:"):])
			if len(stack) == 0 || stack[len(stack)-1] != name {
				return nil, 0, errors.New("icloud: invalid calendar component")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return lines[at+1 : i], i + 1, nil
			}
		}
	}
	return nil, 0, errors.New("icloud: incomplete calendar component")
}

func validTopProperty(name, params, value string) bool {
	switch name {
	case "VERSION":
		return params == "" && value == "2.0"
	case "METHOD":
		return params == "" && strings.EqualFold(value, "PUBLISH")
	case "CALSCALE":
		return params == "" && strings.EqualFold(value, "GREGORIAN")
	case "PRODID", "X-WR-CALNAME", "X-WR-TIMEZONE", "X-PUBLISHED-TTL":
		return params == "" && validCalendarText(value, 512)
	case "REFRESH-INTERVAL":
		return strings.EqualFold(params, "VALUE=DURATION") && validCalendarText(value, 64)
	default:
		return false
	}
}

func parseCalendarEvent(lines []string) (calendarEvent, error) {
	var e calendarEvent
	seen := map[string]bool{}
	for i := 0; i < len(lines); {
		if strings.EqualFold(lines[i], "BEGIN:VALARM") {
			if seen["VALARM"] {
				return e, errors.New("icloud: duplicate alarm")
			}
			body, next, err := calendarComponent(lines, i, "VALARM")
			if err != nil {
				return e, err
			}
			e.alarm, err = parseAlarm(body)
			if err != nil {
				return e, err
			}
			seen["VALARM"] = true
			i = next
			continue
		}
		if strings.HasPrefix(strings.ToUpper(lines[i]), "BEGIN:") || strings.HasPrefix(strings.ToUpper(lines[i]), "END:") {
			return e, errors.New("icloud: invalid event component")
		}
		name, params, value, ok := calendarProperty(lines[i])
		if !ok || seen[name] {
			return e, errors.New("icloud: invalid event property")
		}
		seen[name] = true
		switch name {
		case "UID":
			if params != "" {
				return e, errors.New("icloud: invalid event UID")
			}
			e.uid = strings.TrimSpace(value)
		case "SUMMARY":
			if !textParams(params) {
				return e, errors.New("icloud: invalid event summary")
			}
			e.summary = strings.TrimSpace(text(value))
		case "DESCRIPTION":
			if !textParams(params) {
				return e, errors.New("icloud: invalid event description")
			}
			e.description = strings.TrimSpace(text(value))
		case "LOCATION":
			if !textParams(params) {
				return e, errors.New("icloud: invalid event location")
			}
			e.location = strings.TrimSpace(text(value))
		case "DTSTART":
			e.start, e.startParams = strings.TrimSpace(value), params
		case "DTEND":
			e.end, e.endParams = strings.TrimSpace(value), params
		case "DTSTAMP":
			if params != "" {
				return e, errors.New("icloud: invalid event stamp")
			}
			e.stamp = strings.TrimSpace(value)
		case "TRANSP":
			if params != "" {
				return e, errors.New("icloud: invalid event transparency")
			}
			e.transparency = strings.ToUpper(strings.TrimSpace(value))
		case "URL":
			if params != "" {
				return e, errors.New("icloud: invalid event URL")
			}
			e.url = strings.TrimSpace(value)
		default:
			return e, errors.New("icloud: event property is not allowed")
		}
		i++
	}
	switch {
	case strings.EqualFold(e.startParams, "VALUE=DATE") && strings.EqualFold(e.endParams, "VALUE=DATE"):
		e.allDay = true
	case strings.EqualFold(e.startParams, "TZID=America/Denver") && strings.EqualFold(e.endParams, "TZID=America/Denver"):
	default:
		return e, errors.New("icloud: invalid event times")
	}
	if !validCalendarEvent(e) {
		return e, errors.New("icloud: invalid backend event")
	}
	return e, nil
}

func validCalendarEvent(e calendarEvent) bool {
	if e.summary == "" || !validCalendarText(e.summary, maxSummaryBytes) || !validCalendarText(e.description, maxDescriptionBytes) || !validCalendarText(e.location, 2<<10) {
		return false
	}
	layout := "20060102T150405"
	if e.allDay {
		layout = "20060102"
	}
	start, err := time.Parse(layout, e.start)
	if err != nil || !validSoccerUID(e.uid, e.allDay, start) {
		return false
	}
	end, err := time.Parse(layout, e.end)
	if err != nil || !end.After(start) {
		return false
	}
	if e.stamp != "" {
		if _, err := time.Parse("20060102T150405Z", e.stamp); err != nil {
			return false
		}
	}
	if e.transparency != "" && e.transparency != "OPAQUE" && e.transparency != "TRANSPARENT" {
		return false
	}
	if e.url != "" {
		u, err := url.ParseRequestURI(e.url)
		if err != nil || u.User != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" || len(e.url) > 2048 {
			return false
		}
	}
	return !e.allDay || e.alarm == ""
}

// GameUID is the calendar-file identity for a league game, including fallback
// keys containing punctuation that iCalendar text must escape.
func GameUID(gameID string) string { return "game-" + calendarText(gameID) + soccerUIDSuffix }

// GameID returns the backend selection key for a validated game, or the empty
// string for a Session-over notice.
func (e Event) GameID() string {
	local, ok := strings.CutPrefix(e.UID, "game-")
	if !ok {
		return ""
	}
	return text(strings.TrimSuffix(local, soccerUIDSuffix))
}

func validSoccerUID(uid string, allDay bool, start time.Time) bool {
	local, ok := strings.CutSuffix(uid, soccerUIDSuffix)
	if !ok {
		return false
	}
	if allDay {
		id, ok := strings.CutPrefix(local, "session-over-")
		return ok && numericCalendarID(id)
	}
	id, ok := strings.CutPrefix(local, "game-")
	return ok && (numericCalendarID(id) || validFallbackGameID(id, start))
}

func numericCalendarID(id string) bool {
	return len(id) > 0 && len(id) <= 32 && strings.Trim(id, "0123456789") == ""
}

// For games without a league ID the backend uses the Boise start minute
// and the field name. Keep its escaped UID as written so the CalDAV file
// name and the daily update's stored key still identify the same event.
func validFallbackGameID(id string, start time.Time) bool {
	encodedField, ok := strings.CutPrefix(id, start.Format("20060102T1504")+"-")
	if !ok || len(encodedField) > 2*maxSummaryBytes {
		return false
	}
	field := text(encodedField)
	return len(field) > 0 && len(field) <= maxSummaryBytes && field == strings.TrimSpace(field) &&
		!strings.ContainsAny(field, "/\\") && strings.IndexFunc(field, unicode.IsControl) < 0 &&
		calendarText(field) == encodedField
}

func parseAlarm(lines []string) (string, error) {
	seen := map[string]bool{}
	description := ""
	for _, line := range lines {
		name, params, value, ok := calendarProperty(line)
		if !ok || seen[name] {
			return "", errors.New("icloud: invalid event alarm")
		}
		seen[name] = true
		switch name {
		case "ACTION":
			if params != "" || !strings.EqualFold(value, "DISPLAY") {
				return "", errors.New("icloud: invalid event alarm")
			}
		case "TRIGGER":
			if params != "" || value != "-PT40M" {
				return "", errors.New("icloud: invalid event alarm")
			}
		case "DESCRIPTION":
			if !textParams(params) {
				return "", errors.New("icloud: invalid event alarm")
			}
			description = strings.TrimSpace(text(value))
		case "UID":
			if params != "" || !validCalendarText(value, 256) {
				return "", errors.New("icloud: invalid event alarm")
			}
		default:
			return "", errors.New("icloud: alarm property is not allowed")
		}
	}
	if !seen["ACTION"] || !seen["TRIGGER"] || description == "" || !validCalendarText(description, 512) {
		return "", errors.New("icloud: incomplete event alarm")
	}
	return description, nil
}

func calendarProperty(line string) (name, params, value string, ok bool) {
	quoted := false
	colon := -1
	for at, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ':' && !quoted:
			colon = at
		}
		if colon >= 0 {
			break
		}
	}
	if colon <= 0 || quoted {
		return "", "", "", false
	}
	head := line[:colon]
	if semi := strings.IndexByte(head, ';'); semi >= 0 {
		params = head[semi+1:]
		head = head[:semi]
	}
	name = strings.ToUpper(strings.TrimSpace(head))
	return name, params, line[colon+1:], name != ""
}

func textParams(params string) bool {
	return params == "" || strings.EqualFold(params, "LANGUAGE=en")
}

func validCalendarText(s string, limit int) bool {
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

var denverZone = []string{
	"BEGIN:VTIMEZONE",
	"TZID:America/Denver",
	"BEGIN:STANDARD",
	"DTSTART:19701101T020000",
	"TZOFFSETFROM:-0600",
	"TZOFFSETTO:-0700",
	"RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU",
	"TZNAME:MST",
	"END:STANDARD",
	"BEGIN:DAYLIGHT",
	"DTSTART:19700308T020000",
	"TZOFFSETFROM:-0700",
	"TZOFFSETTO:-0600",
	"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU",
	"TZNAME:MDT",
	"END:DAYLIGHT",
	"END:VTIMEZONE",
}

func renderCalendarEvent(e calendarEvent) []byte {
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//craigdevjohnson.com//The Lobby//EN", "CALSCALE:GREGORIAN"}
	if !e.allDay {
		lines = append(lines, denverZone...)
	}
	lines = append(lines, "BEGIN:VEVENT", "UID:"+e.uid, "SUMMARY:"+calendarText(e.summary))
	if e.allDay {
		lines = append(lines, "DTSTART;VALUE=DATE:"+e.start, "DTEND;VALUE=DATE:"+e.end)
	} else {
		lines = append(lines, "DTSTART;TZID=America/Denver:"+e.start, "DTEND;TZID=America/Denver:"+e.end)
	}
	if e.location != "" {
		lines = append(lines, "LOCATION:"+calendarText(e.location))
	}
	if e.description != "" {
		lines = append(lines, "DESCRIPTION:"+calendarText(e.description))
	}
	if e.stamp != "" {
		lines = append(lines, "DTSTAMP:"+e.stamp)
	}
	if e.transparency != "" {
		lines = append(lines, "TRANSP:"+e.transparency)
	}
	if e.url != "" {
		lines = append(lines, "URL:"+e.url)
	}
	if e.alarm != "" {
		lines = append(lines, "BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT40M", "DESCRIPTION:"+calendarText(e.alarm), "END:VALARM")
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	return foldedCalendar(lines)
}

func calendarText(s string) string {
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r", "", "\n", `\n`)
	return r.Replace(s)
}

func foldedCalendar(lines []string) []byte {
	var out strings.Builder
	for _, original := range lines {
		line, limit := original, 75
		for len(line) > limit {
			at := limit
			for at > 0 && !utf8.RuneStart(line[at]) {
				at--
			}
			out.WriteString(line[:at])
			out.WriteString("\r\n ")
			line, limit = line[at:], 74
		}
		out.WriteString(line)
		out.WriteString("\r\n")
	}
	return []byte(out.String())
}

// text undoes iCalendar's escaping of a text value (RFC 5545 section
// 3.3.11): a backslash before n is a line break, and before a comma, a
// semicolon, another backslash or anything else it is that character alone.
func text(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 == len(value) {
			b.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			b.WriteByte(value[i])
		}
	}
	return b.String()
}
