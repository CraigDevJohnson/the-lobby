# What Foundry asks its backend

Foundry at `/foundry` is the Tool for Craig's Foundry Virtual Tabletop server. Its page is not built yet and its card in The VIP Lobby is not ready; the site's side of the server's status is. The site owns the page, sign-in and the access check; the Foundry backend owns the server's status ([ADR 0001](adr/0001-one-site-separate-tool-backends.md)). The site never reaches the Foundry host or AWS on a Member's behalf.

The backend lives in the [foundry](https://github.com/CraigDevJohnson/foundry) repo: its controller, one Lambda per site environment named `foundry-<environment>-controller`, deployed by that repo's `infra/` (`infra/README.md`, Deploying the controller). Its `docs/status-api.md` is the backend's own copy of this contract; this page is the contract the site's side (`internal/foundry`) is built against. Until `FOUNDRY_BACKEND_URL` names a backend, a Member with the `foundry` grant is told Foundry has no server configured; there is no local default. Nothing is simulated. The controller can be tried on its own: the foundry repo's `task run` serves it at `http://127.0.0.1:8083` (it needs AWS read credentials), and `curl http://127.0.0.1:8083/server` reads the status (its `docs/status-api.md`). It can't be reached through the site on a developer's computer: sign-in is disabled there, so `/api/foundry/server` answers `401 signed_out` and never calls the backend.

## How the site calls it

The site checks the Member's `foundry` grant first (`foundry-admin` does not include it). Only then does it call the backend, signing each request outside a developer's computer as for Let's Plan Soccer ([ADR 0006](adr/0006-signed-requests-to-tool-backends.md)); `FOUNDRY_BACKEND_SIGNING` overrides that the same way `SOCCER_BACKEND_SIGNING` does. The controller's Function URL uses `AWS_IAM` auth and lets only `site-<environment>-server` call it. The Member is named in `X-Lobby-Member` (their email). Reading the status does not use it, and the backend never logs it for a read; it is there for actions that change the server, if they come.

### `GET /server`

`200` with the server's status:

```json
{
  "state": "ready",
  "join_url": "https://foundry.craigdevjohnson.com:30000/join",
  "started_at": "2026-10-13T18:00:00Z",
  "checked_at": "2026-10-13T18:03:12Z",
  "checks": {
    "host": "running",
    "agent": "online",
    "foundry": "responding"
  }
}
```

- `state`: one of
  - `stopped`: the host is stopped.
  - `starting`: the host is starting, or it is running and Foundry's join page has not answered yet, less than 10 minutes after the host started.
  - `ready`: the host is running and Foundry's join page answered. Players can join. Nothing else is ever `ready`.
  - `stopping`: the host is shutting down.
  - `unresponsive`: the host is running but Foundry's join page has not answered, 10 minutes or more after the host started (or the backend can't tell when it started). The host is on but Foundry is not answering players: no world open, the license page after a rebuild, or a crash.
  - `unknown`: the backend could not read the host's state. It never guesses one.
- `join_url`: the join page players open. Always present.
- `started_at`: when the host last started, in UTC (RFC 3339). Present only while `checks.host` is `pending` or `running` and EC2 gave a start time; the page must not count on it.
- `checked_at`: when the backend finished looking, in UTC (RFC 3339).
- `checks`: what the backend saw, for a detail line and for Craig:
  - `host`: the EC2 instance's state: `pending`, `running`, `stopping`, `stopped`, `shutting-down` or `unknown`.
  - `agent`: whether Systems Manager can reach the host: `online`, `offline`, `unknown` or `not_checked`.
  - `foundry`: what the join page answered: `responding`, `unexpected_response`, `unreachable`, `unknown` or `not_checked`.

The answer never carries reasons, provider errors, the join page's content, instance IDs or IP addresses.

`404` with `{"code": "no_server"}` when the host is terminated or gone, for example in the middle of a rebuild. Any other answer is shown as "Foundry's status can't be checked right now".

The backend looks for at most 6 seconds. The site gives up on it after 7 and answers within 8, so a slow look is reported as unavailable, never cut off. The backend also answers `GET /healthz` with `204` for its own post-deploy check; the site never calls it.

### Starting and stopping

Not part of the contract yet. Whether the page will start and stop the server, or run whole game sessions, is not decided; until then the foundry repo's Host control workflow starts and stops the host, and the site has no Foundry route that changes anything. If they come, they follow the Minecraft Launcher's shape (`POST /server/start`, `POST /server/stop`, [docs/minecraft-backend.md](minecraft-backend.md)), and the backend can log the Member named in `X-Lobby-Member` as who asked.

## What the site answers the page

`GET /api/foundry/server` always answers JSON with `Cache-Control: no-store`. Nothing about the server reaches anyone without the `foundry` grant, and the backend is never called for them.

| Answer | When |
|---|---|
| `200` with the backend's status | The backend answered `200` in full, within 64 KiB. Passed on as it is. |
| `404` with the backend's body | The backend answered `404`. The page reads `no_server` from `code`; any other code means the status can't be checked. |
| `401` `signed_out` | No Sign-in session, or one that no longer counts. |
| `403` `no_access` | A Member without the `foundry` grant. |
| `503` `access_unavailable` | Access could not be checked. |
| `503` `not_configured` | `FOUNDRY_BACKEND_URL` is unset. |
| `503` `tool_unavailable` | The backend could not be reached, AWS refused the site's signature (`401` or `403`), the backend answered anything else (a redirect included, which the site never follows), or its answer broke off or ran past 64 KiB. The site logs why. When `FOUNDRY_JOIN_URL` is set, the answer also carries it as `join_url`, so a failed status check never keeps a Member with the grant from joining. |

The site's own answers are `{"code": "...", "message": "..."}`. Only `tool_unavailable` adds `join_url`, and only when `FOUNDRY_JOIN_URL` is set: that answer comes after the grant check, so nobody without the `foundry` grant is ever given it. `FOUNDRY_JOIN_URL` is the same join page as the controller's `FOUNDRY_HTTPS_URL` and is held to the same rule, exactly `https://host[:port]/join`; any other value stops the site from starting.
