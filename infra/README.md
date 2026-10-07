# Infrastructure

OpenTofu for the site. Three roots, one module.

| Path | What it is |
|---|---|
| `bootstrap/` | Run once from a laptop. Creates the private S3 bucket that holds every other root's state. Keeps its own state locally (gitignored). |
| `modules/site/` | Everything one environment of the site needs: the Lambda, API Gateway, the Sign-in sessions table, and the Cloudflare DNS, secret header, bot setting and Access application. |
| `environments/dev/` | `dev.craigdevjohnson.com`. Calls the module, stores state in the bucket. |

Production is not here yet; it comes after the dev trial proves Access and the proxy to API Gateway.

## Running it locally

Needs OpenTofu 1.13 or newer, an AWS SSO login, and a Cloudflare API token in the environment. No token or secret goes in a file.

```sh
export AWS_PROFILE=workloads-admin
aws sso login
export CLOUDFLARE_API_TOKEN=...   # from your password manager, not a file
```

The Cloudflare token needs: Zone / DNS Edit, Zone / Transform Rules Edit, Zone / Bot Management Edit (or Zone Settings Edit on the free plan), Zone / Zone Read, and Account / Access: Apps and Policies Edit, Account / Access: Organizations, Identity Providers, and Groups Edit, all on `craigdevjohnson.com` and its account.

### Order

1. `bootstrap`, once:

   ```sh
   cd infra/bootstrap
   tofu init
   tofu apply
   ```

   This writes `terraform.tfstate` beside the config. It is gitignored; keep it, or re-import the bucket if it is lost (`tofu import aws_s3_bucket.state craigdevjohnson-tofu-state-793680745829`).

2. `environments/dev`:

   ```sh
   cd infra/environments/dev
   cp dev.auto.tfvars.example dev.auto.tfvars   # fill in; gitignored
   tofu init
   tofu plan
   tofu apply
   ```

### Before the first dev apply

- The old site's `dev` custom domain in API Gateway (us-west-2) has been removed. Custom domain names are unique per region, so the new one cannot be created while it exists.
- The Go server has been built and zipped: `dist/site.zip` containing one binary named `bootstrap` (the Makefile or Taskfile does this). `tofu validate` does not need it; `plan` and `apply` do.
- Zero Trust is set up on the Cloudflare account with a team domain (Cloudflare One, Settings, Custom Pages); that domain is the `access_team_domain` value.
- Two records already in the zone belong to the old dev site and will clash with records this root creates. Take them over rather than let the apply fail:
  - the proxied `dev` CNAME: `tofu import 'module.site.cloudflare_dns_record.site' '<zone_id>/<record_id>'`
  - the DNS-only ACM validation CNAME for `dev.craigdevjohnson.com` (its name starts with `_`): `tofu import 'module.site.cloudflare_dns_record.acm_validation["dev.craigdevjohnson.com"]' '<zone_id>/<record_id>'`

  Record ids come from the Cloudflare API (`GET /zones/<zone_id>/dns_records?name=...`). Alternatively delete both by hand first. Either way, remove them from the old site's state too, or its eventual destroy will delete them.
- If the zone already has a Request Header Transform rule, or the account already has a One-time PIN identity provider, import those as well; Cloudflare allows one zone ruleset per phase and one One-time PIN provider per account.
- The zone's SSL mode should be Full (strict); API Gateway serves a real certificate for the hostname.

## What the module does not do yet

- Google as an Access identity provider: a commented placeholder in `modules/site/cloudflare.tf`.
- CI roles and GitHub Actions: later.
- Browser Integrity Check off on the calendar link paths: when those paths exist.
- Bot Fight Mode, the header transform ruleset and the One-time PIN provider are zone- or account-wide. They live in the module for the dev trial; when production is added they move to a shared root, because only one of each can exist.

## Tests

```sh
cd infra/modules/site
tofu init -backend=false
tofu test
```

The tests mock both providers, so they run without credentials and touch nothing.
