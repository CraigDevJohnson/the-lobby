# Agent guidance

The site behind craigdevjohnson.com: a Go server on AWS Lambda, behind Cloudflare, that serves the React app built from `web/`. Cloudflare Access covers only the sign-in paths and `/admin`. Members' routes are guarded by the site's own Sign-in session, and the Landing page, Privacy and calendar links are public (ADR 0004, ADR 0010). Each Tool's backend lives in its own repo. Read [README.md](README.md) first, then [CONTEXT.md](CONTEXT.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Commands

- `task` lists them all.
- `task ci` runs every pull-request check (gofmt, go vet, Go tests, typecheck and tests for the screens, offline OpenTofu checks). Run it before pushing.
- `task run` builds the screens and serves the site locally, with sign-in disabled.
- The screens need Node 20.19 or later to build and Node 22.18 or later for `task ui:test`.

## Rules

- Live changes need Craig's go-ahead for that specific change. That covers `task infra:*:apply`, `task member:add` and `task member:remove`, and anything else that writes to AWS or Cloudflare. Offline checks and plans are fine.
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
