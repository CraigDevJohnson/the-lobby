# Changelog

What has changed on the site, newest first. The site has no version numbers: every merge to `main` that changes more than this file deploys, so entries are grouped by the day they merged. Routine dependency updates are left out.

## 2026-11-13

- Make the Minecraft Launcher's board move with what happens: rows glide to their new height instead of jumping, the device underline travels, and the custard control rises when it becomes the thing to press.
- Tidy Minecraft Admin: mark settings changed this visit "From next start", link the outcome to the Launcher, let the outcome be dismissed, explain what On and Off show, and shorten each mod on phones.
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
