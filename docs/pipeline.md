# Pipeline

How a change gets from a pull request to `craigdevjohnson.com`. Everything runs in GitHub Actions from the workflow files in `.github/workflows/`; the rules come from the baseline's "How it is run". Terms are defined in [CONTEXT.md](../CONTEXT.md).

## On a pull request

Two things can happen.

**Tests, always.** The `CI` workflow runs on every pull request, whoever opened it, and again on every push to `main`. It has no secrets, no cloud role, and a read-only token, so it cannot touch an account:

- Go: the tests, `go vet`, and a check that every file is formatted with `gofmt`.
- Infrastructure: OpenTofu formatting, validation of every root, and the site module's tests, which mock both providers.

`task ci` runs the same checks on a computer, with no credentials.

**A plan, for Craig's own branches only.** The `Plan` workflow builds the Lambda zip from the pull request's code, signs in to AWS as the read-only planner role, and runs `tofu plan` for dev. Production is never planned from a pull request: its origin secret and Cloudflare token exist only in the `production` environment, which only the approved deploy job can reach. It changes nothing. The result is a short summary on the run's page: which resources would be created, changed or destroyed, and the totals. The full plan is kept out of the log on purpose: the repository is public, so its logs are public, and the full plan would show the family devices page's addresses.

## On main

A push to `main` (normally a merged pull request) starts the `Deploy` workflow:

1. **Build**, once. The tests run, `task lambda` makes `dist/site.zip`, and the zip is saved as a workflow artifact named after the commit.
2. **Approval**, before any deploy code runs. The one deploy job targets the protected `production` environment and waits for Craig. That environment is the only place that holds the mutable Cloudflare token, and both AWS deployer roles trust only its OIDC subject (ADR 0018).
3. **dev**, after approval. The same zip is downloaded, the dev deployer role is assumed, and `tofu apply` runs for `infra/environments/dev`. Two checks follow: `https://dev.craigdevjohnson.com/healthz` must answer `200` with the body `ok`, and the raw API Gateway address must answer `403`, which proves that nothing reaches the site around Cloudflare.
4. **production**, in the same approved job. Only after dev succeeded, and only once `infra/environments/prod` exists, the job changes to the production role, applies that root with the production secret, and checks the hostname that root reports (`next.craigdevjohnson.com` until the switch, then `craigdevjohnson.com`).

A push that changes only `CHANGELOG.md` starts no deploy: nothing in it reaches the site, so there is nothing to build or approve.

One build, one approval, deployed in order: what reaches production is byte for byte what ran on dev. Approved deploy jobs never overlap; a second run waits for the first to finish and is never cancelled part-way through an apply.

## What GitHub holds

No AWS keys. Each job signs in to AWS with a short-lived token that GitHub issues for that one run (OIDC), and each AWS role only trusts one kind of job.

| Name | Kind | What it is | Read by |
|---|---|---|---|
| `CLOUDFLARE_API_TOKEN` | environment secret, in `production` only | The Cloudflare token that may change things (see below) | Approved Deploy job |
| `CLOUDFLARE_API_TOKEN_READ_ONLY` | secret | A second Cloudflare token with read permissions only | Plan |
| `ORIGIN_SECRET_DEV` | secret | The secret header value Cloudflare adds and the dev site checks | Plan, approved Deploy job |
| `ORIGIN_SECRET_PROD` | environment secret, in `production` | The same for production | Approved Deploy job |
| `ADMIN_EMAILS_DEV` | secret | Who may open the dev family devices page at `/admin` (ADR 0010), as a JSON list; unset means no page yet | Plan, approved Deploy job |
| `ADMIN_EMAILS_PROD` | secret | The same for production | Approved Deploy job |
| `KIDS_DOH_URL` | secret | The Kids Gateway location's DNS over HTTPS endpoint, from the Zero Trust root's `kids_doh_url` output (`infra/zero-trust`); shared by both environments | Plan, approved Deploy job |
| `ADULTS_DOH_URL` | secret | The Adults location's endpoint, the root's `adults_doh_url` output | Plan, approved Deploy job |
| `GATEWAY_ROOT_CA_PEM` | variable | The Gateway root certificate as PEM, the root's `gateway_root_ca_pem` output | Plan, approved Deploy job |
| `AWS_PR_PLANNER_ROLE_ARN` | variable | The read-only role; trusts pull requests from this repository only | Plan |
| `AWS_DEV_DEPLOYER_ROLE_ARN` | variable | The role that may change dev; trusts only the `production` environment's approved jobs | Approved Deploy job |
| `AWS_PROD_DEPLOYER_ROLE_ARN` | variable | The role that may change production; trusts only the `production` environment's approved jobs | Approved Deploy job |

The family devices page's lists and the two DNS over HTTPS endpoints are secrets, though none is a credential. The lists are personal data, and anyone who learns an endpoint can resolve through that Gateway location and write into its log. A run's log prints each step's environment, a variable's value as it is and a secret's as `***`, and this repository's logs are public, so as variables they were published by every run. The Gateway root certificate stays a variable: it is the public half, which the adult profile installs. The three profile settings are account-wide, so one of each serves both environments, and they change with the Zero Trust root rather than with the site's code, so they stay out of the repository too. Until Craig sets a list the workflows pass an empty one, and the environment deploys without its `/admin` page. The two `tfvars` files that are committed (`infra/environments/dev/dev.tfvars` and `infra/environments/prod/prod.tfvars`) hold only the non-secret settings.

