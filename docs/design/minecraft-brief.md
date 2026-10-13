# Minecraft Launcher design brief

Status: selected design direction, 2026-10-10. Craig chose **Status beside setup** after confirming **status, start, and join**, with **Java and Bedrock plus console help**. The selection confirms the layout; the proposed state behavior and unresolved operational details below remain design guidance. This is design work, not an implementation plan or permission to build, start a real server, publish, or deploy.

Target: `/minecraft`. Mode: Operate. Visual authority: [DESIGN.md](../../DESIGN.md). Product authority: [PRODUCT.md](../../PRODUCT.md), [baseline](../baseline.md), [vocabulary](../../CONTEXT.md), and the confirmed answers above.

## Job and outcome

An invited Member with Minecraft Launcher access wants to get into a game. They may be using the page on a phone while Minecraft runs on a computer or console. The page answers three questions in order: **Can I join? Do I need to start the server? How do I connect on my device?**

Success is verified server readiness followed by understandable connection instructions. A successful start request is not evidence that Minecraft is ready. Copying an address is not evidence that the player joined. The browser does not promise to launch Minecraft or join automatically.

The proposed first view represents one configured server, named **Family server** for the mockups. The server count, actual name, World structure, address, ports and compatible versions are not established by the source documents. The examples do not decide these facts. If the backend exposes several independently joinable servers, settle selection and per-server access before implementation; do not silently add a World switcher. A World is saved game content; a Server is the running Minecraft software.

## What carries over from the examples

Craig supplied `mc-pg1.png`, `mc-pg2.png` and `mc-pg3.png` from the primary checkout. Their [preserved references](references/minecraft/) carry these ideas:

- **mc-pg1:** recognizable server identity, immediate readiness and a clear main action. Its four-server carousel, player list, technical metrics and administrative controls are examples, not confirmed launcher requirements.
- **mc-pg2:** an understandable route from readiness to play. It depicts Foundry; its game-session button, seat count and backup claims do not establish Minecraft behavior.
- **mc-pg3:** a useful boundary for the separate Admin Tool: logs, backups and detailed operations stay there.

The Lobby retains its compact Local Co-op shell: seafoam, navy lettering and outlines, mint reading surfaces, Rubik headings, Nunito Sans body, and the custard control with a coral base. The supplied examples contribute hierarchy and task emphasis. Their dark portfolio navigation, cinematic hero art and glass surfaces do not replace DESIGN.md. No system-wide design change is proposed.

## Selected layout and mockup

**Status beside setup** is the selected layout: [static sketch](../../.impeccable/mocks/decision/minecraft-split-guide.html), [desktop preview](../../.impeccable/mocks/decision/minecraft-split-guide-desktop.png), and [phone preview](../../.impeccable/mocks/decision/minecraft-split-guide-mobile.png).

On desktop, a compact left column holds the server's name, current state, and Start server action. The larger right area holds the explicit device choice, connection details, and joining steps. Readiness remains visible while the Member prepares to connect. On a phone, the same content becomes a single sequence: readiness and Start first, then device choice and joining help. Preserve this arrangement and the established Local Co-op identity when building.

The selected sketch shows a stopped-server example, Java selected, with an explicitly fictional address. It is a static review artifact; its controls do not make network requests. Build the working page from the selected HTML layout, including the responsive behavior and states described below.

The other options remain as unselected exploration:

| Option | Composition | Tradeoff |
| --- | --- | --- |
| [Ready to play](../../.impeccable/mocks/decision/minecraft-readiness-stage.html) | A broad server-readiness area above device-specific joining help. | Strongest emphasis on the required first action; occupies more vertical space. |
| [Quick launch](../../.impeccable/mocks/decision/minecraft-launch-strip.html) | A compact readiness strip above a larger joining guide. | Fast for returning players; readiness has less emphasis. |

Rendered previews: Ready to play ([desktop](../../.impeccable/mocks/decision/minecraft-readiness-stage-desktop.png), [phone](../../.impeccable/mocks/decision/minecraft-readiness-stage-mobile.png)); Quick launch ([desktop](../../.impeccable/mocks/decision/minecraft-launch-strip-desktop.png), [phone](../../.impeccable/mocks/decision/minecraft-launch-strip-mobile.png)); Status beside setup ([desktop](../../.impeccable/mocks/decision/minecraft-split-guide-desktop.png), [phone](../../.impeccable/mocks/decision/minecraft-split-guide-mobile.png)). All six were inspected at 1440px desktop and 390px phone widths with loaded fonts and no horizontal overflow. They show static composition, not working controls, live status or completed accessibility testing.

