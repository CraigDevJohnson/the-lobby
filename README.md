# the-lobby

The site behind [craigdevjohnson.com](https://craigdevjohnson.com): a personal front page for the Tools Craig builds for family and friends, with sign-in for the people he invites. Each Tool's backend lives in its own repo; this one holds the site itself.

- [CONTEXT.md](CONTEXT.md): the words used here and what they mean.
- [docs/baseline.md](docs/baseline.md): what is being built and why.
- [docs/adr/](docs/adr/): the decisions that earned a record.
- [docs/infrastructure.md](docs/infrastructure.md): how it runs.
- [infra/README.md](infra/README.md): how to deploy it.

Run `task` to list the commands. The screens (the public welcome, The VIP Lobby, Privacy, Let's Plan Soccer at `/soccer` and the Minecraft Launcher at `/minecraft`) are a React app in [web/](web/), built by `task ui:build` into `internal/web/dist`, where the Go server embeds them; `task run` builds them first. Building them needs Node 20.19 or later; `task ui:test` needs Node 22.18 or later.

The Let's Plan Soccer screen is here; its league access and calendar files are the [soccer](https://github.com/CraigDevJohnson/soccer) backend's (ADR 0001). The site forwards `/api/soccer/*` and the public Session link address `/soccer/link/{token}.ics` to the address in `SOCCER_BACKEND_URL`, signing each request outside a developer's computer (ADR 0006). `SOCCER_BACKEND_SIGNING` overrides that when set: `lambda` signs even locally, to try the deployed backend from a developer's computer with AWS credentials and a region, and `off` never signs. With `task run` it defaults to `http://127.0.0.1:8081`, where the soccer repo's own `task run` listens. When the backend cannot be reached the page says so; nothing is simulated. The code is public under the MIT licence; it is not taking contributions.

The Minecraft Launcher's page and access check are here; the server's status and Start server belong to the [minecraft](https://github.com/CraigDevJohnson/minecraft) repo's launcher backend. [docs/minecraft-backend.md](docs/minecraft-backend.md) is the contract the site calls at `MINECRAFT_BACKEND_URL` (signed like the soccer backend, overridden by `MINECRAFT_BACKEND_SIGNING`). Until it is set, the page says no server is set up for the Launcher.

Foundry's access check is here, and its page is not built yet; the Foundry server's status belongs to the [foundry](https://github.com/CraigDevJohnson/foundry) repo's controller. [docs/foundry-backend.md](docs/foundry-backend.md) is the contract the site calls at `FOUNDRY_BACKEND_URL` (signed like the soccer backend, overridden by `FOUNDRY_BACKEND_SIGNING`) for `/api/foundry/server`. Until it is set, a Member with the `foundry` grant is told Foundry has no server configured. The foundry repo's `task run` serves the controller locally for `curl`; the site's route can't reach it there, because sign-in is disabled on a developer's computer and it answers `401 signed_out`.
