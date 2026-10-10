// What the Schedule Downloader's screen decides for itself: which games are
// checked, how a game reads, and what a download asks the backend for. The
// league, the schedules and the calendar files belong to the Tool's backend
// (ADR 0001); nothing here stands in for them.

export type Division = { id: string; name: string; code?: string };

export type TeamSummary = { id: string; name: string; division?: Division; season?: string };

export type Game = {
  id: string;
  start: string; // Boise time, with its offset
  end: string;
  field: string;
  home: string;
  away: string;
  remaining: boolean;
};

export type Problem = "league" | "tool" | "gone";

// A Team someone has added, with its games once they have loaded.
export type Chosen = TeamSummary & {
  state: "loading" | "ready" | "failed";
  problem?: Problem;
  games: Game[];
};

export type Picks = {
  teams: Chosen[];
  // The games going into a download, by game ID. A Session link ignores it.
  checked: ReadonlySet<string>;
};

export type Action =
  | { type: "add"; team: TeamSummary }
  | { type: "loaded"; id: string; team: TeamSummary; games: Game[] }
  | { type: "failed"; id: string; problem: Problem }
  | { type: "retry"; id: string }
  | { type: "remove"; id: string }
  | { type: "toggle"; gameId: string; on: boolean };

export const NO_PICKS: Picks = { teams: [], checked: new Set() };

// The backend combines at most this many Teams in one calendar.
export const MAX_TEAMS = 10;

function gameIds(teams: readonly Chosen[], except?: string): Set<string> {
  const ids = new Set<string>();
  for (const t of teams) {
    if (t.id === except) continue;
    for (const g of t.games) ids.add(g.id);
  }
  return ids;
}

export function reduce(picks: Picks, action: Action): Picks {
  switch (action.type) {
    case "add": {
      if (picks.teams.length >= MAX_TEAMS || picks.teams.some((t) => t.id === action.team.id)) return picks;
      return { ...picks, teams: [...picks.teams, { ...action.team, state: "loading", games: [] }] };
    }
    case "loaded": {
      const team = picks.teams.find((t) => t.id === action.id);
      if (!team) return picks; // removed while its games were loading
      // Remaining games start checked. A game already on screen, under this
      // Team or another chosen one, keeps whatever the person left it at.
      const seen = gameIds(picks.teams);
      const checked = new Set(picks.checked);
      for (const g of action.games) {
        if (g.remaining && !seen.has(g.id)) checked.add(g.id);
      }
      const teams = picks.teams.map((t) =>
        t.id === action.id ? { ...t, ...action.team, state: "ready" as const, problem: undefined, games: action.games } : t,
      );
      return { teams, checked };
    }
    case "failed":
      return {
        ...picks,
        teams: picks.teams.map((t) => (t.id === action.id ? { ...t, state: "failed", problem: action.problem } : t)),
      };
    case "retry":
      return {
        ...picks,
        teams: picks.teams.map((t) => (t.id === action.id ? { ...t, state: "loading", problem: undefined } : t)),
      };
    case "remove": {
      const teams = picks.teams.filter((t) => t.id !== action.id);
      // Its games leave the download, except one another chosen Team plays in.
      const kept = gameIds(teams);
      return { teams, checked: new Set([...picks.checked].filter((id) => kept.has(id))) };
    }
    case "toggle": {
      const checked = new Set(picks.checked);
      if (action.on) checked.add(action.gameId);
      else checked.delete(action.gameId);
      return { ...picks, checked };
    }
  }
}

// Every game across the chosen Teams, once each, in start order. A game
// between two chosen Teams is one game.
export function allGames(teams: readonly Chosen[]): Game[] {
  const byId = new Map<string, Game>();
  for (const t of teams) for (const g of t.games) if (!byId.has(g.id)) byId.set(g.id, g);
  return [...byId.values()].sort((a, b) => a.start.localeCompare(b.start));
}

// The body of a download request. The backend puts every game starting at or
// after "since" in the file unless it is excluded, so a since before any
// Session with every unchecked game excluded asks for exactly the checked
// games.
const BEFORE_ANY_SESSION = "2000-01-01T00:00:00Z";

export function downloadChoice(picks: Picks): { team_ids: string[]; excluded: string[]; since: string } {
  return {
    team_ids: picks.teams.map((t) => t.id),
    excluded: allGames(picks.teams)
      .filter((g) => !picks.checked.has(g.id))
      .map((g) => g.id),
    since: BEFORE_ANY_SESSION,
  };
}

// A Session link belongs to a set of Teams, whatever order they were added in.
export function teamsKey(teams: readonly { id: string }[]): string {
  return teams
    .map((t) => t.id)
    .sort()
    .join(",");
}

export type TeamIDCheck = { ok: true; id: string } | { ok: false; message: string };

// A Team ID is kept as text: it is six digits and may start with a zero.
export function checkTeamID(raw: string): TeamIDCheck {
  const id = raw.trim();
  if (!/^[0-9]+$/.test(id)) return { ok: false, message: "Enter a Team ID using numbers." };
  if (id.length !== 6) return { ok: false, message: "A Team ID has 6 numbers." };
  return { ok: true, id };
}

const BOISE = new Intl.DateTimeFormat("en-US", {
  timeZone: "America/Boise",
  weekday: "short",
  day: "numeric",
  month: "short",
  hour: "numeric",
  minute: "2-digit",
  hour12: true,
});

// A game's start as Boise reads it, whatever time zone the device is in.
export function gameWhen(start: string): { date: string; time: string } | null {
  const at = new Date(start);
  if (Number.isNaN(at.getTime())) return null;
  const part: Record<string, string> = {};
  for (const p of BOISE.formatToParts(at)) part[p.type] = p.value;
  return {
    date: `${part.weekday} ${part.day} ${part.month}`,
    time: `${part.hour}:${part.minute} ${part.dayPeriod.toLowerCase()}`,
  };
}

// Who a Team plays, when its own name is one side of the game.
export function matchup(game: Game, teamName: string): string {
  const mine = teamName.trim().toLowerCase();
  if (game.home.trim().toLowerCase() === mine && game.away) return `vs ${game.away}`;
  if (game.away.trim().toLowerCase() === mine && game.home) return `vs ${game.home}`;
  if (game.home && game.away) return `${game.home} vs ${game.away}`;
  return "Opponent not posted";
}

// The league gives a field as a bare number; say what the number is.
export function fieldName(field: string): string {
  const f = field.trim();
  if (!f) return "Field not posted";
  return /^[0-9]+[a-z]?$/i.test(f) ? `Field ${f}` : f;
}

export function plural(n: number, one: string, many = `${one}s`): string {
  return `${n} ${n === 1 ? one : many}`;
}

export function listNames(names: readonly string[]): string {
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

// The address Apple Calendar opens to offer a subscription.
export function webcal(url: string): string {
  return url.replace(/^https?:/, "webcal:");
}
