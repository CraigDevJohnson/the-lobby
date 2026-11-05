---
status: accepted
date: 2026-10-05
---

# The Connected calendar writes into a Google calendar the Member picks

When a Member connects Google Calendar, the site asks to view and edit events on all their calendars and to see their list of calendars, then writes games into a calendar the Member picks when connecting, with their main calendar preselected. Craig wanted games in a calendar of the Member's choosing, as Members of the previous site were used to.

The first step (#52, Craig 2026-11-05) writes only when the Member presses **Add N games**: the Teams and games checked on `/soccer`, into the calendar picked beside the button. The site holds the Google access and makes every Google call itself; the soccer backend only builds the event bodies (`POST /events`). Scores and a daily update that keeps Remembered teams current come later.

## Considered options

- A "Soccer" calendar created and managed by the site alone, using Google's narrower `calendar.app.created` permission. It keeps the site away from every other event the Member has and lets disconnecting remove the calendar cleanly, but the games sit in a separate calendar rather than the one the Member already uses.

## Consequences

- The permission asked for is expected to be in Google's sensitive class; Google does not publish the class, so it is confirmed in the Cloud console (see the baseline's list of things to prove). If it is, then without Google's app verification, Members see a one-time "This app isn't verified" screen, and the Google project carries a lifetime cap of 100 people. The project must be published as "In production", not "Testing", or calendar access expires after seven days.
- Each event the site writes has the league's game ID as its event ID (the old site's choice, so its games are updated, not added twice) and a private `game_id` property, and the site touches no other event. Pressing Add again rewrites those games whole and adds any posted since; a checked game the Member deleted comes back.
- When scores come, they go in the event title, because rewriting the description would wipe the Member's own notes.
- Disconnecting and switching calendar leave the games already added: they are the Member's now, like an imported file. Disconnecting revokes access at Google, which cancels every consent that person gave the same Google project, sign-in included; they consent again at their next Google sign-in.
- Assumed, not yet ruled on by Craig: a Member may grant event access but refuse the calendar list, and the site then offers only their main calendar. Without event access nothing is kept, and what Google gave is revoked.
- Google refusing the access (revoked, or lapsed) drops the connection, and the Member connects again. A calendar Google won't write to asks them to pick another.
- The connection is one `google#<email>` item in the site table, beside ADR 0011's items: the Google address, the chosen calendar and the refresh token, sealed with a key kept in Parameter Store. Removing a Member deletes it. Unlike ADR 0011, this adds IAM: the site role reads the Google client and the key under `/the-lobby/<env>/google/` with `ssm:GetParameters`, and `aws-design.md` §6.4 records it.
