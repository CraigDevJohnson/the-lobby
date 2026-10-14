# Foundry player-page brief

Status: given by Craig on 2026-10-14 (his words below, as he wrote them). He asked to keep The Lobby's existing design and fixed the layout himself, so no layout options or new look were explored. This records design decisions; it does not authorize server controls, deployment, or new grants.

Target: `/foundry`. Mode: Operate. Visual authority: [DESIGN.md](../../DESIGN.md). Product authority: [PRODUCT.md](../../PRODUCT.md), [vocabulary](../../CONTEXT.md) and Craig's brief. Backend contract: `docs/foundry-backend.md`, which arrives with the site's side of the Foundry backend.

## Craig's brief

> Foundry player-page brief
>
> * Purpose: Invited players check availability and open Foundry from `/foundry`.
> * Layout: Keep The Lobby’s existing design. Place “Check availability” on the left and “Join in Foundry” on the right. Stack them in that order on phones.
> * Actions: Show status, when it was checked, Check again, and Open Foundry. Players finish joining inside Foundry. For authorized players, a failed status check should not block the configured joining link.
> * States: Handle checking, confirmed stopped or starting, responding, stale status, unavailable status, and access failures distinctly. “Foundry is responding” does not mean a world is ready to play; that stronger claim requires additional backend evidence.
> * Scope: Player access only. No server controls, campaign picker, player counts, backups, or admin features.

## Job and outcome

A Member with the `foundry` grant wants to get into a game on Craig's Foundry Virtual Tabletop server. The page answers two questions in order: **Is Foundry up, and how sure are we? How do I get in?** Joining itself happens inside Foundry, where the player picks their name and enters their password; The Lobby does not sign anyone in to Foundry.

Success is a player opening Foundry's join page while Foundry is responding. A responding join page is not evidence that a game is open, so the page never says a game or world is ready. Opening the link is not evidence that the player joined.

## Layout

The Lobby's compact shell, unchanged: the header with The Lobby, Back to Tools and an outlined Sign out; the page title **Foundry** and a one-line lead that says what the page is for; and the quiet footer with the 40px robot and Privacy, as on The VIP Lobby. Foundry has no signature move of its own (DESIGN.md allows one per Tool interior; none is proposed).

On wide screens **Check availability** sits on the left: a 24rem column (20rem under 64rem) holding the status card. **Join in Foundry** takes the wider right column. It is structured like the Minecraft Launcher's status beside its joining help, without the Launcher's block motif or terrace. Under 45rem the two stack in that order, which is also the markup and keyboard order.

## Actions

- **Status:** the state in the large state lettering beside a drawn mark, so it reads without colour, and a sentence on what it means for joining.
- **When it was checked:** "Checked at 7:42 PM" in the player's own time, from the backend's `checked_at`. When the site could not reach the backend, the page says when it tried instead.
- **Check again:** a quiet control in the card, always offered once there is a status. It stays in place while it asks, keeps focus, and says "Checked just now" with the result even when nothing changed. The page also checks by itself while it is visible: often while Foundry is starting or shutting down, about once a minute otherwise, and at once when the tab is shown again.
- **Open Foundry:** a real link to the configured join page, with the address it opens shown under it. It takes the custard face when opening it is the sensible next step: Foundry is responding, or its status can't be confirmed. It drops to a quiet control, still working, when the backend has confirmed Foundry is not answering players (stopped, starting, shutting down, not answering, no server), with a line saying why it may not open. It is never disabled or hidden once the page knows the link.

A failed status check never takes the link away. The link comes with every status answer the backend gives. When the site cannot get a status from the backend at all, it sends its own copy of the configured link (`FOUNDRY_JOIN_URL`) with that failure, added with the site's side of the Foundry backend; the page reads it there when present, and otherwise uses a link it already received on this visit. A failed access check hides the link with everything else until Retry, but does not make the page forget it.

## States

