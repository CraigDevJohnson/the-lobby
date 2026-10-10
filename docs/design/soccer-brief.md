# Schedule Downloader design brief

Status: confirmed design direction, 2026-10-09. Craig selected **Team builder** after confirming the full Visitor journey and download-versus-Session-link behavior. This is a brief and approved static mockup, not an implementation plan or permission to build, publish, connect a calendar, or deploy.

Target: `/soccer`. Mode: Operate. Visual authority: [DESIGN.md](../../DESIGN.md), the established Local Co-op system. Product authority: [PRODUCT.md](../../PRODUCT.md), [baseline](../baseline.md), and [vocabulary](../../CONTEXT.md).

## Job and outcome

A Visitor, often on a phone, wants their Boise Team's games in the calendar they already use. They may know a Team name, have a Team ID, or only know its division. They should be able to find the right Team, combine it with other Teams, check the games, and understand the next action for Apple Calendar, Google Calendar, or Outlook without signing into The Lobby.

Success is a correct calendar handoff with understandable instructions. A download request or copied link is not proof that the person saved or added it successfully. Show only the result the site can observe: **Download started**, **Session link copied**, or a handoff instruction. Never claim **Your calendar is connected** for a Visitor download or subscription.

The Tool exists because the league offers no calendar export. Include a quiet, visible note: **Unofficial. Not affiliated with Let's Play Soccer.** The compact site identity returns to `/`; the footer links to `/privacy`. No invitation request, membership upsell, or sign-in gate belongs in the Visitor task.

## Selected direction and mockups

Craig confirmed the full Visitor journey on one phone-first page: find and add Teams → review games → choose Apple, Google or Outlook → get a download or Session link, with Member features secondary. He selected **Team builder**, a single scrolling page that carries that sequence, in the local mockup review. Keep Team identity next to its games; make the calendar destination a choice near the handoff, rather than an obstacle before a Team has been found. The [approved mockup](../../.impeccable/mocks/decision/soccer-team-builder-v2.png) fixes that composition, with the qualifications below.

The first phone viewport contains the compact header, a modest **Schedule Downloader** heading, **Your team's games, in your calendar**, **Boise · No sign-in needed**, and a usable Team finder. Avoid a welcome hero that pushes the task below the fold. The focal moment is the review summary becoming a clear, calendar-specific next action.

Three composition proposals keep the same identity and product scope:

| Proposal | What changes | Tradeoff | Mockup |
| --- | --- | --- | --- |
| Team builder — selected | Finder, selected Teams, game review grouped by Team, then delivery on one page; Apple shown | Easy to understand and revisit; long selections need compact summaries | [Phone mockup](../../.impeccable/mocks/decision/soccer-team-builder-v2.png) |
| One section at a time | Completed sections become editable summaries; calendar setup is open; Google shown | Shorter view, but review requires reopening sections | [Phone mockup](../../.impeccable/mocks/decision/soccer-progressive-sections-v2.png) |
| Combined agenda | Games from all selected Teams appear in date order; Outlook shown | Good for checking the combined week, less direct for inspecting one Team | [Phone mockup](../../.impeccable/mocks/decision/soccer-combined-agenda-v2.png) |

The [review-page content](../../.impeccable/mocks/decision/soccer-options.json) and [composition record](../../.impeccable/mocks/decision/soccer-structure-round.json) preserve the alternatives. Each image has an adjacent `.png.json` file containing its exact built-in image-generation prompt and approval status, plus an embedded prompt. All names, divisions, IDs, dates, opponents, times, fields, and counts are fictional design fixtures, not live league data. The differing calendar selections demonstrate help states; every layout supports all three calendars.

The mockups establish composition, not literal production pixels. Follow DESIGN.md for exact colors, typography and component treatment. Correct generated details during a future build: keep a clearly selected calendar option; keep **Sign in** a quiet link; use flat fills and navy rules; remove decorative empty padding; and retain real phone-size text and targets instead of shrinking the images into a 390px screen. The **Sample teams and games** annotation is design evidence, not a production footer link. Add the affiliation note and the manual Apple fallback even where the compressed composition omits them. Selection and Session-link wording follows the confirmed policy below, not accidental implications of a generated image.

## Visitor sequence

