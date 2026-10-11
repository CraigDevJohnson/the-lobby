# Infrastructure

OpenTofu for the site. Four roots, one module.

| Path | What it is |
|---|---|
| `bootstrap/` | The account-level pieces, applied from a laptop: the private S3 bucket that holds every root's state (its own included), the three roles GitHub Actions assumes, the $5 monthly budget, and the Cloudflare pieces every environment shares (the origin-secret header rules, the calendar-link settings, Bot Fight Mode and the One-time PIN identity provider; ADR 0008). |
| `modules/site/` | Everything one environment of the site needs: the Lambda, API Gateway, the Sign-in sessions table, the Tool backends the site may call, and the Cloudflare DNS records and Access application for its hostname. |
| `environments/dev/` | `dev.craigdevjohnson.com`. Calls the module, stores state in the bucket. |
| `environments/prod/` | `next.craigdevjohnson.com` until the switch, then `craigdevjohnson.com` (docs/baseline.md). The same module; its deployer role, boundary and state key come from `bootstrap`, so the two environments cannot drift apart. |

Cloudflare allows one zone ruleset per phase and one One-time PIN identity provider per account, so those cannot belong to an environment: `bootstrap` declares each once, with one rule per environment's hostname, and every root that applies it needs the Cloudflare token too (ADR 0008).

## Running it locally

Needs OpenTofu 1.13 or newer, an AWS SSO login, and a Cloudflare API token, either in the environment or in the file `~/.config/craigdevjohnson/cloudflare_api_token` (mode 600) that `task infra:dev:plan` and `task infra:dev:apply` read. Nothing secret goes in the repo.

```sh
export AWS_PROFILE=workloads-admin
aws sso login
export CLOUDFLARE_API_TOKEN=...   # or leave it to the token file above
```

The Cloudflare token needs: Zone / DNS Edit, Zone / Transform Rules Edit, Zone / Config Rules Edit, Zone / Bot Management Edit (or Zone Settings Edit on the free plan), Zone / Zone Read, and Account / Access: Apps and Policies Edit, Account / Access: Organizations, Identity Providers, and Groups Edit, all on `craigdevjohnson.com` and its account.

### Where state lives

Every root keeps its state in the bucket `craigdevjohnson-tofu-state-793680745829` (us-west-2, versioned, encrypted, private), one key per root:

