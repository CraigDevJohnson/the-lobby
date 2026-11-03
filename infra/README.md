# Infrastructure

OpenTofu for the site and for the family's Cloudflare Gateway. Five roots, one module.

| Path | What it is |
|---|---|
| `bootstrap/` | The account-level pieces, applied from a laptop: the private S3 bucket that holds every root's state (its own included), the three roles GitHub Actions assumes, the $5 monthly budget, and the Cloudflare pieces every environment shares (the origin-secret header rules, the calendar-link settings, Bot Fight Mode and the One-time PIN identity provider; ADR 0008). |
| `modules/site/` | Everything one environment of the site needs: the Lambda, API Gateway, the Sign-in sessions table, the Tool backends the site may call, and the Cloudflare DNS records and Access applications for its hostname: `/signin` for Members, and `/admin` for the family devices page (ADR 0010). |
| `environments/dev/` | `dev.craigdevjohnson.com`. Calls the module, stores state in the bucket. |
| `environments/prod/` | `next.craigdevjohnson.com` until the switch, then `craigdevjohnson.com` (docs/baseline.md). The same module; its deployer role, boundary and state key come from `bootstrap`, so the two environments cannot drift apart. |
| `zero-trust/` | The family's Cloudflare Gateway configuration (ADR 0009): the Kids and Adults DNS locations and their policies, device enrollment, the Gateway root certificate, the Do Not Inspect policies, and the inspection and proxy switches. Account-wide and nothing of the site's; applied from a laptop like `bootstrap`. See [The family Gateway configuration](#the-family-gateway-configuration). |

Cloudflare allows one zone ruleset per phase and one One-time PIN identity provider per account, so those cannot belong to an environment: `bootstrap` declares each once, with one rule per environment's hostname, and every root that applies it needs the Cloudflare token too (ADR 0008). The module only reads the One-time PIN provider, finding it by type, for the Access application's allowed login methods.

## Running it locally

Needs OpenTofu 1.13 or newer, an AWS SSO login, and a Cloudflare API token, either in the environment or in the file `~/.config/craigdevjohnson/cloudflare_api_token` (mode 600) that `task infra:dev:plan` and `task infra:dev:apply` read. Nothing secret goes in the repo.

```sh
export AWS_PROFILE=workloads-admin
aws sso login
export CLOUDFLARE_API_TOKEN=...   # or leave it to the token file above
```

The Cloudflare token needs: Zone / DNS Edit, Zone / Transform Rules Edit, Zone / Config Rules Edit, Zone / Bot Management Edit (or Zone Settings Edit on the free plan), Zone / Zone Read, and Account / Access: Apps and Policies Edit, Account / Access: Organizations, Identity Providers, and Groups Edit, all on `craigdevjohnson.com` and its account.

The `zero-trust` root is applied by hand with a token that also has Account / Zero Trust Edit (the API calls it Zero Trust Write), for the locations, policies, certificate, Gateway and device settings and the device profile. Its Access needs, Apps and Policies Edit for the enrollment application and Organizations, Identity Providers, and Groups Read for the identity provider lookup, the token above already covers. The deploy token stays as it is: nothing in the pipeline applies this root.

### Where state lives

Every root keeps its state in the bucket `craigdevjohnson-tofu-state-793680745829` (us-west-2, versioned, encrypted, private), one key per root:

| Root | Key |
|---|---|
| `bootstrap` | `the-lobby/bootstrap/terraform.tfstate` |
| `environments/dev` | `the-lobby/dev/terraform.tfstate` |
| `environments/prod` | `the-lobby/prod/terraform.tfstate` |
| `zero-trust` | `the-lobby/zero-trust/terraform.tfstate` |

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
   3. `tofu apply`. From here the two rulesets carry one rule per environment, and the production hostname's origin-secret header exists before the first production deploy needs it. From here, commits from before the move cannot be deployed again, because their dev root would try to create the objects a second time; `docs/pipeline.md` (Rolling back) says what to do instead.

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

4. `zero-trust`, by hand and in two applies, when the family devices are set up; see [The family Gateway configuration](#the-family-gateway-configuration). It does not depend on the environments, but the `/admin` page in each environment consumes its outputs (#42).

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
- GitHub holds the production side (`docs/pipeline.md`): the `production` environment with Craig as required reviewer, its secrets `CLOUDFLARE_API_TOKEN` and `ORIGIN_SECRET_PROD` (the same value as `origin_secrets.prod`), and the variables `ALLOWED_EMAILS_PROD` and `AWS_PROD_DEPLOYER_ROLE_ARN` (bootstrap's `prod_deployer_role_arn`). `ADMIN_EMAILS_PROD` and the three profile settings can follow later; without them production has no `/admin` page yet.
- The Deploy workflow's production job runs on every push to `main` once `environments/prod/main.tf` exists, and waits for approval each time; the first approval creates production.
- Production backends are deployed and their values are in `prod.tfvars` before the Tools are expected to work there; until then each Tool's page says its backend is unavailable.

### The family Gateway configuration

`zero-trust/` declares the family devices' filtering (ADR 0009, the plan in #40): the `Kids` and `Adults` DNS locations with DNS over HTTPS endpoints, the kids' block, SafeSearch and YouTube Restricted Mode policies and the adults' malware and phishing block, device enrollment for the two adults by one-time code, the Gateway root certificate, the Do Not Inspect policies, and the TLS decryption and proxy switches behind `inspect_https`. The addresses and the household's own app ids go in the gitignored `zero-trust.auto.tfvars`; nothing personal is committed.

```sh
cd infra/zero-trust
cp zero-trust.auto.tfvars.example zero-trust.auto.tfvars   # the two addresses, an empty app list; gitignored
export AWS_PROFILE=workloads-admin && aws sso login             # the state bucket only
export CLOUDFLARE_API_TOKEN=...                                 # or the token file above
tofu init
tofu plan
```

`task infra:zero-trust:plan` and `task infra:zero-trust:apply` run the same from the repository root. Before the first apply, take over what the account already has rather than create a second of it; the ids come from the Cloudflare API with the same token:

- A DNS location Gateway made on first use (`GET /accounts/<account_id>/gateway/locations`): rename it `Kids` or `Adults` through the import, or leave it alone, but do not make a second default. `tofu import 'cloudflare_zero_trust_dns_location.location["kids"]' '<account_id>/<location_id>'`.
- The account's Gateway certificate (`GET /accounts/<account_id>/gateway/certificates`; accounts created since 2024-10-17 have one in state `available`): `tofu import cloudflare_zero_trust_gateway_certificate.gateway '<account_id>/<certificate_id>'`. Never generate a second; Cloudflare allows three generations a day and ten available at once.
- The enrollment application, if the dashboard already made one (`GET /accounts/<account_id>/access/apps`, the entry of type `warp`): `tofu import cloudflare_zero_trust_access_application.enrollment 'accounts/<account_id>/<app_id>'`.
- The account's Gateway settings always exist: `tofu import cloudflare_zero_trust_gateway_settings.account '<account_id>'`. The device settings resource cannot be imported; its first apply writes the two proxy toggles, off.

Then plan again and read it: only `cloudflare_zero_trust_*` resources may appear, which `task infra:zero-trust:scope` also checks on the declarations. The first apply, with `inspect_https` at its default `false`, creates everything else and prints `kids_doh_url`, `adults_doh_url` and `gateway_root_ca_pem`; those go into GitHub's variables `KIDS_DOH_URL`, `ADULTS_DOH_URL` and `GATEWAY_ROOT_CA_PEM` and each environment's `secrets.auto.tfvars` for the `/admin` page and the profiles (docs/pipeline.md). The second apply, with `inspect_https = true` in the tfvars, comes only after the adult profile is installed and the certificate trusted on both iPhones (docs/family-devices.md); decryption before that breaks HTTPS on them. `lock_client = true` is a later, optional third.

Every apply and import changes the live Cloudflare account or its state; Craig approves each one first.

### The family devices page

`/admin` serves the family device profiles to the two adults (ADR 0010; the profiles themselves are `internal/profile`). The module takes four settings for it, all personal data or values that change with the Zero Trust root, so none is in `dev.tfvars` or `prod.tfvars`; they go where `allowed_emails` goes, GitHub variables read as `TF_VAR_*` by the workflows (`docs/pipeline.md`) and `secrets.auto.tfvars` on a computer:

| Variable | Where it lands | What it is |
|---|---|---|
| `admin_emails` | the `/admin` Access application's policy | The two adults' addresses, `ADMIN_EMAILS_<ENV>` in GitHub. Empty creates no application, and the server refuses the page until one exists. |
| `kids_doh_url` | the Lambda's `KIDS_DOH_URL` | The Kids location's DNS over HTTPS endpoint, the Zero Trust root's `kids_doh_url` output; `KIDS_DOH_URL` in GitHub. |
| `adults_doh_url` | the Lambda's `ADULTS_DOH_URL` | The Adults location's endpoint, the root's `adults_doh_url` output; `ADULTS_DOH_URL` in GitHub. |
| `gateway_root_ca_pem` | the Lambda's `GATEWAY_ROOT_CA_PEM` | The Gateway root certificate as PEM, the root's `gateway_root_ca_pem` output; `GATEWAY_ROOT_CA_PEM` in GitHub. |

Empty means not configured yet: the page says so and serves no file with a placeholder in it. The plan refuses an endpoint that is not a Gateway one, anything but exactly one PEM certificate, and a certificate too large for the Lambda environment's 4 KB cap. The same two addresses enroll devices in `zero-trust/` (`enrollment_emails`), where they are set by hand.

### Inviting Members

Inviting someone takes two steps (ADR 0004). The first is the environment's Access invite list, `ALLOWED_EMAILS_<ENV>` in GitHub, which the next deploy applies; Access then proves who they are. The second is a Member record in that environment's table, which the site needs before it starts a Sign-in session; without one, sign-in ends on the welcome page with "That address hasn't been invited". A fresh environment has no Members, so after its first apply, and for each new Member later, run the site's Owner command from a computer with an admin profile, naming the environment and the Tools the Member may use:

```sh
aws sso login --profile workloads-admin
task member:add ENV=prod MEMBER=someone@example.com TOOLS="soccer minecraft"
```

The task runs the Owner command against that environment's table in us-west-2 with `AWS_PROFILE`, `workloads-admin` unless set; the line above is the same as:

```sh
AWS_PROFILE=workloads-admin AWS_REGION=us-west-2 SESSIONS_TABLE=site-prod-sessions \
  go run ./cmd/site member add someone@example.com soccer minecraft
```

`TOOLS` is the Member's whole list, because `member add` replaces the Tools they had; give an existing Member's current Tools along with the new one. `task member:list` shows the Members and their Tools, and `task member:remove ENV=prod MEMBER=someone@example.com` drops one, which ends their Sign-in sessions at once. Each environment keeps its own Members: an address invited on dev is not invited on production until it is added there too. `ENV="dev prod"` runs a task on both, one after the other, and `member:list` lists both when `ENV` is left out.

## Connecting a Tool backend

Each Tool's backend is its own Lambda in its own repository, and the site calls it at the backend's Function URL with requests signed by the site's own role, the only caller AWS lets through (ADR 0006). The module takes three settings per Tool, named after the environment variables the Go server reads:

| Variable | Where it lands | What it is |
|---|---|---|
| `soccer_backend_url` | the Lambda's `SOCCER_BACKEND_URL` | The Function URL of the soccer repo's Lambda, from that repo's OpenTofu output. |
| `soccer_backend_function_arn` | the site role's `lambda:InvokeFunctionUrl` grant, and `lambda:InvokeFunction` limited to calls through the URL | The Lambda behind that URL, `soccer-<environment>-server`. |
| `soccer_backend_signing` | the Lambda's `SOCCER_BACKEND_SIGNING` | Only to override signing: `lambda` always signs, `off` never does. Normally empty. |

The Minecraft Launcher's backend takes the same three with `minecraft_`: the minecraft repo's `<prefix>-<environment>-launcher`, whose address is that repo's `launcher_url` output.

Foundry's backend takes the same three with `foundry_`: the foundry repo's `foundry-<environment>-controller`, whose address and ARN are that repo's `controller_urls` and `controller_function_arns` outputs, keyed by environment.

Empty means unset: the Lambda gets no `*_BACKEND_URL`, the site role gets no grant, and the Tool's page says its backend is unavailable. A URL and its ARN are set together. The ARN must name a function in this account and region called `*-<environment>-*`, which is all the site permissions boundary lets the role call, and be the function itself or an alias, never a numbered version, because a Function URL belongs to one of those; and the URL must be a Function URL's root in this region (`https://<id>.lambda-url.<region>.on.aws/`, with no path, query or fragment), the only region the server signs for. OpenTofu refuses anything else at plan time rather than letting the deployed site get `AccessDenied` or `403` from a URL it signed for the wrong region.

The values go in `dev.tfvars` or `prod.tfvars`, committed. They are not secret: a Function URL with `AWS_IAM` auth answers 403 to anyone but the site's role. They belong in the file because the Deploy workflow applies it on every push to `main`, so a value set only on a computer would be undone by the next deploy. The backend's side is its own repository's job: a Function URL with `AWS_IAM` auth and a resource policy naming `site-<environment>-server`.

Calendar apps fetch Session links at `/soccer/link/<token>.ics` with their own headers rather than a browser's, so `bootstrap` turns Browser Integrity Check off on that path of every hostname with a Cloudflare configuration rule (ADR 0005).

## What GitHub Actions may do

`bootstrap/github_actions.tf` defines three roles. GitHub signs a short-lived token for each workflow run; AWS trusts it through the account's GitHub OpenID Connect provider (`token.actions.githubusercontent.com`) and hands out temporary credentials for one role. No AWS key is stored in GitHub. The provider belongs to another repository's stack; this root only looks it up, because IAM allows one per URL. If that stack ever removes it, `bootstrap` fails at plan time and every role stops working.

Each role trusts exactly one token subject, with an exact match (no wildcards), and a session lasts at most an hour.

| Role | Trusts a token from | May |
|---|---|---|
| `the-lobby-pr-planner` | any pull request in `CraigDevJohnson/the-lobby` | Read everything (the AWS `ReadOnlyAccess` policy) and read state under `the-lobby/`. Write nothing; plans run with `-lock=false`, so it never touches a lock file. |
| `the-lobby-dev-deployer` | a job that targets the GitHub environment `dev` | Everything the planner may, plus write `the-lobby/dev/*` state and create, change and delete the `site-dev-*` Lambda, log groups, DynamoDB table and IAM role, the hostname's certificate, the API Gateway pieces, and the CloudWatch Logs delivery that carries the API's access log. |
| `the-lobby-prod-deployer` | a job that targets the GitHub environment `production` | The same, for `site-prod-*` and `the-lobby/prod/*`. |

Both deployers are built from one map (`locals.environments`), so a permission added for one is added for both. Two places are wider than a deployer's own `site-<env>-*` names. API Gateway: its ARNs carry ids rather than names, so a deployer may manage any HTTP API or custom domain in us-west-2. CloudWatch Logs delivery: turning on an API stage's access logging has API Gateway create a log delivery through account-level APIs that take no resource ARN (`logs:CreateLogDelivery` and the rest of the set AWS documents, including `logs:PutResourcePolicy`), so those are granted on `*`, limited to us-west-2 by an `aws:RequestedRegion` condition. The most that reaches is the account's CloudWatch Logs resource policies in that region, which can only let AWS services write log events. The IAM role it creates can only be handed to Lambda (`iam:PassRole` with `iam:PassedToService = lambda.amazonaws.com`).

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

## Tests

```sh
cd infra/modules/site
tofu init -backend=false
tofu test

cd ../../bootstrap
tofu init -backend=false
tofu test

cd ../zero-trust
tofu init -backend=false
tofu test
```

The tests mock the providers, so they run without credentials and touch nothing. The zero-trust tests check that both locations answer DNS over HTTPS only, that every DNS policy is scoped to a location, that the enrollment application allows exactly the adults by one-time code, that decryption, the proxies and the certificate selection follow `inspect_https`, and that the Do Not Inspect policy for named apps exists only when apps are named; `task infra:zero-trust:scope` checks that the root declares nothing but Cloudflare Zero Trust types. The bootstrap tests check that each role trusts exactly its own GitHub subject, that the planner's own policy holds no write action, that each deployer names only its own environment's resources and state, and that the budget is $5 on the `Site` tag.
