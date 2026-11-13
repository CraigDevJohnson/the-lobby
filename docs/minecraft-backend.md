# What the Minecraft Launcher asks its backend

The Minecraft Launcher at `/minecraft` shows each world's status in its own card, offers Start when a world is stopped and Stop when it is ready, and helps a Member join from their device ([brief](design/minecraft-brief.md)). Each world (Survival, Creative, Adventure) runs on its own server at its own address, and starts and stops on its own; several can run at once. The site owns the page, sign-in and the access check; the Minecraft backend owns the server's status and starting and stopping it ([ADR 0001](adr/0001-one-site-separate-tool-backends.md)). The site never reaches the game host, its Crafty Controller or AWS on a Member's behalf.

The backend lives in the [minecraft](https://github.com/CraigDevJohnson/minecraft) repo (`lambda/launcher`, deployed by its `docs/operations/launcher.md`); this page is the contract the site's side (`internal/minecraft`) is built against. Until `MINECRAFT_BACKEND_URL` names a backend, the page says no server is set up for the Launcher, on a developer's computer too. Nothing is simulated.

## How the site calls it

The site checks the Member's grant first: `minecraft` for the `/server` routes and `minecraft-admin` for the `/mods` routes (neither includes the other). Only then does it call the backend, signing each request outside a developer's computer as for Let's Plan Soccer ([ADR 0006](adr/0006-signed-requests-to-tool-backends.md)); `MINECRAFT_BACKEND_SIGNING` overrides that the same way `SOCCER_BACKEND_SIGNING` does. On the `/server` routes the Member is named in `X-Lobby-Member` (their email), which the backend can log as who started or stopped the server. The `/mods` routes carry no Member detail at all: the backend logs a switch as its world, mod and new setting only.

### `GET /server`

`200` with every world's status, in display order:

```json
{
  "name": "Survival",
  "state": "stopped",
  "connection": {
    "java": { "address": "survival.mc.example.net" },
    "bedrock": { "address": "survival.mc.example.net", "port": 19132 }
  },
  "worlds": [
    {
      "id": "survival",
      "name": "Survival",
      "state": "stopped",
      "connection": {
        "java": { "address": "survival.mc.example.net" },
        "bedrock": { "address": "survival.mc.example.net", "port": 19132 }
      }
    },
    {
      "id": "creative",
      "name": "Creative",
      "state": "ready",
      "connection": {
        "java": { "address": "creative.mc.example.net" },
        "bedrock": { "address": "creative.mc.example.net", "port": 19132 }
      }
    }
  ]
}
```

- `worlds`: every world, in the order the page shows them. Each entry is one world's status in the shape described below, plus its `id`, which matches `^[a-z][a-z0-9-]{0,31}$` and is what the page sends to start or stop it. The page leaves out an entry without a usable id, reads an entry whose state it doesn't know as `unknown`, and keeps the first entry for an id listed twice.
- The top-level fields repeat the first world, so a page that does not read `worlds` keeps working. When `worlds` is absent, or holds no usable entry, the page shows the top-level status as one server, as it did before there were worlds, and starts and stops it without a body.
- The backend answers inside the site's 7 seconds, so it reads the worlds side by side.

Each world's status:

- `name`: what the page calls the world.
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

`404` with `{"code": "no_server"}` when no server is configured. Any other answer is shown as "We can't confirm the server status". That includes a redirect, which the site never follows, and an answer that breaks off, runs past 64 KiB or isn't JSON: the site reads each answer whole before passing any of it on.

### Which world a start or stop is for

A start or stop names its world in a JSON body, `{"world": "creative"}`, with `Content-Type: application/json`. The site builds that body itself from the id the page sent, after checking it against the id rule, and signs it with the request; a page body that isn't a JSON object of at most 1 KiB, or names something that isn't a world id, gets `400` with `{"code": "bad_request"}` and the backend is never asked. Nothing else the browser sends reaches the backend, so `force` and `archive` stay with the operator workflows in the minecraft repo. Without a world the site sends no body, which the backend reads as its first world.

Each answers that one world's status, in the shape of one entry above, or one of the codes below. A world the backend doesn't have gets `409` with `{"code": "unknown_world"}`, and nothing changes; the page then reads the list again.

### `POST /server/start`

Starts the world if it is `stopped` or `failed`, then answers `202` (or `200`) with its status. It is idempotent: while the world starts or runs, a second request changes nothing and gets the current status back, so repeated taps, refreshes and several Members starting at once all end up on the same start. It never reports `ready` because a start was accepted.

### `POST /server/stop`

Stops the world if it is `ready`. It answers:

- `202` with its status, with `state` `stopping`, when the stop began. The page then asks every 5 seconds, as it does whenever a world is stopping.
- `200` with the world's current status when nothing changed: its host is already stopped or stopping, or a stop is already under way. As with starting, repeated taps and several Members stopping at once end up on the same stop.
- `409` with `{"code": "players_online", "players": N}` when N players are online. Nothing changes: the backend never kicks players, so until nobody is playing it refuses. The site passes on only the reason and the page says only that someone is playing, so how many never reaches the browser: the Launcher leaves player information out. The page never sends a force flag; that exists only for an operator workflow in the minecraft repo.
- `409` with `{"code": "players_unknown"}` when the game could not be asked who is online (the status ping failed). Nothing changes.
- `409` with `{"code": "starting"}` while a start is still under way. Nothing changes.
- `409` with `{"code": "unknown_world"}`, as above.
- `404` with `{"code": "no_server"}`, as the other routes.

Anything else means the stop could not be confirmed. A stop drains the world's server first (a Crafty save, an archive to S3, then the game stops), then stops its host and parks its DNS; it takes about ten minutes.

## Mods

Each world offers its own mods from a reviewed list, each on or off by default, and a Member with the `minecraft-admin` grant can switch any of them for one world. A switch applies the next time that world starts; a world that is running keeps the mods it started with.

### `GET /mods`

```json
{"worlds": [{"id": "creative", "name": "Creative", "mods": [
  {"id": "fawe", "name": "FastAsyncWorldEdit", "version": "2.16.2",
   "summary": "Bulk building tools: copy, paste, fill and brushes.",
   "source": "https://modrinth.com/mod/tFLdoQMh",
   "default": true, "enabled": false}]}]}
```

The worlds come in the same order as `GET /server`, each with the mods it offers in order of mod id. `default` is whether the mod is on in that world unless switched, and `enabled` whether it is on from the world's next start. World and mod ids follow the world id rule above. The page leaves out a world or mod without a usable id, and a mod without a clear `default` or `enabled`. `source` is the mod's own page and is left out when none is known; the page links the mod's name to it only when it is an `https` address on `modrinth.com` or `github.com`.

### `POST /mods`

The body is `{"world": "creative", "mod": "fawe", "enabled": false}`. As with a start, the site builds it itself from the ids and setting the page sent, after checking both ids against the id rule; anything else, or a body over 1 KiB, gets `400` with `{"code": "bad_request"}` and the backend is never asked. It answers `200` with the whole `GET /mods` shape as the backend now has it, or `409` with `{"code": "unknown_world"}` or `{"code": "unknown_mod"}` when that world doesn't exist or doesn't offer that mod; nothing changes then.

## What the page does with it

The page shows a column per world, in the backend's order, each with that world's address for the device the Member picks. One status check serves every card: the page asks every 5 seconds while any world is starting or stopping and every 30 seconds otherwise, only while it is visible. A start's or stop's answer goes in the card of the world it was sent for. Stop is offered only while a world is `ready`, and asks the Member to confirm in its card before anything is sent; a `409` is shown in that card with its reason and changes nothing. If a start or stop request gets no answer, that card says it couldn't tell whether the request arrived and offers a status check, never the same request again. After four minutes in `starting` a card says the start is slow and that nothing needs pressing; it never calls a slow start failed.

Minecraft Admin at `/minecraft/admin` lists each world's mods, in the backend's order, each with its version, what it does, how it behaves on Bedrock, whether it is on by default, and a switch. One switch is sent at a time, and the page then shows the backend's answer rather than what it asked for, and says in that world's section what changed and that it applies from that world's next start. If a switch is refused or gets no answer, the page reads the list again so it shows only what the backend has; if an unanswered switch's re-read fails too, the page says it can't tell whether the mod was switched until the list can be read. It never says which world is running; the Launcher does that.

The dev site and the production site call Launchers that share the same hosts and records, so a start, a stop or a switch on dev is real.