1. **Find a Team.** Default to a labelled Team-name input and explicit **Search**. Keep **Use a Team ID** and **Browse divisions** visible beside it. These switch the finder in place, preserve selected Teams, and use the same result presentation. Boise is fixed context, not a selector for unavailable Facilities. Division browsing shows the source's division names; explain once that the league calls them “leagues.” For Team ID, explain **Your Team gets a new ID each Session** and use a numeric keyboard while treating the identifier as text.
2. **Choose the right match.** Show Team name, division, Session and Team ID when supplied by the source. An **Add team** action confirms the choice. Similar or identical names stay separate; never pick a match automatically. A search result is not already selected. Show **Added** for a Team already in the selection, and provide an accessible **Remove [Team name]** control in the selection. **Add another team** returns to the finder with existing choices intact.
3. **Review games.** Select remaining games for the download by default. Label this section **Games in your download**, with **Session links include all games for your chosen Teams, including games posted later** beside the checkboxes. Show date, local start time, opponent and field/venue when known, with **Boise time · 45-minute games** nearby. Use labelled checkboxes, a remaining/all-games view, and a selected-game count. Past games are available but not preselected. Changing the view must not silently change the selection. Removing a Team removes its games from the pending output. Adding another Team must preserve the person's existing choices. Put missing time or venue in words rather than inventing it; unresolved event times must not be exported as guessed times.
4. **Choose the calendar and delivery.** Show text-labelled Apple Calendar, Google Calendar and Outlook choices. No provider is silently picked from the device; the mockups show deliberate choices. Show only that calendar's instructions, while keeping the choices editable. Offer **Session link** and **Download .ics**, explaining the difference before the output action. A suggested Session-link default is a proposal, not an accepted baseline decision. A phone Visitor choosing Google sees its computer requirement before copying or downloading.
5. **Complete the handoff.** Display the chosen Teams and output scope next to the action, then the steps needed in the calendar app. Preserve the selection after failure or cancellation. Copy feedback is announced and leaves instructions visible; if clipboard access fails, show the selectable link with manual-copy guidance. A desktop link to the calendar provider supplements the steps and never claims to complete them. Do not add email-to-self, link shortening, QR services, or device-detection infrastructure for this release.

For an empty start, explain only the immediate finder task; do not display a wall of disabled calendar controls. Once games are available, allow a normal in-page **Get your calendar** jump. On the recommended layout, use headings and document scrolling rather than a forced wizard or modal. A zero-game download is unavailable with a visible reason, **Choose at least one game**; that does not disable the Session link for a valid chosen Team. For a Team whose current Session is confirmed but whose games are not posted, the proposed empty-feed treatment is **No games posted yet; your calendar can pick them up when the league posts them**. Prove that an empty feed can be subscribed to in each client before shipping that treatment.

## Calendar help copy

Put this distinction above the output action:

- **Download .ics:** “Save a one-time copy of your selected games. Imported events won't receive later schedule changes.” Add concise reimport guidance: “Importing again may create duplicates.” Do not promise client deduplication.
- **Session link:** “Subscribe to all games for your chosen Teams in this league Session, including games posted later. Your calendar checks for updates, so changes may take time to appear. Use a new link for the next Session.” Add: “Anyone with this link can use it.” Explain Session as one run of league play on first use. Ending a Session does not imply that calendar events are deleted. The checkbox choices affect only the download.
- **Access:** “No Lobby sign-in needed. Your calendar provider may ask you to sign in.” A public Session link is not a secret Member link or a Connected calendar.

Instructions stay inline in the delivery section. Use numbered steps and explicit button labels, not an unexplained .ics filename or three equally prominent action buttons. Keep device requirements above the button. An optional **More help** disclosure may contain troubleshooting, never the computer requirement or snapshot-versus-subscription distinction.

### Apple Calendar

**Session link on iPhone — intended preferred path, subject to device proof**

Button: **Open in Apple Calendar**. Supporting copy: “Tap below, then confirm the subscription in Apple Calendar.” Opening the prompt does not mean the subscription was added. Keep **Copy Session link** and **Add it manually** available.

Manual steps: “Copy the Session link. In Calendar, open Calendars → Add Calendar → Add Subscription Calendar. Paste the link, then tap Find—or Subscribe on older iOS versions—and finish adding it.” If the automated handoff is not proven, make the copy-and-manual route primary instead of advertising one-tap success.

**Download .ics**

“On a Mac, open Calendar → File → Import, choose your downloaded file, and select a calendar.” For iPhone: “For a file import, Apple supports opening an .ics attachment in Mail. For the simpler phone setup, use the Session link.” Do not promise that a Safari download or Files will show **Add All** until that exact multi-game flow is tested.

