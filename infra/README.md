# Infrastructure

OpenTofu for the site. Three roots, one module.

| Path | What it is |
|---|---|
| `bootstrap/` | The account-level pieces, applied from a laptop: the private S3 bucket that holds every root's state (its own included), the three roles GitHub Actions assumes, and the $5 monthly budget. |
| `modules/site/` | Everything one environment of the site needs: the Lambda, API Gateway, the Sign-in sessions table, and the Cloudflare DNS, secret header, bot setting and Access application. |
| `environments/dev/` | `dev.craigdevjohnson.com`. Calls the module, stores state in the bucket. |

Production is not here yet; it comes after the dev trial proves Access and the proxy to API Gateway. Its deployer role and state key already exist in `bootstrap`, so the two environments cannot drift apart.

## Running it locally

Needs OpenTofu 1.13 or newer, an AWS SSO login, and a Cloudflare API token, either in the environment or in the file `~/.config/craigdevjohnson/cloudflare_api_token` (mode 600) that `task infra:dev:plan` and `task infra:dev:apply` read. Nothing secret goes in the repo.

```sh
export AWS_PROFILE=workloads-admin
aws sso login
export CLOUDFLARE_API_TOKEN=...   # or leave it to the token file above
```

The Cloudflare token needs: Zone / DNS Edit, Zone / Transform Rules Edit, Zone / Bot Management Edit (or Zone Settings Edit on the free plan), Zone / Zone Read, and Account / Access: Apps and Policies Edit, Account / Access: Organizations, Identity Providers, and Groups Edit, all on `craigdevjohnson.com` and its account.

### Where state lives

Every root keeps its state in the bucket `craigdevjohnson-tofu-state-793680745829` (us-west-2, versioned, encrypted, private), one key per root:

| Root | Key |
|---|---|
| `bootstrap` | `the-lobby/bootstrap/terraform.tfstate` |
| `environments/dev` | `the-lobby/dev/terraform.tfstate` |
| `environments/prod` (later) | `the-lobby/prod/terraform.tfstate` |

Locking uses the backend's own lock file (`<key>.tflock` next to the state); there is no DynamoDB lock table. The pipeline's roles can only see keys under `the-lobby/`, and each deployer can only write under its own environment's prefix.

#### Moving the existing state (once)

Both roots that exist today were set up differently: `bootstrap` kept a local `terraform.tfstate`, and `dev` used the key `website/dev/terraform.tfstate`. The config now names the new keys, and OpenTofu moves the state when it sees the change:

```sh
cd infra/bootstrap
tofu init -migrate-state        # local file  ->  the-lobby/bootstrap/terraform.tfstate

cd ../environments/dev
tofu init -migrate-state        # website/dev/...  ->  the-lobby/dev/terraform.tfstate
```

Answer `yes` when asked to copy the state. A `tofu plan` straight after should show no change to anything that already exists (`bootstrap` will list the new roles and budget to add). The old `website/dev/terraform.tfstate` object stays in the bucket; delete it once the plan from the new key looks right. `bootstrap/terraform.tfstate` stays on disk and gitignored; keep it until the same check passes there, then delete it. If the bootstrap state is ever lost, the bucket can be taken back with `tofu import aws_s3_bucket.state craigdevjohnson-tofu-state-793680745829`; the roles and budget are cheap to recreate.

### Order

