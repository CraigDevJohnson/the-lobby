---
status: accepted
date: 2026-10-11
---

# Zone-wide Cloudflare settings live in bootstrap

Cloudflare allows one zone ruleset per phase and one One-time PIN identity provider per account. The origin-secret header (ADR 0005), the calendar-link settings, Bot Fight Mode and the identity provider are therefore declared once, in `infra/bootstrap`, with one rule per environment's hostname. An environment root keeps only what belongs to its hostname: the DNS records, the certificate and the Access application, which finds the identity provider by type.

Until production existed, the dev root declared all of them. Production serves `next.craigdevjohnson.com` in the same zone, and its first apply would have failed on a second ruleset per phase.

## Considered options

- The production root owns the zone-wide pieces and takes dev's hostname and origin secret as inputs. Asymmetric: a change to dev's header would need a production deploy and its approval, and the production job would hold dev's secret.
- Each environment in its own Cloudflare zone. Not possible: both hostnames are in `craigdevjohnson.com`.
- Dropping the header in favour of something not zone-wide. API Gateway's HTTP API has no resource policy to allow only Cloudflare, and ADR 0005 stands.

## Consequences

- `bootstrap` is applied with the Cloudflare token as well as the AWS profile, and its gitignored `bootstrap.auto.tfvars` holds every environment's origin secret beside the environment's own `secrets.auto.tfvars` and GitHub's `ORIGIN_SECRET_<ENV>`. Rotating a secret touches all three and needs a `bootstrap` apply and a deploy.
- Adding a hostname (the switch to `craigdevjohnson.com`) changes `bootstrap`'s environment map and the environment's hostname together.
- The move is a one-time import into `bootstrap` after dev forgets the objects through `removed` blocks; `infra/README.md` has the steps.
