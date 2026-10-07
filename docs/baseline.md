# Baseline

Confirmed by Craig on 2026-10-05, at the end of the design discussion. This is the shared understanding that design and build start from. Terms are defined in [CONTEXT.md](../CONTEXT.md); the decisions that earned a record are in [docs/adr/](adr/); the infrastructure sketch is in [docs/infrastructure.md](infrastructure.md).

## Landing-page amendment, 2026-10-07

Craig replaced the public per-Tool catalog below with a general "Welcome to The Lobby" page. Signing in changes the page substantially to a Member's own selection, showing only Tools they have access to. "The VIP Lobby" is the proposed name for that experience.

The planned inventory is Schedule Downloader, Minecraft Launcher, Minecraft Admin, Foundry, and Foundry Admin. Their release order and the detailed Minecraft and Foundry workflows remain open. The older catalog's diagrams, code links, and bio have no newly agreed location. Craig confirmed that `/soccer` remains publicly usable without sign-in and that Visitors cannot request access for now. Calendar subscription links also work without interactive sign-in. [PRODUCT.md](../PRODUCT.md) carries the current product direction. The sections below retain the original baseline and its evidence.

## What it is

- A personal site fronting Tools Craig builds for family and friends, on `craigdevjohnson.com`.
- Readers: Craig, the people who use each Tool, and a technical stranger looking at how it is built.
- Entirely new: new repos, design and code. Only the domain, the `/soccer` address and the two existing Google projects carry over from the previous site.

## First release, shipped together, then the switch

### Landing page

- Public. An entry per Tool: what it is and how it is built, with a diagram and a link to the code. Two sentences about Craig and a GitHub link.
- The Schedule Downloader entry says why it exists (the league offers no way to get a schedule into a calendar) and that it is unofficial and not affiliated with Let's Play Soccer.

### Schedule Downloader for Visitors

