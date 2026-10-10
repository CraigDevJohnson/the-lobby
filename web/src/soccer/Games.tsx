import { useState } from "react";
import { allGames, fieldName, gameWhen, listNames, matchup, plural, type Chosen, type Game, type Picks } from "./logic";
import { Dots, Icon } from "./parts";

type View = "remaining" | "all";

// Review the games, grouped by Team. The checkboxes decide what goes into a
// download only; a Session link always carries every game.
export function Games({
  picks,
  onToggle,
  onRetry,
  onRemove,
}: {
  picks: Picks;
  onToggle: (gameId: string, on: boolean) => void;
  onRetry: (teamId: string) => void;
  onRemove: (teamId: string) => void;
}) {
  const [view, setView] = useState<View>("remaining");
  // Teams the person has opened or closed; the rest follow the default.
  const [opened, setOpened] = useState<Record<string, boolean>>({});

  const games = allGames(picks.teams);
  const selected = games.filter((g) => picks.checked.has(g.id));
  const earlier = selected.filter((g) => !g.remaining).length;
  // Which chosen Teams play in each game, to say when one is shared.
  const players = new Map<string, string[]>();
  for (const t of picks.teams) for (const g of t.games) players.set(g.id, [...(players.get(g.id) ?? []), t.name]);

  return (
    <section className="soccer-card" aria-labelledby="games-title">
      <div className="soccer-card-head">
        <h2 id="games-title" className="soccer-card-title">
          Games in your download
        </h2>
        <p className="soccer-count" role="status">
          {selected.length} of {plural(games.length, "game")} selected
        </p>
      </div>
      <p className="soccer-help">Session links include all games for your chosen Teams, including games posted later.</p>

      <div className="soccer-view" role="group" aria-label="Games shown">
        <button type="button" className="soccer-view-option" aria-pressed={view === "remaining"} onClick={() => setView("remaining")}>
          Remaining games
        </button>
        <button type="button" className="soccer-view-option" aria-pressed={view === "all"} onClick={() => setView("all")}>
          All games
        </button>
      </div>
      {view === "remaining" && earlier > 0 && (
        <p className="soccer-hint">Your download also has {plural(earlier, "earlier game")}, shown under All games.</p>
      )}

      {picks.teams.map((team, i) => {
        const open = opened[team.id] ?? (i === 0 || team.state !== "ready");
        return (
          <TeamGames
            key={team.id}
            team={team}
            view={view}
            open={open}
            checked={picks.checked}
            players={players}
            onOpen={() => setOpened((o) => ({ ...o, [team.id]: !open }))}
            onShowAll={() => setView("all")}
            onToggle={onToggle}
            onRetry={() => onRetry(team.id)}
            onRemove={() => onRemove(team.id)}
          />
        );
      })}

      <p className="soccer-footnote">Boise time · 45-minute games</p>
      <p className="soccer-hint">The league may post more games later. A download won’t pick those up.</p>
    </section>
  );
}

function TeamGames({
  team,
  view,
  open,
  checked,
  players,
  onOpen,
  onShowAll,
  onToggle,
  onRetry,
  onRemove,
}: {
  team: Chosen;
  view: View;
  open: boolean;
  checked: ReadonlySet<string>;
  players: ReadonlyMap<string, string[]>;
  onOpen: () => void;
  onShowAll: () => void;
  onToggle: (gameId: string, on: boolean) => void;
  onRetry: () => void;
  onRemove: () => void;
}) {
  const shown = view === "all" ? team.games : team.games.filter((g) => g.remaining);
  const selected = team.games.filter((g) => checked.has(g.id)).length;
  const panel = `games-${team.id}`;

  const summary =
    team.state === "loading"
      ? "Loading…"
      : team.state === "failed"
        ? "Didn’t load"
        : team.games.length === 0
          ? "No games posted"
          : `${selected} of ${team.games.length} selected`;

  return (
    <div className="soccer-team">
      <h3 className="soccer-team-heading">
        <button type="button" className="soccer-team-toggle" aria-expanded={open} aria-controls={panel} onClick={onOpen}>
          <span className="soccer-team-name">{team.name}</span>
          <span className="soccer-team-summary">{summary}</span>
          <Icon name="chevron" />
        </button>
      </h3>

      <div id={panel} hidden={!open}>
        {team.state === "loading" && (
          <p className="soccer-status" role="status">
            <Dots />
            Loading games for {team.name}…
          </p>
        )}

        {team.state === "failed" && (
          <div className="soccer-notice" role="alert">
            <p className="soccer-notice-title">
              {team.problem === "gone" ? `The league no longer lists ${team.name}.` : `Games for ${team.name} didn’t load.`}
            </p>
            <p>
              {team.problem === "gone"
                ? "Its Team ID may be from an earlier Session. Remove it, then find the Team again."
                : team.problem === "league"
                  ? "The league’s schedule service isn’t answering. Your other Teams and choices are still here."
                  : "The Schedule Downloader isn’t answering right now. Your other Teams and choices are still here."}
            </p>
            <div className="soccer-notice-actions">
              {team.problem !== "gone" && (
                <button type="button" className="quiet-button" onClick={onRetry}>
                  Retry<span className="visually-hidden"> {team.name}</span>
                </button>
              )}
              <button type="button" className="quiet-button" onClick={onRemove}>
                Remove<span className="visually-hidden"> {team.name}</span>
              </button>
            </div>
          </div>
        )}

        {team.state === "ready" && team.games.length === 0 && (
          <p className="soccer-empty">
            The league hasn’t posted games for this Team yet. A Session link includes games the league posts later.
          </p>
        )}

        {team.state === "ready" && team.games.length > 0 && shown.length === 0 && (
          <div className="soccer-empty">
            <p>No remaining games. The Session may be finished.</p>
            <button type="button" className="soccer-link" onClick={onShowAll}>
              Show all games
            </button>
          </div>
        )}

        {shown.length > 0 && (
          <ul className="soccer-games">
            {shown.map((g) => (
              <GameRow
                key={g.id}
                game={g}
                team={team}
                others={(players.get(g.id) ?? []).filter((n) => n !== team.name)}
                on={checked.has(g.id)}
                onToggle={onToggle}
              />
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function GameRow({
  game,
  team,
  others,
  on,
  onToggle,
}: {
  game: Game;
  team: Chosen;
  // Other chosen Teams in this game: it is one game, checked once for both.
  others: string[];
  on: boolean;
  onToggle: (id: string, on: boolean) => void;
}) {
  const when = gameWhen(game.start);
  return (
    <li>
      <label className="soccer-game">
        <input type="checkbox" checked={on} onChange={(e) => onToggle(game.id, e.target.checked)} />
        <span className="soccer-game-text">
          <span className="soccer-game-when">
            {when ? (
              <>
                <span>{when.date}</span>
                <span>{when.time}</span>
              </>
            ) : (
              <span>Time not posted</span>
            )}
            {!game.remaining && <span className="soccer-game-past">Earlier game</span>}
          </span>
          <span className="soccer-game-detail">
            <span>{matchup(game, team.name)}</span>
            <span>{fieldName(game.field)}</span>
          </span>
          {others.length > 0 && <span className="soccer-game-shared">One game, also under {listNames(others)}</span>}
        </span>
      </label>
    </li>
  );
}
