// Calls to the site's /api/soccer routes, which forward to the Schedule
// Downloader's backend. Every answer shown on the page comes from there.
import type { Division, Game, TeamSummary } from "./logic";

// Why a call failed, in the terms the page explains:
// "league": the league's schedule service is not answering.
// "tool": the Schedule Downloader itself could not be reached.
// "gone": the league has no Team with that Team ID.
// "invalid": the request was not usable as sent.
export type FailureKind = "league" | "tool" | "gone" | "invalid";

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
