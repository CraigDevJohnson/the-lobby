# Changelog

What has changed on the site, newest first. The site has no version numbers: every merge to `main` that changes more than this file deploys, so entries are grouped by the day they merged. Routine dependency updates are left out.

## 2026-11-20

- Change a Member's Google or iCloud calendar connection only while it is still the one the request read. Before, a press of Add or the daily update's switch already on its way could bring back a connection the Member had just disconnected, or change one they had made again. The daily update now reads the connection again before each game it sends to Apple, so it stops for a Member who disconnected, picked another calendar or turned it off meanwhile. Disconnecting Google leaves a connection made while Google was revoking the old one (#131, ADR 0007, ADR 0015).
- Tie each Sign-in session to the invitation it began under, so a session left from before a Member was removed never works again when the address is added back, and have `task member:remove` end the Member's Sign-in sessions as Lobby Admin does. Adding a Member or changing their Tools, from the command or on Lobby Admin, writes nothing when the Member was added, removed or invited again meanwhile, and Lobby Admin no longer counts an earlier invitation's session among a Member's devices. Run `task member:add` and `task member:remove` from a checkout at or after this change, once that environment's Deploy has finished (`infra/README.md`, #128).
- Limit each deployer role's API Gateway writes to its own environment's API and custom domain, so a dev deploy can no longer change production's API Gateway before Craig approves. Neither deployer can create an API or a custom domain any more, so the Deploy workflow refuses a plan that would delete or replace one, before it changes anything, and an administrator makes that change by hand (`infra/README.md`, #125).
- Deny the three pipeline roles the contents of CloudWatch logs (log events, Logs Insights queries and Live Tail), so a pull request's job can no longer read production's logs from AWS. A plan still reads each log group's settings (#126).
- Renew a Sign-in session only while it still exists. Before, a request already on its way when a Member signed out, or was signed out everywhere from Lobby Admin, could write the session back and undo the sign-out (#132).

## 2026-11-19

- Ask Apple whether a moved game is still in a Member's iCloud calendar before replacing it. The first version sent one request with `If-Match: *`, which iCloud does not treat as the standard says: on dev it left a game that was in the calendar unmoved and brought back one the Member had deleted. Found on dev, before production (#105, #123, ADR 0017).
- Move a game in a Member's iCloud calendar when the league moves it, and email them what changed. With the daily update on, a moved game is replaced at its new time or field, anything the Member changed on it in iCloud included, and a game they deleted is left alone; one email names each moved game with its old and new time and field (#105, #121, ADR 0017).
- Record that only `main` may use the `dev` GitHub environment, so a pushed branch or tag can no longer get the dev deployer role or the Cloudflare token that can read production's origin secret. Roll back by revert; deploying an older commit by hand from a tag is gone (`docs/pipeline.md`, #119).

## 2026-11-18

- Correct the iPad steps in the family devices runbook (`docs/family-devices.md`): the kids profile is refused on an iPad that is not supervised, and restoring a backup made on iPadOS 26 or earlier can undo supervising, so the iPad is updated to iPadOS 27 before it is backed up. The steps now follow Apple's current Configurator and deployment guides.

## 2026-11-17

- Give the Plan and Deploy workflows the family devices page's lists and the two DNS over HTTPS endpoints as secrets, not variables. A run's log is public and printed each variable's value; it prints `***` for a secret. `infra/zero-trust/wizard.sh` sets them as secrets too (#117).
- Deny the three pipeline roles secret values, DynamoDB items, other environments' state and input files, and other environments' Lambda settings, so a pull request's job can no longer read production's origin secret or SecureStrings from AWS (#113).
- Add `task infra:inputs:pull` and `task infra:inputs:push`, which copy the OpenTofu roots' gitignored input files from and to the state bucket, so a cloud thread can fetch them before a plan once Craig's computer has uploaded them (ADR 0016, #111).
- The four input files are in the state bucket: a cloud thread uploaded them, pulled them back and planned `bootstrap` and `zero-trust` from them, so neither plan needs Craig's computer (#112).

## 2026-11-16

- Add a button to the iCloud card that puts the Teams on the page into My calendar when it holds none, so the daily update can be turned on there. Before, the switch stayed grey until the Teams were added under My calendar (#103, #108).
- Add a daily update to Add to iCloud Calendar: a Member who turns it on gets the new games of the Teams in My calendar added to their iCloud calendar once a day, and games already there are left as they are (#103, #106, ADR 0015).

## 2026-11-15

- Email a new Member their invite when Lobby Admin adds them: how to sign in, their Tools, the Minecraft worlds' addresses and how to join Foundry, sent through Fastmail from lobby@craigdevjohnson.com. The page says whether it went and still shows the line to copy (#101, ADR 0014).
- Offer the Minecraft allowlist on Lobby Admin to someone who also holds `minecraft-admin`: inviting a Member with Minecraft Launcher can add their Java account name or Bedrock gamertag, and ticking Minecraft Launcher for a Member asks for it. The site keeps no player name, so taking a player off stays on Minecraft Admin (#100).
- Add a Site card to Lobby Admin: whether each Tool's backend is set up and answering, whether Google and iCloud calendars are offered, and the commit the site runs (#98).

## 2026-11-14

- Build Lobby Admin at /lobby/admin for the new `lobby-admin` grant: list every Member with their Tools, invite and remove Members, grant and take away Tools, see when each was last here, and sign a Member out everywhere (ADR 0013, #97).
- Make adding a Member the whole invite: Cloudflare's sign-in emails a code to any address, the site still lets in only Members, and the `ALLOWED_EMAILS_DEV` and `ALLOWED_EMAILS_PROD` lists are no longer used (#97).
- Wait longer for a change to the Minecraft allowlist or a world's operators, which a running world can take a while to confirm, and say so when the wait runs out (#96).
- Add Players, Operators and Backups to Minecraft Admin: change the allowlist, make a player an operator of a world, and list a world's backups and restore one. Each needs the Minecraft backend's matching routes (#95).
- Add to iCloud Calendar: a Member with Let's Plan Soccer can connect iCloud with an app-specific password and add their checked games straight into an iCloud calendar they pick (#93).
- Build Foundry Admin at /foundry/admin for the `foundry-admin` grant and move Start Foundry and Stop Foundry there; the Foundry page keeps the status and the way in, and the site refuses a start or stop from anyone without `foundry-admin` (#84).
- List a mod once on Minecraft Admin when it has a build for each loader, instead of a row per build (#83).
- Remove the on or off filter from Minecraft Admin and keep the search (#85).
- Make the Minecraft Launcher's board move with what happens: rows glide to their new height instead of jumping, the device underline travels, and the custard control rises when it becomes the thing to press (#82).

## 2026-11-13

- Tidy Minecraft Admin: mark settings changed this visit "From next start", link the outcome to the Launcher, let the outcome be dismissed, explain what On and Off show, and shorten each mod on phones (#80).
- Stop a changelog-only merge from starting a deploy.
- Sort Minecraft Admin's mods by name, filter them by on or off, and stop naming defaults (#77).
- Link each mod's name on Minecraft Admin to its own page, and drop the unused "On Bedrock" note (#75).

## 2026-11-12

- Give each Minecraft world its own drawn place that comes alive when the world is ready.
- Group each Minecraft world's address with its own Start and Stop (#71).

## 2026-11-11

- Give The VIP Lobby the Arcade Cabinet design and bring it to life (#69, #70).
- Hold GitHub Actions updates from Dependabot for review (#68).

## 2026-11-10

- Add Minecraft Admin to switch each world's mods (#65).
- Show one card per Minecraft world, each started and stopped on its own (#63).
- Auto-merge Dependabot minor and patch updates once checks pass (#64).

## 2026-11-08

- Give The VIP Lobby the Navy Frame design (#62).

## 2026-11-05

- Add games straight into a Member's Google calendar (#59), with fixes to its setup wizard (#60, #61).
- Show My calendar on /soccer for Members with Let's Plan Soccer, and say so on Privacy (#58).
- Keep Members' Remembered teams and serve their Member link (#57).

## 2026-11-04

- Check every dependency weekly with Dependabot (#54).
- Add issue forms, a pull request template, CODEOWNERS and contributor docs (#51).
- Fix the Zero Trust root's first and second applies, and keep its wizard in the repo (#49, #50).

## 2026-11-03

- Add the family device profiles, the Gateway root and the /admin page (#47).

## 2026-10-27

- Offer Start Foundry and Stop Foundry on the player page (#38).
- Keep calendar link tokens out of the API Gateway access log (#35).
- Record the calendar event title format the soccer backend sets (#37).

## 2026-10-18

- Tell Google Calendar subscribers how to get a reminder before each game (#34).

## 2026-10-16

- Give Soccer and Foundry the shared blocky Tool theme (#31).
- Harden backend answers, end both sign-ins on sign-out, and trim the Privacy page (#27).
- Bring `infra/README.md` in line with the zone-wide move to bootstrap (#33).

## 2026-10-14

- Add the member tasks to `Taskfile.yaml` (#26).
- Connect the site to the deployed Foundry controllers (#25).

## 2026-10-13

- Build the Foundry player page at /foundry, with status, start and stop forwarded to its backend (#22, #24).
- Give the Minecraft Launcher the Blocky look (#21).
- Offer next Session's Teams from an ended Session's link (#20).

## 2026-10-12

- Offer Stop server on the Minecraft Launcher (#19).
- Rename the Schedule Downloader to Let's Plan Soccer (#18).

## 2026-10-11

- Build the Minecraft Launcher at /minecraft and show it in The VIP Lobby (#10, #13).
- Wire the site to its Tool backends, and connect dev and production to soccer and Minecraft (#11, #12, #16, #17).
- Add the production root and move the zone-wide Cloudflare pieces to bootstrap (#14).
- Let the deployers enable API access logging (#15).

## 2026-10-10

- Build the Schedule Downloader at /soccer (#8).

## 2026-10-09

- Build the Privacy page at /privacy (#4).
- Build the public welcome and The VIP Lobby (#2).
