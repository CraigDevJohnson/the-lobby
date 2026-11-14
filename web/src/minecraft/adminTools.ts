// Minecraft Admin's players, operators and backups, read and changed through
// the site's /api/minecraft/admin routes, which check the minecraft-admin
// grant first (docs/minecraft-backend.md). Nothing here guesses: an entry the
// page can't read is left out.

const ID = /^[a-z][a-z0-9-]{0,31}$/;
const ARCHIVE = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z\.tar\.gz$/;

export type Edition = "java" | "bedrock";
export type Player = { edition: Edition; name: string };
export type WorldOperators = { id: string; operators: Player[] };
export type Archive = { name: string; size: number; made: Date };

const byName = new Intl.Collator("en", { sensitivity: "base", numeric: true });

// A player is the same player whatever the case of their name.
export const samePlayer = (a: Player, b: Player) =>
  a.edition === b.edition && a.name.toLowerCase() === b.name.toLowerCase();

function players(raw: unknown): Player[] {
  const list: Player[] = [];
  for (const entry of Array.isArray(raw) ? raw : []) {
    const e = (entry ?? {}) as { edition?: unknown; name?: unknown };
    const name = typeof e.name === "string" ? e.name.trim() : "";
    if ((e.edition !== "java" && e.edition !== "bedrock") || !name) continue;
    const player = { edition: e.edition, name } as Player;
    if (!list.some((p) => samePlayer(p, player))) list.push(player);
  }
  return list.sort((a, b) => byName.compare(a.name, b.name) || (a.edition < b.edition ? -1 : 1));
}

// parsePlayers reads the allowlist, A to Z. Unreadable is null.
export function parsePlayers(raw: unknown): Player[] | null {
  const listed = (raw as { players?: unknown } | null)?.players;
  return Array.isArray(listed) ? players(listed) : null;
}

// parseOperators reads each world's operators. Unreadable is null.
export function parseOperators(raw: unknown): WorldOperators[] | null {
  const listed = (raw as { worlds?: unknown } | null)?.worlds;
  if (!Array.isArray(listed)) return null;
  const worlds: WorldOperators[] = [];
  for (const entry of listed) {
    const e = (entry ?? {}) as { id?: unknown; operators?: unknown };
    if (typeof e.id !== "string" || !ID.test(e.id) || worlds.some((w) => w.id === e.id)) continue;
    worlds.push({ id: e.id, operators: players(e.operators) });
  }
  return worlds;
}

// parseArchives reads one world's archives, newest first. An archive's name
// is the UTC time it was made, which is the time the page shows. Unreadable
// is null.
export function parseArchives(raw: unknown): Archive[] | null {
  const listed = (raw as { archives?: unknown } | null)?.archives;
  if (!Array.isArray(listed)) return null;
  const archives: Archive[] = [];
  for (const entry of listed) {
    const e = (entry ?? {}) as { name?: unknown; size?: unknown };
    const at = typeof e.name === "string" ? ARCHIVE.exec(e.name) : null;
    if (!at || archives.some((a) => a.name === e.name)) continue;
    const [y, mo, d, h, mi, s] = at.slice(1).map(Number);
    const made = new Date(Date.UTC(y, mo - 1, d, h, mi, s));
    const size = typeof e.size === "number" && e.size >= 0 ? e.size : 0;
    archives.push({ name: e.name as string, size, made });
  }
  return archives.sort((a, b) => b.made.getTime() - a.made.getTime());
}

// How big an archive is, in the unit that keeps the number short.
export function sizeOf(bytes: number): string {
  if (bytes <= 0) return "";
  if (bytes < 1e6) return `${Math.max(1, Math.round(bytes / 1e3))} KB`;
  if (bytes < 1e9) return `${Math.round(bytes / 1e6)} MB`;
  return `${(bytes / 1e9).toFixed(1)} GB`;
}

