import { useEffect, useRef, useState, type FormEvent, type ReactNode, type RefObject } from "react";
import { divisionTeams, listDivisions, loadTeam, searchTeams, ToolError, type FailureKind, type TeamSchedule } from "./api";
import { checkTeamID, MAX_TEAMS, plural, type Division, type TeamSummary } from "./logic";
import { Dots, LeagueDown, ToolDown } from "./parts";

type Mode = "name" | "id" | "division";

type Results =
  | { kind: "idle" }
  | { kind: "loading"; what: string }
  | { kind: "found"; teams: TeamSummary[]; schedule?: TeamSchedule }
  | { kind: "none"; mode: Mode }
  | { kind: "failed"; why: FailureKind };

const MODES: Record<Mode, string> = {
  name: "Search by Team name",
  id: "Use a Team ID",
  division: "Browse divisions",
};

// Find a Team by name, by Team ID, or by browsing divisions. The three ways
// swap in place and share one list of matches; nothing is ever added without
// the person choosing it.
export function Finder({
  added,
  onAdd,
  focusRef,
}: {
  added: ReadonlySet<string>;
  onAdd: (team: TeamSummary, schedule?: TeamSchedule) => void;
  // Points at the current way's first control, for "Add another team".
  focusRef: RefObject<HTMLInputElement | HTMLSelectElement | null>;
}) {
  const [mode, setMode] = useState<Mode>("name");
  const [name, setName] = useState("");
  const [teamID, setTeamID] = useState("");
  const [fieldError, setFieldError] = useState("");
  const [results, setResults] = useState<Results>({ kind: "idle" });
  const [divisions, setDivisions] = useState<Division[] | null>(null);
  const [divisionsFailed, setDivisionsFailed] = useState<FailureKind | null>(null);
  const [division, setDivision] = useState("");

  // Only the newest lookup may fill the list.
  const latest = useRef(0);
  const retry = useRef<() => void>(() => {});
  const switched = useRef(false);

  const full = added.size >= MAX_TEAMS;
  const busy = results.kind === "loading";

  async function look(what: string, find: () => Promise<Results>) {
    const id = ++latest.current;
    retry.current = () => void look(what, find);
    setResults({ kind: "loading", what });
    try {
      const found = await find();
      if (id === latest.current) setResults(found);
    } catch (err) {
      if (id === latest.current) setResults({ kind: "failed", why: err instanceof ToolError ? err.kind : "tool" });
    }
  }

  function searchByName(e: FormEvent) {
    e.preventDefault();
    const q = name.trim();
    if (q.length < 2) {
      setFieldError("Enter at least 2 letters of the Team name.");
      focusRef.current?.focus();
      return;
    }
    setFieldError("");
    void look("Searching for teams…", async () => {
      const teams = await searchTeams(q);
      return teams.length ? { kind: "found", teams } : { kind: "none", mode: "name" };
    });
  }

  function findByID(e: FormEvent) {
    e.preventDefault();
    const check = checkTeamID(teamID);
    if (!check.ok) {
      setFieldError(check.message);
      focusRef.current?.focus();
      return;
    }
    setFieldError("");
    void look("Looking up that Team ID…", async () => {
      try {
        const schedule = await loadTeam(check.id);
        return { kind: "found", teams: [schedule.team], schedule };
      } catch (err) {
        if (err instanceof ToolError && (err.kind === "gone" || err.kind === "invalid")) return { kind: "none", mode: "id" };
        throw err;
      }
    });
  }

  function browse(id: string) {
    setDivision(id);
    if (!id) {
      latest.current++;
      setResults({ kind: "idle" });
      return;
    }
    void look("Loading that division’s teams…", async () => {
      const teams = await divisionTeams(id);
      return teams.length ? { kind: "found", teams } : { kind: "none", mode: "division" };
    });
  }

  function loadDivisions() {
    setDivisionsFailed(null);
    listDivisions().then(setDivisions, (err: unknown) => setDivisionsFailed(err instanceof ToolError ? err.kind : "tool"));
  }

  function change(next: Mode) {
    latest.current++;
    switched.current = true;
    setMode(next);
    setFieldError("");
    setDivision("");
    setResults({ kind: "idle" });
    if (next === "division" && !divisions) loadDivisions();
  }

  // After switching ways, focus follows to the control that replaced the old one.
  useEffect(() => {
    if (!switched.current) return;
    switched.current = false;
    focusRef.current?.focus();
  }, [mode, focusRef]);

  const others = (Object.keys(MODES) as Mode[]).filter((m) => m !== mode);

  return (
    <section className="soccer-card" aria-labelledby="find-title">
      <h2 id="find-title" className="soccer-card-title">
        Find a team
      </h2>

      {mode === "name" && (
        <form className="soccer-find" onSubmit={searchByName} noValidate>
          <label className="soccer-label" htmlFor="team-name">
            Team name
          </label>
          <div className="soccer-find-row">
            <input
              ref={focusRef as RefObject<HTMLInputElement>}
              id="team-name"
              className="soccer-input"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              autoCapitalize="words"
              spellCheck={false}
              enterKeyHint="search"
              maxLength={100}
              aria-invalid={fieldError ? true : undefined}
              aria-describedby={fieldError ? "find-error" : undefined}
            />
            <button className="tactile tactile-small" type="submit" disabled={busy}>
              Search
            </button>
          </div>
        </form>
      )}

      {mode === "id" && (
        <form className="soccer-find" onSubmit={findByID} noValidate>
          <label className="soccer-label" htmlFor="team-id">
            Team ID
          </label>
          <div className="soccer-find-row">
            <input
              ref={focusRef as RefObject<HTMLInputElement>}
              id="team-id"
              className="soccer-input"
              type="text"
              inputMode="numeric"
              value={teamID}
              onChange={(e) => setTeamID(e.target.value)}
              autoComplete="off"
              enterKeyHint="search"
              maxLength={12}
              aria-invalid={fieldError ? true : undefined}
              aria-describedby={fieldError ? "find-error team-id-hint" : "team-id-hint"}
            />
            <button className="tactile tactile-small" type="submit" disabled={busy}>
              Find team
            </button>
          </div>
          <p id="team-id-hint" className="soccer-hint">
            Your Team gets a new ID each Session.
          </p>
        </form>
      )}

      {mode === "division" && (
        <div className="soccer-find">
          <label className="soccer-label" htmlFor="division">
            Division
          </label>
          {divisionsFailed ? (
            <Problem why={divisionsFailed} onRetry={loadDivisions} />
          ) : divisions === null ? (
            <p className="soccer-status" role="status">
              <Dots />
              Loading divisions…
            </p>
          ) : null}
          <select
            ref={focusRef as RefObject<HTMLSelectElement>}
            id="division"
            className="soccer-input soccer-select"
            value={division}
            onChange={(e) => browse(e.target.value)}
            disabled={!divisions}
            aria-describedby="division-hint"
          >
            <option value="">Choose a division</option>
            {divisions?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </select>
          <p id="division-hint" className="soccer-hint">
            Let’s Play Soccer calls these leagues. Divisions with a game in the next week are listed.
          </p>
        </div>
      )}

      {fieldError && (
        <p id="find-error" className="soccer-error" role="alert">
          {fieldError}
        </p>
      )}

      <p className="soccer-ways">
        {others.map((m) => (
          <button key={m} type="button" className="soccer-link" onClick={() => change(m)}>
            {MODES[m]}
          </button>
        ))}
      </p>

      <div className="soccer-results" aria-busy={busy || undefined}>
        {results.kind === "loading" && (
          <p className="soccer-status" role="status">
            <Dots />
            {results.what}
          </p>
        )}

        {results.kind === "failed" && <Problem why={results.why} onRetry={() => retry.current()} />}

        {results.kind === "none" && (
          <div className="soccer-notice" role="status">
            <p className="soccer-notice-title">No teams found</p>
            <p>{NONE[results.mode]}</p>
          </div>
        )}

        {results.kind === "found" && (
          <>
            <p className="soccer-results-count" role="status">
              {plural(results.teams.length, "team")} found
              {results.teams.length >= 50 && ". Showing the first 50; add more of the name to narrow it"}
            </p>
            {full && <p className="soccer-hint">You can combine up to {MAX_TEAMS} Teams. Remove one to add another.</p>}
            <ul className="soccer-rows">
              {results.teams.map((t) => (
                <li key={t.id} className="soccer-row">
                  <div className="soccer-row-text">
                    <p className="soccer-row-name">{t.name}</p>
                    <p className="soccer-row-detail">
                      <Details team={t} />
                    </p>
                  </div>
                  {added.has(t.id) ? (
                    <p className="soccer-added">Added</p>
                  ) : (
                    <button
                      type="button"
                      className="quiet-button soccer-add"
                      disabled={full}
                      onClick={() => onAdd(t, results.schedule?.team.id === t.id ? results.schedule : undefined)}
                    >
                      Add team
                      <span className="visually-hidden">
                        : {t.name}, Team ID {t.id}
                      </span>
                    </button>
                  )}
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </section>
  );
}

const NONE: Record<Mode, string> = {
  name: "Check the spelling, or use a Team ID or browse divisions. Search lists Teams with a game in the next week.",
  id: "No Team has that Team ID. Check the number, or search by Team name. Team IDs change each Session.",
  division: "No Teams in this division have a game in the next week. Try a Team ID.",
};

// What tells one Team from another with the same name: its division, its
// Session when the league gives one, and its Team ID.
function Details({ team }: { team: TeamSummary }) {
  const parts: ReactNode[] = [];
  if (team.division?.name) parts.push(team.division.name);
  if (team.season) parts.push(`Session ${team.season}`);
  parts.push(`Team ID ${team.id}`);
  return (
    <>
      {parts.map((p, i) => (
        <span key={i}>{p}</span>
      ))}
    </>
  );
}

function Problem({ why, onRetry }: { why: FailureKind; onRetry: () => void }) {
  return why === "league" ? <LeagueDown onRetry={onRetry} /> : <ToolDown onRetry={onRetry} />;
}