An independent review of the brief and all six captures passed at design-review scope with no required corrections. A single mechanical detector pass reported typography-size advisories: these exploratory sketches use some literal sizes outside the documented token ramp. Normalize the selected layout to existing typography tokens during implementation; this does not authorize changes to DESIGN.md. Brief links and JSON records were checked, and the three reference copies match their originals.

Hook follow-up: two cramped-padding warnings in Status beside setup were false positives on the header and footer wrappers. Browser measurements found 22.5px / 24px clearance at desktop width and 19.5px / 22px on the phone; a second reviewer confirmed the existing captures. The layout stays unchanged. Only `cramped-padding` is ignored for this one static sketch through Impeccable's configuration; other rules and files remain covered.

The selected layout keeps The Lobby identity, Back to Tools, secondary Sign out, and a quiet Privacy footer. The signature interaction is the existing tactile Start server button; the status area keeps its place as the server moves through starting to ready. Do not add decorative loading choreography or a fake progress percentage.

**Restyle, 2026-10-13.** Craig found the built page too plain. A critique ([.impeccable/critique/2026-10-13T05-41-44Z__web-src-minecraft-launcher-tsx.md](../../.impeccable/critique/2026-10-13T05-41-44Z__web-src-minecraft-launcher-tsx.md)) offered three looks, each sketched live on made-up data, and he chose the Blocky look. The layout, states and behaviour above stay; the page gains the block motif recorded in DESIGN.md. The server card carries an original server block whose lamp lights at Ready, and the state in large lettering beside a block-shaped glyph. The device tabs show a glyph and where each plays, and the connection values sit in a framed slot whose Copy address takes the custard face once the server is ready. Step numbers sit on navy tiles, and the footer is a stepped navy ground where the host stands and waves once when the server turns ready while the page is open. Stop server moved under the server card, after the joining help on phones. The critique's usability fixes came with it: Check status again and Retry keep focus and answer even when nothing changed, the slow-start note is read out, the tab title leads with the state, and values wrap after their dots and colons instead of mid-name.

## Task and interaction

1. **Check access, then status.** Do not render private server details before the server confirms the Member's `minecraft` grant. Access failures, missing permission and an offline game server are different states.
2. **Start when needed.** Only a confirmed stopped server offers Start server. The control becomes unavailable immediately during the request. Repeated clicks, refreshes or another Member starting concurrently must resolve to the same current operation. A pending request must never be presented as a running game.
3. **Wait in place.** Show plain progress such as **Starting the server…**. Preserve the chosen edition/device and joining help. Only show milestones or an estimate when the backend can support them. A slow start and an unknown outcome need recovery guidance, not a fictional failure or another blind start request.
4. **Connect in Minecraft.** Once readiness is confirmed, say **Ready to join** and make the relevant connection details the main task. Use **Copy address**, **Copy port** where applicable, and device-specific steps. Changing the device help must not start, stop or change the server.

The proposed device choices are **Java Edition**, **Bedrock** (computer, phone, tablet), and **Console help** (Xbox, PlayStation, Switch). Keep the choice explicit and reversible rather than guessing from the browser device. The website may be open on a different device from the game.

Connection details may remain available while stopped/starting, with **Join after the server is ready** nearby. Connection details are never a success indicator. If a real address cannot yet be supplied, show **Connection details unavailable** rather than a usable-looking placeholder. The mockups use `play.example.net` only as a labelled example.

Copy feedback says **Address copied** or **Port copied** and is announced to assistive technology. Clipboard failure leaves the full value selectable with manual-copy guidance. Do not imply that copying on a phone transfers the value to a console or computer.

## States and ranges

| State | Message and behavior |
| --- | --- |
| Checking access | A labelled, stable loading region; no private server identity or connection data. |
| Signed out / expired | A sign-in prompt, then a return to `/minecraft` after successful sign-in. Remove private data on expiry or sign-out. |
| Access denied | Plain **You don't have access to this Tool** and Back to Tools. No request-access or upgrade flow. |
| Access lookup failed | **We couldn't check your access** and Retry; never treat failure as permission denial. |
| Checking server | **Checking server status…**; no inferred readiness or enabled Start server. |
| Stopped | **Stopped — start the server before joining.** One Start server action. |
| Start requested / starting | **Starting the server…**; disable duplicate starts and retain joining help. |
| Ready | **Ready to join** based on game-level readiness, not merely an accepted request or running Machine. Connection details become the focus. |
| Stopping / maintenance | State the condition and withhold conflicting actions. A Member with the grant may stop a ready server from a secondary Stop server control that asks before it acts; the backend refuses while anyone is online or a start is under way, and the page says so and changes nothing. |
| Start failed | Plain failure only when confirmed; provide a supported recovery action. No raw infrastructure errors. |
| Status stale / unavailable | **We can't confirm the server status.** Label any last-known state as such; Retry status, not an automatic restart. |
| No configured server / details absent | A clear unavailable message and Back to Tools. Do not expose setup forms or empty selectors to Members. |

