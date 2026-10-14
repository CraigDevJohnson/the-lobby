# What Foundry asks its backend

Foundry at `/foundry` is the Tool for Craig's Foundry Virtual Tabletop server. Its player page is the-lobby #24; the page that offers starting and stopping, and Foundry Admin's page, are not built yet. This is the site's side of the server's status, and of starting and stopping it. The site owns the pages, sign-in and the access check; the Foundry backend owns the server's status and the changes to it ([ADR 0001](adr/0001-one-site-separate-tool-backends.md)). The site never reaches the Foundry host or AWS on a Member's behalf.

The backend lives in the [foundry](https://github.com/CraigDevJohnson/foundry) repo: its controller, one Lambda per site environment named `foundry-<environment>-controller`, deployed by that repo's `infra/` (`infra/README.md`, Deploying the controller). Its `docs/status-api.md` is the backend's own copy of this contract; this page is the contract the site's side (`internal/foundry`) is built against. Until `FOUNDRY_BACKEND_URL` names a backend, a Member with the `foundry` grant is told Foundry has no server configured; there is no local default. Nothing is simulated. The controller can be tried on its own: the foundry repo's `task run` serves it at `http://127.0.0.1:8083` (it needs AWS read credentials), and `curl http://127.0.0.1:8083/server` reads the status (its `docs/status-api.md`). It can't be reached through the site on a developer's computer: sign-in is disabled there, so `/api/foundry/server` answers `401 signed_out` and never calls the backend.

## How the site calls it

The site checks the Member's grant first: `foundry` or `foundry-admin` for every route. The player page at `/foundry` reads only the status; any Member with either grant may also start and stop the server (Craig's "Every player" choice), which a later page offers. Only then does it call the backend, signing each request, body included, outside a developer's computer as for Let's Plan Soccer ([ADR 0006](adr/0006-signed-requests-to-tool-backends.md)); `FOUNDRY_BACKEND_SIGNING` overrides that the same way `SOCCER_BACKEND_SIGNING` does. The controller's Function URL uses `AWS_IAM` auth and lets only `site-<environment>-server` call it. The Member is named in `X-Lobby-Member` (their email). Reading the status does not use it, and the backend never logs it for a read; a start or stop logs it as who asked, with the site role AWS verified for the signature.

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
    "foundry": "responding",
    "stop": "none"
  }
}
```

- `state`: one of
  - `stopped`: the host is stopped.
  - `starting`: the host is starting, or it is running and Foundry's join page has not answered yet, less than 10 minutes after the host started.
  - `ready`: the host is running, no stop is under way, and Foundry's join page answered. Players can join. Nothing else is ever `ready`.
  - `stopping`: the host is shutting down, or it is still running while a stop sent through The Lobby shuts Foundry down and powers it off (`checks.stop` is `in_progress`, or `succeeded`, less than 15 minutes after the stop was sent). The page shows `stopping` on a running host then, whatever the join page answers.
  - `unresponsive`: the host is running but Foundry's join page has not answered, 10 minutes or more after the host started (or the backend can't tell when it started). The host is on but Foundry is not answering players: no world open, the license page after a rebuild, or a crash.
  - `unknown`: the backend could not read the host's state. It never guesses one.
- `join_url`: the join page players open. Always present.
- `started_at`: when the host last started, in UTC (RFC 3339). Present only while `checks.host` is `pending` or `running` and EC2 gave a start time; the page must not count on it.
- `checked_at`: when the backend finished looking, in UTC (RFC 3339).
- `checks`: what the backend saw, for a detail line and for Craig:
  - `host`: the EC2 instance's state: `pending`, `running`, `stopping`, `stopped`, `shutting-down` or `unknown`.
  - `agent`: whether Systems Manager can reach the host: `online`, `offline`, `unknown` or `not_checked`.
  - `foundry`: what the join page answered: `responding`, `unexpected_response`, `unreachable`, `unknown` or `not_checked`.
  - `stop`: the latest stop sent since the host last started: `none`, `in_progress`, `succeeded` (Foundry is down and the host powers off a minute later), `failed` (it failed, timed out or was cancelled, and the host stays on), `unknown` or `not_checked`.

The answer never carries reasons, provider errors, the join page's content, instance IDs or IP addresses.

`404` with `{"code": "no_server"}` when the host is terminated or gone, for example in the middle of a rebuild. Any other answer is shown as "Foundry's status can't be checked right now".

The backend looks for at most 6 seconds. The site gives up on it after 7 and answers within 8, so a slow look is reported as unavailable, never cut off. The backend also answers `GET /healthz` with `204` for its own post-deploy check; the site never calls it.

### `POST /server/start`

Starts a stopped host. The site sends no body.

- `202` with the status, in the shape of `GET /server`, when EC2 accepted the start: `state` `starting`, the checks it could not take yet `not_checked`, and no `started_at` yet. Foundry starts with the host.
- `200` with the current status when there is nothing to start: the host is already starting or running with no stop under way. A start never restarts Foundry.
- `409` with `{"code": "stopping"}` while the host is stopping or a stop is under way. It can be started once it is `stopped`.
- `404` with `{"code": "no_server"}` as for `GET /server`.
- `503` with `{"code": "status_unknown"}` or `{"code": "start_failed"}` when the backend could not read the host or EC2 refused the start. Nothing is retried.

### `POST /server/stop`

Shuts Foundry down as the foundry repo's `task host:stop` does, then powers the host off; EC2 stops it a minute later. The backend can't see who is playing, so the body must be the Member's confirmation that nobody is, `{"nobody_playing": true}`. The site sends exactly that, and only after the page sent it.

- `202` with the status, `state` `stopping` and `checks.stop` `in_progress`, when the stop was sent. The status follows it from there: `stopping`, then `stopped`. If the shutdown fails, the host stays on and `checks.stop` turns `failed`; Craig gets an alert.
- `200` with the current status when there is nothing to stop: the host is already stopped or stopping, or a stop is under way. So a stop asked for after another was sent sends nothing; two at the same moment can each send one, and the second finds Foundry already down.
- `409` with `{"code": "starting"}` while EC2 reports the host `pending` (`checks.host`), or `{"code": "agent_offline"}` when the host's Systems Manager agent is not connected yet, for example in the first minute after it booted. Nothing was sent; the page can ask again shortly.
- `400` with `{"code": "confirmation_required"}` without the confirmation. The site never sends a stop without it.
- `404` with `{"code": "no_server"}` as for `GET /server`.
- `503` with `{"code": "status_unknown"}` or `{"code": "stop_failed"}` when the backend could not read the host or the stop recorded on it, or Systems Manager refused the command. Nothing is retried.

A stop takes no backup; it is not a game session's end. Both site environments' backends act on the one host, so a Member on the dev site stops the host production shows.

## What the site answers the page

Every route always answers JSON with `Cache-Control: no-store`. Nothing about the server reaches anyone without the route's grant, and the backend is never called for them.

### `GET /api/foundry/server`

The status, for Members with the `foundry` grant or the `foundry-admin` grant.

| Answer | When |
|---|---|
| `200` with the backend's status | The backend answered `200` in full, as JSON, within 64 KiB. Passed on as it is. |
| `404` `no_server` | The backend answered `404` with the code `no_server`. Only `{"code": "no_server"}` comes through; any other `404` is `tool_unavailable`. |
| `401` `signed_out` | No Sign-in session, or one that no longer counts. |
| `403` `no_access` | A Member with neither grant. |
| `503` `access_unavailable` | Access could not be checked. |
| `503` `not_configured` | `FOUNDRY_BACKEND_URL` is unset. |
| `503` `tool_unavailable` | The backend could not be reached, AWS refused the site's signature (`401` or `403`), the backend answered anything else (a redirect, which the site never follows, or a `404` other than `no_server` included), or its answer broke off, ran past 64 KiB or was not exactly one JSON value. The site logs why. When `FOUNDRY_JOIN_URL` is set, the answer also carries it as `join_url`, so a failed status check never keeps a Member with either grant from joining. |

The site's own answers are `{"code": "...", "message": "..."}`. Only `tool_unavailable` adds `join_url`, and only when `FOUNDRY_JOIN_URL` is set: that answer comes after the grant check, so nobody without either grant is ever given it. `FOUNDRY_JOIN_URL` is the same join page as the controller's `FOUNDRY_HTTPS_URL` and is held to the same rule, exactly `https://host[:port]/join`; any other value stops the site from starting.