1. `bootstrap`, once:

   ```sh
   cd infra/bootstrap
   cp bootstrap.auto.tfvars.example bootstrap.auto.tfvars   # budget_email; gitignored
   tofu init                       # or `tofu init -migrate-state` if the local state file is still there
   tofu plan
   tofu apply
   ```

   The budget is created on the first apply but matches no cost until the `Site` tag is activated as a cost allocation tag from the management account; see [Cost control](#cost-control).

   The apply prints three role ARNs (`pr_planner_role_arn`, `dev_deployer_role_arn`, `prod_deployer_role_arn`). The workflows in `.github/` use them; `docs/pipeline.md` says where they go.

   On a completely fresh account the bucket does not exist yet, so the backend in `bootstrap/versions.tf` has nothing to talk to: comment that block out, `tofu init` and `tofu apply` with local state, put the block back and run `tofu init -migrate-state`. The fresh account also needs the GitHub OIDC provider, which this root looks up rather than creates (see below).

2. GitHub: the repository needs two environments, `dev` and `production`, because each deployer role trusts only the token a job gets when it targets that environment. Setting them up is part of `docs/pipeline.md`.

3. `environments/dev`. The non-secret settings are committed in `dev.tfvars`; the secrets and the Access invite list go in the gitignored `secrets.auto.tfvars`, which OpenTofu loads by itself:

   ```sh
   cd infra/environments/dev
   cp secrets.auto.tfvars.example secrets.auto.tfvars   # fill in; gitignored
   tofu init                       # or `tofu init -migrate-state` for the key change
   tofu plan -var-file=dev.tfvars
   tofu apply -var-file=dev.tfvars
   ```

   `task infra:dev:plan` and `task infra:dev:apply` run the same two commands from the repository root.

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

## What GitHub Actions may do

`bootstrap/github_actions.tf` defines three roles. GitHub signs a short-lived token for each workflow run; AWS trusts it through the account's GitHub OpenID Connect provider (`token.actions.githubusercontent.com`) and hands out temporary credentials for one role. No AWS key is stored in GitHub. The provider belongs to another repository's stack; this root only looks it up, because IAM allows one per URL. If that stack ever removes it, `bootstrap` fails at plan time and every role stops working.

Each role trusts exactly one token subject, with an exact match (no wildcards), and a session lasts at most an hour.

| Role | Trusts a token from | May |
|---|---|---|
| `the-lobby-pr-planner` | any pull request in `CraigDevJohnson/the-lobby` | Read everything (the AWS `ReadOnlyAccess` policy) and read state under `the-lobby/`. Write nothing; plans run with `-lock=false`, so it never touches a lock file. |
| `the-lobby-dev-deployer` | a job that targets the GitHub environment `dev` | Everything the planner may, plus write `the-lobby/dev/*` state and create, change and delete the `site-dev-*` Lambda, log groups, DynamoDB table and IAM role, the hostname's certificate, and the API Gateway pieces. |
| `the-lobby-prod-deployer` | a job that targets the GitHub environment `production` | The same, for `site-prod-*` and `the-lobby/prod/*`. |

Both deployers are built from one map (`locals.environments`), so a permission added for one is added for both. API Gateway is the one place a deployer is wider than its own `site-<env>-*` names: its ARNs carry ids rather than names, so a deployer may manage any HTTP API or custom domain in us-west-2. The IAM role it creates can only be handed to Lambda (`iam:PassRole` with `iam:PassedToService = lambda.amazonaws.com`).

`ReadOnlyAccess` reads a lot, including Lambda environment variables and every object in every bucket, so the planner role is for this repository's own pull requests only; the workflows must never request a token for a pull request from someone else's copy of the repo (`docs/pipeline.md`).

## Cost control

`bootstrap/budget.tf` creates the budget `the-lobby-monthly`: $5 a month on everything tagged `Site = craigdevjohnson.com`, which every root sets through `default_tags`. It emails `budget_email` twice: when the month's spend passes $5, and when the forecast for the month does.

A budget can only filter on a tag that is active as a *cost allocation tag*. This account belongs to an AWS Organization, so only the management account can activate one, and AWS only allows it once the tag has shown up in billing data, up to 24 hours after the first tagged resource exists. Run once, from the management account:

```sh
aws ce update-cost-allocation-tags-status --profile mgmt-admin --cost-allocation-tags-status TagKey=Site,Status=Active
```

Until then the budget exists but matches no cost. Budgets and Cost Explorer are global services; the AWS provider sends their calls to us-east-1 by itself, so the root keeps its single us-west-2 provider.

The other cost brake, the concurrency cap on the Lambda, lives in `modules/site`.

## What the module does not do yet

- Google as an Access identity provider: a commented placeholder in `modules/site/cloudflare.tf`.
- GitHub Actions workflows: in `.github/`, described in `docs/pipeline.md`. The roles they assume are in `bootstrap`.
- Browser Integrity Check off on the calendar link paths: when those paths exist.
- Bot Fight Mode, the header transform ruleset and the One-time PIN provider are zone- or account-wide. They live in the module for the dev trial; when production is added they move to a shared root, because only one of each can exist.

## Tests

```sh
cd infra/modules/site
tofu init -backend=false
tofu test

cd ../../bootstrap
tofu init -backend=false
tofu test
```

The tests mock the providers, so they run without credentials and touch nothing. The bootstrap tests check that each role trusts exactly its own GitHub subject, that the planner's own policy holds no write action, that each deployer names only its own environment's resources and state, and that the budget is $5 on the `Site` tag.