// When an archive was made, in the reader's own time.
export function madeAt(made: Date): string {
  return made.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

// nameProblem says what is wrong with a name before the server is asked, by
// the server's own rules. Empty when it could be a player's name.
export function nameProblem(edition: Edition, name: string): string {
  if (name === "") return "Type the player’s name.";
  if (edition === "java") {
    return /^[A-Za-z0-9_]{3,16}$/.test(name) ? "" : "A Java name is 3 to 16 letters, digits or underscores.";
  }
  return name.length <= 16 && /^[\p{L}\p{N} _]+$/u.test(name)
    ? ""
    : "A gamertag is up to 16 letters, digits, spaces or underscores.";
}

export const EDITION: Record<Edition, string> = { java: "Java", bedrock: "Bedrock" };

// What the page says after a change to the allowlist. code is empty when it
// went through.
export function playerOutcome(operation: "add" | "remove", p: Player, code: string): string {
  switch (code) {
    case "":
      return operation === "add"
        ? `${p.name} is on the allowlist. A world that is starting takes the change at its next start.`
        : `${p.name} is off the allowlist.`;
    case "player_not_found":
      return p.edition === "java" ? `No Java account is named ${p.name}.` : `No Bedrock player has the gamertag ${p.name}.`;
    case "linked_account":
      return `${p.name} is linked to a Java account. Add that Java account’s name instead.`;
    default:
      return `We couldn’t confirm that ${p.name} was ${operation === "add" ? "added" : "removed"}. The list has been read again and shows what the server has; a running world can take a minute or two to catch up.`;
  }
}

// What the page says after a change to a world's operators.
export function operatorOutcome(operation: "add" | "remove", p: Player, world: string, code: string): string {
  switch (code) {
    case "":
      return operation === "add" ? `${p.name} is an operator in ${world}.` : `${p.name} is no longer an operator in ${world}.`;
    case "not_allowlisted":
      return `${p.name} isn’t on the allowlist, so can’t be an operator.`;
    case "unknown_world":
      return `The server doesn’t have ${world} any more.`;
    default:
      return `We couldn’t confirm the change to ${p.name} in ${world}. The list has been read again and shows what the server has; a running world can take a minute or two to catch up.`;
  }
}

// What the page says after asking for a restore.
export function restoreOutcome(world: string, code: string): string {
  switch (code) {
    case "":
      return `Restoring ${world}. It starts once the backup is in place, which can take up to an hour.`;
    case "not_stopped":
      return `${world} has to be stopped first. Stop it on the Launcher, which also saves a backup of how it is now.`;
    case "unknown_world":
      return `The server doesn’t have ${world} any more.`;
    default:
      return `We couldn’t tell whether the restore of ${world} started. Check the Launcher before trying again.`;
  }
}

// One answer from the site: what it sent back, or the reason it gave. Any
// trouble without a reason the page knows is "unavailable".
export type Answer = { ok: true; body: unknown } | { ok: false; code: string };

async function ask(path: string, body?: unknown): Promise<Answer> {
  try {
    const res = await fetch(`/api/minecraft/admin/${path}`, {
      credentials: "same-origin",
      method: body === undefined ? "GET" : "POST",
      headers: { Accept: "application/json", ...(body === undefined ? {} : { "Content-Type": "application/json" }) },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const answer: unknown = await res.json().catch(() => null);
    if (res.ok) return { ok: true, body: answer };
    const code = (answer as { code?: unknown } | null)?.code;
    const known = (res.status === 404 || res.status === 409) && typeof code === "string";
    return { ok: false, code: known ? code : "unavailable" };
  } catch {
    return { ok: false, code: "unavailable" };
  }
}

export async function getPlayers(): Promise<Player[] | null> {
  const a = await ask("players");
  return a.ok ? parsePlayers(a.body) : null;
}

export async function getOperators(): Promise<WorldOperators[] | null> {
  const a = await ask("operators");
  return a.ok ? parseOperators(a.body) : null;
}

export async function getArchives(world: string): Promise<Archive[] | null> {
  const a = await ask(`archives?world=${encodeURIComponent(world)}`);
  return a.ok ? parseArchives(a.body) : null;
}

// Each change answers the reason it didn't go through, or "" when it did.
const reason = (a: Answer) => (a.ok ? "" : a.code);

export const changePlayer = async (operation: "add" | "remove", p: Player) =>
  reason(await ask("players", { operation, edition: p.edition, name: p.name }));

export const changeOperator = async (world: string, operation: "add" | "remove", p: Player) =>
  reason(await ask("operators", { world, operation, edition: p.edition, name: p.name }));

export const restoreArchive = async (world: string, archive: string) => reason(await ask("restore", { world, archive }));
