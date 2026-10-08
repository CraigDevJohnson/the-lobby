# The VIP Lobby design brief

Status: draft for Craig's confirmation. The Grouped Cards layout was selected on 2026-10-08. This records design decisions, not an implementation plan or authorization to build.

Target: the signed-in state of `/`. Mode: Operate. This extends the confirmed [landing-page brief](landing-page-brief.md) and [product context](../../PRODUCT.md).

## Job and outcome

A Member arrives to recognize and open a Tool they can use. The VIP Lobby replaces the large public welcome with a compact, useful collection. Its name and Local Co-op identity are already confirmed.

Success is one clear open action per destination, with no inaccessible Tools to sort through. The planned maximum currently contains five names; the design must also work with Schedule Downloader alone.

## Access and grouping

Craig confirmed both rules on 2026-10-08: Schedule Downloader always appears for every Member, and granted Admin Tools belong in a separate section.

| Section | Display order and eligibility |
| --- | --- |
| Your Tools | Schedule Downloader always; Minecraft Launcher and Foundry when the Member has the corresponding access. |
| Admin Tools | Minecraft Admin, then Foundry Admin, each only with its own grant. Hide the entire section when neither is granted. |

Public Schedule Downloader access does not grant its extra Member features. Remembered teams and Connected calendar still require the relevant grant. An admin grant does not automatically confer access to another Tool, and these Admin Tools do not introduce an Owner access-management screen.

Only expose destinations that are ready to use. The [selected preview](../../.impeccable/mocks/decision/vip-round1-cards.png) assumes one Member can use all five planned Tools; it is a design example, not a claim that those Tools are implemented. Its [prompt record](../../.impeccable/mocks/decision/vip-round1-cards.png.json) preserves the reference image and generation provenance.

## Selected layout

**Grouped Cards** keeps both sections visible without tabs. A compact header contains The Lobby identity and secondary Sign out control. The page title is The VIP Lobby, followed by “Choose a Tool.”

The desktop collection uses three compact cards across for Your Tools and up to two matching cards below under Admin Tools. Each card has the full Tool name, a supporting glyph, and one clear Open action. Keep the same stable ordering after filtering by access; do not leave placeholders or numbered gaps for hidden Tools. A single card stays a reasonable width rather than stretching across the page.

Preserve Local Co-op's seafoam shell, deep navy text, custard actions, coral tactile detail, and mature rounded type character. Reading surfaces use a lighter seafoam tint. The small cartoon robot is decorative and subordinate, as in the selected footer. No realistic people, large welcome illustration, or marketing-scale title.

Keep the recognizable glyphs and full labels together, especially for Minecraft Launcher versus Minecraft Admin and Foundry versus Foundry Admin. The signature interaction is a restrained tactile response on the Open control. The layout introduces no server status, activity feed, recent-use history, search, personalization controls, or Tool-specific operations.

## States and ranges

- **Checking access:** show a stable, labelled loading state without private Tool names. Public `/soccer` remains available independently of sign-in.
- **Normal access:** show Schedule Downloader and the eligible private entries. The all-five example is the current largest planned collection, not a fixed number of slots.
- **No private grants:** show Schedule Downloader normally and a short explanation that no private Tools are assigned. Omit the Admin Tools section.
- **Access check fails:** show a plain error and Retry. Do not treat failure as an empty collection or reveal previously authorized private entries as if access were confirmed.
- **Access changes:** re-render the permitted collection when updated access is known. A removed grant must not remain usable through a stale screen; the server checks every protected action.
- **Sign-out or expiry:** return to the public welcome and remove private Tool content. Preserve the recovery behavior described in the parent brief for sign-in failures.

Do not add locked cards, unavailable future placeholders, upgrade language, request-access controls, or totals that count hidden Tools.

## Interaction and responsive behavior

Use standard links and buttons. Each Tool provides one coherent keyboard target for opening it, with an accessible name that identifies the Tool. Do not nest buttons inside a separately clickable card. Sign out remains a secondary header action; Privacy remains a quiet footer link.

On narrower screens, reduce the card columns and finish with one card per row. Preserve Your Tools before Admin Tools, allow long names to wrap, and retain useful touch targets. Let content scroll naturally. Do not shrink the desktop screenshot, force a fixed viewport height, or make the user uncover Tool names through hover.

Keep focus visible and distinct, text contrast sufficient, and decorative glyphs or character art out of the accessibility tree when adjacent text already names their purpose. Motion is brief feedback only, respects reduced-motion preferences, and never delays the collection. These are acceptance conditions to verify during implementation, not completed accessibility tests.

## Delivery and remaining implementation decisions

This remains a web interface using the accepted React/TypeScript/Vite and Go/Chi architecture. The server determines access; client-side filtering is presentation, not authorization. Tool interiors, new invitations flows, infrastructure, deployment, and production cutover are outside this shaping task.

The current trial exposes role, email, and a grant array through [the identity endpoint](../../internal/web/web.go). It does not provide a display name, avatar, recency, or service-health data. The eventual implementation must distinguish access-check failures from normal signed-out state and safely handle an empty or omitted grant list; the trial cannot yet support every planned state correctly.

Before implementation, resolve the exact per-Tool grant identifiers, configured destinations, release readiness, final fonts and icon assets, and behavior when an accessible Tool is unavailable. Do not infer those values or new Tool behavior from the mockup. The selected layout needs responsive and state verification in the eventual working interface. Shape creates no application code, DESIGN.md, or implementation direction contract.