Each origin secret lives in three places that must agree: GitHub (`ORIGIN_SECRET_<ENV>`, for the deploy that gives it to the Lambda), the environment's `secrets.auto.tfvars` where one is kept, and `infra/bootstrap`'s `origin_secrets`, from which Cloudflare's header rule gets it (ADR 0008). The last two sit in private input files, `secrets.auto.tfvars` and `bootstrap.auto.tfvars`, which `task infra:inputs:pull` and `task infra:inputs:push` carry between a checkout and the state bucket (ADR 0016, `infra/README.md`). Rotating one means all three, then a `bootstrap` apply and a deploy, and a push of the files that changed.

The `CI` workflow reads none of these. The `Plan` workflow reads the read-only token, the dev origin secret, the dev lists, the three profile settings and the planner role. The approved deploy job reads both environments' settings, changes from the dev AWS role to the production AWS role between applies, and reads the mutable Cloudflare token from the protected `production` environment.

There is one GitHub deployment environment: `production`, with Craig as its required reviewer and `main` as its only deployment branch. There is no `dev` environment: if it held the mutable token, merged code could name it in another job and bypass the production approval. The AWS policies still separate dev from production; only their GitHub OIDC approval subject is shared. Check the production branch rule with the two commands below. The first prints `null` when any branch or tag may use it; the second should print exactly `branch main`.

```sh
gh api repos/CraigDevJohnson/the-lobby/environments/production --jq '.deployment_branch_policy'
gh api repos/CraigDevJohnson/the-lobby/environments/production/deployment-branch-policies --jq '.branch_policies[] | "\(.type) \(.name)"'
```

The AWS roles and their trust in GitHub are created in OpenTofu; see `infra/README.md`.

## Keeping forks away from the accounts

Anyone can open a pull request from their own copy of the repository. For such a pull request:

- `CI` runs as normal. It never had access to anything.
- `Plan` is skipped. Its job runs only when the pull request's branch lives in this repository (`github.event.pull_request.head.repo.full_name == github.repository`). GitHub also withholds secrets and the OIDC token from fork pull requests, so even without the guard the job would have nothing to sign in with. The planner role's trust names this repository, which keeps every other repository out as well.
- `Deploy` does not run on pull requests at all, only on `main` and by hand.

Dependabot's own pull requests are treated like forks for the plan: GitHub runs them without the repository secrets, so the plan is skipped rather than failing.

## Approving production

When a `Deploy` run reaches the deploy job, it pauses before the job starts. GitHub emails Craig and shows a "Review deployments" button at the top of the run. Approve, and the job deploys dev, checks it, then deploys production; reject, and neither environment changes. The approval rule is the `production` environment's required-reviewers setting; nobody else can approve.

## Rolling back

Revert. `git revert` the commit on `main` and push. The pipeline builds the reverted code and waits for approval before deploying dev and production, like any other change. This leaves a record and keeps `main` in step with what is deployed.

There is no deploying an older commit by hand. Run workflow takes a branch or tag, and the `production` environment takes only `main`: allowing a tag would let anyone who can push a branch ask to run unmerged deploy code with both roles and the Cloudflare token. "Run workflow" on `main` still deploys `main` again.

A revert rolls back the infrastructure too, since each commit carries its own OpenTofu. For a code-only problem that is harmless. If the bad change added resources, the revert removes them, which is usually what is wanted, but read the plan summary on the run before approving production.

## The one Cloudflare token

Cloudflare offers no keyless sign-in, so one API token that may change the zone is stored as `CLOUDFLARE_API_TOKEN` in the protected `production` environment only. A second token with read permissions only, `CLOUDFLARE_API_TOKEN_READ_ONLY`, serves pull-request plans; until it exists the plan step is skipped. These are the only long-lived credentials the pipeline holds. The mutable token's permissions are listed in `infra/README.md`; they cover the `craigdevjohnson.com` zone and its account, including the Transform Rule that holds both origin secrets (ADR 0008). The deploy job cannot receive that token until Craig approves its `production` environment deployment. The `Plan` workflow receives only the read-only token. If either token leaks, roll it in the Cloudflare dashboard and replace its one GitHub secret.

The move from the former automatic dev deployment has a one-time live step. After this change reaches `main`, but before approving its Deploy run, apply `infra/bootstrap` so the dev deployer role trusts the `production` environment subject. Make `main` that environment's only deployment branch. Then create a replacement mutable Cloudflare token, store it only as the `production` environment's `CLOUDFLARE_API_TOKEN`, delete the old `dev` GitHub environment, and revoke the old token. The waiting Deploy run can then be approved. Until the old token is revoked and the `dev` environment is gone, that old environment remains a route around the new approval boundary.

On a developer's computer, `task infra:dev:plan` and `task infra:dev:apply` read the token from the `CLOUDFLARE_API_TOKEN` environment variable, or, if that is not set, from the file named by `CLOUDFLARE_API_TOKEN_FILE` (default `~/.config/craigdevjohnson/cloudflare_api_token`). They need `AWS_PROFILE` set and an `aws sso login` session. The secrets OpenTofu needs come from the gitignored `infra/environments/dev/secrets.auto.tfvars`; copy `secrets.auto.tfvars.example` to start, or fetch the file with `task infra:inputs:pull` where the state bucket holds one (`infra/README.md`).

## Keeping dependencies current

Every action in the workflows is pinned to a commit, with the release it corresponds to in a comment; Dependabot updates the commit and the comment together. Once a week it checks the actions, the Go modules, the screens' npm packages in `web/` and the OpenTofu providers of every root under `infra/` (`.github/dependabot.yml`). Minor and patch updates arrive as one grouped pull request per ecosystem, each major version as its own pull request, and security fixes grouped together. Version updates wait until a release is seven days old; security fixes do not wait. Minor and patch updates merge on their own once the required checks pass (`.github/workflows/dependabot-auto-merge.yml`); major updates, minor updates of a 0.x dependency, and every GitHub Actions update wait for Craig's review.
