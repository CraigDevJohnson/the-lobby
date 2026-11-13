// Minecraft Admin's mods: each world's list from the backend, read and
// switched through the site's /api/minecraft/mods routes, which check the
// minecraft-admin grant first (docs/minecraft-backend.md). A switch applies
// at that world's next start. Nothing here guesses: an entry the page can't
// read is left out rather than shown with made-up values.
import type { Problem } from "./api";

// The ids the site proxy accepts for a world or a mod. Written out here, as
// in logic.ts, so the tests can load this file on its own.
const ID = /^[a-z][a-z0-9-]{0,31}$/;

export type Mod = {
  id: string;
  name: string;
  version: string;
  summary: string;
  bedrock: string; // how it behaves for Bedrock players
  default: boolean; // whether it is on in this world unless switched
  enabled: boolean; // whether it is on from the world's next start
};

export type ModWorld = { id: string; name: string; mods: Mod[] };

const text = (v: unknown) => (typeof v === "string" ? v.trim() : "");

function mod(raw: unknown): Mod | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  const id = text(r.id);
  if (!ID.test(id) || typeof r.enabled !== "boolean" || typeof r.default !== "boolean") return null;
  return {
    id,
    name: text(r.name) || id,
    version: text(r.version),
    summary: text(r.summary),
    bedrock: text(r.bedrock),
    default: r.default,
    enabled: r.enabled,
  };
}

// parseMods reads GET /mods (and a switch's answer, which has the same
// shape): the worlds in the backend's order, each with its mods. A world or
// mod without a usable id is left out, as is a mod without a clear on or
// off. Unreadable is null, which the page treats like a backend that did not
// answer.
export function parseMods(raw: unknown): ModWorld[] | null {
  const listed = raw && typeof raw === "object" ? (raw as { worlds?: unknown }).worlds : undefined;
  if (!Array.isArray(listed)) return null;
  const worlds: ModWorld[] = [];
  for (const entry of listed) {
    if (!entry || typeof entry !== "object") continue;
    const e = entry as { id?: unknown; name?: unknown; mods?: unknown };
    const id = text(e.id);
    if (!ID.test(id) || worlds.some((w) => w.id === id)) continue;
    const mods: Mod[] = [];
    for (const m of Array.isArray(e.mods) ? e.mods : []) {
      const read = mod(m);
      if (read && !mods.some((x) => x.id === read.id)) mods.push(read);
    }
    worlds.push({ id, name: text(e.name) || id, mods });
  }
  return worlds;
}

// A mod's row in the admin grid: one line per mod however many worlds offer
// it, with a cell per world in the backend's world order (null where that
// world doesn't offer it). Its words come from the first world listing it;
// the version is shown only when every world has the same one.
export type ModLine = {
  id: string;
  name: string;
  version: string;
  summary: string;
  bedrock: string;
  cells: (Mod | null)[];
};

export function modGrid(worlds: ModWorld[]): ModLine[] {
  const lines: ModLine[] = [];
  const byId = new Map<string, ModLine>();
  worlds.forEach((w, i) => {
    for (const m of w.mods) {
      let line = byId.get(m.id);
      if (!line) {
        const { id, name, version, summary, bedrock } = m;
        line = { id, name, version, summary, bedrock, cells: worlds.map(() => null) };
        byId.set(m.id, line);
        lines.push(line);
      } else if (line.version !== m.version) {
        line.version = "";
      }
      line.cells[i] = m;
    }
  });
  return lines;
}

// changed says whether a mod is set differently from its default in any
// world that offers it.
export const changed = (line: ModLine) => line.cells.some((m) => m !== null && m.enabled !== m.default);

// findMods narrows the grid to what was asked for: lines whose name, id or
// summary contain every word typed, and, with changedOnly, only those set
// differently from their default. keep lists lines that stay regardless, so
// a row just switched doesn't vanish from under the pointer.
export function findMods(lines: ModLine[], query: string, changedOnly: boolean, keep: ReadonlySet<string> = new Set()) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  return lines.filter((l) => {
    if (keep.has(l.id)) return true;
    if (changedOnly && !changed(l)) return false;
    const hay = `${l.name} ${l.id} ${l.summary}`.toLowerCase();
    return words.every((w) => hay.includes(w));
  });
}

// switched is what the page says after a switch went through.
export function switched(world: ModWorld, m: Mod): string {
  return `${m.name} is ${m.enabled ? "on" : "off"} in ${world.name} from its next start.`;
}

// unswitched is what the page says when a switch didn't go through. The page
// reads the list again either way, so what it shows is the backend's.
export function unswitched(reason: "unknown_world" | "unknown_mod" | "unavailable", name: string): string {
  switch (reason) {
    case "unknown_world":
      return `${name} couldn’t be switched: the server doesn’t have that world any more. The list has been read again.`;
    case "unknown_mod":
      return `${name} couldn’t be switched: that world doesn’t offer it any more. The list has been read again.`;
    case "unavailable":
      return `We couldn’t tell whether ${name} was switched. The list has been read again, so it shows what the server has.`;
  }
}

export type ModsAnswer = { kind: "mods"; worlds: ModWorld[] } | Problem;
export type SwitchAnswer = ModsAnswer | { kind: "refused"; code: "unknown_world" | "unknown_mod" };

async function call(init?: RequestInit): Promise<SwitchAnswer> {
  let res: Response;
  try {
    res = await fetch("/api/minecraft/mods", {
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
  if (res.ok) {
    const worlds = parseMods(body);
    return worlds ? { kind: "mods", worlds } : { kind: "unavailable" };
  }
  const code = (body as { code?: unknown } | null)?.code;
  if (res.status === 409 && (code === "unknown_world" || code === "unknown_mod")) return { kind: "refused", code };
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

export async function getMods(): Promise<ModsAnswer> {
  const answer = await call();
  return answer.kind === "refused" ? { kind: "unavailable" } : answer;
}

export const switchMod = (world: string, mod: string, enabled: boolean) =>
  call({
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ world, mod, enabled }),
  });
