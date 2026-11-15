// Lobby Admin's Members, read and changed through the site's
// /api/lobby/members routes, which check the lobby-admin grant first
// (internal/members). Every change answers the whole list as the site now
// has it, so the page shows what was saved rather than what was asked.
import type { Edition } from "../minecraft/adminTools";

export type Member = {
  email: string;
  tools: string[];
  addedAt: Date | null;
  // When one of their Sign-in sessions was last used, right to the day;
  // null when they aren't signed in anywhere.
  lastHere: Date | null;
  devices: number;
};

export type Roster = { you: string; members: Member[] };

// Lobby Admin's own grant, which nobody can take from themselves.
export const OWN_GRANT = "lobby-admin";

// The Tool whose players join through the Minecraft allowlist.
export const MINECRAFT = "minecraft";

// offersAllowlist says whether the page can offer to put a Member's player
// on the Minecraft allowlist. The site lets only minecraft-admin change it,
// so the person using the page needs it on their own row.
export function offersAllowlist(r: Roster): boolean {
  return r.members.some((m) => m.email === r.you && m.tools.includes("minecraft-admin"));
}

// The player name field's label, in Minecraft Admin's words. It can be left
// empty when inviting a Member.
export function playerLabel(edition: Edition, optional: boolean): string {
  const label = edition === "java" ? "Java account name" : "Bedrock gamertag";
  return optional ? `${label} (optional)` : label;
}

function date(raw: unknown): Date | null {
  if (typeof raw !== "string" || raw === "") return null;
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? null : d;
}

// parseRoster reads the site's list. A Member without a readable address is
// left out; anything else unreadable makes the whole list null.
export function parseRoster(raw: unknown): Roster | null {
  const r = (raw ?? {}) as { you?: unknown; members?: unknown };
  if (typeof r.you !== "string" || !Array.isArray(r.members)) return null;
  const members: Member[] = [];
  for (const entry of r.members) {
    const e = (entry ?? {}) as Record<string, unknown>;
    if (typeof e.email !== "string" || !e.email.includes("@")) continue;
    if (members.some((m) => m.email === e.email)) continue;
    const tools = Array.isArray(e.tools) ? e.tools.filter((t): t is string => typeof t === "string") : [];
    const devices = typeof e.devices === "number" && e.devices > 0 ? Math.floor(e.devices) : 0;
    members.push({ email: e.email, tools, addedAt: date(e.added_at), lastHere: date(e.last_here), devices });
  }
  members.sort((a, b) => (a.email < b.email ? -1 : a.email > b.email ? 1 : 0));
  return { you: r.you, members };
}

// Grants the page has no box for (known lists the ones it has), which the
// site keeps on the record.
export const otherGrants = (m: Member, known: readonly string[]) => m.tools.filter((t) => !known.includes(t));

const midnight = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

const shortDate = (d: Date, now: Date) =>
  d.toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    ...(d.getFullYear() === now.getFullYear() ? {} : { year: "numeric" }),
  });

// lastHereLine says when a Member was last here and on how many devices,
// in whole days of the reader's own calendar.
export function lastHereLine(m: Member, now: Date): string {
  if (!m.lastHere || m.devices === 0) return "Not signed in anywhere";
  const days = Math.round((midnight(now) - midnight(m.lastHere)) / 86_400_000);
  const when =
    days <= 0 ? "today" : days === 1 ? "yesterday" : days < 7 ? `${days} days ago` : `on ${shortDate(m.lastHere, now)}`;
  return `Last here ${when}, signed in on ${m.devices === 1 ? "1 device" : `${m.devices} devices`}`;
}

export function addedLine(m: Member, now: Date): string {
  return m.addedAt ? `Added ${shortDate(m.addedAt, now)}` : "";
}

// emailProblem says what is wrong with a typed address before the site is
// asked, by the site's own rule. Empty when it could be an address.
export function emailProblem(raw: string): string {
  const email = raw.trim();
  if (email === "") return "Type their email address.";
  const at = email.indexOf("@");
  const domain = email.slice(at + 1).replace(/^\.+|\.+$/g, "");
  if (at <= 0 || email.length > 254 || /\s/.test(email) || email.indexOf("@", at + 1) !== -1 || !domain.includes(".")) {
    return "That doesn’t look like an email address.";
  }
  return "";
}

