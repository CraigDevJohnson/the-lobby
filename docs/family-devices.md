# Family devices: the hands-on steps

The steps only Craig can do, in order, for the family's devices (#40). The reasoning behind each decision is in #40 (Decisions already made, Facts checked) and in ADR 0009 and ADR 0010; this page does not restate it. It names no person and no address: those live in GitHub's secrets and in gitignored `tfvars` files.

## 1. What exists where

| Piece | Where | What it does |
|---|---|---|
| The family Gateway configuration | `infra/zero-trust` (ADR 0009) | The Kids and Adults DNS locations and their policies, device enrollment for the two adults, the Gateway root certificate, the Do Not Inspect policies, and the inspection switch. Applied by hand, twice: everything first, then `inspect_https = true` after step 4. |
| The family devices page | `/admin` on the site (ADR 0010): `https://next.craigdevjohnson.com/admin` until the switch, `https://dev.craigdevjohnson.com/admin` on dev | Lists the two profiles with their install steps and serves the files, behind a Cloudflare Access application for the two adults. Its settings are the Zero Trust root's outputs, carried into GitHub as `KIDS_DOH_URL`, `ADULTS_DOH_URL` and `GATEWAY_ROOT_CA_PEM`, with the adults' list as `ADMIN_EMAILS_DEV` and `ADMIN_EMAILS_PROD` (secrets, apart from the certificate; `docs/pipeline.md`). The page is live once a deploy has run with those set. |
| The profiles | `internal/profile`; `site profile kids` and `site profile adult`, or `task profiles` | The same two files the page serves, written on the Mac from the same three settings in the environment. The page's copy is the canonical one. |

Everything in this page happens after the Zero Trust root's first apply and a deploy with those set; `infra/zero-trust/wizard.sh`, run on the Mac from a checkout of main, does both. The second apply (step 5) comes last.

## 2. Before the iPads

Family Sharing is set up so that Ask to Buy covers both iPads and the approval requests arrive on Craig's iPhone. The kids profile leaves the App Store on (`allowAppInstallation` stays true) for exactly this: a new app is a question to a parent, not a locked door.

## 3. Each iPad

The kids profile goes onto a supervised iPad only. On one that is not supervised, Safari refuses the download with "Profile Error: The field "ProhibitDisablement" contains an invalid value". Supervising erases the iPad once, and it needs a cable and Apple Configurator on the Mac (version 2.21 needs macOS 15.7 or later).

The order matters. Up to iPadOS 26 a backup can carry whether the iPad was supervised, so restoring the iPad's own backup, from iCloud as well as from the Mac, can bring the unsupervised state back. From iPadOS 27, Apple says, a backup no longer holds the supervision status. So the iPad is updated before it is backed up.

1. Update the iPad to iPadOS 27 (Settings › General › Software Update). If it cannot run 27, set it up as new in step 3.5 instead of restoring.
2. Once it is on 27, back it up: Settings › the Apple Account › iCloud › iCloud Backup › Back Up Now.
3. Erase it: Settings › General › Transfer or Reset iPad › Erase All Content and Settings, with the iPad's passcode and its Apple Account password. Leave it on "Hello" and connect it to the Mac.
4. In Apple Configurator, select the iPad and choose Prepare. Manual Configuration, with Supervise devices on and "Allow devices to pair with other computers" off. "Do not enroll in Device Management". Skip the Managed Apple Account sign-in. A new organization with only its name filled in, and "Generate a new supervision identity"; the second iPad takes the same organization. Skip no Setup Assistant panes, so the restore choice still appears.
5. Set the iPad up again: "Set Up Without Another Device", then "From iCloud Backup" on the Transfer Your Apps & Data screen, and the backup from step 3.2. Never restore from a backup on the Mac: Configurator's restore leaves a supervised device unsupervised, and the Finder's can.
6. Check before going on. The main Settings page says "This iPad is supervised" under the Search field; without that line the profile is refused again. Find My and iCloud Backup are on, because the profile locks the account's settings.
7. Put the kids profile on. On the Mac, download `kids-ipad.mobileconfig` from `https://next.craigdevjohnson.com/admin` (one-time code to the adult's address first), then in Configurator select the iPad and choose Add › Profiles; it goes on with nothing to tap on the iPad. Or open the page's link in Safari on the iPad, then Settings › General › VPN & Device Management › the downloaded profile › Install. Both land with removal disallowed, because the device is supervised.
8. In Configurator, Settings › Organizations › the organization › the Gear button › Export Organization. Keep the file and its password somewhere other than the Mac: with pairing to other computers off, another Mac needs this organization before it can manage these iPads.

A supervised iPad is not protected by Activation Lock unless a device management service allows it, and none does here.

The steps were checked on 2026-11-18 against Apple's Configurator guide for version 2.21 and its Platform Deployment guide ("Back up managed Apple devices", "About Apple device supervision", "Activation Lock"). Both iPads were supervised and given the profile that day.

## 4. Each iPhone

1. Install the Cloudflare One Agent from the App Store (the WARP client's current name). Open it, choose the Zero Trust login, team name `craigdevjohnson`, and sign in with the one-time code sent to the adult's address.
2. Open `https://next.craigdevjohnson.com/admin` in Safari, sign in with the one-time code, and tap the adult profile. Then Settings › General › VPN & Device Management › the downloaded profile › Install.
3. Settings › General › About › Certificate Trust Settings: turn on full trust for the Cloudflare Gateway certificate. Inspection fails silently without this step.

A profile installed by hand applies on cellular as well as Wi-Fi, which an MDM-pushed one would not.

## 5. Only after step 4 on both iPhones

The second apply of the Zero Trust root, with `inspect_https = true` in `infra/zero-trust/zero-trust.auto.tfvars`. The wizard sets it, plans, asks, applies and reads the result back:

```sh
infra/zero-trust/wizard.sh inspect
```

By hand it is the same as setting the line and running `task infra:zero-trust:plan` (the plan changes the Gateway settings and the device settings only) and `task infra:zero-trust:apply`.

Optionally, later, `lock_client = true` in the same file and another apply (`infra/zero-trust/wizard.sh lock`): the agent's switch is locked and an enrolled phone cannot leave the organisation ("Lock device client switch" in Cloudflare's words).

## 6. Reinstalling or updating a profile

The identifiers inside each profile are stable, so installing a new copy replaces the old one. On an iPhone that is step 4.2 again. On an iPad it is a Configurator step: the kids profile both disallows its own removal and, through `allowUIConfigurationProfileInstallation = false`, stops profiles being installed by hand on the device, so Safari cannot do it a second time. Connect the iPad to the Mac and add the new file in Configurator.

Erasing an iPad removes supervision as well, so section 3 starts again. With the profile on, the iPad cannot be erased from its own Settings; Configurator does it (Actions › Advanced › Erase All Content and Settings).

## Verification checklist

On an iPad:

- `https://1.1.1.1/help` reports DNS over HTTPS in use.
- Settings shows "This iPad is supervised" under the Search field.
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
