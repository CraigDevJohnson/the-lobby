# the-lobby

The site behind [craigdevjohnson.com](https://craigdevjohnson.com): a personal front page for the Tools Craig builds for family and friends, with sign-in for the people he invites. Each Tool's backend lives in its own repo; this one holds the site itself.

- [CONTEXT.md](CONTEXT.md): the words used here and what they mean.
- [docs/baseline.md](docs/baseline.md): what is being built and why.
- [docs/adr/](docs/adr/): the decisions that earned a record.
- [docs/infrastructure.md](docs/infrastructure.md): how it runs.
- [infra/README.md](infra/README.md): how to deploy it.

Run `task` to list the commands. The screens (the public welcome, The VIP Lobby, Privacy and the Schedule Downloader at `/soccer`) are a React app in [web/](web/), built by `task ui:build` into `internal/web/dist`, where the Go server embeds them; `task run` builds them first. Building them needs Node 20.19 or later; `task ui:test` needs Node 22.18 or later.

The Schedule Downloader's screen is here; its league access and calendar files are the [soccer](https://github.com/CraigDevJohnson/soccer) backend's (ADR 0001). The site forwards `/api/soccer/*` and the public Session link address `/soccer/link/{token}.ics` to the address in `SOCCER_BACKEND_URL`, signing each request outside a developer's computer (ADR 0006). `SOCCER_BACKEND_SIGNING` overrides that when set: `lambda` signs even locally, to try the deployed backend from a developer's computer with AWS credentials and a region, and `off` never signs. With `task run` it defaults to `http://127.0.0.1:8081`, where the soccer repo's own `task run` listens. When the backend cannot be reached the page says so; nothing is simulated. The code is public under the MIT licence; it is not taking contributions.
