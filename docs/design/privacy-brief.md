# Privacy design brief

Status: confirmed by Craig on 2026-10-08. The approved brief is a short public note covering the agreed first release, using **What we keep / What you control**. This records design decisions, not authorization to implement or publish.

Target: `/privacy`. Mode: Read.

## Job and outcome

A Visitor or Member follows Privacy from either Lobby footer to understand what information the site keeps, why it needs it, and what choices they have. The page should answer those questions in a few minutes, in language suitable for family, friends, and public Schedule Downloader users.

Cover the agreed first release, including Remembered teams and Connected calendars. This defines the content the release will need; it does not claim those features are implemented today. The published note must describe the behavior actually being released.

## Content

Use the title **Privacy**, a short introduction, and roughly 350–550 words divided into clearly named sections. Preserve these distinctions within the paired layout:

- **Visiting and signing in:** the Schedule Downloader is public. Membership stores an email address and Tool access; a sign-in cookie remembers the session for up to 90 days and renews during use. Signing out ends that session, not membership.
- **Remembered teams and calendar links:** explain what is saved and why. Distinguish the public Session link from the private, replaceable Member link. Anyone who has a usable calendar link can fetch its contents without interactive sign-in; replacing a Member link gives the Member a way to withdraw that link.
- **Connected Google calendars:** distinguish the permission requested from the behavior promised. The agreed permissions allow viewing the list of calendars to choose a destination and viewing and editing events across calendars; The Lobby must change only events it created in the selected calendar. Do not claim Google limits access to that calendar or those events.
- **Choices and leaving:** explain replacing a Member link, disconnecting a calendar, and asking for membership removal. Disconnecting removes upcoming site-added games, keeps played games, and revokes Google access; the agreed shared-project behavior also requires consent again at the next Google sign-in. Removing a Member deletes their access record and Remembered teams. Avoid claiming immediate deletion of every record everywhere.
- **Running the site:** explain necessary service logs, output-button and calendar-client counts, and AWS/Cloudflare involvement. The baseline specifies log-based counts with no analytics service. Avoid equating that with no logging or no provider processing.

These are content requirements, not final publication copy. Avoid invented guarantees about selling data, cookie counts, IP collection, encryption, universal deletion, or provider retention.

## Visual direction and layout

Inherit Local Co-op from [DESIGN.md](../../DESIGN.md): the seafoam frame, navy type and linework, Rubik headings, Nunito Sans prose, and a calm lighter seafoam reading surface. Keep the compact site identity. Use a small decorative robot only if it fits the quiet footer. Scenery stays on the welcome page.

**What we keep / What you control** is the selected layout. The [approved preview](../../.impeccable/mocks/decision/privacy-round1-paired.png) and [prompt record](../../.impeccable/mocks/decision/privacy-round1-paired.png.json) preserve that choice. Preview wording is illustrative first-release copy and must not be copied without the checks below.

A compact masthead and return link lead into Privacy, its introduction, and a brief note that visiting the Schedule Downloader needs no sign-in. Below, paired rows put **What we keep** beside **What you control**. Each row covers one topic: sign-in and membership, Remembered teams and calendar links, then Connected Google calendars. Keep the relevant choices directly beside the corresponding data use. Finish with a short full-width explanation of service providers and logs, and the confirmed contact method.

The focal moment is the connection between information and a person’s choices. This is a readable explanation, not a table of live account settings. The paired structure uses headings and prose, with no disabled controls or implied self-service actions.

The chosen preview establishes reading structure, not every generated detail. Use modest page-heading scale, navy rules, a smooth reading ground, and a footer robot no larger than the existing compact 40px treatment. Keep prose around 60–75 characters per line on larger screens; do not stretch it to fill the frame or copy an oversized heading, footer wave, or generated texture.

The page needs a clear reading hierarchy, a comfortable prose measure, and a standard return link to `/`. Keep all substantive content visible rather than hiding it in accordions, tabs, tooltips, or sign-in states. On phones, turn each pair into a topic section: its data explanation immediately followed by its choices, then the next topic. Do not place all data explanations before all choices or require horizontal scrolling. All content remains visible. Use normal document scrolling; no topic rail is needed for this selected layout.

## Scope, behavior, and constraints

The page is public and identical for Visitors and Members. It must open directly and from both existing footers without checking access or requiring sign-in. The return link lets the existing root route choose the welcome or VIP Lobby. An expired session must not prevent reading Privacy.

Use the existing React/TypeScript/Vite and Go/Chi architecture, with readable HTML available without client-side JavaScript. Reuse the current typography, links, focus treatment, and spacing conventions. No new UI library or service is needed. Use semantic headings and landmarks, visible keyboard focus, readable contrast, useful touch targets, and layouts that survive zoom and long text.

This task does not add privacy settings, a consent banner, deletion forms, account management, Tool interiors, new data collection, or legal compliance claims. It does not change `DESIGN.md`, implement a route, deploy, or publish.

## Evidence and decisions required before publication

Source inspection found Member/session storage and service logging in this repository. Calendar behavior and Remembered teams remain requirements. Live configuration and Tool backends have not been verified in this shaping task.

| Topic | Evidence and remaining decision |
| --- | --- |
| Member and session data | [Store types](../../internal/store/store.go), [session behavior](../../internal/session/session.go). Confirm final release behavior; membership records and sign-in sessions have different lifecycles. |
| Logs and retention | [Request logger](../../internal/app/app.go) and [infrastructure variables](../../infra/modules/site/variables.tf). The source defaults CloudWatch retention to 30 days; verify deployed settings and provider logging before stating a retention period. |
| Google permissions and disconnect | [ADR 0007](../adr/0007-connected-calendar-writes-into-member-chosen-calendar.md). Verify the implemented permission request, event boundaries, token handling, and disconnect effects before publishing promises. |
| Remembered teams, links, and removal | [First-release baseline](../baseline.md). Verify the Tool backend, link replacement, exclusion of link secrets from logs, and deletion behavior. |
| Contact | Craig must choose a public contact method for privacy questions and membership-removal requests. Do not invent an address or imply a self-service deletion screen exists. |
| Document date | Use a real publication or revision date when the note ships, not the preview generation date. |

During implementation, verify the actual `/privacy` HTTP response, both footer links, direct signed-out access, reading without JavaScript, keyboard navigation, mobile layout, and zoom. These are future acceptance checks, not completed tests.
