# Pipeline

How a change gets from a pull request to `craigdevjohnson.com`. Everything runs in GitHub Actions from the workflow files in `.github/workflows/`; the rules come from the baseline's "How it is run". Terms are defined in [CONTEXT.md](../CONTEXT.md).

## On a pull request

Two things can happen.

**Tests, always.** The `CI` workflow runs on every pull request, whoever opened it, and again on every push to `main`. It has no secrets, no cloud role, and a read-only token, so it cannot touch an account:

- Go: the tests, `go vet`, and a check that every file is formatted with `gofmt`.
- Infrastructure: OpenTofu formatting, validation of every root, and the site module's tests, which mock both providers.

`task ci` runs the same checks on a computer, with no credentials.

**A plan, for Craig's own branches only.** The `Plan` workflow builds the Lambda zip from the pull request's code, signs in to AWS as the read-only planner role, and runs `tofu plan` for dev. Production is never planned from a pull request: its secrets exist only in the `production` environment, which only the approved deploy job can reach. It changes nothing. The result is a short summary on the run's page: which resources would be created, changed or destroyed, and the totals. The full plan is kept out of the log on purpose: the repository is public, so its logs are public, and the full plan would show the Access invite list.

## On main

A push to `main` (normally a merged pull request) starts the `Deploy` workflow:

1. **Build**, once. The tests run, `task lambda` makes `dist/site.zip`, and the zip is saved as a workflow artifact named after the commit.
2. **dev**, with no approval. The same zip is downloaded, the dev deployer role is assumed, and `tofu apply` runs for `infra/environments/dev`. Two checks follow: `https://dev.craigdevjohnson.com/healthz` must answer `200` with the body `ok`, and the raw API Gateway address must answer `403`, which proves that nothing reaches the site around Cloudflare.
3. **production**, after approval. Only after dev succeeded, and only once `infra/environments/prod` exists. The job waits for Craig to approve it, then does the same against production with the production role, secret and invite list, and checks the hostname that root reports (`next.craigdevjohnson.com` until the switch, then `craigdevjohnson.com`).

One build, deployed in order: what reaches production is byte for byte what ran on dev. Two deploys of the same environment never overlap; a second run waits for the first to finish and is never cancelled part-way through an apply.

## What GitHub holds

No AWS keys. Each job signs in to AWS with a short-lived token that GitHub issues for that one run (OIDC), and each AWS role only trusts one kind of job.

| Name | Kind | What it is | Read by |
|---|---|---|---|
| `CLOUDFLARE_API_TOKEN` | environment secret, in `dev` and `production` | The Cloudflare token that may change things (see below) | Deploy dev, Deploy production |
| `CLOUDFLARE_API_TOKEN_READ_ONLY` | secret | A second Cloudflare token with read permissions only | Plan |
| `ORIGIN_SECRET_DEV` | secret | The secret header value Cloudflare adds and the dev site checks | Plan, Deploy dev |
| `ORIGIN_SECRET_PROD` | environment secret, in `production` | The same for production | Deploy production |
| `ALLOWED_EMAILS_DEV` | variable | The dev Access invite list, as a JSON list such as `["a@example.com"]` | Plan, Deploy dev |
| `ALLOWED_EMAILS_PROD` | variable | The same for production | Plan, Deploy production |
| `ADMIN_EMAILS_DEV` | variable | Who may open the dev family devices page at `/admin` (ADR 0010), as a JSON list; unset means no page yet | Plan, Deploy dev |
| `ADMIN_EMAILS_PROD` | variable | The same for production | Deploy production |
| `KIDS_DOH_URL` | variable | The Kids Gateway location's DNS over HTTPS endpoint, from the Zero Trust root's `kids_doh_url` output (`infra/zero-trust`); shared by both environments | Plan, Deploy dev, Deploy production |
| `ADULTS_DOH_URL` | variable | The Adults location's endpoint, the root's `adults_doh_url` output | Plan, Deploy dev, Deploy production |
| `GATEWAY_ROOT_CA_PEM` | variable | The Gateway root certificate as PEM, the root's `gateway_root_ca_pem` output | Plan, Deploy dev, Deploy production |
| `AWS_PR_PLANNER_ROLE_ARN` | variable | The read-only role; trusts pull requests from this repository only | Plan |
| `AWS_DEV_DEPLOYER_ROLE_ARN` | variable | The role that may change dev; trusts jobs in the `dev` environment only | Deploy dev |
| `AWS_PROD_DEPLOYER_ROLE_ARN` | variable | The role that may change production; trusts jobs in the `production` environment only | Deploy production |

The invite lists and the family devices page's lists are variables rather than secrets because they are not secret, but they are personal data, so they live in GitHub, never in the repository, and the workflows never print them. The three profile settings are account-wide, so one variable each serves both environments; they are not secret either, but anyone who learns a DNS over HTTPS endpoint can resolve through that Gateway location and write into its log, and they change with the Zero Trust root rather than with the site's code, so they stay out of the repository too. Until Craig sets a list the workflows pass an empty one, and the environment deploys without its `/admin` page. The two `tfvars` files that are committed (`infra/environments/dev/dev.tfvars` and `infra/environments/prod/prod.tfvars`) hold only the non-secret settings.

