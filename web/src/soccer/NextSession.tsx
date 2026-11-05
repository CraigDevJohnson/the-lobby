import { useCallback, useEffect, useRef, useState } from "react";
import { MAX_MINE, nextSession, ToolError, type FailureKind } from "./api";
import { afterTeamIDs, earlierName, MAX_TEAMS, nextSummary, type NextSessionAnswer, type TeamSummary } from "./logic";
import { Dots, LeagueDown, TeamDetails, ToolDown } from "./parts";

type Look = { kind: "loading" } | { kind: "found"; answer: NextSessionAnswer } | { kind: "failed"; why: FailureKind };

const LOADING: Look = { kind: "loading" };

// A Member with Let's Plan Soccer access can also put a Team straight into
// My calendar, carrying on the Remembered team it comes after: held is the
// newest Team ID of each Remembered team, and onAdd reports whether it worked.
// onAdd answers null once the Team is added, or why it wasn't: "full" when
// My calendar already holds its most Teams.
export type MineAdd = {
  held: ReadonlySet<string>;
  onAdd: (team: TeamSummary, after: string) => Promise<"full" | "failed" | null>;
};

// The way on from an ended Session. A Session link's end-of-Session notice
// opens /soccer?after=<Team ID>&after=…, and for each of those Teams this
// shows the Teams in the next Session that may be it. The link itself never
// follows them, and nothing is added until the person chooses. A Member
// link's notice opens the same page, where the pick joins My calendar.
export function NextSession({
  added,
  onAdd,
  mine,
}: {
  added: ReadonlySet<string>;
  onAdd: (team: TeamSummary) => void;
  mine?: MineAdd;
}) {
  // Read once mounted: the page is prerendered without an address, so the
  // server and the browser's first render both show nothing here.
  const [ids, setIds] = useState<readonly string[]>([]);
  useEffect(() => {
    setIds(afterTeamIDs(window.location.search));
  }, []);

  const [looks, setLooks] = useState<Record<string, Look>>({});
  // Only a Team's newest lookup may fill its group.
  const latest = useRef<Record<string, number>>({});

  const look = useCallback(async (id: string, signal?: AbortSignal) => {
    const n = (latest.current[id] ?? 0) + 1;
    latest.current[id] = n;
    setLooks((l) => ({ ...l, [id]: LOADING }));
    let result: Look;
    try {
      result = { kind: "found", answer: await nextSession(id, signal) };
    } catch (err) {
      result = { kind: "failed", why: err instanceof ToolError ? err.kind : "tool" };
    }
    if (!signal?.aborted && latest.current[id] === n) setLooks((l) => ({ ...l, [id]: result }));
  }, []);

  // One lookup at a time, in the link's order, to go easy on the league.
  useEffect(() => {
    if (ids.length === 0) return;
    const stop = new AbortController();
    void (async () => {
      for (const id of ids) {
        if (stop.signal.aborted) return;
        await look(id, stop.signal);
      }
    })();
    return () => stop.abort();
  }, [ids, look]);

  if (ids.length === 0) return null;
  const full = added.size >= MAX_TEAMS;

  return (
    <section className="soccer-card" aria-labelledby="next-title">
      <h2 id="next-title" className="soccer-card-title">
        Next Session
      </h2>
      <p className="soccer-help">
        {mine && ids.some((id) => mine.held.has(id))
          ? "Add next Session’s Team to my calendar, and its games join your Member link with the Session before. Or add Teams here for a new Session link below."
          : mine
            ? "Add next Session’s Team to my calendar, or add Teams here for a new Session link below."
            : "A Session link covers one Session. Add next Session’s Teams here, then get a new calendar below."}
      </p>
      {full && <p className="soccer-hint">You can combine up to {MAX_TEAMS} Teams. Remove one to add another.</p>}
      {ids.map((id) => (
        <Earlier
          key={id}
          id={id}
          look={looks[id] ?? LOADING}
          added={added}
          full={full}
          onAdd={onAdd}
          mine={mine}
          onRetry={() => void look(id)}
        />
      ))}
    </section>
  );
}