- Boise only, designed for phones first.
- Find a Team by Team ID, by name search, or by browsing the divisions at the Facility (which Let's Play Soccer calls leagues).
- Several Teams combine into one calendar, remaining games selected by default, 45-minute events.
- Output is a download or a Session link, with steps per calendar app: Apple Calendar is one tap on the link; Outlook pastes the link into Outlook on the web; Google Calendar is told plainly that it needs a computer.

### Schedule Downloader for Members

- Remembered teams, re-found each Session only when exactly one current Team has the identical name; otherwise the Member picks or pastes a Team ID. The site never guesses.
- A secret, replaceable Member link that follows the Member's Remembered teams from Session to Session.
- A Connected calendar: a Google calendar the Member picks when connecting, main one preselected, kept current once a day with games and scores. The site touches only events it added. A deleted game stays deleted. The site rewrites title, time, place and score when the league changes them and leaves notes, reminders and colour alone, so scores live in the title.
- Disconnecting, switching calendar or dropping a Remembered team removes the upcoming games the site added (re-added on a switch); played games stay. Disconnecting also gives up access at Google, which cancels the Member's Google sign-in consent too; they consent again at their next Google sign-in.
- A one-day notice in the calendar, through the Member link or the Connected calendar, when a Team cannot be followed.

### Sign-in

- Invite-only, by emailed code or Google, with a 90-day Sign-in session renewed on each visit.
- Craig adds Members in two steps: the Access invite list, kept in OpenTofu, then a command in the site that grants Tool access. There is no Owner screen in the first release.

### Also

- A short plain-language privacy note linked from the footer. Removing a Member deletes their Remembered teams and access record.
- A plain message, with a link to the league's site, when the league's schedule service is not answering. Craig gets an email alert. No automatic workarounds.
- Counts of which output button people press and which calendar app fetches each link, kept as log lines and queried when wanted. No analytics service. The Member link's secret stays out of the logs. Mentioned in the privacy note.

## How it is built

- One site owns every screen and sign-in: a React app served by a Go/Chi server. Each Tool has its own Go/Chi backend in its own repo with its own data (ADR 0001, 0003).
- Everything runs on AWS Lambda in Oregon, in the AWS account Craig already uses for his other workloads (ADR 0002), with DynamoDB for data.
- Cloudflare sits in front and proves identity through Access, with Cognito as the fallback. API Gateway connects Cloudflare to the site; the site calls Tool backends with signed requests (ADR 0004, 0005, 0006).
- The calendar link addresses sit outside Access. Browser Integrity Check is off for them and Bot Fight Mode is off for the domain, all set in OpenTofu.
- Google projects: the two that exist, split by environment (dev and production), each serving both sign-in and calendars. The production one is published "In production", unverified. Members see Google's one-time warning when connecting a calendar; the project carries a lifetime cap of 100 people (ADR 0007).
- The landing page's HTML is generated at build time.

## How it is run

- Public repos, MIT licence, no outside contributions. Pull requests from other people's copies only ever run tests.
- OpenTofu and GitHub Actions. Each program is built once; the same build goes to `dev.craigdevjohnson.com`, then to production on Craig's approval.
- Pull-request checks use a read-only AWS role; deploys use a separate role per environment. No AWS keys are stored in GitHub. One Cloudflare token is, because Cloudflare offers no keyless option.
- OpenTofu state sits in a private bucket that only the pipeline's roles can read.
- Cost controls: a $5 monthly alert on this site's tagged resources, and a low concurrency cap on every Lambda.

## Switching over

- New dev takes over `dev.craigdevjohnson.com` straight away. Production is built at `next.craigdevjohnson.com` and takes the main name once everything works. `/soccer` stays, so existing links keep working.
- The old site is removed about two weeks after the switch, when Craig says so.
- Target: as soon as possible. The old site serves until then.

## Proven

- 2026-10-06, on `dev.craigdevjohnson.com`: Cloudflare Access hands a signed-in person to the site's own Sign-in session. Craig signed in by emailed code; the site verified Access's token, found his Member record and issued a 90-day session; `/api/me` then reported him a Member.
- 2026-10-06: Cloudflare proxies to API Gateway, and the secret header works: the raw API Gateway address answers 403.

## To prove before building on them

- Google, Apple and Outlook.com each fetching a link through Cloudflare.
- What an Android phone does with the downloaded file.
- How Google's console classes the calendar permission (expect "sensitive").

## Assumptions Craig has seen but not ruled on

- The site's repo is `the-lobby` (Craig created it); the Soccer backend's repo is assumed to be `soccer`.
- Members' Google access is stored encrypted, with the key in AWS's free parameter store rather than a paid managed key.
- Links reuse league data for up to six hours.
- A Member who used the previous site's "Add" sees this Session's remaining games twice in that calendar until they delete the old ones.
- A Member who grants event access but refuses the calendar list gets their main calendar.
- If a Member's picked calendar disappears, there is nowhere to put the notice; they learn of it on their next visit.

## Left for later on purpose

Visual design, calendar event wording, emails and notifications, an Owner screen, other Facilities, the league-account link, per-game Google buttons, a direct Outlook connection, and Minecraft, which gets its own round of questions.

## Facts this rests on

Checked against the sources on 2026-10-05. Nothing was tested against a real account.

- The league posts the first game of a Session about a week before it starts and the rest the day after that first game (Craig). Team names carry over between Sessions under new Team IDs; nothing in the data links the old ID to the new one. Name search works per Facility. Times are local but labelled UTC. Boise game slots are 45 minutes. The league offers no calendar export.
- A Google project in "Testing" status issues calendar access that expires after seven days; an unverified project "In production" shows the "This app isn't verified" screen for sensitive permissions and is capped at 100 people for its lifetime. https://support.google.com/cloud/answer/15549945
- Revoking one Google token removes every permission that person granted to the whole Google project. https://developers.google.com/identity/protocols/oauth2/web-server
- Google's Calendar API is free within quotas of 600 requests a minute per person and 10,000 a minute per project. Google has said that use beyond the quotas is planned to be charged later in 2026. https://developers.google.com/workspace/calendar/api/guides/quota
- Google Calendar's phone apps cannot subscribe to a calendar address or import a file; both need a computer. https://support.google.com/calendar/answer/37100 and https://support.google.com/calendar/answer/37118
- Tapping a link to a calendar subscribes in Apple Calendar on iPhone (https://support.apple.com/guide/iphone/iph3d1110d4/ios), and a subscription stored in iCloud appears on all the person's devices (https://support.apple.com/en-us/102301).
- Outlook.com and Outlook on the web subscribe by pasting the address (Add calendar, Subscribe from web) and refresh every few hours, sometimes more than a day. https://support.microsoft.com/en-us/office/import-or-subscribe-to-a-calendar-in-outlook-com-or-outlook-on-the-web-cff1429c-5af6-41ec-a5b4-74f2c278e98c
- Microsoft documents that the Windows Outlook app can fail silently, with no error, against some servers, naming AWS, and points people to Outlook on the web. https://learn.microsoft.com/en-us/outlook/troubleshoot/calendaring/cannot-add-an-internet-calendar
- On Cloudflare's free plan, Bot Fight Mode covers the whole domain and cannot be skipped for a path or a user agent; Browser Integrity Check is on by default and can be switched off per path with a Configuration Rule. https://developers.cloudflare.com/bots/get-started/bot-fight-mode/ and https://developers.cloudflare.com/waf/tools/browser-integrity-check/
- A client that cannot sign in gets a redirect from Cloudflare Access (https://developers.cloudflare.com/cloudflare-one/access-controls/access-settings/session-management/); a path can be left open with a Bypass policy (https://developers.cloudflare.com/cloudflare-one/access-controls/policies/).
- CloudWatch's free allowance covers 5 GB of logs a month and 10 custom metrics; at a few hundred requests a day, counting costs nothing. https://aws.amazon.com/cloudwatch/pricing/
