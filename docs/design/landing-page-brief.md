# The Lobby landing-page brief

Status: confirmed by Craig on 2026-10-07, including the Local Co-op visual direction and The VIP Lobby name. This is a design brief, not an implementation plan or authorization to build.

Product authority: [PRODUCT.md](../../PRODUCT.md). Vocabulary: [CONTEXT.md](../../CONTEXT.md).

## Job and audience

The public `/` page welcomes people to The Lobby and gives invited Members a clear way to sign in. It is a shared-place introduction, with no public catalog of private Tools. Its mode is Persuade, with the narrow purpose of making arrival and the next action clear.

Signing in changes the page substantially. Its title is **The VIP Lobby**. Its mode is Operate: people recognize the Tools they can use and open one. The shared visual identity connects the two experiences, but the large public greeting gives way to useful choices.

## Outcome and evidence

Visitors can distinguish the primary **Sign in** action from the quiet **Schedule Downloader** link. `/soccer` remains public and does not require sign-in. The page explains that membership is by invitation and offers no sign-up or request-access flow.

Members see only Tools they can actually use. Schedule Downloader always appears in every Member's collection, as confirmed on 2026-10-08. Its public access remains distinct from its granted Member features. Being invited to another Tool does not grant Remembered teams or Connected calendar access.

The planned inventory is Schedule Downloader, Minecraft Launcher, Minecraft Admin, Foundry, and Foundry Admin. These are planned names, not a promise that all five are implemented or will launch together. Admin Tools require their own access and appear in a separate Admin Tools section, as confirmed on 2026-10-08. Their names do not add an Owner access-management screen.

## Selected direction

**Local Co-op** borrows the approachable rhythm of a shared game's title screen. The selected [public welcome preview](../../.impeccable/mocks/decision/round3-coop.png) is the visual reference; its [prompt record](../../.impeccable/mocks/decision/round3-coop.png.json) preserves its provenance.

- A seafoam field, deep navy text, a custard sign-in control, and coral details make one coherent scene.
- Large, substantial rounded lettering gives the public greeting its focal point. A small original cartoon robot supports the welcome without becoming the product or a promised game feature.
- Broad edge shapes and the darker footer frame the page. There is one main action and no competing feature panels.
- A brief tactile response to focus or activation is the signature interaction. Any optional character motion is decorative; the page is immediately usable with motion disabled.

Avoid realistic people. A clearly illustrated or cartoon character is acceptable. Keep the result welcoming across ages rather than resembling a preschool game. The title-screen reference does not introduce player counts, multiplayer status, sounds, controller requirements, or a press-key gate.

The VIP Lobby keeps the colours, type character, and control treatment, while reducing the greeting and illustration. Its focal point becomes a compact collection of accessible Tools. Use the Tool name and a clear open action; do not invent activity feeds, server status, statistics, or Tool-specific controls.

## Scope and boundaries

This brief covers the public welcome and the structural transition to the Member entry page. The selected image depicts only the public state. The Grouped Cards composition and Member entry behavior are recorded in the separate [VIP Lobby brief](vip-lobby-brief.md), approved by Craig on 2026-10-08.

It does not design Tool interiors, Minecraft or Foundry administration, a new authentication provider, invitations management, or an access-request workflow. It does not authorize application code, deployment, production cutover, or new infrastructure. Existing calendar-link access and the `/soccer` address remain intact.

The public welcome contains the site identity, greeting, one-sentence purpose, sign-in action, invitation explanation, public Schedule Downloader link, and privacy link. The earlier public Tool diagrams, technical articles, code links, and Craig bio have no newly agreed placement and are outside this page's content.

## States and content ranges

| State | Intended behavior |
| --- | --- |
| Visitor | Show the public welcome, sign-in action, and public `/soccer` link. |
| Checking identity or access | Keep a stable, clearly labelled loading state; show no private Tool names until access is known. |
| Member with access | Show the VIP Lobby and the available Tools the person can use. Public access and private grants remain distinct. |
| No private grants | Explain that no private Tools are assigned. Keep public Schedule Downloader access available; do not show locked private Tools. |
| Access lookup fails | Say that access could not be checked and offer retry. Do not misreport failure as an empty collection or reveal the private inventory. |
| Sign-in fails or is cancelled | Return to a usable public welcome with a plain recovery message when appropriate. |
| Sign-out or expired sign-in | Return to the public welcome and remove private Tool content. |

Design for one usable Tool, a few Tools, and the five currently named Tools. The collection must feel complete at the small end. Give Minecraft Admin and Foundry Admin enough room to remain distinguishable from their ordinary counterparts. Do not add unavailable future Tools just to fill space.

## Interaction and layout

On desktop, the public page is a compact, complete welcome composition with its main action and utility links in view. On phones, preserve the same reading order and clear sign-in action, let the title wrap naturally, reduce the character, and allow normal scrolling when content or text size needs it. Do not shrink the desktop image into a phone layout or lock the page to viewport height.

The Member page uses a smaller shared header, a clearly named Tool collection, and visible sign-out. Adapt the collection to a single reading column on narrow screens. Focus and hover may reinforce an action, but never reveal information needed to find or use it.

Use real text, links, and buttons in any eventual implementation. Keep focus visible, contrast sufficient, keyboard order logical, touch targets usable, and decorative imagery out of the accessibility tree. Decorative motion must respect reduced-motion preferences and must not delay content. These are agreed acceptance conditions, not completed accessibility tests.

## Delivery and open decisions

The platform is web. React and TypeScript with Vite, served by Go/Chi, remain the accepted stack. The public welcome must retain the agreed build-time HTML delivery. Private content must depend on server-verified access; hiding a link is not authorization.

The selected preview establishes direction. It has not been implemented, and responsive behavior, authentication transitions, accessibility, and actual Tool routing have not been tested. Image-first remains the recorded workflow default. No DESIGN.md or implementation direction contract is created by shape.

Later implementation preparation must resolve per-Tool destinations and grants, which Tools are ready to expose, final font assets, and the treatment of any unavailable Tool. It must not infer new Tool behavior or a release date from these previews.

## Direction contract

Development-only build contract, recorded 2026-10-08 for the code-led build of the confirmed Local Co-op direction (no image generation was available, so the selected preview serves as the critique reference).

- THESIS: The public welcome is one title-screen menu with a single entry action. It refuses the copy-left, image-right landing page and any row of feature cards.
- OWN-WORLD: Matte seafoam field with faint print grain, deep navy type and outlines, a custard Sign in face on a coral base, a low custard sun, layered seafoam foliage at the edges, and a navy ground wave carrying the footer. Rubik for lettering, Nunito Sans for reading text.
- STORY: A Visitor learns this is a shared place for family and friends, sees that access is by invitation, and either signs in or takes the quiet public Schedule Downloader link.
- FIRST VIEWPORT: Identity top left; "Welcome to" over a very large "The Lobby" centred; the purpose line; a wide tactile Sign in control; the invitation line; the small robot host to the lower right of the control; the footer links on the navy ground, all in view at 1280 to 1600 wide.
- FORM: Local Co-op title menu, pinned by this brief (no concept roll ran).
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
