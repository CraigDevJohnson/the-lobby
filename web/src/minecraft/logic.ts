// What the Minecraft Launcher shows, worked out from what the backend says
// (docs/minecraft-backend.md). Nothing here guesses: a state the backend did
// not send is "unknown", and a connection value it did not send is absent.

export const STATES = ["stopped", "starting", "ready", "stopping", "maintenance", "failed", "unknown"] as const;
export type ServerState = (typeof STATES)[number];

export type Endpoint = { address: string; port?: number };

// Java and Bedrock each have their own address; Bedrock always needs a port.
export type Connection = { java?: Endpoint; bedrock?: Endpoint & { port: number } };

export type ServerStatus = {
  name: string;
  state: ServerState;
  // With "unknown" only: the last state the backend confirmed, if any.
  lastKnown?: Exclude<ServerState, "unknown">;
  connection: Connection;
};

// The name used when the backend could not be asked for the real one.
export const UNNAMED = "Minecraft server";

function isState(v: unknown): v is ServerState {
  return typeof v === "string" && (STATES as readonly string[]).includes(v);
}

function port(v: unknown): number | undefined {
  return typeof v === "number" && Number.isInteger(v) && v >= 1 && v <= 65535 ? v : undefined;
}

function endpoint(v: unknown): Endpoint | undefined {
  if (!v || typeof v !== "object") return undefined;
  const { address, port: p } = v as { address?: unknown; port?: unknown };
  if (typeof address !== "string") return undefined;
  const a = address.trim();
  if (!a || a.length > 255 || /\s/.test(a)) return undefined;
  const n = port(p);
  return n === undefined ? { address: a } : { address: a, port: n };
}

// parseStatus reads the backend's status. Anything unreadable is null, which
// the page treats like a backend that did not answer.
export function parseStatus(raw: unknown): ServerStatus | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as { name?: unknown; state?: unknown; last_known?: unknown; connection?: unknown };
  if (!isState(r.state)) return null;
  const name = typeof r.name === "string" && r.name.trim() ? r.name.trim() : UNNAMED;
  const status: ServerStatus = { name, state: r.state, connection: {} };
  if (r.state === "unknown" && isState(r.last_known) && r.last_known !== "unknown") {
    status.lastKnown = r.last_known;
  }
  const c = (r.connection && typeof r.connection === "object" ? r.connection : {}) as {
    java?: unknown;
    bedrock?: unknown;
  };
  const java = endpoint(c.java);
  if (java) status.connection.java = java;
  const bedrock = endpoint(c.bedrock);
  if (bedrock?.port !== undefined) status.connection.bedrock = { address: bedrock.address, port: bedrock.port };
  return status;
}

// A World runs on its own server, with its own address, and starts and stops
// on its own (docs/minecraft-backend.md). Its id is what the page sends to
// start or stop it. A backend that lists no worlds has one server, which the
// page shows the same way without an id and addresses without a body.
export type World = ServerStatus & { id?: string };

// The ids the backend may give a world: what the site's proxy checks too.
export const WORLD_ID = /^[a-z][a-z0-9-]{0,31}$/;

// parseWorlds reads GET /server: every world in the backend's order, or the
// one server when it lists none. A world without a usable id is left out,
// since the page could not start or stop it, and one whose state the page
// doesn't know reads as unconfirmed rather than taking down the others. When
// no listed world is usable the page falls back to the top-level status,
// which repeats the first world. Unreadable is null, as for parseStatus.
export function parseWorlds(raw: unknown): World[] | null {
  const listed = raw && typeof raw === "object" ? (raw as { worlds?: unknown }).worlds : undefined;
  const worlds: World[] = [];
  if (Array.isArray(listed)) {
    for (const entry of listed) {
      const id = entry && typeof entry === "object" ? (entry as { id?: unknown }).id : undefined;
      if (typeof id !== "string" || !WORLD_ID.test(id) || worlds.some((w) => w.id === id)) continue;
      const state = (entry as { state?: unknown }).state;
      const status = parseStatus(isState(state) ? entry : { ...entry, state: "unknown" });
      if (status) worlds.push({ id, ...status });
    }
  }
  if (worlds.length) return worlds;
  const status = parseStatus(raw);
  return status ? [status] : null;
}

// worldKey names a world on the page: its id, or "" for the one server.
export const worldKey = (w: World) => w.id ?? "";

// unconfirmed is what the page shows when the backend could not be asked:
// the last status it gave, no longer vouched for.
export function unconfirmed<S extends ServerStatus>(previous: S): S;
export function unconfirmed(previous: null): ServerStatus;
export function unconfirmed(previous: ServerStatus | null): ServerStatus {
  if (!previous) return { name: UNNAMED, state: "unknown", connection: {} };
  const lastKnown = previous.state === "unknown" ? previous.lastKnown : previous.state;
  return { ...previous, state: "unknown", ...(lastKnown ? { lastKnown } : {}) };
}

