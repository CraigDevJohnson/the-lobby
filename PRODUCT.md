# The Lobby

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

The primary audience is people using the Tools: Craig, family, friends, and public Visitors using the Schedule Downloader. They come to complete a task or reach something they use. Technical readers exploring how the Tools are built are a secondary audience. Craig confirmed this priority on 2026-10-07.

A Visitor is not signed in. A Member is someone Craig has invited to at least one Tool, with access granted per Tool. Craig is the Owner and the only person who invites Members and grants access.

## Product Purpose

The Lobby is a shared place to check schedules, reach useful Tools, and get into games. It brings the Tools Craig builds for family and friends into one site at `craigdevjohnson.com`.

The public Landing page is a general welcome to The Lobby. Signing in changes the experience substantially to a Member's own selection of Tools. A person sees only Tools they have access to. Craig confirmed this direction on 2026-10-07, replacing the earlier public Tool catalog. Visitors cannot request access for now. The Schedule Downloader remains publicly usable at `/soccer` without signing in.

The planned inventory is Schedule Downloader, Minecraft Launcher, Minecraft Admin, Foundry, and Foundry Admin. These names do not establish their release order or imply that all five ship together. Detailed Minecraft and Foundry requirements remain open.

The VIP Lobby always includes Schedule Downloader for every Member, even without access to its extra Member features. Granted Minecraft Admin and Foundry Admin entries belong in a separate Admin Tools section. Craig confirmed these collection rules on 2026-10-08.

The Schedule Downloader's agreed purpose is to put a Team's games into a person's calendar. Members with Schedule Downloader access can keep following Remembered teams across Sessions without repeatedly finding new Team IDs.

## Positioning

The Lobby centers the people using it. Its Tools address needs Craig and the people around him actually have. The Schedule Downloader exists because the league offers no calendar export. It is unofficial and not affiliated with Let's Play Soccer.

The public Landing page welcomes people to the shared place. The signed-in experience helps each Member reach the Tools available to them. It does not advertise inaccessible Tools as locked or upgradeable choices. The location of technical explanations, diagrams, code links, and Craig's bio from the earlier catalog plan remains open.

## Operating Context

The Schedule Downloader starts with Boise and is designed for phones first. People find a Team by Team ID, name search, or browsing divisions at its Facility. Several Teams can share one calendar output. Remaining games are selected by default, with 45-minute events.

Calendar delivery must account for Apple Calendar, Google Calendar, and Outlook. The first-release baseline specifies instructions for each, including when a computer is required. Real-client behavior still needs the proof listed in the baseline; the instructions must not imply that those checks have already passed.

The league assigns new Team IDs each Session. A Remembered team follows an identical name only when exactly one current Team matches. Otherwise the Member chooses a Team or supplies its Team ID. The site never guesses.

## Capabilities and Constraints

### Existing Tool and sign-in requirements

The requirements below come from the earlier first-release baseline. The 2026-10-07 welcome-page direction supersedes its public catalog. Craig confirmed that the Schedule Downloader's web interface remains public at `/soccer`; calendar subscription links also work without interactive sign-in.

- A public Landing page and Visitor Schedule Downloader, with a calendar download or a Session link. Anyone with a Session link can use it for that Session.
- Members with Schedule Downloader access get Remembered teams, a secret replaceable Member link that follows them across Sessions, and a Connected calendar in a Google calendar they choose. The main calendar is preselected; updates run once a day with games and scores.
- The Connected calendar changes only events the site added. A game the Member deletes stays deleted. League changes update title, time, place, and score while preserving the Member's notes, reminders, and colour. Scores belong in the title.
- Disconnecting, switching calendars, or dropping a Remembered team removes upcoming games the site added. A switch adds them to the new calendar; played games stay. Preserve the disconnect and Google consent behavior recorded in the baseline and ADR 0007.
- A one-day calendar notice tells a Member when a Team cannot be followed. A league outage gets a plain message and a link to the league, with an email alert to Craig and no automatic workarounds.
- Sign-in is invite-only, by emailed code or Google, with a 90-day Sign-in session renewed on visits. Access is per Tool. The first release uses Owner commands instead of an Owner screen.
- A short privacy note explains the relevant data use. Removing a Member deletes their access record and Remembered teams. Output-button and calendar-client counts use log lines, with no analytics service; Member-link secrets must stay out of logs.

