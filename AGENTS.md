# Agent guidance

The site behind craigdevjohnson.com: a Go server on AWS Lambda, behind Cloudflare, that serves the React app built from `web/`. Cloudflare Access covers only the sign-in paths and `/admin`. Members' routes are guarded by the site's own Sign-in session, and the Landing page, Privacy and calendar links are public (ADR 0004, ADR 0010). Each Tool's backend lives in its own repo. Read [README.md](README.md) first, then [CONTEXT.md](CONTEXT.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Commands

- `task` lists them all.
- `task ci` runs every pull-request check (gofmt, go vet, Go tests, typecheck and tests for the screens, offline OpenTofu checks). Run it before pushing.
- `task run` builds the screens and serves the site locally, with sign-in disabled.
- The screens need Node 20.19 or later to build and Node 22.18 or later for `task ui:test`.

## Rules

- Live changes need Craig's go-ahead for that specific change. That covers `task infra:*:apply`, `task infra:inputs:push`, `task member:add` and `task member:remove`, and anything else that writes to AWS or Cloudflare. Offline checks and plans are fine, and so is `task infra:inputs:pull`, which only reads.
- A step these docs describe as run from a computer, a laptop or Craig's Mac can also run in a cloud thread: an agent session in a cloud container, not on Craig's computer. The thread signs in with `aws sso login --sso-session identity-center --use-device-code` and posts the link and the code for Craig to approve on his phone (aws-setup's `aws-design.md` §5.2). Do not ask Craig to use his own computer for a step a cloud thread can do; [infra/README.md](infra/README.md#running-it-locally) says what it cannot do yet. The approval rule above is unchanged there. A thread pulls a root's private input files before it plans, never opens or prints one, and deletes them before it signs out ([infra/README.md](infra/README.md#private-input-files)).
- Deploys run in GitHub Actions: a merge to `main` deploys dev, and production waits for Craig's approval ([docs/pipeline.md](docs/pipeline.md)).
- The repository and its Actions logs are public. Keep email addresses, tokens, calendar links (Session links and Member links) and other personal details out of code, issues, pull requests and logs.
- Screen design follows [PRODUCT.md](PRODUCT.md), [DESIGN.md](DESIGN.md), the briefs in [docs/design/](docs/design/) and the mockups in `.impeccable/`.

## Agent skills

### Issue tracker

Issues and specs are tracked in GitHub Issues for `CraigDevJohnson/the-lobby`. See [docs/agents/issue-tracker.md](docs/agents/issue-tracker.md).

### Triage labels

Use the default labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See [docs/agents/triage-labels.md](docs/agents/triage-labels.md).

### Domain docs

Use a single-context layout with root `CONTEXT.md` and `docs/adr/`. See [docs/agents/domain.md](docs/agents/domain.md).
