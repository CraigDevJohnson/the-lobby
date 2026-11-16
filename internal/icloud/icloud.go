// Package icloud is the site's side of Add to iCloud Calendar (ADR 0012): a
// Member gives the site their Apple Account address and an app-specific
// password, and when they press Add the site writes the games they ticked
// into an iCloud calendar they pick. For a Member who turns the daily update
// on, the site also adds their new games once a day (ADR 0015). Apple has no
// consent screen or web API for this, so the calls are CalDAV (RFC 4791)
// with that password. The soccer backend only writes the calendar file and
// never sees the password.
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
	code, err := c.put(ctx, l, at, e.Data, true)
	if err == nil && code == http.StatusPreconditionFailed {
		code, err = c.put(ctx, l, at, e.Data, false)
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
	code, err := c.put(ctx, l, at, e.Data, true)
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

// eventAddress is where the calendar stores the event: a file named for its
// UID. The calendar must be at iCloud.
func (c *Client) eventAddress(calendarID string, e Event) (string, error) {
	if _, err := c.resolve(calendarID, ""); err != nil {
		return "", err
	}
	return strings.TrimSuffix(calendarID, "/") + "/" + url.PathEscape(e.UID) + ".ics", nil
}

func (c *Client) put(ctx context.Context, l Login, at string, data []byte, onlyNew bool) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, at, bytes.NewReader(data))
	if err != nil {
		return 0, errors.New("icloud: bad calendar address")
	}
	req.SetBasicAuth(l.AppleID, l.Password)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	if onlyNew {
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

// Event is one game as a calendar file of its own, which is how CalDAV
// stores an event, and the UID it is stored under.
type Event struct {
	UID  string
	Data []byte
}

// Events splits the backend's calendar file into one file per event, each
// with the file's time zones. The file's METHOD is left out: a stored event
// may not carry one.
func Events(file []byte) ([]Event, error) {
	lines := strings.Split(strings.ReplaceAll(string(file), "\r\n", "\n"), "\n")
	if len(lines) == 0 || !strings.EqualFold(strings.TrimSpace(lines[0]), "BEGIN:VCALENDAR") {
		return nil, errors.New("icloud: not a calendar file")
	}
	var (
		head, zones []string
		events      [][]string
		in          string // the top-level component being read
		depth       int
	)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case in == "" && strings.HasPrefix(upper, "BEGIN:"):
			in, depth = upper[len("BEGIN:"):], 1
			if in == "VEVENT" {
				events = append(events, nil)
			}
		case in != "" && strings.HasPrefix(upper, "BEGIN:"):
			depth++
		case in != "" && strings.HasPrefix(upper, "END:"):
			depth--
		case in == "" && (strings.HasPrefix(upper, "VERSION:") || strings.HasPrefix(upper, "PRODID:") || strings.HasPrefix(upper, "CALSCALE:")):
			head = append(head, line)
		}
		switch in {
		case "VEVENT":
			events[len(events)-1] = append(events[len(events)-1], line)
		case "VTIMEZONE":
			zones = append(zones, line)
		}
		if in != "" && depth == 0 {
			in = ""
		}
	}
	if !hasPrefix(head, "VERSION:") {
		head = append([]string{"VERSION:2.0"}, head...)
	}
	if !hasPrefix(head, "PRODID:") {
		head = append(head, "PRODID:-//craigdevjohnson.com//The Lobby//EN")
	}
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		uid := eventUID(ev)
		if uid == "" {
			continue
		}
		all := append([]string{"BEGIN:VCALENDAR"}, head...)
		all = append(all, zones...)
		all = append(all, ev...)
		all = append(all, "END:VCALENDAR", "")
		out = append(out, Event{UID: uid, Data: []byte(strings.Join(all, "\r\n"))})
	}
	return out, nil
}

func hasPrefix(lines []string, prefix string) bool {
	for _, l := range lines {
		if strings.HasPrefix(strings.ToUpper(l), prefix) {
			return true
		}
	}
	return false
}

// eventUID is the event's own UID, not a reminder's inside it, with a
// folded line put back together.
func eventUID(ev []string) string {
	depth := 0
	for i, line := range ev {
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "BEGIN:"):
			depth++
		case strings.HasPrefix(upper, "END:"):
			depth--
		case depth == 1 && strings.HasPrefix(upper, "UID:"):
			uid := line[len("UID:"):]
			for _, more := range ev[i+1:] {
				if !strings.HasPrefix(more, " ") && !strings.HasPrefix(more, "\t") {
					break
				}
				uid += more[1:]
			}
			return strings.TrimSpace(uid)
		}
	}
	return ""
}
