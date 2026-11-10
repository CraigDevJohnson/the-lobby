// Calls to the site's /api/minecraft routes. The site checks the Member's
// access first, then asks the Minecraft backend; every status shown on the
// page comes from there.
import { parseRefusal, parseStatus, parseWorlds, type Refusal, type ServerStatus, type World } from "./logic";

// Why the page has no status to show.
export type Problem =
  | { kind: "signed-out" } // no Sign-in session, or it expired
  | { kind: "denied" } // a Member without the Launcher's grant
  | { kind: "access-failed" } // the site couldn't check access
  | { kind: "not-configured" } // no server is set up for the Launcher
  | { kind: "unavailable" }; // the backend didn't answer, or not readably

type Reply = { kind: "ok"; body: unknown } | ({ kind: "refused" } & Refusal) | Problem;

async function call(path: string, init?: RequestInit): Promise<Reply> {
  let res: Response;
  try {
    res = await fetch(`/api/minecraft${path}`, {
      credentials: "same-origin",
      ...init,
      headers: { Accept: "application/json", ...init?.headers },
    });
  } catch {
    return { kind: "unavailable" };
  }
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // Not one of the site's own answers.
  }
  if (res.ok) return { kind: "ok", body };
  if (res.status === 409) {
    const refusal = parseRefusal(body);
    return refusal ? { kind: "refused", ...refusal } : { kind: "unavailable" };
  }
  const code = (body as { code?: unknown } | null)?.code;
  switch (code) {
    case "signed_out":
      return { kind: "signed-out" };
    case "no_access":
      return { kind: "denied" };
    case "access_unavailable":
      return { kind: "access-failed" };
    case "not_configured":
    case "no_server":
      return { kind: "not-configured" };
    default:
      return { kind: "unavailable" };
  }
}

// What GET /server gives: every world, or why there are none to show.
export type StatusAnswer = { kind: "worlds"; worlds: World[] } | Problem;

// What a start or stop gives: that one world's status, a refusal with its
// reason, or why there is no status.
export type WorldAnswer = { kind: "status"; status: ServerStatus } | ({ kind: "refused" } & Refusal) | Problem;

export async function getStatus(): Promise<StatusAnswer> {
  const reply = await call("/server");
  if (reply.kind === "ok") {
    const worlds = parseWorlds(reply.body);
    return worlds ? { kind: "worlds", worlds } : { kind: "unavailable" };
  }
  // Reading the status is never refused; a refusal is an answer the page
  // can't act on, so it counts as the backend not answering readably.
  return reply.kind === "refused" ? { kind: "unavailable" } : reply;
}

// A start or stop names its world in the body. The one server of a backend
// that lists no worlds is addressed without one, as before there were
// worlds.
function post(world: string | undefined): RequestInit {
  if (!world) return { method: "POST" };
  return { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ world }) };
}

function toWorld(reply: Reply, refusals: readonly Refusal["code"][]): WorldAnswer {
  if (reply.kind === "ok") {
    const status = parseStatus(reply.body);
    return status ? { kind: "status", status } : { kind: "unavailable" };
  }
  if (reply.kind === "refused" && !refusals.includes(reply.code)) return { kind: "unavailable" };
  return reply;
}

// Starting is idempotent at the backend: a second request while the world
// starts or runs gets the same status back. It is refused only for a world
// the backend no longer has.
export const startWorld = async (world?: string) => toWorld(await call("/server/start", post(world)), ["unknown_world"]);

// Stopping is refused, with why, while anyone is playing; nothing changes
// then. A second request while the world stops gets the same status back.
export const stopWorld = async (world?: string) =>
  toWorld(await call("/server/stop", post(world)), ["players_online", "players_unknown", "starting", "unknown_world"]);