Sources checked for this draft: [Apple calendar subscriptions](https://support.apple.com/en-us/102301), [iPhone account and calendar guide](https://support.apple.com/guide/iphone/set-up-mail-contacts-and-calendar-accounts-ipha0d932e96/ios), [Mac calendar import](https://support.apple.com/en-gb/guide/calendar/icl1023/mac).

### Google Calendar

Lead with **Setup needs a computer** for both outputs. “The Google Calendar phone app cannot add a calendar from a link or import this file.” Never present the public Session link as a Google authorization flow or as a phone shortcut around that limit.

**Session link**

1. “On a computer, open Google Calendar.”
2. “Choose + beside Other calendars → From URL.”
3. “Paste the Session link and add the calendar.”
4. “On your phone, make sure that calendar is selected in the app's menu.”

Button: **Copy Session link**. On a phone, say “Keep this link somewhere you can open on your computer.” Clipboard copying by itself does not transfer it to another device. Offer the same selectable-link fallback used elsewhere.

**Download .ics**

“On a computer, open Google Calendar → Settings → Import & Export. Select your downloaded .ics file, choose a calendar, and import it.” Button: **Download .ics**. On a phone, keep the computer requirement visible and explain that the file must be accessible there. Do not imply that downloading imports events into the phone app.

Sources checked for this draft: [Google subscription setup](https://support.google.com/calendar/answer/37100?co=GENIE.Platform%3DDesktop&hl=en), [phone calendar visibility](https://support.google.com/calendar/answer/37100?co=GENIE.Platform%3DAndroid&hl=en), [Google import](https://support.google.com/calendar/answer/37118?hl=en). No numeric Google refresh deadline is established by these instructions.

### Outlook

Lead with **Use Outlook on the web on a computer** as the supported setup recommendation. This is not a claim that every Outlook phone route is impossible.

**Session link**

“Open Calendar → Add calendar → Subscribe from web. Paste the Session link and finish adding it.” Button: **Copy Session link**. Add: “Schedule changes can take more than 24 hours to appear.” Do not substitute a direct Windows Outlook handoff for this documented web route.

**Download .ics**

“In Outlook on the web, open Calendar → Add calendar → Upload from file. Choose your downloaded .ics file, select a calendar, and import it.” Button: **Download .ics**. Repeat that this is a one-time copy.

Source checked for this draft: [Microsoft import and subscription instructions](https://support.microsoft.com/en-us/outlook/import-or-subscribe-to-a-calendar-in-outlook-com-or-outlook-on-the-web). Google Calendar on Android is one client; these instructions do not establish behavior for every Android calendar app.

## Layout, access and interaction

Phone first: design the task at 390px and check 320px through 430px. Use the compact shared header and footer, an approximately 17px body size, labelled controls, and at least 44px touch targets. Titles may wrap. Keep the finder's submission action below the input when needed; long Team names wrap rather than truncate the distinguishing information. Calendar choices may stack. Avoid a horizontal schedule table, clipped tabs, sticky overlays above the keyboard, and bottom bars that cover controls or device safe areas.

At tablet widths, retain the same reading order. On a wide screen, the recommended layout can place the finder and game review in a main column with a narrower calendar-setup area to its right once results exist; do not leave an empty side panel on first arrival. Keep the semantic order find → review → deliver, a bounded content width, and short instruction lines. No desktop-only task or data is hidden on phones.

Use seafoam for the field, pale mint for work areas, and navy for text, rules, icons and focus. Rubik carries headings and actions; Nunito Sans carries help and game details. Only the current primary action gets the custard face and coral base. Cards and notices stay flat. Scenery belongs to the public welcome, not this Tool; the existing tiny decorative footer robot is optional. Preserve DESIGN.md rather than redefining its tokens.

Use semantic headings, fieldsets and legends for delivery choices, native buttons and checkboxes, visible focus, full-row game labels, and programmatically named remove controls. Announce results, selection changes, copy success and errors without moving focus unexpectedly. Give each input a persistent label and associate errors with it. Preserve information at 200% zoom and reflow at 400%; validate contrast and keyboard operation in an actual build. Motion only explains a change; respect reduced motion. No raster mockup is evidence that these checks pass.

## States and content ranges

Design fixtures exercise zero, one, a few and many results; one, two and several selected Teams; empty, partial and long schedules; long names; and repeated names across divisions or Sessions. These are coverage cases, not invented league limits or product caps. Keep source filters and bounded result pages manageable without hiding selected Teams.

| State | Visible behavior and recovery |
| --- | --- |
| First visit | Useful finder; Boise context and no-sign-in promise; no Membership prerequisite |
| Invalid Team ID | Inline “Enter a Team ID using numbers”; keep input; do not send an unusable lookup |
| Searching / loading schedule | Named progress near the affected area; preserve selected Teams and layout; prevent duplicate submission |
| No match | “No teams found”; keep query and offer Team ID or division browsing |
| Ambiguous names | Show distinguishing source details; require the Visitor to choose |
| Team already added | Say “Added”; never add the same Team twice |
| Schedule not yet published | “The league hasn't posted games for this Team yet”; do not confuse this with an outage or promise a publication date |
| Partial schedule | Say more games may be posted; show a verified update time only if actually available |
| No remaining games | Explain the Session may be finished; offer all games or another Team |
| Nothing checked | Disable the download action with “Choose at least one game”; keep checkboxes and controls available |
| One Team fails to load | Keep other Teams, identify the failed Team, and require retry or explicit removal before an output that could look complete |
| League unavailable | Plain message, Retry and a verified link to Let's Play Soccer; no alternate scraping or invented cached-success claim |
| Link generation / download failure | Keep selection and chosen calendar; named error plus retry; no success claim |
| Copy denied | Reveal a selectable URL with manual-copy instructions |
| App handoff cancelled | Keep the page and selection usable; leave manual steps available |
| Sign-in expired or identity check failed | Public lookup and output remain available; private Member content must not leak |

## Member boundary

Visitor work leads this brief. A signed-in Member without the Schedule Downloader grant gets the same public capabilities; do not display locked or upgradeable Member features. Granted Members may later receive Remembered teams, their replaceable secret Member link, and a Connected Google calendar as a secondary, separately shaped area. Avoid the term “sync” in UI copy. The baseline's once-a-day updates and event-preservation promises belong to the Connected calendar, not public subscriptions.

Do not design a Google-connect button into this Visitor mockup or imply that arbitrary Visitors can request access. Detailed Remembered-team resolution, destination selection, disconnect, calendar switching and revoked-access screens need their own Member-flow brief preserving [ADR 0007](../adr/0007-connected-calendar-writes-into-member-chosen-calendar.md).

## Decisions and proof before implementation or release

**Design selection confirmed.** Craig selected **Team builder** in review round `28df2e8e`; its `soccer-team-builder-v2.png` sidecar records approval. The other two compositions are unapproved alternatives. No implementation contract was written in this shape-only task.

**Session-link selection policy confirmed by Craig, 2026-10-09.** The download contains exactly the checked games. The Session link follows all games for the chosen Teams during the current Session, including games published later. Individual checkbox exclusions do not change a subscribed calendar. Label checkboxes **Games in your download** and show **2 Teams · this Session** for a Session link instead of a fixed selected-game count. Do not infer that links span Sessions or that changing the browser selection edits an already-issued link. Link creation before games are published still needs the empty-feed proof described above.

The baseline also leaves cache age, migration duplicates, Google calendar-list refusal, and missing Member calendars unresolved. This design does not approve those assumptions. Calendar event title/description wording remains deferred; game-row sample copy is not a new event format contract.

Before describing the experience as ready:

- Prove Apple, Google and Outlook can fetch a public Session link through Cloudflare without interactive sign-in. Keep link secrets out of logs where applicable.
- Test the actual iPhone subscription prompt, manual fallback, and multi-game file behavior; test the actual Android file outcome. Official documentation is guidance, not real-client proof for this site.
- Verify 45-minute duration, local Boise start times and daylight-saving transitions; the baseline warns that league-local times are labelled UTC. Verify multiple-Team output, duplicate games when both participating Teams are selected, and stable event identity across updates before promising deduplication or continuity.
- Verify no-match versus unavailable-service behavior, partial schedules, game selection, output retry, and clipboard failure at the implemented UI boundary.
- Validate the phone and desktop layout, long content, keyboard, screen-reader feedback and zoom once semantic UI exists.
- Keep Member Google permission classification and consent/audience limits as separate proof gates before presenting Connected calendars as ready.

## Current evidence and scope

Inspected checkout: `f429b36`, detached isolated worktree `a801/the-lobby`, clean before this design work. The site contains Welcome, VIP Lobby and Privacy screens; `web/src/tools.ts` links to `/soccer`, but the Go route registration does not implement `/soccer`. PRODUCT.md's earlier statement that no React interface exists is stale; source inspection establishes the current scope without repairing that document as a side effect.

Deliverables are this brief, the selected Team builder mockup, two alternate revised phone mockups (`-v2`), their prompt records and a local composition review page. The original three images are retained as the preceding proposal, not the current selection set. Exact prompts and the revision relationship are in the adjacent JSON files; images were made with built-in image generation. Local references, JSON validity and embedded prompt provenance were checked. A separate read-only brief review found one wording issue, corrected to **Download started**. The calendar guidance was checked against the official sources cited above; no account was accessed.

No application source, product baseline, design system, authentication, backend, infrastructure or live calendar was changed. No tests of the proposed route or real calendar accounts have run because this request is design-only. The app remains React/TypeScript/Vite served by Go/Chi; the Tool backend remains a separate repository and data boundary. Do not turn a layout approval into implementation or publication authorization.
