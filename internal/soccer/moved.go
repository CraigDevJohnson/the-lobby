package soccer

import (
	"strings"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/invite"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

// The email the daily iCloud update sends a Member when the league has
// moved a game it had put in their iCloud calendar (ADR 0017): what moved,
// from when and where to when and where, and which moved games weren't found
// in the calendar and so weren't changed there. It is plain text, sent the
// way the invite is (internal/invite), one per Member per run. It carries no
// calendar link, Member link or Session link.

// movedSubject is the email's subject line: plain ASCII, so it needs no
// encoding in the header, and no Team name.
const movedSubject = "A soccer game moved"

// movedGame is one game the league moved: its title as the calendar shows
// it, the start and field kept for it, the league's new ones, and whether
// the game was found in the Member's calendar and replaced there.
type movedGame struct {
	Title    string
	Was, Now store.GameTime
	Found    bool
}

// movedEmail builds the email for the Member at to. base is the site's own
// address (SITE_BASE_URL), as in the invite, so each environment's email
// names that environment's page. games has at least one game.
func movedEmail(base, to string, games []movedGame) invite.Email {
	var found, missing []movedGame
	for _, g := range games {
		if g.Found {
			found = append(found, g)
		} else {
			missing = append(missing, g)
		}
	}
	var b strings.Builder
	line := func(s ...string) {
		b.WriteString(strings.Join(s, ""))
		b.WriteString("\n")
	}
	list := func(games []movedGame) {
		for _, g := range games {
			line()
			line(g.name())
			line("  Was: ", when(g.Was))
			line("  Now: ", when(g.Now))
		}
		line()
	}
	if len(found) == 1 {
		line("The league moved a game that is in your iCloud calendar, so The Lobby moved it there too.")
	} else if len(found) > 1 {
		line("The league moved games that are in your iCloud calendar, so The Lobby moved them there too.")
	}
	if len(found) > 0 {
		list(found)
	}
	switch {
	case len(missing) == 1 && len(found) > 0:
		line("This game moved too, but it wasn't found in your iCloud calendar, so nothing was changed there:")
	case len(missing) > 1 && len(found) > 0:
		line("These games moved too, but they weren't found in your iCloud calendar, so nothing was changed there:")
	case len(missing) == 1:
		line("The league moved a game, but it wasn't found in your iCloud calendar, so nothing was changed there:")
	case len(missing) > 1:
		line("The league moved games, but they weren't found in your iCloud calendar, so nothing was changed there:")
	}
	if len(missing) > 0 {
		list(missing)
	}
	if len(found) > 0 {
		line("Anything you changed on a moved game in iCloud, such as notes or alerts, was replaced.")
		line()
	}
	// Signing in is one of the steps: the page offers Add to iCloud Calendar
	// only to a signed-in Member, and whoever opens this may be signed out.
	line(`To stop these emails, turn off "Add new games every day": open `, strings.TrimSuffix(base, "/"), "/soccer, sign in, find a Team, choose Apple Calendar, then Add to iCloud Calendar.")
	return invite.Email{To: to, Subject: movedSubject, Body: b.String()}
}

// name is the game without its field: the title up to the " - Field 1" the
// backend ends it with.
func (g movedGame) name() string {
	return strings.TrimSuffix(g.Title, " - "+g.Now.Field)
}

// when is a start and field as a person reads them, such as "Sat, Nov 21 at
// 7:15 PM, Field 3". The start's digits are already Boise time, so they are
// read and written with no time zone: the site's Lambda carries no zone
// data. A start that can't be read is written as it is.
func when(at store.GameTime) string {
	out := at.Start
	if t, err := time.Parse("20060102T150405", at.Start); err == nil {
		out = t.Format("Mon, Jan 2 at 3:04 PM")
	}
	switch {
	case at.Field == "":
		return out
	case out == "":
		return at.Field
	}
	return out + ", " + at.Field
}
