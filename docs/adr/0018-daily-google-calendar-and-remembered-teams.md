---
status: accepted
date: 2026-11-25
---

# Follow Remembered teams in a Member's Google or iCloud calendar

Members can remember up to five Teams and turn on **Add new games every day** for a Google or iCloud calendar. Each connection has its own switch and destination. Existing Google connections start with the switch off. A one-time copy to another calendar leaves the daily destination in place. If that destination becomes unavailable, the Member can stop updates or explicitly choose another calendar. **Add N games** still copies the games the Member checked.

Each daily run sends the Member's saved Team IDs to the soccer backend's `POST /feeds`. When the league identifies a single successor by team name and coach in the earliest eligible later Session, the site saves the new Team ID alongside the previous one. When a Team cannot be followed, the feed can supply an all-day notice linking to the next-Session picker. Choosing a successor there updates the same Remembered team. Neither path requires a calendar subscription.

Both providers use the same daily update rules. The site asks for upcoming games after resolving the Teams. It also selects tracked games whose time or field changed, so a game moved earlier than today’s run can still be corrected. Google gets its game bodies from `POST /events`, as on a button press. Each destination keeps the identity, league start and field of the games it has added or recognized. An unchanged game needs no provider call. A changed start or field updates the existing event and can overwrite personal edits. A tracked game the Member deleted stays deleted. After saving the change, the site emails the Member which games moved and which were missing. A game disappearing from a league answer never authorizes deleting the Member's event. When the league omits a game ID, its fallback key uses the start and field. Moving such a game can leave the old copy behind because the new listing has a different identity. Scores and changes made in a calendar do not flow back to the league.

The existing 16:00 UTC EventBridge rule sends a separate event to the site's Lambda for each provider. This gives each its own 25-second work budget and isolates a provider outage. A run saves successful writes before it stops. The next run retries unfinished games. Google uses stable event IDs on insert and checks its ownership marker and the stored event's ETag before replacing a game. A connection or destination change invalidates an older run's checkpoint. [Google Calendar conditional updates](https://developers.google.com/workspace/calendar/api/guides/version-resources)

Remembering Teams no longer creates a public address. Previously issued Member links continue serving the same Teams. A Member can revoke their old address without affecting either connected calendar. Public Session links and file downloads remain available.
