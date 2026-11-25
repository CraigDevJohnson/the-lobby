package soccer

import (
	"os"
	"strings"
	"testing"

	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

func at(start, field string) store.GameTime { return store.GameTime{Start: start, Field: field} }

// One moved game and one that wasn't found: the email names each without
// its field, says when and where it was and is, in Boise time as the
// backend wrote it, and how to stop the emails at this environment's own
// address.
func TestTheMovedEmailSaysWhatMovedAndWhatWasNotFound(t *testing.T) {
	got := movedEmail("https://next.example.com/", "friend@example.com", "iCloud", []movedGame{
		{Title: "ROVERS vs KICKERS - Field 1", Was: at("20261121T191500", "Field 3"), Now: at("20261122T183000", "Field 1"), Found: true},
		{Title: "LIONS vs TIGERS - Field 2", Was: at("20261205T090000", "Field 2"), Now: at("20261205T120000", "Field 2")},
	})
	want := `The league moved a game that is in your iCloud calendar, so The Lobby moved it there too.

ROVERS vs KICKERS
  Was: Sat, Nov 21 at 7:15 PM, Field 3
  Now: Sun, Nov 22 at 6:30 PM, Field 1

This game moved too, but it wasn't found in your iCloud calendar, so nothing was changed there:

LIONS vs TIGERS
  Was: Sat, Dec 5 at 9:00 AM, Field 2
  Now: Sat, Dec 5 at 12:00 PM, Field 2

Anything you changed on a moved game in iCloud, such as notes or alerts, was replaced.

To stop these emails, turn off "Add new games every day": open https://next.example.com/soccer, sign in, and choose Apple Calendar.
`
	if got.Body != want {
		t.Errorf("body:\n%s\nwant:\n%s", got.Body, want)
	}
	if got.To != "friend@example.com" || got.Subject != "A soccer game moved" {
		t.Errorf("to %q, subject %q", got.To, got.Subject)
	}
	// It carries no calendar link, Member link or Session link: the only
	// address in it is the page's.
	if strings.Count(got.Body, "http") != 1 || strings.Contains(got.Body, LinkPrefix) || strings.Contains(got.Body, ".ics") || strings.Contains(got.Body, "webcal") {
		t.Errorf("the email carries a link other than the page's:\n%s", got.Body)
	}
}

// More than one game in a section is said in the plural.
func TestTheMovedEmailSaysSeveralGamesInThePlural(t *testing.T) {
	got := movedEmail("https://next.example.com", "friend@example.com", "iCloud", []movedGame{
		{Title: "ROVERS vs KICKERS - Field 1", Was: at("20261121T191500", "Field 3"), Now: at("20261122T183000", "Field 1"), Found: true},
		{Title: "LIONS vs TIGERS - Field 2", Was: at("20261205T090000", "Field 2"), Now: at("20261205T120000", "Field 2")},
		{Title: "ROVERS vs OWLS - North Court", Was: at("20261128T183000", "Field 3"), Now: at("20261128T183000", "North Court"), Found: true},
		{Title: "LIONS vs KICKERS, UNITED FC - Field 4", Was: at("20261212T090000", "Field 2"), Now: at("20261213T090000", "Field 4")},
	}).Body
	want := `The league moved games that are in your iCloud calendar, so The Lobby moved them there too.

ROVERS vs KICKERS
  Was: Sat, Nov 21 at 7:15 PM, Field 3
  Now: Sun, Nov 22 at 6:30 PM, Field 1

ROVERS vs OWLS
  Was: Sat, Nov 28 at 6:30 PM, Field 3
  Now: Sat, Nov 28 at 6:30 PM, North Court

These games moved too, but they weren't found in your iCloud calendar, so nothing was changed there:

LIONS vs TIGERS
  Was: Sat, Dec 5 at 9:00 AM, Field 2
  Now: Sat, Dec 5 at 12:00 PM, Field 2

LIONS vs KICKERS, UNITED FC
  Was: Sat, Dec 12 at 9:00 AM, Field 2
  Now: Sun, Dec 13 at 9:00 AM, Field 4

Anything you changed on a moved game in iCloud, such as notes or alerts, was replaced.

To stop these emails, turn off "Add new games every day": open https://next.example.com/soccer, sign in, and choose Apple Calendar.
`
	if got != want {
		t.Errorf("body:\n%s\nwant:\n%s", got, want)
	}
}

// A section with no games is left out, and with it, when no game was
// replaced, the line about what was replaced.
func TestTheMovedEmailLeavesOutASectionWithNoGames(t *testing.T) {
	found := movedGame{Title: "ROVERS vs KICKERS - Field 1", Was: at("20261121T191500", "Field 3"), Now: at("20261122T183000", "Field 1"), Found: true}
	missing := movedGame{Title: "LIONS vs TIGERS - Field 2", Was: at("20261205T090000", "Field 2"), Now: at("20261205T120000", "Field 2")}
	const stop = `To stop these emails, turn off "Add new games every day": open https://next.example.com/soccer, sign in, and choose Apple Calendar.` + "\n"

	got := movedEmail("https://next.example.com", "friend@example.com", "iCloud", []movedGame{found}).Body
	want := "The league moved a game that is in your iCloud calendar, so The Lobby moved it there too.\n\n" +
		"ROVERS vs KICKERS\n  Was: Sat, Nov 21 at 7:15 PM, Field 3\n  Now: Sun, Nov 22 at 6:30 PM, Field 1\n\n" +
		"Anything you changed on a moved game in iCloud, such as notes or alerts, was replaced.\n\n" + stop
	if got != want {
		t.Errorf("only a moved game:\n%s\nwant:\n%s", got, want)
	}

	got = movedEmail("https://next.example.com", "friend@example.com", "iCloud", []movedGame{missing}).Body
	want = "The league moved a game, but it wasn't found in your iCloud calendar, so nothing was changed there:\n\n" +
		"LIONS vs TIGERS\n  Was: Sat, Dec 5 at 9:00 AM, Field 2\n  Now: Sat, Dec 5 at 12:00 PM, Field 2\n\n" + stop
	if got != want {
		t.Errorf("only a game not found:\n%s\nwant:\n%s", got, want)
	}

	got = movedEmail("https://next.example.com", "friend@example.com", "iCloud", []movedGame{missing, missing}).Body
	if !strings.HasPrefix(got, "The league moved games, but they weren't found in your iCloud calendar, so nothing was changed there:\n\n") || strings.Contains(got, "was replaced") {
		t.Errorf("only games not found:\n%s", got)
	}
}

// A start is read and written with no time zone, a start that can't be read
// is written as it is, and a title that doesn't end with the field is
// written whole.
func TestTheMovedEmailWritesWhatItCanNotRead(t *testing.T) {
	for _, c := range []struct {
		at   store.GameTime
		want string
	}{
		{at("20261121T191500", "Field 3"), "Sat, Nov 21 at 7:15 PM, Field 3"},
		{at("20270102T000500", "North Court"), "Sat, Jan 2 at 12:05 AM, North Court"},
		{at("20261121T191500Z", "Field 3"), "20261121T191500Z, Field 3"},
		{at("20261220", ""), "20261220"},
		{at("", "Field 3"), "Field 3"},
	} {
		if got := when(c.at); got != c.want {
			t.Errorf("%+v: %q, want %q", c.at, got, c.want)
		}
	}
	for _, c := range []struct {
		game movedGame
		want string
	}{
		{movedGame{Title: "ROVERS vs KICKERS - Field 1", Now: at("", "Field 1")}, "ROVERS vs KICKERS"},
		{movedGame{Title: "ROVERS vs KICKERS - Field 3", Now: at("", "Field 1")}, "ROVERS vs KICKERS - Field 3"},
		{movedGame{Title: "ROVERS vs KICKERS", Now: at("", "Field 1")}, "ROVERS vs KICKERS"},
	} {
		if got := c.game.name(); got != c.want {
			t.Errorf("%+v: %q, want %q", c.game, got, c.want)
		}
	}
}

// The email names the switch and the two choices that lead to it as the
// page does: the labels in web/src/soccer.
func TestTheMovedEmailNamesThePagesOwnLabels(t *testing.T) {
	body := movedEmail("https://next.example.com", "friend@example.com", "iCloud", []movedGame{{Found: true}}).Body
	for file, labels := range map[string][]string{
		"../../web/src/soccer/ConnectedCalendar.tsx": {"Add new games every day"},
		"../../web/src/soccer/Delivery.tsx":          {"Apple Calendar"},
	} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, label := range labels {
			if !strings.Contains(string(src), ">"+label+"<") && !strings.Contains(string(src), `"`+label+`"`) {
				t.Errorf("%s no longer shows %q, which the email names", file, label)
			}
			if !strings.Contains(body, label) {
				t.Errorf("the email doesn't name %q", label)
			}
		}
	}
}
