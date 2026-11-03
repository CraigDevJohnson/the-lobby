---
status: accepted
date: 2026-11-03
---

# The family Gateway configuration has its own OpenTofu root

The family's devices (#40) are filtered by Cloudflare Gateway: two DNS locations with their policies, device enrollment, the Gateway root certificate, the Do Not Inspect policies and the inspection and proxy switches. All of it is declared in `infra/zero-trust`, a root of its own, applied by hand like `infra/bootstrap`, with its state at `the-lobby/zero-trust/terraform.tfstate` in the same bucket.

ADR 0008 put the zone-wide Cloudflare pieces in `bootstrap` because every environment of the site shares them. The Gateway configuration is account-wide too, but it is not the site's: it belongs to no hostname and no environment, its policies change on their own schedule, and applying it needs a token with Zero Trust Edit, which the deploy token does not have and should not get.

## Considered options

- In `bootstrap`. Every apply of the roles, budget and rulesets would then carry the Gateway policies along, under a wider token, and a policy change would go through the same root as the state bucket.
- In `modules/site`. Nothing here is per hostname, and an environment deploy would apply account-wide filtering.
- A dashboard runbook instead of code. The handoff allowed it where Cloudflare was not in code; Cloudflare is in code in this repository, so only what the provider cannot express goes in the runbook (#44).

## Consequences

- The root declares only `cloudflare_zero_trust_*` resources and data sources: no DNS records, rulesets, identity provider or AWS resource. `task infra:check` fails on anything else (`infra:zero-trust:scope`), and the root's `tofu test` checks the policies, enrollment and switches against a mocked provider.
- The two adults' addresses and the household's own app ids are personal data. They live in the root's gitignored `zero-trust.auto.tfvars`, as `bootstrap.auto.tfvars` holds the origin secrets, and are never committed.
- The root is applied twice: everything first, and `inspect_https = true` only after the adult profile is installed and trusted on both iPhones (#44), because decryption before that breaks HTTPS on them.
- Its outputs, the two DNS over HTTPS addresses and the certificate PEM, are what the device profiles (#41) and the `/admin` page (#42) consume; Craig carries them into GitHub's variables and the environments' `secrets.auto.tfvars`.
- aws-setup's `aws-design.md` lists this repository's roots and state keys; the new root and `the-lobby/zero-trust/terraform.tfstate` belong there too (that repository, separate change).