// allUnconfirmed is every world on the page, no longer vouched for, or the
// one unnamed server when the page has shown none yet.
export function allUnconfirmed(previous: readonly World[] | null): World[] {
  return previous?.length ? previous.map((w) => unconfirmed(w)) : [unconfirmed(null)];
}

// javaAddress is what a Java player types: the address, with the port only
// when the server uses one other than Java's own default.
export function javaAddress(e: Endpoint): string {
  if (e.port === undefined) return e.address;
  const host = e.address.includes(":") && !e.address.startsWith("[") ? `[${e.address}]` : e.address;
  return `${host}:${e.port}`;
}

export const canStart = (s: ServerState) => s === "stopped" || s === "failed";

// Only a ready server offers Stop. The backend then stops it only when nobody
// is playing; it never kicks anyone, and the page never asks it to.
export const canStop = (s: ServerState) => s === "ready";

// Why the backend would not stop the server, or start or stop a world
// (docs/minecraft-backend.md). Nothing changed when it says so. The page
// never says how many are playing, even when the backend does: the Launcher
// leaves player information out (docs/design/minecraft-brief.md).
// unknown_world means the backend no longer has the world the page showed.
export type Refusal =
  | { code: "players_online" }
  | { code: "players_unknown" }
  | { code: "starting" }
  | { code: "unknown_world" };

// parseRefusal reads the backend's 409. A reason the page does not know is
// null, which the page treats like a request it could not confirm.
export function parseRefusal(raw: unknown): Refusal | null {
  if (!raw || typeof raw !== "object") return null;
  const { code } = raw as { code?: unknown };
  switch (code) {
    case "players_online":
      return { code: "players_online" };
    case "players_unknown":
      return { code: "players_unknown" };
    case "starting":
      return { code: "starting" };
    case "unknown_world":
      return { code: "unknown_world" };
    default:
      return null;
  }
}

// refusalMessage is what the page says in a world's card when the backend
// would not start or stop it. Without a world's name it speaks of the one
// server, as the page did before there were worlds.
export function refusalMessage(r: Refusal, world?: string): string {
  switch (r.code) {
    case "players_online":
      return world
        ? `Someone is still playing in ${world}. Ask them to leave first.`
        : "Someone is still playing. Ask them to leave first.";
    case "players_unknown":
      return `${world ?? "The server"} could not be asked who is playing. Try again in a moment.`;
    case "starting":
      return `${world ?? "The server"} is still starting. Wait for it to be ready.`;
    case "unknown_world":
      return "The server doesn’t have this world any more. The list has been checked again.";
  }
}

export type Description = {
  label: string; // the state in a few words, beside the status mark
  message: string; // what it means for joining
  mark: "off" | "busy" | "on" | "problem" | "unknown";
};

const LABELS: Record<ServerState, string> = {
  stopped: "Stopped",
  starting: "Starting",
  ready: "Ready to join",
  stopping: "Stopping",
  maintenance: "Down for maintenance",
  failed: "Didn’t start",
  unknown: "Status unknown",
};

// describe says a world's state in words. Without a world's name it speaks
// of the one server, as the page did before there were worlds.
export function describe(s: ServerStatus, world?: string): Description {
  const it = world ?? "the server";
  const It = world ?? "The server";
  switch (s.state) {
    case "stopped":
      return { label: LABELS.stopped, message: `Start ${it} before joining.`, mark: "off" };
    case "starting":
      return { label: LABELS.starting, message: `Starting ${it}…`, mark: "busy" };
    case "ready":
      return {
        label: LABELS.ready,
        message: hasDetails(s.connection)
          ? "Open Minecraft and join with the details here."
          : "Its address didn’t come through, so there’s nothing to copy yet. This page keeps checking.",
        mark: "on",
      };
    case "stopping":
      return {
        label: LABELS.stopping,
        message: `${It} is shutting down. It can be started again once it has stopped.`,
        mark: "busy",
      };
    case "maintenance":
      return {
        label: LABELS.maintenance,
        message: `${It} can’t be started right now. Please check back later.`,
        mark: "problem",
      };
    case "failed":
      return {
        label: LABELS.failed,
        message: `${It} didn’t start. You can try starting it again.`,
        mark: "problem",
      };
    case "unknown": {
      const whose = world ? `${world}’s status` : "the server status";
      return {
        label: LABELS.unknown,
        message: s.lastKnown
          ? `We can’t confirm ${whose}. Last known: ${LABELS[s.lastKnown].toLowerCase()}, not confirmed now.`
          : `We can’t confirm ${whose}.`,
        mark: "unknown",
      };
    }
  }
}