| Root | Key |
|---|---|
| `bootstrap` | `the-lobby/bootstrap/terraform.tfstate` |
| `environments/dev` | `the-lobby/dev/terraform.tfstate` |
| `environments/prod` | `the-lobby/prod/terraform.tfstate` |

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
   cp bootstrap.auto.tfvars.example bootstrap.auto.tfvars   # budget_email and origin_secrets; gitignored
   tofu init                       # or `tofu init -migrate-state` if the local state file is still there
   tofu plan
   tofu apply
   ```

   The budget is created on the first apply but matches no cost until the `Site` tag is activated as a cost allocation tag from the management account; see [Cost control](#cost-control).

   The apply prints three role ARNs (`pr_planner_role_arn`, `dev_deployer_role_arn`, `prod_deployer_role_arn`). The workflows in `.github/` use them; `docs/pipeline.md` says where they go.

   `origin_secrets` holds, per environment, the value Cloudflare adds in the `X-Origin-Secret` header for that environment's hostname. It must equal the `origin_secret` that environment's own root gives its Lambda (`secrets.auto.tfvars` there, `ORIGIN_SECRET_<ENV>` in GitHub), or that site refuses every request. Rotating one means changing it in all three places and applying `bootstrap` and the environment.

   #### Moving the zone-wide Cloudflare pieces into bootstrap (once)

   Until October 2026 the dev root owned the two zone rulesets, Bot Fight Mode and the One-time PIN provider. The dev root now carries `removed` blocks that drop them from its state without deleting them from Cloudflare, and `bootstrap` takes the same objects over by import. Do this before the first production deploy, in this order:

   1. Merge and let the Deploy workflow apply dev, so dev forgets the four objects (its summary shows them under "removed", nothing destroyed). Delete the `removed` blocks in `environments/dev/main.tf` afterwards.
   2. In `bootstrap`, with `bootstrap.auto.tfvars` filled in and the Cloudflare token set, import the four objects. The ids come from the Cloudflare API: `GET /zones/<zone_id>/rulesets` lists the rulesets with their phases, `GET /accounts/<account_id>/access/identity_providers` the identity providers.

      ```sh
      cd infra/bootstrap
      tofu import cloudflare_ruleset.origin_secret 'zones/<zone_id>/<ruleset_id of phase http_request_late_transform>'
      tofu import cloudflare_ruleset.calendar_links 'zones/<zone_id>/<ruleset_id of phase http_config_settings>'
      tofu import cloudflare_bot_management.zone '<zone_id>'
      tofu import cloudflare_zero_trust_access_identity_provider.otp 'accounts/<account_id>/<identity_provider_id>'
      tofu plan
      ```

      The plan changes nothing but the rulesets: each gains its production rule beside the dev rule it already had, and the names lose the `site-dev-` prefix. Anything else in the plan means an import took the wrong object; stop and read it.
   3. `tofu apply`. From here the two rulesets carry one rule per environment, and the production hostname's origin-secret header exists before the first production deploy needs it.

   On a completely fresh account the bucket does not exist yet, so the backend in `bootstrap/versions.tf` has nothing to talk to: comment that block out, `tofu init` and `tofu apply` with local state, put the block back and run `tofu init -migrate-state`. The fresh account also needs the GitHub OIDC provider, which this root looks up rather than creates (see below).

2. GitHub: the repository needs two environments, `dev` and `production`, because each deployer role trusts only the token a job gets when it targets that environment. Setting them up is part of `docs/pipeline.md`.

3. `environments/dev`, and `environments/prod` the same way with `prod.tfvars`. The non-secret settings are committed in `dev.tfvars`; the secrets and the Access invite list go in the gitignored `secrets.auto.tfvars`, which OpenTofu loads by itself:

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
- Cloudflare allows one zone ruleset per phase and one One-time PIN identity provider per account; `bootstrap` owns them (ADR 0008). If the account already has a One-time PIN provider, import it there. If the zone already has a Request Header Transform ruleset or a Configuration Rules ruleset, read its rules before importing it: `bootstrap`'s `cloudflare_ruleset` declares the whole phase, so once the ruleset is imported the next apply deletes every rule in it that it does not declare, including rules for other hostnames. Carry any rule worth keeping into `bootstrap` (or recreate it elsewhere) first, then import, and read the plan before applying. Without the import, Cloudflare refuses a second ruleset for the phase and the apply fails, leaving the existing rules alone.
- The zone's SSL mode should be Full (strict); API Gateway serves a real certificate for the hostname.

### Before the first production apply

- `bootstrap` has been applied with the production hostname in `origin_secrets` (above), so Cloudflare already adds the header for `next.craigdevjohnson.com`; without it the smoke test's `/healthz` answers 403.
- GitHub holds the production side (`docs/pipeline.md`): the `production` environment with Craig as required reviewer, its secrets `CLOUDFLARE_API_TOKEN` and `ORIGIN_SECRET_PROD` (the same value as `origin_secrets.prod`), and the variables `ALLOWED_EMAILS_PROD` and `AWS_PROD_DEPLOYER_ROLE_ARN` (bootstrap's `prod_deployer_role_arn`).
- The Deploy workflow's production job runs on every push to `main` once `environments/prod/main.tf` exists, and waits for approval each time; the first approval creates production.
- Production backends are deployed and their values are in `prod.tfvars` before the Tools are expected to work there; until then each Tool's page says its backend is unavailable.

## Connecting a Tool backend

Each Tool's backend is its own Lambda in its own repository, and the site calls it at the backend's Function URL with requests signed by the site's own role, the only caller AWS lets through (ADR 0006). The module takes three settings per Tool, named after the environment variables the Go server reads:

| Variable | Where it lands | What it is |
|---|---|---|
| `soccer_backend_url` | the Lambda's `SOCCER_BACKEND_URL` | The Function URL of the soccer repo's Lambda, from that repo's OpenTofu output. |
| `soccer_backend_function_arn` | the site role's `lambda:InvokeFunctionUrl` grant, and `lambda:InvokeFunction` limited to calls through the URL | The Lambda behind that URL, `soccer-<environment>-server`. |
| `soccer_backend_signing` | the Lambda's `SOCCER_BACKEND_SIGNING` | Only to override signing: `lambda` always signs, `off` never does. Normally empty. |

The Minecraft Launcher's backend takes the same three with `minecraft_`: the minecraft repo's `<prefix>-<environment>-launcher`, whose address is that repo's `launcher_url` output.

Empty means unset: the Lambda gets no `*_BACKEND_URL`, the site role gets no grant, and the Tool's page says its backend is unavailable. A URL and its ARN are set together. The ARN must name a function in this account and region called `*-<environment>-*`, which is all the site permissions boundary lets the role call, and be the function itself or an alias, never a numbered version, because a Function URL belongs to one of those; and the URL must be a Function URL in this region (`https://<id>.lambda-url.<region>.on.aws/`), the only region the server signs for. OpenTofu refuses anything else at plan time rather than letting the deployed site get `AccessDenied` or `403` from a URL it signed for the wrong region.

The values go in `dev.tfvars` or `prod.tfvars`, committed. They are not secret: a Function URL with `AWS_IAM` auth answers 403 to anyone but the site's role. They belong in the file because the Deploy workflow applies it on every push to `main`, so a value set only on a computer would be undone by the next deploy. The backend's side is its own repository's job: a Function URL with `AWS_IAM` auth and a resource policy naming `site-<environment>-server`.

Calendar apps fetch Session links at `/soccer/link/<token>.ics` with their own headers rather than a browser's, so `bootstrap` turns Browser Integrity Check off on that path of every hostname with a Cloudflare configuration rule (ADR 0005).

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
- Bot Fight Mode, the two zone rulesets (the header transform and the calendar link settings) and the One-time PIN provider are zone- or account-wide. They live in the module for the dev trial; when production is added they move to a shared root, because only one of each can exist.

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
