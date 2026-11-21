---
status: accepted
date: 2026-11-15
---

# Lobby Admin changes Member records from the site

Until now Craig added, changed and removed Members only with the site's Owner command (`go run ./cmd/site member ...`, behind `task member:*`), from a computer with an admin profile. On 2026-11-15 he asked for an admin page for managing The Lobby, its Members and their grants included. Lobby Admin, at `/lobby/admin`, lists every Member with their Tools, adds and removes Members, grants and takes away Tools, shows when each was last here, and signs a Member out everywhere. The site does it with its own role, in its own environment's table, behind a new `lobby-admin` grant that only Craig holds.

`/admin` is taken by the family devices page (ADR 0010), so the page is under `/lobby/`.

## Considered options

- Keep the Access invite list. Either the page hands Craig a deploy after each add, the step he asked to stop doing, or the site edits the list with a Cloudflare token that could also change the family devices page's gate.
- Keep the Owner command only. Every invite, grant and lost phone would still need a computer with an admin sign-in, which is what Craig asked to stop doing.
- A page behind its own Cloudflare Access application, like `/admin`. That adds a second sign-in and a second list for one person who is already a Member, and the routes would still need the site's role to write the table.
- One page that changes both environments. The dev role cannot reach production's table, by design, so each environment's page manages that environment's Members, as the Owner command already does.

## Consequences

- The site role's statement on its table gains `dynamodb:Scan`, the only new permission: the page lists the Members and their Sign-in sessions by reading the table whole, filtered by kind, as the Owner command's `member list` already did with an admin profile. The table holds a handful of items per Member, so a scan stays small. ADR 0011's "GetItem, PutItem and DeleteItem only" now reads with Scan added. The bootstrap boundary already allowed it.
- Every route under `/api/lobby/members` needs the `lobby-admin` grant, checked from the Member record on each request like the other grants. A change also needs a same-origin, JSON request, so another site's page or form cannot make one with the Member's cookie.
- The page will not remove its own Member or take away its own `lobby-admin`, so the person using it cannot lock themselves out. The Owner command stays as the way back in and the way to grant `lobby-admin` in the first place.
- Removing a Member deletes every one of their Sign-in sessions, then their record and their soccer, Google and iCloud items as the Owner command does. The sessions go first, so a removal that fails part way leaves a Member who can be removed again, never sessions that would count again if the address were added back. Sign out everywhere deletes a Member's Sign-in sessions and keeps the asker's own when they sign themselves out of their other devices.
- Changed on 2026-11-20 (#130): after the Sign-in sessions, a removal marks the Member's record as being removed, then deletes their Member link and their soccer, Google and iCloud items, and deletes the record last. A marked record is nobody's for access: sign-in turns the address away as it does one with no record, a Sign-in session started meanwhile is a Visitor's, every Tool refuses, their Member link answers 404 and the daily iCloud update passes them over. So a removal that fails after the mark no longer leaves a Member who can sign in again. They stay on the page's list and in `site member list`, shown like any other Member, and pressing Remove again or running `site member remove` again finishes the removal. Until it has, adding the address is refused (`already_member` on the page), and a grant answers `not_member` and changes nothing, so nothing but the removal takes the mark off. The mark is one attribute, `removing`, that no record made before it has.
- "Last here" is worked out from each Sign-in session's expiry, which the site renews at most daily, so it is right to the day. A session's ID is its cookie's value, so the page and its answers never carry one, and the routes log what changed without the Member's address.
- Adding a Member on the page is the whole invite. Craig chose on 2026-11-15 to drop the Access invite list (`ALLOWED_EMAILS_<ENV>`, ADR 0004's first step), which the page could not change without a Cloudflare token inside the site: the sign-in application's policy now lets anyone with an emailed code through, and the site's existing Member check at `/signin` turns away an address with no Member record ("That address hasn't been invited"). Access still only proves who someone is. Anyone can now ask for a code, and Cloudflare's free plan counts each person who signs in through Access as a seat, 50 in all, so a stranger who finishes signing in uses one until removed from Zero Trust's users. The family devices page's own application and list (ADR 0010) are unchanged.
- The page's Tool checkboxes come from the same list as The VIP Lobby's Tools, and a test keeps the backend's list of grants equal to it.
