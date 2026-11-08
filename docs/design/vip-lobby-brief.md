# The VIP Lobby design brief

Status: access, grouping and state behavior approved by Craig on 2026-10-08. On 2026-11-07, Craig selected and accepted **Navy Frame** from three live variants, superseding the original Grouped Cards visual treatment. The selected styling is implemented locally in `web/src/VipLobby.tsx` and `web/src/styles.css`; this does not establish deployment.

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

**Navy Frame**, selected on 2026-11-07, retains the grouped collection without tabs. A compact navy header contains The Lobby identity in pale seafoam and a secondary pale Sign out control. The navy continues into the title band, with The VIP Lobby in Rubik 760 and “Choose a Tool.” in seafoam. The title stays at the existing compact scale, up to 2.875rem.

The seafoam collection, header and footer content share a 76rem maximum width. Cards use an auto-fitting grid with a 17rem minimum column width and 1.5rem gaps. Each card has the full Tool name in bold Rubik, a larger supporting glyph, and one clear Open action at the lower right. The Let's Plan Soccer card also carries its tagline, **Link. Plan. Score**, under the name. Section labels are smaller at 1.25rem, weight 650. Keep the same stable ordering after filtering by access; do not leave placeholders or numbered gaps for hidden Tools. A single card is capped at 30rem.

Preserve Local Co-op's seafoam collection field, pale card surfaces, deep navy text and outlines, custard actions, coral tactile detail, and mature rounded type character. The footer repeats the navy frame, with a custard Privacy link and the decorative 40px robot. No realistic people, large welcome illustration, or marketing-scale title. The accepted spacing uses 2.75rem vertical title-band padding and 1.75rem card padding on desktop; under 40rem these become 2.2rem and 1.5rem. Glyphs are 3.75rem on desktop and 3rem under 40rem.

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

Use standard links and buttons. Each Tool provides one coherent keyboard target for opening it, with an accessible name that identifies the Tool. Do not nest buttons inside a separately clickable card. Sign out remains a secondary header action; Privacy remains a quiet footer link.

On narrower screens, let the grid reduce its columns and finish with one card per row. Cards have a 14rem minimum height, reduced to 12rem under 40rem; Open controls remain at least 3rem tall. Preserve Your Tools before Admin Tools, allow long names to wrap, and retain useful touch targets. Let content scroll naturally. Do not shrink the desktop screenshot, force a fixed viewport height, or make the user uncover Tool names through hover.

Keep focus visible and distinct, text contrast sufficient, and decorative glyphs or character art out of the accessibility tree when adjacent text already names their purpose. Motion is brief feedback only, respects reduced-motion preferences, and never delays the collection. These are acceptance conditions to verify during implementation, not completed accessibility tests.

## Delivery and remaining implementation decisions

The following records the original 2026-10-08 implementation-discovery scope. The 2026-11-07 change accepts the visual treatment only; it adds no Tool capability or deployment claim.

This remains a web interface using the accepted React/TypeScript/Vite and Go/Chi architecture. The server determines access; client-side filtering is presentation, not authorization. Tool interiors, new invitations flows, infrastructure, deployment, and production cutover are outside this shaping task.

The current trial exposes role, email, and a grant array through [the identity endpoint](../../internal/web/web.go). It does not provide a display name, avatar, recency, or service-health data. The eventual implementation must distinguish access-check failures from normal signed-out state and safely handle an empty or omitted grant list; the trial cannot yet support every planned state correctly.

Before implementation, resolve the exact per-Tool grant identifiers, configured destinations, release readiness, final fonts and icon assets, and behavior when an accessible Tool is unavailable. Do not infer those values or new Tool behavior from the mockup. The selected layout needs responsive and state verification in the eventual working interface. Shape creates no application code, DESIGN.md, or implementation direction contract.

## Direction contract

Development-only build contract, first recorded 2026-10-08 and updated 2026-11-07 for the accepted Navy Frame variant. The current source and [design system](../../DESIGN.md) record its visual treatment; the original mockup records the earlier grouping decision.

- THESIS: A compact working launcher in Local Co-op's materials. It refuses a hero, dashboards, status, and any locked or placeholder Tool.
- OWN-WORLD: Navy header, title band and footer around a seafoam collection; pale cards with 2px navy outlines and 16px corners; larger navy glyphs and bold Tool names; custard Open controls on a coral base; a 40px robot in the footer.
- STORY: A Member recognizes the Tools they can use, in Your Tools then Admin Tools, and opens one.
- FIRST VIEWPORT: Compact navy header with The Lobby and a pale Sign out; "The VIP Lobby" and "Choose a Tool." left aligned in the navy band; Your Tools in an adaptive collection, Admin Tools below when granted and ready; navy footer with Privacy.
- FORM: Navy Frame, selected from three live variants on 2026-11-07, preserving the Grouped Cards access and section rules (original surface seed 24d493bb).
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
