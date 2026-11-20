---
status: accepted
date: 2026-11-20
---

# Shared Cloudflare authority requires production approval

The dev and production hostnames share one Cloudflare zone and account. The environment roots need DNS and Access permissions that Cloudflare cannot limit to one hostname, and the zone-wide Transform Rule in `infra/bootstrap` contains both origin secrets (ADR 0008). A mutable token available to an automatic dev job could therefore read or change production before its approval.

The mutable Cloudflare token lives only in GitHub's protected `production` environment. One job targets that environment and waits for Craig's approval before any of its repository code runs. After approval it assumes the dev AWS role, applies and checks dev, then assumes the production AWS role and applies and checks production. Both roles trust only the `production` environment's OIDC subject, while their AWS policies and state access remain separate by site environment. Pull-request plans keep their separate read-only Cloudflare token.

The former `dev` GitHub environment is deleted and the mutable token it held is replaced and revoked during the move. A dependency between jobs would not be an equivalent boundary: repository code could add another job targeting `dev` and read any secret left there.

## Considered options

- Give dev a hostname-scoped token. Cloudflare's DNS and account-level Access permissions cannot be constrained to that hostname, and the shared ruleset still contains production's origin secret.
- Put dev in another zone or account. The hostname is part of `craigdevjohnson.com`, and a separate Access account would be a much larger service change.
- Keep automatic dev deployment and approve only a later production job. That leaves production-capable Cloudflare authority in code that runs before the approval.

## Consequences

- A merge still builds once and deploys the same artifact to dev before production, but neither deployment starts until Craig approves the run.
- Rejecting the deployment leaves both environments unchanged. A failed dev apply or smoke check still prevents production.
- The bootstrap root gives both environment-specific AWS deployer roles the same protected GitHub OIDC subject; their resource permissions do not broaden.
- Moving to this design requires an approved bootstrap apply, a replacement token stored only in `production`, deletion of the old `dev` environment and revocation of the old token before the first new deployment is approved.