// announcement is what a screen reader hears when a world's state changes,
// or nothing when it did not.
export function announcement(before: ServerState | undefined, after: ServerStatus, world?: string): string {
  if (before === after.state) return "";
  const It = world ?? "The server";
  switch (after.state) {
    case "ready":
      return `${It} is ready to join.`;
    case "starting":
      return `${It} is starting.`;
    case "stopped":
      return before === undefined ? "" : `${It} has stopped.`;
    case "unknown":
      return world ? `We can’t confirm ${world}’s status.` : "We can’t confirm the server status.";
    default:
      return world ? `${world}: ${describe(after).label}.` : `Server status: ${describe(after).label}.`;
  }
}

// How long the page waits before asking again: often while any world is on
// its way somewhere, rarely otherwise.
const busyState = (s: ServerState) => s === "starting" || s === "stopping";
export const pollDelay = (worlds: readonly ServerStatus[]) =>
  worlds.some((w) => busyState(w.state)) ? 5_000 : 30_000;

// After this long in "starting" as seen from the page, it says the start is
// slow and that nothing needs pressing. It never calls a slow start failed.
export const SLOW_START_MS = 4 * 60_000;

// hasDetails says whether the backend sent anything a player can join with.
export const hasDetails = (c: Connection) => Boolean(c.java || c.bedrock);

const LEADS: Record<ServerState, string> = {
  stopped: "Start the server, then join from Minecraft.",
  starting: "Get set up to join while the server starts.",
  ready: "The server is up. Copy the address and join from Minecraft.",
  stopping: "Joining is closed while the server shuts down.",
  maintenance: "Joining is closed while the server is down for maintenance.",
  failed: "The server needs another start before anyone can join.",
  unknown: "Check the server’s status before starting it or joining.",
};

// names lists worlds in prose: "Creative", "Survival and Creative",
// "Survival, Creative and Adventure".
export function names(worlds: readonly { name: string }[]): string {
  const n = worlds.map((w) => w.name);
  return n.length < 2 ? (n[0] ?? "") : `${n.slice(0, -1).join(", ")} and ${n[n.length - 1]}`;
}

// lead is the line under the page title: what to do now. Until the page has
// a server to show, it says what the page is for. With several worlds it
// names the ones that are up, or on their way up, and otherwise says how
// the worlds work.
export function lead(worlds: readonly World[] | null): string {
  if (!worlds?.length) return LEADS.stopped;
  if (worlds.length === 1 && !worlds[0].id) {
    const s = worlds[0];
    if (s.state === "ready" && !hasDetails(s.connection))
      return "The server is up, but there’s no address to join with yet.";
    return LEADS[s.state];
  }
  const up = worlds.filter((w) => w.state === "ready" && hasDetails(w.connection));
  if (up.length === 1) return `${up[0].name} is up. Copy its address and join from Minecraft.`;
  if (up.length > 1) return `${names(up)} are up. Copy an address and join from Minecraft.`;
  const starting = worlds.filter((w) => w.state === "starting");
  if (starting.length)
    return `Get set up to join while ${names(starting)} ${starting.length === 1 ? "starts" : "start"}.`;
  if (worlds.every((w) => w.state === "unknown")) return "Check the worlds’ status before starting one or joining.";
  return "Each world runs on its own. Start the one you want, then join it from Minecraft.";
}

// RECHECKED is what the page says when Check status again brings back the
// same unconfirmed answer, so pressing it is never met with silence.
export const RECHECKED = "Checked just now. We still can’t confirm it.";

// STILL_NO_ACCESS is what the page says when Retry still can't check access.
export const STILL_NO_ACCESS = "Checked just now. We still couldn’t check your access.";

// SLOW_START is the note for a start that has run past SLOW_START_MS.
export const SLOW_START =
  "This is taking longer than usual. It’s still starting, and there’s nothing to press. If it isn’t ready in a few more minutes, let Craig know.";

// pageTitle puts the state first, so a Member waiting in another tab sees it:
// the one server's state, or with several worlds the ones that are up, else
// the ones starting.
export function pageTitle(worlds: readonly World[] | null): string {
  const page = "Minecraft Launcher · The Lobby";
  if (!worlds?.length) return page;
  if (worlds.length === 1 && !worlds[0].id) return `${LABELS[worlds[0].state]} · ${page}`;
  const up = worlds.filter((w) => w.state === "ready");
  if (up.length) return `${names(up)} ${up.length === 1 ? "is" : "are"} ready · ${page}`;
  const starting = worlds.filter((w) => w.state === "starting");
  if (starting.length) return `${names(starting)} starting · ${page}`;
  return page;
}

// segments splits a connection value after each "." and ":", so on screen it
// wraps only between its parts. Joined, the parts are the value exactly.
export function segments(value: string): string[] {
  return value.match(/[^.:]*[.:]|[^.:]+$/g) ?? [value];
}