// One Team from the ended Session, and the Teams that may be it now.
function Earlier({
  id,
  look,
  added,
  full,
  onAdd,
  mine,
  onRetry,
}: {
  id: string;
  look: Look;
  added: ReadonlySet<string>;
  full: boolean;
  onAdd: (team: TeamSummary) => void;
  mine?: MineAdd;
  onRetry: () => void;
}) {
  const team = look.kind === "found" ? look.answer.team : null;
  const name = earlierName(id, team);
  const down = look.kind === "failed" && (look.why === "league" || look.why === "tool");
  // The same status line goes from looking to what was found, so the change
  // is spoken where it happens. A screen reader speaks the line but not the
  // heading above it, so the line names its Team first, unseen; the line
  // about an unknown Team ID names it already.
  const line =
    look.kind === "loading"
      ? "Looking for next Session’s Team…"
      : look.kind === "found"
        ? nextSummary(look.answer)
        : `No Team has Team ID ${id}.`;

  return (
    <div className="soccer-team soccer-earlier">
      <h3 className="soccer-team-heading">
        <span className="soccer-team-name">{name}</span>{" "}
        {team?.season && <span className="soccer-team-summary">Session {team.season}</span>}
      </h3>

      {down ? (
        look.why === "league" ? (
          <LeagueDown onRetry={onRetry} retryFor={name} />
        ) : (
          <ToolDown onRetry={onRetry} retryFor={name} />
        )
      ) : (
        <p className="soccer-status" role="status">
          {look.kind === "loading" && <Dots />}
          {look.kind !== "failed" && <span className="visually-hidden">{name}: </span>}
          {line}
        </p>
      )}

      {look.kind === "found" && (
        <Choices answer={look.answer} added={added} full={full} onAdd={onAdd} mine={mine && { ...mine, after: id }} />
      )}
    </div>
  );
}

type MineAfter = MineAdd & { after: string };

function Choices({
  answer,
  added,
  full,
  onAdd,
  mine,
}: {
  answer: NextSessionAnswer;
  added: ReadonlySet<string>;
  full: boolean;
  onAdd: (team: TeamSummary) => void;
  mine?: MineAfter;
}) {
  const none = answer.match === "none" || answer.teams.length === 0;
  const follow = answer.match === "follow" && answer.teams.length === 1;
  const similar = `similar-${answer.team.id}`;

  if (!none) {
    return (
      <ul className="soccer-rows">
        {answer.teams.map((t) => (
          <Choice
            key={t.id}
            team={t}
            note={follow ? undefined : t.same_coach ? "Same coach" : "Different coach"}
            added={added}
            full={full}
            onAdd={onAdd}
            mine={mine}
          />
        ))}
      </ul>
    );
  }
  if (answer.similar.length === 0) return null;
  return (
    <>
      <h4 id={similar} className="soccer-label soccer-similar">
        Same coach, new name
      </h4>
      <ul className="soccer-rows" aria-labelledby={similar}>
        {answer.similar.map((t) => (
          <Choice key={t.id} team={t} added={added} full={full} onAdd={onAdd} mine={mine} />
        ))}
      </ul>
    </>
  );
}

function Choice({
  team,
  note,
  added,
  full,
  onAdd,
  mine,
}: {
  team: TeamSummary;
  note?: string;
  added: ReadonlySet<string>;
  full: boolean;
  onAdd: (team: TeamSummary) => void;
  mine?: MineAfter;
}) {
  const [adding, setAdding] = useState(false);
  const [failed, setFailed] = useState<"full" | "failed" | null>(null);
  const summary = { id: team.id, name: team.name, division: team.division, season: team.season };
  async function addToMine() {
    if (!mine || adding) return;
    setAdding(true);
    setFailed(await mine.onAdd(summary, mine.after));
    setAdding(false);
  }
  return (
    <li className="soccer-row">
      <div className="soccer-row-text">
        <p className="soccer-row-name">{team.name}</p>
        <p className="soccer-row-detail">
          <TeamDetails team={team} />
        </p>
        {note && <p className="soccer-row-detail">{note}</p>}
        {failed && (
          <p className="soccer-blocked" role="alert">
            {failed === "full"
              ? `Not added: my calendar holds ${MAX_MINE} Teams. Remove one from it to add this one.`
              : "Not added to my calendar. Please try again in a moment."}
          </p>
        )}
      </div>
      {mine &&
        (mine.held.has(team.id) ? (
          <p className="soccer-added">In my calendar</p>
        ) : (
          <button type="button" className="quiet-button soccer-add" disabled={adding} onClick={() => void addToMine()}>
            {adding ? "Adding…" : "Add to my calendar"}
            <span className="visually-hidden">
              : {team.name}, Team ID {team.id}
            </span>
          </button>
        ))}
      {added.has(team.id) ? (
        <p className="soccer-added">Added</p>
      ) : (
        <button
          type="button"
          className="quiet-button soccer-add"
          disabled={full}
          onClick={() => onAdd(summary)}
        >
          Add team
          <span className="visually-hidden">
            : {team.name}, Team ID {team.id}
          </span>
        </button>
      )}
    </li>
  );
}