These are agreed requirements, not a claim that all these capabilities are implemented. Detailed behavior remains in [the first-release baseline](docs/baseline.md) and [accepted decisions](docs/adr/).

### Technical and release constraints

One site owns the screens and sign-in. The accepted interface is React and TypeScript built by Vite, served by Go/Chi, with Landing page HTML generated at build time. Each Tool has a separate Go backend, repository, and data. The server enforces access on Member actions.

The agreed hosting uses AWS Lambda and DynamoDB behind Cloudflare and API Gateway. Keep operations small and affordable, with low Lambda concurrency caps and the site's planned $5 monthly cost alert. Code is public under the MIT licence and is not taking outside contributions.

This is a new implementation. The domain, `/soccer` address, and two existing Google projects carry over. Preserve `/soccer` links. Production cutover and retirement of the old site remain separate release decisions.

### Deliberately open or deferred

The baseline's unruled assumptions remain open, including six-hour league-data reuse, duplicate calendar games during migration, behavior when Google calendar-list access is refused, and notice delivery when a chosen calendar disappears. Do not treat them as approved behavior.

Calendar-client fetching through Cloudflare, Android file handling, and Google's permission classification remain proof gates. The Google consent and audience-limit assumptions in the baseline require confirmation before the Connected calendar is presented as ready.

Calendar event wording and additional email/notification design are deferred. The explicitly agreed sign-in codes, outage alert, and calendar notices remain in scope. An Owner screen, other Facilities, league-account linking, per-game Google buttons, and a direct Outlook connection are deferred. The named Minecraft and Foundry Tools await requirements discovery and release planning.

## Brand Commitments

The name is The Lobby. The approved shared-place framing is "The Lobby centers the people using it," with "Meet me in The Lobby" as a spoken invitation. Copy should welcome the people using the Tools and explain what they can do. "Welcome to The Lobby" is the public page's framing. "The VIP Lobby" is the confirmed name for the signed-in experience, recorded in the [landing-page brief](docs/design/landing-page-brief.md) on 2026-10-07.

[CONTEXT.md](CONTEXT.md) owns the vocabulary: Landing page, Tool, Visitor, Member, Owner, Sign-in session, and Connected calendar. A Soccer Session is one run of league play and is distinct from a Sign-in session. Preserve the meanings and avoided terms documented there.

## Evidence on Hand

- [docs/baseline.md](docs/baseline.md) records the first-release scope confirmed on 2026-10-05, its assumptions, deferred work, and historical dev proofs from 2026-10-06.
- [docs/adr/](docs/adr/) records the accepted architecture and Connected calendar decisions.
- [internal/web/web.go](internal/web/web.go) currently implements the trial placeholder, health endpoint, identity endpoint, sign-in, and sign-out. The React interface and Schedule Downloader screens are not present in this checkout as inspected on 2026-10-07.
- The repository contains no committed finished UI captures or brand artwork. Product usage metrics, testimonials, and a completed production release are not established by these sources; do not invent them.

## Product Principles

1. Put the person using a Tool first. Keep the technical explanation available to readers who want it.
2. Make calendar continuity dependable. Resolve ambiguous Teams with the Member and preserve their calendar edits.
3. Make the public welcome inviting and the signed-in selection relevant. Show each person only the Tools they have access to.
4. State limitations and failures plainly. Distinguish agreed behavior, implemented behavior, and behavior proven with real clients.
5. Keep the site affordable for Craig to operate and maintain. Add infrastructure or services only for a demonstrated need.
