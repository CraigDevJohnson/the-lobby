---
status: accepted
date: 2026-11-14
---

# Add to iCloud Calendar goes through CalDAV with the Member's app-specific password

A Member with the `soccer` grant can have the games they check written into an iCloud calendar they pick, when they press **Add N games**, as Add to Google Calendar does for Google (ADR 0007). Craig asked for this on 2026-11-14, knowing what it takes. Changed by ADR 0015 (2026-11-16): a Member can also turn on a daily update, and the site then uses the password once a day, with nobody there, to add new games without rewriting any.

Apple gives a website no consent screen and no web API for iCloud Calendar. What it does serve is CalDAV at `caldav.icloud.com`, signed in to with the Apple Account address and an app-specific password the Member makes at `account.apple.com`. So the Member gives the site that pair, the site checks it by asking Apple where the account's calendars are, and keeps it. On each press the site lists the calendars the account can add events to, asks the soccer backend for the checked games as a calendar file (`POST /calendar`, the download), splits it into one file per game and stores each in the chosen calendar under the game's UID. The soccer backend is unchanged and never sees the password.

## Considered options

- The subscription the page already offers (Session link or Member link, **Open in Apple Calendar**). It needs no password and updates by itself, but the games sit in a separate subscribed calendar, every game for the Team comes along, and the Member can't edit them.
- The download, opened on the iPhone. No connection at all, but one-time, and unproven for several games at once (soccer brief).
- A native app using EventKit. Out of scope for a website.

## Consequences

- An app-specific password is not limited to calendars or to the site's events: it opens whatever iCloud data Apple serves to outside apps, mail and contacts included. The page says so before the form, and Privacy has its own topic. It needs two-factor authentication on the Apple Account. The site never asks for the Apple Account's own password and cannot tell the two apart, so the form says which to enter.
- The site cannot end the password. Disconnect deletes what the site kept; the Member removes the password at Apple. Apple refusing the password (removed, or the account's password changed) drops the connection, and the Member connects again.
- The connection is one `icloud#<email>` item in the site table, beside ADR 0011's items: the Apple Account address, where Apple keeps the account's calendars, the chosen calendar and the password, sealed with the same token key as Google's refresh tokens (`/the-lobby/<env>/google/token-key`), bound to the Member and to iCloud so it opens nowhere else. Sharing the key means no new parameter and no IAM change, and that Add to iCloud Calendar is offered only where Add to Google Calendar is set up. Removing a Member deletes the item.
- The password is sent only to `caldav.icloud.com` and to the `icloud.com` hosts Apple's own answers name, never to an address a request names, and no redirect is followed. Only a calendar Apple has just listed is written to. Neither the address nor the password is logged.
- Each game is stored as `<UID>.ics`, the UID the backend gives it in every calendar file, so pressing Add again rewrites those games whole and adds any posted since, and the site touches no other event. A checked game the Member deleted comes back. Disconnecting and switching calendar leave the games already added.
- iCloud has no main calendar to preselect; the calendars are offered by name and the first is chosen until the Member picks.
- Not proved against iCloud itself: the client is written to RFC 4791 and tested against a made-up server. Before Members are told about it, prove with a real Apple Account that the calendars list, that a game lands with its reminder and Boise time, that a second press updates and doesn't duplicate, and what Apple answers when a game with the same UID is already in that calendar from an imported file.
