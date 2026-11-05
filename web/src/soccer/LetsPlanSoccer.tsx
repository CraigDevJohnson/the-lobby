import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import { ToolFooter } from "../ToolChrome";
import { addMyTeams, getMyCalendar, loadTeam, ToolError, type MyCalendar, type TeamSchedule } from "./api";
import { PitchCalendar } from "./art";
import { Delivery } from "./Delivery";
import { Finder } from "./Finder";
import { Games } from "./Games";
import { MyCalendarCard } from "./MyCalendar";
import { NextSession } from "./NextSession";
import { NO_PICKS, plural, reduce, type Chosen, type Problem, type TeamSummary } from "./logic";
import { Dots, Icon } from "./parts";

// The Let's Plan Soccer screen (docs/design/soccer-brief.md): find Boise
// Teams, review their games, then take them to a calendar as a download or a
// Session link. Visitors and Members see the same page, except that a Member
// with Let's Plan Soccer access also gets My calendar: their Remembered teams
// and the Member link that serves them (ADR 0011).
export function LetsPlanSoccer() {
  const [picks, dispatch] = useReducer(reduce, NO_PICKS);
  // Said to screen readers when the chosen Teams change.
  const [message, setMessage] = useState("");
  // The same words twice in a row are still spoken.
  const setSaid = useCallback((text: string) => {
    setMessage("");
    requestAnimationFrame(() => setMessage(text));
  }, []);
  // The header offers sign-in to Visitors and the way back to a Member. The
  // page itself is the same for both; if the check fails, nothing changes.
  const [member, setMember] = useState(false);
  // The signed-in Member with Let's Plan Soccer access, or "" for none. My
  // calendar is read again whenever this changes, so a different Member
  // signing in from another tab never sees the last one's calendar.
  const [owner, setOwner] = useState("");
  const granted = owner !== "";
  // Sign in comes back here with the address's ?after=, so a Member who
  // follows a Member link's notice signed out can still carry the Team on.
  const [signin, setSignin] = useState("/signin?next=/soccer");
  useEffect(() => setSignin(`/signin?next=${encodeURIComponent("/soccer" + location.search)}`), []);
  // Checked again whenever the page is shown, so My calendar goes once
  // sign-in ends in another tab.
  useEffect(() => {
    const check = () =>
      fetch("/api/me", { credentials: "same-origin", headers: { Accept: "application/json" } })
        .then((res) => (res.ok ? res.json() : null))
        .then((me: { role?: string; email?: string; tools?: string[] } | null) => {
          setMember(me?.role === "member");
          setOwner(me?.role === "member" && (me.tools ?? []).includes("soccer") ? (me.email ?? "member") : "");
        })
        .catch(() => {});
    void check();
    const onVisible = () => {
      if (document.visibilityState === "visible") void check();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, []);
  // My calendar, once read. If it can't be read, the Teams list stays
  // hidden; adding Teams still works and brings it back. A change made
  // before the first read answers wins over that read.
  const [mine, setMine] = useState<MyCalendar | null>(null);
  const changedMine = useRef(false);
  const changeMine = useCallback((m: MyCalendar) => {
    changedMine.current = true;
    setMine(m);
  }, []);
  useEffect(() => {
    changedMine.current = false;
    setMine(null);
    if (!owner) return;
    const stop = new AbortController();
    getMyCalendar(stop.signal)
      .then((m) => {
        if (!changedMine.current) setMine(m);
      })
      .catch(() => {});
    return () => stop.abort();
  }, [owner]);
  const addMine = useCallback(
    async (team: TeamSummary, after: string) => {
      try {
        changeMine(await addMyTeams([team.id], after));
        setSaid(`${team.name} added to my calendar.`);
        return null;
      } catch (err) {
        return err instanceof ToolError && err.kind === "full" ? "full" : "failed";
      }
    },
    [changeMine, setSaid],
  );
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
  }, [setSaid]);

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
    [picks.teams, setSaid],
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
          {member ? (
            <a className="page-back" href="/">
              The VIP Lobby
            </a>
          ) : (
            <a className="page-back" href={signin}>
              Sign in
            </a>
          )}
        </div>
      </header>

      <main className="soccer-main">
        <div className="soccer-opening">
          <div className="soccer-opening-copy">
            <h1 className="soccer-title">Let’s Plan Soccer</h1>
            <p className="soccer-lead">Your team’s games, in your calendar.</p>
            <p className="soccer-context">Boise · No sign-in needed</p>
          </div>
          <PitchCalendar selected={any} />
        </div>

        <noscript>
          <p className="soccer-notice soccer-noscript">
            Let’s Plan Soccer needs JavaScript to find teams and make calendar files and links.
          </p>
        </noscript>

        <div className="soccer-flow" data-teams={any || undefined}>
          <div className="soccer-build">
            <NextSession
              added={added}
              onAdd={add}
              mine={granted ? { held: new Set(mine?.teams.map((t) => t.team_ids[0])), onAdd: addMine } : undefined}
            />

            {mine && mine.teams.length > 0 && <MyCalendarCard key={owner} mine={mine} onChange={changeMine} say={setSaid} />}

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

          {any && <Delivery picks={picks} mine={granted ? { calendar: mine, onChange: changeMine } : undefined} />}
        </div>

        <p className="soccer-unofficial">Unofficial. Not affiliated with Let’s Play Soccer.</p>
        <p className="visually-hidden" role="status">
          {message}
        </p>
      </main>

      <ToolFooter />
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
