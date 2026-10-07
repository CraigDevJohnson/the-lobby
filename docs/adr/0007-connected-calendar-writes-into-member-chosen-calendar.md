---
status: accepted
date: 2026-10-05
---

# The Connected calendar writes into a Google calendar the Member picks

When a Member connects Google Calendar, the site asks to view and edit events on all their calendars and to see their list of calendars, then writes games into a calendar the Member picks when connecting, with their main calendar preselected. Craig wanted games in a calendar of the Member's choosing, as Members of the previous site were used to.

## Considered options

- A "Soccer" calendar created and managed by the site alone, using Google's narrower `calendar.app.created` permission. It keeps the site away from every other event the Member has and lets disconnecting remove the calendar cleanly, but the games sit in a separate calendar rather than the one the Member already uses.

## Consequences

- The permission asked for is expected to be in Google's sensitive class; Google does not publish the class, so it is confirmed in the Cloud console (see the baseline's list of things to prove). If it is, then without Google's app verification, Members see a one-time "This app isn't verified" screen, and the Google project carries a lifetime cap of 100 people. The project must be published as "In production", not "Testing", or calendar access expires after seven days.
- The site must mark and record every event it creates and never touch any other event. A game the Member deleted stays deleted.
- Scores go in the event title, because rewriting the description would wipe the Member's own notes.
- Disconnecting, switching calendar or dropping a Remembered team removes the upcoming games the site added (re-added on a switch); played games stay. Disconnecting also revokes access at Google, which cancels every consent that person gave the same Google project, sign-in included; they consent again at their next Google sign-in.
- Assumed, not yet ruled on by Craig: a Member may grant event access but refuse the calendar list, and the site then uses their main calendar.
