# Foundry player-page brief

Status: given by Craig on 2026-10-14 (his words below, as he wrote them). He asked to keep The Lobby's existing design and fixed the layout himself, so no layout options or new look were explored. This records design decisions; it does not authorize deployment or new grants. Server controls were kept off the page then; Craig added them on 2026-10-27 ("Start and stop" below).

Target: `/foundry`. Mode: Operate. Visual authority: [DESIGN.md](../../DESIGN.md). Product authority: [PRODUCT.md](../../PRODUCT.md), [vocabulary](../../CONTEXT.md) and Craig's brief. Backend contract: `docs/foundry-backend.md`, which arrives with the site's side of the Foundry backend.

## Shared blocky world, 2026-10-15

Craig approved extending Minecraft's blocky visual language across the Tool pages: Soccer gets pitch and calendar artwork; Foundry gets tabletop artwork. Each uses the existing palette, navy outlines, chunky controls, number tiles and shared stepped footer. This changes presentation, not Tool capabilities, access rules or deployment authority. The surface remains Operate mode. See DESIGN.md for the shared materials and state-linked illustration rules.

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

The Lobby's compact header retains The Lobby, Back to Tools and an outlined Sign out. The page title **Foundry** and a one-line lead introduce the task. The availability card includes the tabletop miniature; the shared stepped footer carries the decorative robot and Privacy.

