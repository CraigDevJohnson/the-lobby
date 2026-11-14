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
  source: string; // the mod's own page, or empty when none is known
  default: boolean; // the backend's own starting setting; the page doesn't show it
  enabled: boolean; // whether it is on from the world's next start
};

export type ModWorld = { id: string; name: string; mods: Mod[] };

const text = (v: unknown) => (typeof v === "string" ? v.trim() : "");

// A mod's page is linked only when it is an https address on a site the
// catalog uses; anything else is left out, so the name stays plain text.
const SOURCES = ["modrinth.com", "github.com"];

function source(v: unknown): string {
  try {
    const url = new URL(text(v));
    return url.protocol === "https:" && SOURCES.includes(url.hostname) ? url.href : "";
  } catch {
    return "";
  }
}

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
    source: source(r.source),
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

// A mod's row in the admin grid: one line per mod name however many worlds
// offer it and whichever build each runs, listed by name, with a cell per world in the backend's world order
// (null where that world doesn't offer it). Its words and link come from the
// first world listing it;
// the version is shown only when every world has the same one.
export type ModLine = {
  id: string;
  name: string;
  version: string;
  summary: string;
  source: string;
  cells: (Mod | null)[];
};

export function modGrid(worlds: ModWorld[]): ModLine[] {
  const lines: ModLine[] = [];
  const rows = new Map<string, ModLine>();
  worlds.forEach((w, i) => {
    for (const m of w.mods) {
      // A mod with a build per loader has an id per build and one name, so
      // rows go by name. Two of a name in one world keep a row each.
      const named = m.name.toLowerCase();
      const own = `${named}\n${m.id}`;
      const key = rows.has(own) || rows.get(named)?.cells[i] ? own : named;
      let line = rows.get(key);
      if (!line) {
        const { id, name, version, summary, source } = m;
        line = { id, name, version, summary, source, cells: worlds.map(() => null) };
        rows.set(key, line);
        lines.push(line);
      } else if (line.version !== m.version) {
        line.version = "";
      }
      line.cells[i] = m;
    }
  });
  return lines.sort((a, b) => byName.compare(a.name, b.name) || (a.id < b.id ? -1 : 1));
}

// Rows run A to Z by name whatever the case, with numbers in counting order.
const byName = new Intl.Collator("en", { sensitivity: "base", numeric: true });

// findMods narrows the grid to what was typed: lines whose name, id or
// summary contain every word.
export function findMods(lines: ModLine[], query: string) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  return lines.filter((l) => {
    const hay = `${l.name} ${l.id} ${l.summary}`.toLowerCase();
    return words.every((w) => hay.includes(w));
  });
}

// countLine is the line under the search: how many mods there are, or how
// many of them the search leaves.
export function countLine(c: { total: number; matching: number; query: string }): string {
  const mods = c.total === 1 ? "mod" : "mods";
  if (c.query.trim() === "") return `${c.total} ${mods}`;
  return `${c.matching} of ${c.total} ${mods} ${c.matching === 1 ? "matches" : "match"} your search`;
}

// What the grid says in place of rows when the search leaves none.
export const NONE_FOUND = "No mods match that.";

// notOffered is the one line a stacked row gives the worlds that don't offer
// its mod, in place of a line each. Empty when every world offers it.
export function notOffered(worlds: string[]): string {
  if (worlds.length === 0) return "";
  const last = worlds[worlds.length - 1];
  const rest = worlds.slice(0, -1).join(", ");
  return `Not offered in ${rest ? `${rest} or ${last}` : last}`;
}

// stackUnder is the window width, in rem, under which the grid has no room
// for a column per world and stacks each mod over its worlds instead. A
// column needs 9.25rem and the mod's words 15rem (the grid's own minimums in
// styles.css), inside the card's 2px outline and the page's gutters, which
// are 5% of the window a side. The extra 1rem is room for a classic
// scrollbar, which the media query and the gutters both count as window.
// Never under 48rem, where the page is a phone's. Past the page's widest
// (84rem less its gutters) the columns never fit.
export function stackUnder(worlds: number): number {
  const card = 15.25 + 9.25 * worlds;
  if (card > 73) return 9999;
  return Math.max(48, Math.ceil((card + 1) / 0.9));
}

// The setting each cell had when the page first showed it, a cell to a key.
// An id can't hold "/", so a key is one world's one mod.
export type Settings = ReadonlyMap<string, boolean>;

// firstSeen adds the cells an answer has that the page hasn't shown before,
// and never changes one it has: a cell can only be switched once it has been
// shown, so its first setting is the one from before any switch this visit.
// With nothing to add it gives back the same map, so nothing is drawn again.
export function firstSeen(prev: Settings | null, worlds: ModWorld[]): Settings {
  let next: Map<string, boolean> | null = null;
  for (const w of worlds)
    for (const m of w.mods) {
      const key = `${w.id}/${m.id}`;
      if (prev?.has(key) || next?.has(key)) continue;
      next ??= new Map(prev);
      next.set(key, m.enabled);
    }
  return next ?? prev ?? new Map();
}

// changedSince says whether a mod's setting in a world differs from the one
// it had when the page first showed it, which is what the grid marks "From
// next start". Switching it back clears the mark.
export function changedSince(first: Settings | null, world: string, m: Mod): boolean {
  const was = first?.get(`${world}/${m.id}`);
  return was !== undefined && was !== m.enabled;
}

// switched is what the page says after a switch went through.
export function switched(world: ModWorld, m: Mod): string {
  return `${m.name} is ${m.enabled ? "on" : "off"} in ${world.name} from its next start.`;
}

// launcherCue follows a switch that went through: where a world's next start
// happens. The Launcher starts worlds and can stop a ready one; it has no
// restart, so none is promised. The cue is the same whatever any world is
// doing, because this page doesn't know, and it is only a link: the Launcher
// checks its own grant.
export function launcherCue() {
  return { before: "Worlds start and stop on ", link: "the Launcher", after: ".", href: "/minecraft" };
}

// What the page says in place of a list it has nothing to put in.
export const NO_WORLDS = {
  title: "The server lists no worlds yet.",
  detail: "Mods are switched for one world at a time, so there’s nothing to switch until it lists one.",
  still: "Checked just now. The server still lists no worlds.",
};

// What the page says aloud when a Retry on a panel brings the grid.
export const LISTED = "The mods are listed now.";

export const NO_MODS = {
  title: "No mods are offered for any world yet.",
  detail: "When the server offers one, it’s listed here with a switch for each world that has it.",
};

// Why a switch takes no presses while another is on its way, for assistive
// technology; the dimmed switches and the wait pointer say it to the eye.
export const WAITING = "Waiting for another switch to finish";

// A mod's name links to its own page in a new tab, which the link says.
export const NEW_TAB = "opens in a new tab";

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