### `POST /api/foundry/server/start` and `POST /api/foundry/server/stop`

Starting and stopping, for Members with the `foundry` grant or the `foundry-admin` grant, in this order of checks. Neither offers `join_url`: the status does.

| Answer | When |
|---|---|
| `403` `cross_site` | The browser says the request came from another site (`Sec-Fetch-Site` other than `same-origin`). Checked first, so another site can't change the server through a Member's browser. |
| `401` `signed_out`, `403` `no_access`, `503` `access_unavailable` | As for the status. |
| `400` `confirmation_required` | A stop whose body is not a JSON object of at most 1 KiB with `nobody_playing` set to `true`. The backend is never called. |
| `503` `not_configured` | `FOUNDRY_BACKEND_URL` is unset. |
| `202` or `200` with the backend's status | The backend began the change, or there was nothing to change. Passed on as it is, as JSON within 64 KiB. |
| `404` `no_server` | As for the status. |
| `409` with only the backend's `code` | The backend refused for a reason that route gives: `stopping` for a start, `starting` or `agent_offline` for a stop. Nothing else from the refusal comes through, and any other reason is `tool_unavailable`. |
| `503` `tool_unavailable` | Anything else, as for the status, including a `409` without one of those codes, the backend's own `503`s and `400`. The message is "Foundry's server can't be changed right now". The page can't tell whether the change happened, so it reads the status rather than asking again. |
