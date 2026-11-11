# The VIP Lobby design brief

Craig approved access, grouping, and state behavior on 2026-10-08. On 2026-11-11, Craig selected **Arcade Cabinet** from five overdrive variants. The accepted layout is implemented locally in `web/src/VipLobby.tsx` and `web/src/styles.css`.

Target: the signed-in state of `/`. Mode: Operate. This extends the confirmed [landing-page brief](landing-page-brief.md) and [product context](../../PRODUCT.md).

## Job and outcome

A Member arrives to recognize and open a Tool they can use. The VIP Lobby replaces the large public welcome with a compact, useful collection. Its name and Local Co-op identity are already confirmed.

Success is one clear open action per destination, with no inaccessible Tools to sort through. The planned maximum currently contains five names; the design must also work with Let's Plan Soccer alone.

## Access and grouping

Craig confirmed both rules on 2026-10-08: Let's Plan Soccer always appears for every Member, and granted Admin Tools belong in a separate section.

| Section | Display order and eligibility |
| --- | --- |
| Your Tools | Let's Plan Soccer always; Minecraft Launcher and Foundry when the Member has the corresponding access. |
| Admin Tools | Minecraft Admin, then Foundry Admin, each only with its own grant. Hide the entire section when neither is granted. |

Public Let's Plan Soccer access does not grant its extra Member features. Remembered teams and Connected calendar still require the relevant grant. An admin grant does not automatically confer access to another Tool, and these Admin Tools do not introduce an Owner access-management screen.

Only expose destinations that are ready to use. The [original Grouped Cards preview](../../.impeccable/mocks/decision/vip-round1-cards.png) assumes one Member can use all five planned Tools; it remains a historical grouping example, not the current visual reference or a claim that those Tools are implemented. Its [prompt record](../../.impeccable/mocks/decision/vip-round1-cards.png.json) preserves the reference image and generation provenance.

## Selected layout

**Arcade Cabinet** keeps both Tool groups in one pale seafoam enclosure. A compact navy header contains The Lobby identity and a pale Sign out control. The centred title uses Rubik 760 at up to 4.6rem, with “Choose a Tool.” in seafoam beneath it.

The cabinet, header, and footer content share a 67rem maximum width. Tiles use an auto-fitting grid with a 14rem minimum column width and 0.875rem gaps. Each seafoam tile pairs the full Tool name in bold Rubik with its glyph and a full-width custard Open control. The Let’s Plan Soccer tile also carries its tagline, **Link. Plan. Score**, under the name. Section labels are 1.25rem at weight 650 with a navy rule underneath. The grid keeps its order after filtering and a single tile fills the row.

Local Co-op's navy outlines and type, seafoam surfaces, and custard controls with coral bases carry the design. The navy footer holds a custard Privacy link and the decorative 40px robot. Glyphs are 3.75rem on desktop and 3rem at 40rem and below.

Keep the recognizable glyphs and full labels together, especially for Minecraft Launcher versus Minecraft Admin and Foundry versus Foundry Admin. The signature interaction is a restrained tactile response on the Open control. The layout introduces no server status, activity feed, recent-use history, search, personalization controls, or Tool-specific operations.

## States and ranges

- **Checking access:** show a stable, labelled loading state without private Tool names. Public `/soccer` remains available independently of sign-in.
- **Normal access:** show Let's Plan Soccer and the eligible private entries. The all-five example is the current largest planned collection, not a fixed number of slots.
- **No private grants:** show Let's Plan Soccer normally and a short explanation that no private Tools are assigned. Omit the Admin Tools section.
- **Access check fails:** show a plain error and Retry. Do not treat failure as an empty collection or reveal previously authorized private entries as if access were confirmed.
- **Access changes:** re-render the permitted collection when updated access is known. A removed grant must not remain usable through a stale screen; the server checks every protected action.
- **Sign-out or expiry:** return to the public welcome and remove private Tool content. Preserve the recovery behavior described in the parent brief for sign-in failures.

Do not add locked cards, unavailable future placeholders, upgrade language, request-access controls, or totals that count hidden Tools.

## Interaction and responsive behavior

Use standard links and buttons. Each Tool tile expands the hit area of its Open link and provides one keyboard target with an accessible name that identifies the Tool. Sign out remains a secondary header action and Privacy remains a quiet footer link.

On narrower screens, let the grid reduce its columns and finish with one tile per row. Above 35rem, tiles have a 13rem minimum height. At 35rem and below, each tile places the glyph beside the name and its height follows the content. Open controls remain at least 3rem tall. Keep Your Tools before Admin Tools and allow long names to wrap.

Keep focus visible and distinct, text contrast sufficient, and decorative glyphs or character art out of the accessibility tree when adjacent text already names their purpose. Motion is brief feedback only, respects reduced-motion preferences, and never delays the collection. These are acceptance conditions to verify during implementation, not completed accessibility tests.

## Delivery and remaining implementation decisions

The following records the original 2026-10-08 implementation-discovery scope. The 2026-11-11 selection covers the visual treatment.

This remains a web interface using the accepted React/TypeScript/Vite and Go/Chi architecture. The server determines access; client-side filtering is presentation, not authorization. Tool interiors, new invitations flows, infrastructure, deployment, and production cutover are outside this shaping task.

The current trial exposes role, email, and a grant array through [the identity endpoint](../../internal/web/web.go). It does not provide a display name, avatar, recency, or service-health data. The eventual implementation must distinguish access-check failures from normal signed-out state and safely handle an empty or omitted grant list; the trial cannot yet support every planned state correctly.

Before implementation, resolve the exact per-Tool grant identifiers, configured destinations, release readiness, final fonts and icon assets, and behavior when an accessible Tool is unavailable. Do not infer those values or new Tool behavior from the mockup. The selected layout needs responsive and state verification in the eventual working interface. Shape creates no application code, DESIGN.md, or implementation direction contract.

## Direction contract

Development-only build contract, first recorded 2026-10-08 and updated 2026-11-11 for Arcade Cabinet. The [design system](../../DESIGN.md) records the current visual treatment.

- THESIS: A working launcher with the feel of a Local Co-op arcade cabinet.
- OWN-WORLD: Navy field, pale cabinet, seafoam launch tiles with 2px navy outlines, and custard Open controls on a coral base. The 40px robot stays in the footer.
- STORY: A Member recognizes the Tools they can use, in Your Tools then Admin Tools, and opens one.
- FIRST VIEWPORT: A compact header, centred title, and the cabinet with Your Tools first. Admin Tools appear below when granted and ready. The footer holds Privacy.
- FORM: Arcade Cabinet, selected from five overdrive variants on 2026-11-11. Each tile is the hit area for its Open link.
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
