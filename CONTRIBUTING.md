# Contributing

This is Craig's personal site, and it is not taking outside contributions. Bug reports are welcome: open an issue with the bug form, and keep personal details out of it, since the repository is public. Report security problems privately as [SECURITY.md](SECURITY.md) describes.

The rest of this page is how changes get made here, by Craig or an agent working for him.

## Making a change

1. Branch from `main`.
2. Run `task` to list the commands. `task ci` runs everything the pull-request checks run, and needs no credentials.
3. Open a pull request into `main`. The template asks what changes, what was checked (with screenshots for a change to the screens), and what happens after merge.
4. CI (the Go and Infrastructure checks) must pass before `main` takes the change, and Codex reviews Craig's pull requests. A pull request from a branch in this repository (not Dependabot's) also gets a Plan of dev, showing what deploying it would change in AWS and Cloudflare.
5. Pull requests are squash-merged. A merge to `main` deploys dev at once; production waits for Craig's approval in that Deploy run ([docs/pipeline.md](docs/pipeline.md)).

## Conventions

- **Titles**: one plain sentence in the imperative that says what changes, such as "Offer Start Foundry and Stop Foundry on the player page".
- **Words**: use the terms in [CONTEXT.md](CONTEXT.md), such as Member, Tool and The VIP Lobby, not their listed synonyms.
- **Decisions**: a decision worth recording gets an ADR in [docs/adr/](docs/adr/).
- **Docs**: update README.md and the docs a change touches in the same pull request.
- **Secrets**: never commit them. Secret OpenTofu variables go in `secrets.auto.tfvars`, which is ignored.

## Labels

- Type: `bug`, `enhancement`, `documentation`, `question`, `accessibility`, `security`.
- Triage: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix` ([docs/agents/triage-labels.md](docs/agents/triage-labels.md)).
- Priority: `P0` (emergency) to `P3` (nice to have).
- `dependencies` marks Dependabot's pull requests.