// The line to send a new Member, so they know where to go and which address
// to sign in with.
export function inviteText(origin: string, email: string): string {
  return `You’re invited to The Lobby. Go to ${origin}, choose Sign in and enter ${email}. You’ll get a code by email to finish signing in.`;
}

// What the page says when a change answered unclearly and the list couldn't
// be read again either, so nothing on screen shows what the site has.
export const unsureNote = (adding: boolean) =>
  adding ? "We couldn’t tell whether they were added." : "We couldn’t tell whether that change was saved.";

const COULD_NOT_TELL = "We couldn’t tell whether that change was saved. The list has been read again and shows what the site has.";

// What the page says after a change. code is empty when it went through;
// tool is the Tool's name as the page shows it.
export function grantOutcome(email: string, tool: string, granted: boolean, code: string): string {
  switch (code) {
    case "":
      return granted ? `${email} can open ${tool} now.` : `${email} can’t open ${tool} any more.`;
    case "self":
      return "You can’t take away your own Lobby Admin, so you can’t lock yourself out.";
    case "not_member":
      return `${email} isn’t a Member any more.`;
    case "unknown_tool":
      return `The site doesn’t offer ${tool} any more.`;
    default:
      return COULD_NOT_TELL;
  }
}

export function addOutcome(email: string, code: string): string {
  switch (code) {
    case "":
      return `${email} is a Member. Send them this:`;
    case "already_member":
      return `${email} is already a Member. Change their Tools in the list above.`;
    case "invalid_email":
      return "That doesn’t look like an email address.";
    case "unknown_tool":
      return "One of those Tools isn’t offered any more. The list has been read again.";
    default:
      return `We couldn’t tell whether ${email} was added. The list has been read again and shows what the site has.`;
  }
}

export function removeOutcome(email: string, code: string): string {
  switch (code) {
    case "":
      return `${email} is no longer a Member and is signed out everywhere.`;
    case "self":
      return "You can’t remove yourself.";
    case "not_member":
      return `${email} isn’t a Member any more.`;
    default:
      return COULD_NOT_TELL;
  }
}

export function signOutOutcome(email: string, self: boolean, code: string): string {
  switch (code) {
    case "":
      return self ? "Your other devices are signed out." : `${email} is signed out everywhere. They can sign in again.`;
    case "not_member":
      return `${email} isn’t a Member any more.`;
    default:
      return COULD_NOT_TELL;
  }
}

// One answer from the site: the list, a reason the change was refused, or
// why the page can't show the Members at all.
export type Answer =
  | { kind: "roster"; roster: Roster }
  | { kind: "refused"; code: string }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed" }
  | { kind: "unavailable" };

// The reasons a change is refused that the page can explain.
const REFUSALS = new Set(["already_member", "invalid_email", "unknown_tool", "not_member", "self"]);

export function readAnswer(status: number, body: unknown): Answer {
  if (status === 200) {
    const roster = parseRoster(body);
    return roster ? { kind: "roster", roster } : { kind: "unavailable" };
  }
  const code = (body as { code?: unknown } | null)?.code;
  if (status === 401 && code === "signed_out") return { kind: "signed-out" };
  if (status === 403 && code === "no_access") return { kind: "denied" };
  if (status === 503 && code === "access_unavailable") return { kind: "access-failed" };
  if (status >= 400 && status < 500 && typeof code === "string" && REFUSALS.has(code)) return { kind: "refused", code };
  return { kind: "unavailable" };
}

async function ask(path: string, body?: unknown): Promise<Answer> {
  try {
    const res = await fetch(`/api/lobby/members${path}`, {
      credentials: "same-origin",
      method: body === undefined ? "GET" : "POST",
      headers: { Accept: "application/json", ...(body === undefined ? {} : { "Content-Type": "application/json" }) },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const answer: unknown = await res.json().catch(() => null);
    return readAnswer(res.status, answer);
  } catch {
    return { kind: "unavailable" };
  }
}

export const getRoster = () => ask("");
export const addMember = (email: string, tools: string[]) => ask("/add", { email: email.trim(), tools });
export const removeMember = (email: string) => ask("/remove", { email });
export const setGrant = (email: string, tool: string, granted: boolean) => ask("/grant", { email, tool, granted });
export const signOutMember = (email: string) => ask("/sign-out", { email });