Design for short and multi-line server names, long hostnames, both IPv4 and IPv6 if the backend supports them, and explicit numeric ports without clipping. Connection values wrap or remain selectable; never truncate away the information needed to join. Capacity and player information are omitted from this scope; their existence, audience and freshness have not been established.

## Joining copy and console boundary

**Java Edition:** “When the server is ready, open Minecraft Java Edition. Choose Multiplayer → Add Server, paste the server address, choose Done, then Join Server.” A non-default Java port must be included in the actual connection value when required; do not infer it from the mockup.

**Bedrock on computer or mobile:** “Choose Play → Servers → Add Server. Enter the server name, address and port shown here.” Each field must come from verified configuration. Do not publish a guessed default port or claim compatible game versions before the backend supplies them.

**Console help:** “Playing on Xbox, PlayStation or Switch? Console setup needs extra help.” Put the limitation before the steps: these consoles do not provide the same direct Add Server path as Bedrock on computer or mobile. Offer device-specific guidance only for a method selected and tested for this server. Until then, say **Joining this server from your console hasn't been confirmed yet**, and provide help information without a working-connection promise. No DNS changes, third-party bridge or account linking is prescribed by this brief.

General prerequisites can explain sign-in, multiplayer permissions and any applicable console online membership. The Lobby grant does not itself prove that a Minecraft account is allowed into the server. Game-account access and troubleshooting need a confirmed operational process before launch.

Primary documentation checked for design copy on 2026-10-10:

- [Minecraft server guide](https://www.minecraft.net/en-us/article/how-play-minecraft-server).
- [Minecraft Java joining guide](https://help.minecraft.net/hc/en-us/articles/32899741198989).
- [Minecraft Bedrock joining guide and console restriction](https://help.minecraft.net/hc/en-us/articles/4410316619533).
- [Minecraft multiplayer requirements](https://help.minecraft.net/hc/en-us/articles/35930809028749-Minecraft-Requirements-to-Play-Minecraft-Multiplayer-Games).
- [Geyser console guidance](https://geysermc.org/wiki/geyser/using-geyser-with-consoles/) for the existence and limits of additional console setup, not approval of a particular method.

Documentation is not device testing. Java/Bedrock interoperability and console joining still need proof against the configured server.

## Responsive and accessibility requirements

On a phone, put identity, current state and its next action before device help. Let content scroll normally. Reflow any desktop split into this order; do not shrink a desktop dashboard or preserve a fixed viewport height. Device labels wrap without horizontal clipping. Touch targets should be at least 44px, with legible connection values and explicit Copy labels.

Use one h1, labelled sections, real buttons and links, proper tab or radio semantics for device selection, keyboard navigation and a persistent selected state. Status must be understandable without color. Announce status changes and copy feedback without stealing focus. Preserve the user's focus and device choice during polling; stop decorative motion under reduced-motion preferences. The mockups illustrate layout only; they do not prove interactive or assistive-technology behavior.

## Scope and implementation boundaries

The `minecraft` grant owns the Launcher. The separate `minecraft-admin` grant and `/minecraft/admin` surface do not automatically confer Launcher access. Members with the grant may stop a ready server; it is refused while anyone is online, and nobody is ever kicked. Restart, command console, backups, performance charts, activity feeds, player management, World management, invitations and Owner access controls are outside this brief.

Keep the accepted React/TypeScript/Vite and Go/Chi architecture. The Lobby owns screens, Sign-in sessions and authorization; the Minecraft Tool backend owns operational status and start behavior. The browser must not hold infrastructure credentials or call an administrative control plane directly. No new service or deployment decision follows from this design.

Before building, settle: actual server/World topology and Member visibility; verified connection values and versions; status freshness and game-readiness criteria; start authorization, duplicate handling and failure recovery; game-account admission; the console method and its device proof. Do not invent APIs, timers, automatic shutdown, notifications or technical guarantees from these mockups.

Current-checkout evidence: `web/src/tools.ts` names `/minecraft` and `/minecraft/admin` separately and keeps both `ready: false`; the current server registers neither Minecraft route. This shaping task does not change those flags or application files. The dated statement in PRODUCT.md that React screens are absent is historical; current Welcome, VIP Lobby and Privacy source already exist.