Each origin secret lives in three places that must agree: GitHub (`ORIGIN_SECRET_<ENV>`, for the deploy that gives it to the Lambda), the environment's `secrets.auto.tfvars` on a computer, and `infra/bootstrap`'s `origin_secrets`, from which Cloudflare's header rule gets it (ADR 0008). Rotating one means all three, then a `bootstrap` apply and a deploy.

The `CI` workflow reads none of these. The `Plan` workflow reads the read-only token, the dev origin secret, the dev lists, the three profile settings and the planner role. Each deploy job reads only its own environment's secret, lists and role, plus the token and the three profile settings.

The two GitHub environments: `dev` has no reviewers; `production` requires Craig's approval. Both are under Settings, Environments. The AWS roles and their trust in GitHub are created in OpenTofu; see `infra/README.md`.

## Keeping forks away from the accounts

Anyone can open a pull request from their own copy of the repository. For such a pull request:

- `CI` runs as normal. It never had access to anything.
- `Plan` is skipped. Its job runs only when the pull request's branch lives in this repository (`github.event.pull_request.head.repo.full_name == github.repository`). GitHub also withholds secrets and the OIDC token from fork pull requests, so even without the guard the job would have nothing to sign in with. The planner role's trust names this repository, which keeps every other repository out as well.
- `Deploy` does not run on pull requests at all, only on `main` and by hand.

Dependabot's own pull requests are treated like forks for the plan: GitHub runs them without the repository secrets, so the plan is skipped rather than failing.

## Approving production

When a `Deploy` run reaches the production job, it pauses. GitHub emails Craig and shows a "Review deployments" button at the top of the run. Approve, and the job continues; reject, and the run ends with dev deployed and production untouched. The approval rule is the `production` environment's required-reviewers setting; nobody else can approve.

## Rolling back

Two ways, in order of preference.

1. **Revert.** `git revert` the commit on `main` and push. The pipeline builds the reverted code, deploys dev, and waits for approval for production, like any other change. This leaves a record and keeps `main` in step with what is deployed.
2. **Deploy an older commit by hand.** Open Actions, choose `Deploy`, press "Run workflow", and pick a branch or tag. Run workflow takes a branch or tag rather than a commit, so first put a tag on the commit to go back to (`git tag rollback-2026-10-06 <commit> && git push origin rollback-2026-10-06`), then pick that tag. The same can be done with `gh workflow run deploy.yml --ref rollback-2026-10-06`. The run builds that commit, deploys dev, and waits for approval for production. The next push to `main` deploys `main` again, so follow up with a revert.

Both roll back the infrastructure too, since each commit carries its own OpenTofu. For a code-only problem that is harmless. If the bad change added resources, going back removes them, which is usually what is wanted, but read the plan summary on the run before approving production.

Commits from before the zone-wide Cloudflare pieces moved into `bootstrap` (October 2026, ADR 0008) cannot be deployed again. Their dev root still declares the header and calendar-link rulesets, Bot Fight Mode and the One-time PIN provider, which now live in `bootstrap`'s state, so their dev apply tries to create a second ruleset per phase and a second provider, and Cloudflare refuses both. The run then fails part-way: nothing in Cloudflare changes, and the Lambda may already be on the older code. To roll back a change from before that point, revert it on `main` instead (the first way above). To recover from such a run, deploy `main` again (push, or "Run workflow" on `main`); the next successful dev apply puts dev back in step.

## The one Cloudflare token

Cloudflare offers no keyless sign-in, so an API token that may change the zone is stored as the secret `CLOUDFLARE_API_TOKEN` in the `dev` and `production` environments, where only deploy jobs can read it. A second token with read permissions only, `CLOUDFLARE_API_TOKEN_READ_ONLY`, serves pull-request plans; until it exists the plan step is skipped. These are the only long-lived credentials the pipeline holds. Its permissions are listed in `infra/README.md`; it is scoped to the `craigdevjohnson.com` zone and its account. Only the `Plan` and `Deploy` workflows read it, and only on branches in this repository. If it leaks, roll it in the Cloudflare dashboard and replace the secret in GitHub.

On a developer's computer, `task infra:dev:plan` and `task infra:dev:apply` read the token from the `CLOUDFLARE_API_TOKEN` environment variable, or, if that is not set, from the file named by `CLOUDFLARE_API_TOKEN_FILE` (default `~/.config/craigdevjohnson/cloudflare_api_token`). They need `AWS_PROFILE` set and an `aws sso login` session. The secrets OpenTofu needs come from the gitignored `infra/environments/dev/secrets.auto.tfvars`; copy `secrets.auto.tfvars.example` to start.

## Keeping dependencies current

Every action in the workflows is pinned to a commit, with the release it corresponds to in a comment; Dependabot updates the commit and the comment together. Once a week it checks the actions, the Go modules, the screens' npm packages in `web/` and the OpenTofu providers of every root under `infra/` (`.github/dependabot.yml`). Minor and patch updates arrive as one grouped pull request per ecosystem, each major version as its own pull request, and security fixes grouped together. Version updates wait until a release is seven days old; security fixes do not wait.