| State | What the page says and does |
| --- | --- |
| Checking | Both sections are in place from the first paint: the status card says "Checking Foundry…" beside the loading dots, and Open Foundry's place is kept empty, so nothing jumps when the first answer lands. Nothing about the server is on screen, and the prerendered page holds nothing private. |
| Stopped | **Foundry is stopped.** There's no game to join until it's started. Open Foundry is quiet. |
| Starting | **Foundry is starting.** The page checks again every few seconds. Open Foundry is quiet. No estimate, no progress. |
| Shutting down | **Foundry is shutting down** (the backend's `stopping`). Treated with stopped and starting as a confirmed state that is not responding. |
| Responding | **Foundry is responding.** Its join page answers; whether a game is open shows in Foundry. Open Foundry takes the custard face. |
| Not answering | **Foundry isn't answering** (`unresponsive`): the server is on but the join page doesn't answer. If it lasts, let Craig know. |
| No server | **Foundry has no server right now** (`404 no_server`), for example during a rebuild. Check back later. |
| Stale | The last confirmed state is kept in the past tense (**Foundry was responding**) with "Last checked at …", when a newer check failed or when it is more than five minutes old (the tab slept, say). Its mark is that state's own, drawn open and still on a dashed outline, so it reads apart from both a confirmed state and Unavailable. The meaning line opens with **This may be out of date.** Check again; Open Foundry stays available. |
| Unavailable | **We can't check Foundry right now** (the backend's `unknown`, the site's `tool_unavailable`, or no readable answer) with nothing confirmed earlier on this visit. That doesn't mean Foundry is down; Open Foundry is offered when the link is known. |
| Signed out | "Sign in to use Foundry." and Sign in, returning to `/foundry`. Anything private leaves the screen. |
| No access | "You don't have access to this Tool." and Back to Tools. No request-access flow. |
| Access check failed | "We couldn't check your access." and Retry, which keeps focus. Never treated as no access. |
| Not set up | "Foundry isn't available here yet." and Back to Tools, when the site has no Foundry backend configured. |

Announce state changes and Check again's result without moving focus. The tab title leads with the state. When focus was on something an answer removed, it moves to the new status or problem.

## Joining copy and the Foundry boundary

Steps inside Foundry, as help rather than promises: open Foundry; choose your name from its list; enter your password if you have one, then join the game. A player who lands on Foundry's setup screen instead has reached Foundry with no game open; the page says so and suggests letting Craig know. Foundry names and passwords come from Craig, not from The Lobby, and the Lobby grant does not prove a Foundry user exists for the player.

Foundry's own documentation could not be reached from the cloud build on 2026-10-14. The steps stay at that general level, matching hosting guides that describe the join page as a list of player names with an optional password, and need checking against the running server.

## Responsive and accessibility requirements

One h1, two labelled sections in reading order, real links and buttons, 44px targets, visible focus, and no horizontal scroll from 320px up. The address wraps after its dots, colons and slashes rather than mid-name. Status is said in words beside its mark; motion is the existing loading dots and a turning mark while starting or shutting down, both stopped under reduced motion.

## Scope and implementation boundaries

Player access only: no start, stop or restart, no world or campaign picker, no player counts, no backups, no admin features. Start and stop routes on the backend are for a later page and are not called here. The page shows only what the backend says: `ready` means the join page answered, so it is shown as responding. A claim that a game is open needs new backend evidence first.

The site owns the page, Sign-in sessions and the `foundry` grant check; the Foundry backend owns the status. The browser never calls the backend or AWS directly.

## Direction contract

Development-only build contract, recorded 2026-10-14 for the code-led build of Craig's pinned layout.

- THESIS: Foundry's door, not its dashboard. One card says whether Foundry is responding and when that was checked, and the way in stays open beside it. It refuses a server console, a "game ready" claim, and any control that changes the server.
- OWN-WORLD: The VIP Lobby's materials unchanged: seafoam field, 1.5px navy rules under the header and over the footer, a card-mint status card with a 2px navy outline and 1rem corners, the state in the Rubik state lettering beside a round navy mark from the Tool glyph stroke family, an outlined Check again, one custard Open Foundry on its coral base, the 40px robot in the footer.
- STORY: A player sees whether Foundry is responding and how fresh that is, checks again if they doubt it, and opens Foundry to finish joining there.
- FIRST VIEWPORT: Compact header (The Lobby; Back to Tools and outlined Sign out); "Foundry" with a one-line lead; on the left a 24rem column titled Check availability holding the status card (mark and state, checked time, meaning, Check again); on the right the wider Join in Foundry column with Open Foundry, the address it opens, and the steps inside Foundry. Phones stack the two in that order.
- FORM: Craig's pinned two-column layout, in his words: "Keep The Lobby’s existing design. Place “Check availability” on the left and “Join in Foundry” on the right. Stack them in that order on phones." Structured like the Minecraft Launcher's status beside setup. A precisely specified request on the established world is shaped directly, so no concept roll ran: seed: none (layout pinned by Craig's brief, quoted here).
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
