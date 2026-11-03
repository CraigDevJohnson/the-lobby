---
status: accepted
date: 2026-11-03
---

# The family devices page is gated by Access alone

The family device profiles (#40) are served at `/admin`: a page listing the two profiles with their install steps, and the files themselves. The path sits behind a second Cloudflare Access application, for the two adults only, with its own list (`admin_emails`), and the server checks that application's token on every request. No Sign-in session, no Member record and no Tool grant are involved.

ADR 0004 says only sign-in paths sit behind Access and everything else behind the site's own Sign-in session. This page is the exception. It is a two-person utility page: it is not a Tool, it is not linked from navigation, and the people who use it do so from a phone in Safari to install a profile, where Access's emailed code is the whole sign-in they need.

## Considered options

- A Member grant (`devices`) behind the Sign-in session, like the Tools. It would put the adults into the Member model for a page that has nothing to do with Tools, add a grant that means "is one of the two adults", and make the files reachable through a 90-day session cookie rather than Access's own 24-hour sign-in.
- The files under `/signin`, so the existing application covers them. The sign-in application's list is the Member invite list; the two adults are a different set, and a Member would be able to fetch the profiles.
- Serving the profiles from a bucket or by hand. The profiles are rendered from the Zero Trust root's outputs and change when it does; a page that renders them from the server's settings is always current, and the server never serves one with a placeholder in it.

## Consequences

- `modules/site` creates the second application, on `<hostname>/admin`, which Cloudflare applies to that path and every path under it, so the files at `/admin/profiles/...` are covered by the same entry. It exists only while `admin_emails` names someone; until then the server refuses the page, so a deploy before Craig sets the variable still goes through.
- The server checks the token against the application's audience (`ACCESS_ADMIN_AUD`) with a second verifier, so a misconfigured or removed application still leaves the routes closed; the origin is closed to anything that did not come through Cloudflare already (ADR 0005). On a developer's computer, where Access is not configured, the routes serve without the check, with the same kind of warning sign-in logs.
- The adults' addresses and the three profile settings are personal data or travel with the Zero Trust root (ADR 0009), so they are GitHub variables and `secrets.auto.tfvars` entries, never committed, like the Member invite list.
- Someone signed in to Access for `/signin` may still be asked for a code at `/admin`: the two applications are separate, and that is Access's business.
- `aws-setup`'s `aws-design.md` lists the site's Access application; the second one belongs there too (that repository, separate change).
