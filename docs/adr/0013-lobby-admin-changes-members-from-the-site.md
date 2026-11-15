---
status: accepted
date: 2026-11-15
---

# Lobby Admin changes Member records from the site

Until now Craig added, changed and removed Members only with the site's Owner command (`go run ./cmd/site member ...`, behind `task member:*`), from a computer with an admin profile. On 2026-11-15 he asked for an admin page for managing The Lobby, its Members and their grants included. Lobby Admin, at `/lobby/admin`, lists every Member with their Tools, adds and removes Members, grants and takes away Tools, shows when each was last here, and signs a Member out everywhere. The site does it with its own role, in its own environment's table, behind a new `lobby-admin` grant that only Craig holds.

`/admin` is taken by the family devices page (ADR 0010), so the page is under `/lobby/`.

## Considered options

- Keep the Owner command only. Every invite, grant and lost phone would still need a computer with an admin sign-in, which is what Craig asked to stop doing.
- A page behind its own Cloudflare Access application, like `/admin`. That adds a second sign-in and a second list for one person who is already a Member, and the routes would still need the site's role to write the table.
- One page that changes both environments. The dev role cannot reach production's table, by design, so each environment's page manages that environment's Members, as the Owner command already does.

## Consequences

- The site role's statement on its table gains `dynamodb:Scan`, the only new permission: the page lists the Members and their Sign-in sessions by reading the table whole, filtered by kind, as the Owner command's `member list` already did with an admin profile. The table holds a handful of items per Member, so a scan stays small. ADR 0011's "GetItem, PutItem and DeleteItem only" now reads with Scan added. The bootstrap boundary already allowed it.
- Every route under `/api/lobby/members` needs the `lobby-admin` grant, checked from the Member record on each request like the other grants. A change also needs a same-origin, JSON request, so another site's page or form cannot make one with the Member's cookie.
- The page will not remove its own Member or take away its own `lobby-admin`, so the person using it cannot lock themselves out. The Owner command stays as the way back in and the way to grant `lobby-admin` in the first place.
- Removing a Member deletes their record, their soccer, Google and iCloud items as the Owner command does, and every one of their Sign-in sessions. Sign out everywhere deletes a Member's Sign-in sessions and keeps the asker's own when they sign themselves out of their other devices.
- "Last here" is worked out from each Sign-in session's expiry, which the site renews at most daily, so it is right to the day. A session's ID is its cookie's value, so the page and its answers never carry one, and the routes log what changed without the Member's address.
- Cloudflare Access still sends sign-in codes only to the addresses on each environment's invite list (ADR 0004), which the page cannot change. Adding a Member on the page writes the Member record; the address goes on `ALLOWED_EMAILS_<ENV>` and a deploy applies it, and the page says so after an add.
- The page's Tool checkboxes come from the same list as The VIP Lobby's Tools, and a test keeps the backend's list of grants equal to it.
