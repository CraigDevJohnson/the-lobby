# What the Minecraft Launcher asks its backend

The Minecraft Launcher at `/minecraft` shows the family server's status, offers Start server when it is stopped and Stop server when it is ready, and helps a Member join from their device ([brief](design/minecraft-brief.md)). The site owns the page, sign-in and the access check; the Minecraft backend owns the server's status and starting and stopping it ([ADR 0001](adr/0001-one-site-separate-tool-backends.md)). The site never reaches the game host, its Crafty Controller or AWS on a Member's behalf.

The backend lives in the [minecraft](https://github.com/CraigDevJohnson/minecraft) repo (`lambda/launcher`, deployed by its `docs/operations/launcher.md`); this page is the contract the site's side (`internal/minecraft`) is built against. Until `MINECRAFT_BACKEND_URL` names a backend, the page says no server is set up for the Launcher, on a developer's computer too. Nothing is simulated.

## How the site calls it

The site checks the Member's `minecraft` grant first (`minecraft-admin` does not include it). Only then does it call the backend, signing each request outside a developer's computer as for the Schedule Downloader ([ADR 0006](adr/0006-signed-requests-to-tool-backends.md)); `MINECRAFT_BACKEND_SIGNING` overrides that the same way `SOCCER_BACKEND_SIGNING` does. The Member is named in `X-Lobby-Member` (their email), which the backend can log as who started or stopped the server.

### `GET /server`

`200` with the server's status:

```json
{
  "name": "Family server",
  "state": "stopped",
  "connection": {
    "java": { "address": "mc.example.net" },
    "bedrock": { "address": "mc.example.net", "port": 19132 }
  }
}
```

- `name`: what the page calls the server.
- `state`: one of
  - `stopped`: not running; the page offers Start server.
  - `starting`: a start is under way.
  - `ready`: players can join. This means the game itself accepts players, not that the host or Crafty is up. The page offers Stop server.
  - `stopping`: shutting down; the page offers nothing.
  - `maintenance`: not available to Members; the page offers nothing.
  - `failed`: a start was confirmed to have failed; the page offers to start again.
  - `unknown`: the backend can't confirm the state. It may add `last_known` (any state but `unknown`), which the page labels as not confirmed now.
- `connection.java`: the Java address, with `port` only when it is not Java's default. Leave it out when there is no verified value; the page then says connection details are unavailable.
- `connection.bedrock`: the Bedrock address and port, both required. Leave it out when either is unverified.

`404` with `{"code": "no_server"}` when no server is configured. Any other answer is shown as "We can't confirm the server status".

### `POST /server/start`

Starts the server if it is `stopped` or `failed`, then answers `202` (or `200`) with the status, in the same shape as `GET /server`. It is idempotent: while the server starts or runs, a second request changes nothing and gets the current status back, so repeated taps, refreshes and several Members starting at once all end up on the same start. It never reports `ready` because a start was accepted.

### `POST /server/stop`

Stops the server if it is `ready`. The body may be empty. It answers:

- `202` with the status, in the same shape as `GET /server` and with `state` `stopping`, when the stop began. The page then asks every 5 seconds, as it does whenever the server is stopping.
- `200` with the current status when nothing changed: the host is already stopped or stopping, or a stop is already under way. As with starting, repeated taps and several Members stopping at once end up on the same stop.
- `409` with `{"code": "players_online", "players": N}` when N players are online. Nothing changes: the backend never kicks players, so until nobody is playing it refuses. The page never sends a force flag; that exists only for an operator workflow in the minecraft repo.
- `409` with `{"code": "players_unknown"}` when the game could not be asked who is online (the status ping failed). Nothing changes.
- `409` with `{"code": "starting"}` while a start is still under way. Nothing changes.
- `404` with `{"code": "no_server"}`, as the other routes.

Anything else means the stop could not be confirmed. A stop drains the server first (a Crafty save, an archive to S3, then the game stops), then stops the host and parks DNS; it takes about ten minutes.

## What the page does with it

The page asks every 5 seconds while the server is starting or stopping and every 30 seconds otherwise, only while it is visible. Stop server is offered only while the server is `ready`, and asks the Member to confirm before anything is sent; a `409` is shown with its reason and changes nothing on the page. If a start or stop request gets no answer, the page says it couldn't tell whether the request arrived and offers a status check, never the same request again. After four minutes in `starting` it says the start is slow and that nothing needs pressing; it never calls a slow start failed.