On wide screens **Check availability** sits on the left: a 24rem column (20rem under 64rem) holding the status card. **Join in Foundry** takes the wider right column. It shares the Minecraft Launcher's status-beside-help layout, number tiles, framed controls and stepped terrace, with its own tabletop artwork. Under 45rem the two stack in that order, which is also the markup and keyboard order.

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
| Stopped | **Foundry is stopped.** There's no game to join until it's started. Open Foundry is quiet; Start Foundry is offered ("Start and stop" below). |
| Starting | **Foundry is starting.** The page checks again every few seconds. Open Foundry is quiet. No estimate, no progress. |
| Shutting down | **Foundry is shutting down** (the backend's `stopping`). Treated with stopped and starting as a confirmed state that is not responding. |
| Responding | **Foundry is responding.** Its join page answers; whether a game is open shows in Foundry. Open Foundry takes the custard face. Stop Foundry is offered, quiet. |
| Not answering | **Foundry isn't answering** (`unresponsive`): the server is on but the join page doesn't answer. If it lasts, let Craig know. Stop Foundry is offered, so the server can be stopped and started afresh. |
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

As first built, player access only. Since 2026-10-27 the page also starts and stops Foundry ("Start and stop" below); otherwise no restart, no world or campaign picker, no player counts, no backups, no admin features. The page shows only what the backend says: `ready` means the join page answered, so it is shown as responding. A claim that a game is open needs new backend evidence first.

The site owns the page, Sign-in sessions and the `foundry` grant check; the Foundry backend owns the status. The browser never calls the backend or AWS directly.

## Direction contract

Development-only build contract, recorded 2026-10-14 for the code-led build of Craig's pinned layout.

- THESIS: Foundry's door, not its dashboard. One card says whether Foundry is responding and when that was checked, and the way in stays open beside it. It refuses a server console, a "game ready" claim, and any control that changes the server.
- OWN-WORLD: The VIP Lobby's materials unchanged: seafoam field, 1.5px navy rules under the header and over the footer, a card-mint status card with a 2px navy outline and 1rem corners, the state in the Rubik state lettering beside a round navy mark from the Tool glyph stroke family, an outlined Check again, one custard Open Foundry on its coral base, the 40px robot in the footer.
- STORY: A player sees whether Foundry is responding and how fresh that is, checks again if they doubt it, and opens Foundry to finish joining there.
- FIRST VIEWPORT: Compact header (The Lobby; Back to Tools and outlined Sign out); "Foundry" with a one-line lead; on the left a 24rem column titled Check availability holding the status card (mark and state, checked time, meaning, Check again); on the right the wider Join in Foundry column with Open Foundry, the address it opens, and the steps inside Foundry. Phones stack the two in that order.
- FORM: Craig's pinned two-column layout, in his words: "Keep The Lobby’s existing design. Place “Check availability” on the left and “Join in Foundry” on the right. Stack them in that order on phones." Structured like the Minecraft Launcher's status beside setup. A precisely specified request on the established world is shaped directly, so no concept roll ran: seed: none (layout pinned by Craig's brief, quoted here).
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Start and stop, 2026-10-27

Status: decided by Craig on 2026-10-27 in the "The Lobby open issues" thread ([the-lobby #29](https://github.com/CraigDevJohnson/the-lobby/issues/29)). Five questions were put to him with a recommendation each, pointing at `docs/foundry-backend.md`, foundry's `CONTEXT.md` and `docs/status-api.md`, and `PRODUCT.md`; he took every recommendation ("As recommended for all questions"). This section supersedes the Scope line that kept server controls off this page and the direction contract's "any control that changes the server". It changes what the status card offers, not the page's layout, look, access rules or deployment authority.

### Decisions

1. **Where.** On `/foundry` itself, in the status card, the Minecraft Launcher's pattern. The backend already lets any Member with the `foundry` grant start and stop (Craig's "Every player" choice), the player who finds Foundry stopped is the one who needs Start, and Foundry Admin stays for things only Craig does. The Foundry Admin card in The VIP Lobby is unchanged and still not ready. (Alternatives put to him: a separate Foundry Admin page for the `foundry-admin` grant; Start here and Stop only there.)
2. **What a stop asks.** The Launcher's shape: pressing Stop Foundry turns the control into a question, **Is anyone playing or preparing a game in Foundry? Stopping disconnects them.**, with the answers **Nobody is, stop** and **Keep it running**. No checkbox or typed text. Pressing "Nobody is, stop" is the Member's confirmation that nobody is playing, which the site sends as `{"nobody_playing": true}`; foundry's Host control asks the same ("playing or preparing") and foundry's `CONTEXT.md` counts a GM preparing alone as a session.
3. **A change under way, or failed.** The existing Starting and Shutting down states carry the change, with their turning mark and the fast rechecks; no control is offered while one is under way. When the backend reports that the latest stop failed (`checks.stop` is `failed`), the meaning line adds **A stop sent earlier didn't finish, so Foundry's server is still on. Craig has been alerted.** and Stop Foundry is offered again. A refused change says why and what to do instead (below). No new backend work.
4. **No snapshot or backup status.** The controller has no snapshot read, and a stop takes no backup; the scheduled snapshot already holds the finished session.
5. **No "who last started or stopped it".** The backend logs the Member who asked; the status does not carry it, and players don't need it.

### What the card offers

Only a state the backend has just confirmed offers a change. A stale state (a newer check failed, or the last one is more than five minutes old) and an unconfirmed status offer nothing: Check again comes first.

| Confirmed state | Control |
| --- | --- |
| Stopped | **Start Foundry**, the custard tactile control at the Launcher's Start server size, full width of the card. Open Foundry is quiet then, so one custard control stays the rule. |
| Responding, Not answering | **Stop Foundry**, quiet, under the meaning line. Not answering offers it because the server is on and a stop then a fresh start is what clears it. |
| Starting, Shutting down, No server | Nothing. |

The controls sit between the meaning line and Check again, so the card reads state, when it was checked, what it means, the one change that fits, Check again. They are inside the card on every width.

### While a change is on its way

The state's own words stay as the backend last confirmed them; the meaning line says **Asking Foundry to start…** or **Asking Foundry to stop…**. Start Foundry reads **Starting…**, unavailable and sunk at full strength like Retry; the question's **Nobody is, stop** reads **Stopping…** the same way and **Keep it running** is disabled. The page's own rechecks pause until the answer lands, so the answer is the one shown. The status card announces "Asking Foundry to start." or "…to stop." without moving focus.

The answer is the backend's status, shown as any check's answer is: a `202` lands in Starting or Shutting down, a `200` in the state that had nothing to change. When the control the Member pressed goes with the new state, focus moves to the state line.

### Refusals and no answer

A `409` changes nothing. Its reason takes the question's place as a notice above the control, and focus returns to Stop Foundry:

- a start while the host is still shutting down: **Foundry is still shutting down. It can be started once it has stopped.**
- a stop while the host is still starting: **Foundry's server is still starting up. Try stopping it again in a minute.**
- a stop before the host's agent is connected: **Foundry's server has only just started and can't take a stop yet. Try again in a minute.**

A refusal is read only for a reason that change can be refused for; any other `409`, and `400`, `503` or an answer the page can't read, is a change with no answer. Then the page can't tell whether the request reached Foundry: the status is shown as the last check, now possibly out of date, the meaning line says **We couldn't tell whether Foundry got the start request. Check again before trying again.** (or stop), no control is offered, and Check again's confirmed answer brings the controls back. The same request is never offered until a check lands.

### Focus and announcements

Pressing Stop Foundry puts focus on the question; Keep it running, Escape and a refusal put it back on Stop Foundry. The question goes by itself when the state moves on (another Member stopped Foundry, say, or the confirmed state aged). Refusals are announced as said; the status change that follows a start or stop is announced as any status change is.

### Boundaries

The site's `/api/foundry/server/start` and `/stop` routes and the backend are unchanged (`docs/foundry-backend.md`). Both site environments' backends act on the one host: a Member on the dev site stops the host production shows. No restart, no player counts, no snapshot status, no Foundry Admin surface.
