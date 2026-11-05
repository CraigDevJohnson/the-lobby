// Calls to the site's /api/soccer routes, which forward to the
// Let's Plan Soccer backend. Every answer shown on the page comes from there.
import type { Division, Game, NextSessionAnswer, TeamSummary } from "./logic";

// Why a call failed, in the terms the page explains:
// "league": the league's schedule service is not answering.
// "tool": Let's Plan Soccer itself could not be reached.
// "gone": the league has no Team with that Team ID.
// "invalid": the request was not usable as sent.
// "full": My calendar already holds as many Teams as it can.
export type FailureKind = "league" | "tool" | "gone" | "invalid" | "full";

export class ToolError extends Error {
  kind: FailureKind;
  constructor(kind: FailureKind) {
    super(kind);
    this.kind = kind;
  }
}

const KINDS: Record<string, FailureKind> = {
  league_unavailable: "league",
  team_not_found: "gone",
  invalid_team_id: "invalid",
  bad_request: "invalid",
  too_many_teams: "full",
};

async function call(path: string, init?: RequestInit): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(`/api/soccer${path}`, { ...init, headers: { Accept: "application/json", ...init?.headers } });
  } catch {
    throw new ToolError("tool");
  }
  if (res.ok) return res;
  let code = "";
  try {
    code = ((await res.json()) as { code?: string }).code ?? "";
  } catch {
    // Not one of the backend's own errors.
  }
  throw new ToolError(KINDS[code] ?? "tool");
}

async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await call(path, init);
  try {
    return (await res.json()) as T;
  } catch {
    throw new ToolError("tool");
  }
}

const post = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

export async function searchTeams(query: string, signal?: AbortSignal): Promise<TeamSummary[]> {
  return (await json<{ teams: TeamSummary[] }>(`/teams?q=${encodeURIComponent(query)}`, { signal })).teams;
}

export async function listDivisions(signal?: AbortSignal): Promise<Division[]> {
  return (await json<{ divisions: Division[] }>("/divisions", { signal })).divisions;
}

export async function divisionTeams(divisionID: string, signal?: AbortSignal): Promise<TeamSummary[]> {
  return (await json<{ teams: TeamSummary[] }>(`/divisions/${encodeURIComponent(divisionID)}/teams`, { signal })).teams;
}

export type TeamSchedule = { team: TeamSummary; games: Game[] };

// One Team and every game the league lists for it.
export async function loadTeam(teamID: string, signal?: AbortSignal): Promise<TeamSchedule> {
  const s = await json<{ teams: TeamSummary[]; games: Game[] }>(`/teams/${encodeURIComponent(teamID)}`, { signal });
  const team = s.teams[0];
  if (!team) throw new ToolError("gone");
  return { team, games: s.games ?? [] };
}

const MATCHES: readonly string[] = ["follow", "ask", "none"];

// The Teams in a later Session that may be this ended Session's Team. The
// backend only suggests; nothing is added until the person chooses.
export async function nextSession(teamID: string, signal?: AbortSignal): Promise<NextSessionAnswer> {
  const a = await json<Partial<NextSessionAnswer>>(`/teams/${encodeURIComponent(teamID)}/next`, { signal });
  if (!a.team || !a.match || !MATCHES.includes(a.match)) throw new ToolError("tool");
  return { team: a.team, match: a.match, teams: a.teams ?? [], similar: a.similar ?? [] };
}

// A Session link for these Teams: all their games, including later ones.
export async function createLink(teamIDs: string[]): Promise<string> {
  const made = await json<{ url?: string }>("/links", post({ team_ids: teamIDs }));
  if (!made.url) throw new ToolError("tool");
  return made.url;
}

export type CalendarFile = { blob: Blob; filename: string };

// The checked games as a one-time .ics file.
export async function downloadCalendar(choice: unknown): Promise<CalendarFile> {
  const res = await call("/calendar", post(choice));
  const named = /filename="?([^";]+)"?/.exec(res.headers.get("Content-Disposition") ?? "");
  return { blob: await res.blob(), filename: named?.[1] ?? "soccer-schedule.ics" };
}

// A Remembered team: a name and its Team IDs, newest first, the second one
// from the Session before.
export type RememberedTeam = { name: string; team_ids: string[] };

// My calendar, for a Member with Let's Plan Soccer access: their Remembered
// teams and the Member link that serves them, which exists from the first
// Team on.
export type MyCalendar = { url?: string; teams: RememberedTeam[] };

// The most Teams My calendar holds.
export const MAX_MINE = 5;

async function mine(path: string, init?: RequestInit): Promise<MyCalendar> {
  const a = await json<Partial<MyCalendar>>(`/me${path}`, init);
  return { url: a.url, teams: a.teams ?? [] };
}

export const getMyCalendar = (signal?: AbortSignal) => mine("", { signal });

// Adds Teams to My calendar. With after, the one Team carries on that
// Remembered team from an ended Session, as a follow does.
export const addMyTeams = (teamIDs: string[], after?: string) =>
  mine("/teams", post(after ? { team_ids: teamIDs, after } : { team_ids: teamIDs }));

export const removeMyTeam = (teamID: string) => mine(`/teams/${encodeURIComponent(teamID)}`, { method: "DELETE" });

// A new Member link; the old address stops at once.
export const replaceMyLink = () => mine("/link", { method: "POST" });
