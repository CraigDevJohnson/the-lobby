import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import { Robot } from "../Robot";
import { loadTeam, ToolError, type TeamSchedule } from "./api";
import { Delivery } from "./Delivery";
import { Finder } from "./Finder";
import { Games } from "./Games";
import { NO_PICKS, plural, reduce, type Chosen, type Problem, type TeamSummary } from "./logic";
import { Dots, Icon } from "./parts";

// The Schedule Downloader's public screen (docs/design/soccer-brief.md): find
// Boise Teams, review their games, then take them to a calendar as a download
// or a Session link. It is the same for Visitors and Members and never asks
// who is looking. Remembered teams, Member links and Connected calendars are
// not part of it.
export function ScheduleDownloader() {
  const [picks, dispatch] = useReducer(reduce, NO_PICKS);
  // Said to screen readers when the chosen Teams change.
  const [said, setSaid] = useState("");
  const finder = useRef<HTMLInputElement | HTMLSelectElement | null>(null);
  const findCard = useRef<HTMLDivElement>(null);
  const teamList = useRef<HTMLUListElement>(null);

  const load = useCallback(async (id: string, name: string, ready?: TeamSchedule) => {
    try {
      const { team, games } = ready ?? (await loadTeam(id));
      dispatch({ type: "loaded", id, team, games });
      setSaid(`${team.name} added with ${plural(games.length, "game")}.`);
    } catch (err) {
      const kind = err instanceof ToolError ? err.kind : "tool";
      const problem: Problem = kind === "league" ? "league" : kind === "gone" || kind === "invalid" ? "gone" : "tool";
      dispatch({ type: "failed", id, problem });
      setSaid(`Games for ${name} didn’t load.`);
    }
  }, []);

  const add = useCallback(
    (team: TeamSummary, schedule?: TeamSchedule) => {
      dispatch({ type: "add", team });
      void load(team.id, team.name, schedule);
    },
    [load],
  );

  const retry = useCallback(
    (id: string) => {
      const team = picks.teams.find((t) => t.id === id);
      if (!team) return;
      dispatch({ type: "retry", id });
      void load(id, team.name);
    },
    [picks.teams, load],
  );

  // Where focus goes once a removed Team's own controls are gone.
  const afterRemove = useRef(false);
  const remove = useCallback(
    (id: string) => {
      const team = picks.teams.find((t) => t.id === id);
      if (!team) return;
      afterRemove.current = true;
      dispatch({ type: "remove", id });
      setSaid(`${team.name} removed.`);
    },
    [picks.teams],
  );
  useEffect(() => {
    if (!afterRemove.current) return;
    afterRemove.current = false;
    const next = teamList.current?.querySelector<HTMLButtonElement>("button");
    (next ?? finder.current)?.focus();
  }, [picks.teams.length]);

  const addAnother = () => {
    findCard.current?.scrollIntoView({ block: "start" });
    finder.current?.focus({ preventScroll: true });
  };

  const added = new Set(picks.teams.map((t) => t.id));
  const any = picks.teams.length > 0;

  return (
    <div className="page soccer">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <a className="page-back" href="/signin">
            Sign in
          </a>
        </div>
      </header>

      <main className="soccer-main">
        <div className="soccer-opening">
          <h1 className="soccer-title">Schedule Downloader</h1>
          <p className="soccer-lead">Your team’s games, in your calendar.</p>
          <p className="soccer-context">Boise · No sign-in needed</p>
        </div>

        <div className="soccer-flow" data-teams={any || undefined}>
          <div className="soccer-build">
            <div ref={findCard} className="soccer-anchor">
              <Finder added={added} onAdd={add} focusRef={finder} />
            </div>

            {any && (
              <section className="soccer-card" aria-labelledby="teams-title">
                <div className="soccer-card-head">
                  <h2 id="teams-title" className="soccer-card-title">
                    Selected teams
                  </h2>
                  <a className="soccer-jump" href="#get-your-calendar">
                    Get your calendar
                  </a>
                </div>
                <ul ref={teamList} className="soccer-rows">
                  {picks.teams.map((t) => (
                    <li key={t.id} className="soccer-row">
                      <div className="soccer-row-text">
                        <p className="soccer-row-name">{t.name}</p>
                        <p className="soccer-row-detail">
                          <TeamLine team={t} />
                        </p>
                      </div>
                      <button type="button" className="soccer-remove" aria-label={`Remove ${t.name}`} onClick={() => remove(t.id)}>
                        <Icon name="close" />
                      </button>
                    </li>
                  ))}
                </ul>
                <button type="button" className="quiet-button soccer-another" onClick={addAnother}>
                  <Icon name="plus" />
                  Add another team
                </button>
              </section>
            )}

            {any && (
              <Games
                picks={picks}
                onToggle={(gameId, on) => dispatch({ type: "toggle", gameId, on })}
                onRetry={retry}
                onRemove={remove}
              />
            )}
          </div>

          {any && <Delivery picks={picks} />}
        </div>

        <p className="soccer-unofficial">Unofficial. Not affiliated with Let’s Play Soccer.</p>
        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      <footer className="page-footer">
        <div className="page-footer-inner">
          <Robot className="lobby-robot" />
          <a href="/privacy">Privacy</a>
        </div>
      </footer>
    </div>
  );
}

function TeamLine({ team }: { team: Chosen }) {
  if (team.state === "loading") {
    return (
      <span className="soccer-inline-status">
        <Dots />
        Loading games…
      </span>
    );
  }
  if (team.state === "failed") return <span>{team.problem === "gone" ? "No longer listed by the league" : "Games didn’t load"}</span>;
  const remaining = team.games.filter((g) => g.remaining).length;
  return (
    <>
      {team.division?.name && <span>{team.division.name}</span>}
      <span>{team.games.length === 0 ? "No games posted yet" : `${plural(team.games.length, "game")}, ${remaining} remaining`}</span>
    </>
  );
}
