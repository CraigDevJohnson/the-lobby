# Family devices: the hands-on steps

The steps only Craig can do, in order, for the family's devices (#40). The reasoning behind each decision is in #40 (Decisions already made, Facts checked) and in ADR 0009 and ADR 0010; this page does not restate it. It names no person and no address: those live in GitHub's variables and in gitignored `tfvars` files.

## 1. What exists where

| Piece | Where | What it does |
|---|---|---|
| The family Gateway configuration | `infra/zero-trust` (ADR 0009) | The Kids and Adults DNS locations and their policies, device enrollment for the two adults, the Gateway root certificate, the Do Not Inspect policies, and the inspection switch. Applied by hand, twice: everything first, then `inspect_https = true` after step 4. |
| The family devices page | `/admin` on the site (ADR 0010): `https://next.craigdevjohnson.com/admin` until the switch, `https://dev.craigdevjohnson.com/admin` on dev | Lists the two profiles with their install steps and serves the files, behind a Cloudflare Access application for the two adults. Its settings are the Zero Trust root's outputs, carried into the GitHub variables `KIDS_DOH_URL`, `ADULTS_DOH_URL` and `GATEWAY_ROOT_CA_PEM`, and the adults' list into `ADMIN_EMAILS_DEV` and `ADMIN_EMAILS_PROD` (`docs/pipeline.md`). The page is live once a deploy has run with those set. |
| The profiles | `internal/profile`; `site profile kids` and `site profile adult`, or `task profiles` | The same two files the page serves, written on the Mac from the same three settings in the environment. The page's copy is the canonical one. |

Everything in this page happens after the Zero Trust root's first apply and a deploy with the variables set. The second apply (step 5) comes last.

## 2. Before the iPads

Family Sharing is set up so that Ask to Buy covers both iPads and the approval requests arrive on Craig's iPhone. The kids profile leaves the App Store on (`allowAppInstallation` stays true) for exactly this: a new app is a question to a parent, not a locked door.

## 3. Each iPad

1. Confirm the iPad's iCloud backup is current (Settings › the Apple Account › iCloud › iCloud Backup › Back Up Now).
2. On the Mac, open Apple Configurator, connect the iPad, choose Prepare. Supervise devices on. "Allow devices to pair with other computers" off. No MDM server.
3. Set the iPad up again. During Setup Assistant, restore from the iCloud backup. Never restore from a Finder backup afterwards: that can undo supervision.
4. Put the kids profile on. Either open `https://next.craigdevjohnson.com/admin/profiles/kids-ipad.mobileconfig` in Safari on the iPad (one-time code to the adult's address first), then Settings › General › VPN & Device Management › the downloaded profile › Install; or drag `kids-ipad.mobileconfig` onto the device in Configurator. Both land with removal disallowed, because the device is supervised.

Apple's pages on Configurator's Prepare options and the restore caveat could not be re-read when this was written (#44); confirm each on the screen as you go and correct this page where Apple's words differ.

## 4. Each iPhone

1. Install the Cloudflare One Agent from the App Store (the WARP client's current name). Open it, choose the Zero Trust login, team name `craigdevjohnson`, and sign in with the one-time code sent to the adult's address.
2. Open `https://next.craigdevjohnson.com/admin` in Safari, sign in with the one-time code, and tap the adult profile. Then Settings › General › VPN & Device Management › the downloaded profile › Install.
3. Settings › General › About › Certificate Trust Settings: turn on full trust for the Cloudflare Gateway certificate. Inspection fails silently without this step.

A profile installed by hand applies on cellular as well as Wi-Fi, which an MDM-pushed one would not.

## 5. Only after step 4 on both iPhones

The second apply of the Zero Trust root, with `inspect_https = true` in `infra/zero-trust/zero-trust.auto.tfvars`:

```sh
cd ~/Projects/the-lobby
task infra:zero-trust:plan     # the plan changes the Gateway settings and the device settings only
task infra:zero-trust:apply
```

Optionally, later, `lock_client = true` in the same file and another apply: the agent's switch is locked and an enrolled phone cannot leave the organisation ("Lock device client switch" in Cloudflare's words).

## 6. Reinstalling or updating a profile

The identifiers inside each profile are stable, so installing a new copy replaces the old one. On an iPhone that is step 4.2 again. On an iPad it is a Configurator step: the kids profile both disallows its own removal and, through `allowUIConfigurationProfileInstallation = false`, stops profiles being installed by hand on the device, so Safari cannot do it a second time. Connect the iPad to the Mac and add the new file in Configurator.

## Verification checklist

On an iPad:

- `https://1.1.1.1/help` reports DNS over HTTPS in use.
- Settings › General › VPN & Device Management › the profile shows no Remove button.
- A VPN app from the App Store cannot turn on.
- Settings › General › Transfer or Reset iPad › Erase All Content and Settings is greyed out.
- Zero Trust › Insights › Logs › DNS shows the iPad's queries under the Kids location.

On an iPhone:

- With the agent on, Zero Trust › Insights › Logs › HTTP shows full URLs under the adult's identity, and Safari shows the Cloudflare-issued certificate on an inspected site (tap the padlock).
- With the agent off, queries appear under the Adults location. This also confirms the plan's assumption that iOS sets the DNS profile aside while the tunnel is up and uses it again when it is not.

And the deploy's own proof, which the smoke test checks on every deploy: `/admin` and the profile routes redirect to Access without a token, and with one the two headers are right (`Content-Type: application/x-apple-aspen-config`, `Content-Disposition: attachment`).

## What this page leaves out

The option analysis and the MDM comparison are in #40. An app allow-list waits for an MDM; nothing here depends on one.
