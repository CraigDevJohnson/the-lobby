import { useRef, useState } from "react";
import { addMyTeams, MAX_MINE, removeMyTeam, revokeMyLink, ToolError, type MyCalendar } from "./api";
import { plural, type TeamSummary } from "./logic";
import { Icon, LeagueDown, ToolDown } from "./parts";

export function MyCalendarCard({
  mine,
  selected,
  onChange,
  say,
}: {
  mine: MyCalendar | null;
  selected: TeamSummary[];
  onChange: (m: MyCalendar) => boolean;
  say: (text: string) => void;
}) {
  const [working, setWorking] = useState<"add" | "remove" | "revoke" | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [failure, setFailure] = useState<"add" | "full" | "league" | "gone" | "remove" | "revoke" | null>(null);
  const list = useRef<HTMLUListElement>(null);
  const title = useRef<HTMLHeadingElement>(null);
  const revokeButton = useRef<HTMLButtonElement>(null);
  const held = new Set(mine?.teams.flatMap((team) => team.team_ids));
  const newTeams = selected.filter((team) => !held.has(team.id));
  const over = (mine?.teams.length ?? 0) + newTeams.length > MAX_MINE;

  async function remember() {
    if (working || over || newTeams.length === 0) return;
    setWorking("add");
    setFailure(null);
    try {
      if (onChange(await addMyTeams(newTeams.map((team) => team.id)))) {
        say(`Remembered ${plural(newTeams.length, "Team")} for daily calendar updates.`);
        requestAnimationFrame(() => title.current?.focus());
      }
    } catch (err) {
      const why = err instanceof ToolError ? err.kind : "tool";
      setFailure(why === "full" || why === "league" || why === "gone" ? why : "add");
    } finally {
      setWorking(null);
    }
  }

  async function remove(id: string, name: string) {
    if (working) return;
    setFailure(null);
    setWorking("remove");
    try {
      if (onChange(await removeMyTeam(id))) {
        say(`${name} removed from your remembered teams.`);
        requestAnimationFrame(() =>
          (list.current?.querySelector<HTMLButtonElement>("button") ?? title.current)?.focus(),
        );
      }
    } catch {
      setFailure("remove");
    } finally {
      setWorking(null);
    }
  }

  async function revoke() {
    if (working) return;
    setFailure(null);
    setWorking("revoke");
    try {
      if (onChange(await revokeMyLink())) {
        setConfirming(false);
        say("Your old Member link has stopped working.");
        requestAnimationFrame(() => title.current?.focus());
      }
    } catch {
      setFailure("revoke");
    } finally {
      setWorking(null);
    }
  }

  return (
    <section id="remembered-teams" className="soccer-card" aria-labelledby="mine-title">
      <h2 ref={title} tabIndex={-1} id="mine-title" className="soccer-card-title">
        Remembered teams
      </h2>
      <p className="soccer-help">
        Remember up to {MAX_MINE} teams for daily calendar updates. Each team follows its next Session when the league
        lists a clear match.
      </p>
      {mine?.teams.length ? (
        <ul ref={list} className="soccer-rows">
          {mine.teams.map((team) => (
            <li key={team.team_ids[0]} className="soccer-row">
              <div className="soccer-row-text">
                <p className="soccer-row-name">{team.name}</p>
                <p className="soccer-row-detail">
                  <span>Team ID {team.team_ids[0]}</span>
                  {team.team_ids[1] && <span>Previous Session · {team.team_ids[1]}</span>}
                </p>
              </div>
              <button
                type="button"
                className="soccer-remove"
                aria-label={`Forget ${team.name}`}
                disabled={working !== null}
                onClick={() => void remove(team.team_ids[0], team.name)}
              >
                <Icon name="close" />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      {newTeams.length > 0 ? (
        <button
          type="button"
          className="quiet-button soccer-another"
          onClick={() => void remember()}
          disabled={working !== null || over}
        >
          <Icon name="plus" />
          {working === "add" ? "Remembering…" : `Remember ${plural(newTeams.length, "selected team")}`}
        </button>
      ) : (
        selected.length === 0 && <p className="soccer-hint">Use Find a team to choose a team to remember.</p>
      )}
      {(over || failure === "full") && (
        <p className="soccer-blocked">
          You can remember up to {MAX_MINE} teams. Remove a selected or remembered team to make room.
        </p>
      )}
      {failure === "league" && <LeagueDown what="Your teams couldn’t be remembered." onRetry={() => void remember()} />}
      {failure === "gone" && (
        <p className="soccer-blocked" role="alert">
          The league no longer lists a selected team. Find its current Team ID and try again.
        </p>
      )}
      {failure === "add" && <ToolDown what="Your teams couldn’t be remembered." onRetry={() => void remember()} />}
      {failure === "remove" && <ToolDown what="That team wasn’t removed." />}
      {mine?.url && (
        <div className="soccer-also">
          <p className="soccer-hint">If you still subscribe to your old Member link, using daily updates in the same calendar app can show games twice.</p>
          {confirming ? (
            <>
              <p id="revoke-warning" className="soccer-blocked">
                Calendars using your old Member link will stop receiving updates. Your remembered teams and connected
                calendars stay here.
              </p>
              <div className="soccer-confirm">
                <button
                  type="button"
                  className="quiet-button"
                  autoFocus
                  aria-describedby="revoke-warning"
                  onClick={() => void revoke()}
                  disabled={working !== null}
                >
                  Stop Member link
                </button>
                <button
                  type="button"
                  className="quiet-button"
                  disabled={working !== null}
                  onClick={() => {
                    setConfirming(false);
                    requestAnimationFrame(() => revokeButton.current?.focus());
                  }}
                >
                  Keep link
                </button>
              </div>
            </>
          ) : (
            <button
              ref={revokeButton}
              type="button"
              className="quiet-button soccer-also-action"
              disabled={working !== null}
              onClick={() => setConfirming(true)}
            >
              Stop old Member link…
            </button>
          )}
          {failure === "revoke" && <ToolDown what="Your old Member link wasn’t stopped." />}
        </div>
      )}
    </section>
  );
}
