import { useState } from "react";
import { removeMyTeam, replaceMyLink, type MyCalendar } from "./api";
import { Icon, ToolDown } from "./parts";

type Working = "remove" | "replace" | null;
type Outcome = { kind: "copied" } | { kind: "manual" } | { kind: "replaced" } | { kind: "failed"; what: "remove" | "replace" };

// Teams in my calendar: a Member's Remembered teams and the Member link that
// serves them (docs/adr/0011-member-soccer-calendar-in-the-site-table.md).
// Each Team moves on to next Session's by itself when the league makes it
// clear which Team that is; otherwise the Session-over notice in the
// calendar leads back here.
export function MyCalendarCard({ mine, onChange }: { mine: MyCalendar; onChange: (m: MyCalendar) => void }) {
  const [working, setWorking] = useState<Working>(null);
  const [confirming, setConfirming] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  async function remove(id: string) {
    if (working) return;
    setOutcome(null);
    setWorking("remove");
    try {
      onChange(await removeMyTeam(id));
    } catch {
      setOutcome({ kind: "failed", what: "remove" });
    } finally {
      setWorking(null);
    }
  }

  async function replace() {
    if (working) return;
    setOutcome(null);
    setWorking("replace");
    try {
      onChange(await replaceMyLink());
      setConfirming(false);
      setOutcome({ kind: "replaced" });
    } catch {
      setOutcome({ kind: "failed", what: "replace" });
    } finally {
      setWorking(null);
    }
  }

  async function copy() {
    if (!mine.url) return;
    try {
      await navigator.clipboard.writeText(mine.url);
      setOutcome({ kind: "copied" });
    } catch {
      setOutcome({ kind: "manual" });
    }
  }

  return (
    <section className="soccer-card" aria-labelledby="mine-title">
      <h2 id="mine-title" className="soccer-card-title">
        Teams in my calendar
      </h2>
      <p className="soccer-help">
        Your Member link holds every game of these Teams. Each one moves on to next Session’s Team by itself when the
        league makes it clear which Team that is.
      </p>
      <ul className="soccer-rows">
        {mine.teams.map((t) => (
          <li key={t.team_ids[0]} className="soccer-row">
            <div className="soccer-row-text">
              <p className="soccer-row-name">{t.name}</p>
              <p className="soccer-row-detail">
                <span>Team ID {t.team_ids[0]}</span>
                {t.team_ids[1] && <span>Session before: {t.team_ids[1]}</span>}
              </p>
            </div>
            <button
              type="button"
              className="soccer-remove"
              aria-label={`Remove ${t.name} from my calendar`}
              disabled={working !== null}
              onClick={() => void remove(t.team_ids[0])}
            >
              <Icon name="close" />
            </button>
          </li>
        ))}
      </ul>

      {mine.url && (
        <div className="soccer-linkbox">
          <label className="soccer-label" htmlFor="member-link">
            Your Member link
          </label>
          <input
            id="member-link"
            className="soccer-input soccer-linkfield"
            type="text"
            readOnly
            value={mine.url}
            onFocus={(e) => e.currentTarget.select()}
          />
          <button type="button" className="quiet-button soccer-another" onClick={() => void copy()}>
            <Icon name="link" />
            Copy Member link
          </button>
          <p className="soccer-hint">
            Anyone with this link can see these Teams’ games. Add it to your calendar once, the way a Session link is
            added below; Teams you add or remove here change what it shows on its next update.
          </p>
        </div>
      )}

      <div className="soccer-outcome" role="status">
        {outcome?.kind === "copied" && <p className="soccer-done">Member link copied.</p>}
        {outcome?.kind === "manual" && (
          <p className="soccer-done">Copying didn’t work in this browser. Select the link above and copy it yourself.</p>
        )}
        {outcome?.kind === "replaced" && (
          <p className="soccer-done">
            Link replaced. The old one has stopped working: add this one to your calendar in its place.
          </p>
        )}
      </div>
      {outcome?.kind === "failed" && (
        <ToolDown
          what={outcome.what === "remove" ? "That Team wasn’t removed." : "Your link wasn’t replaced."}
          onRetry={outcome.what === "replace" ? () => void replace() : undefined}
        />
      )}

      {mine.url && (
        <div className="soccer-also">
          {confirming ? (
            <>
              <p className="soccer-blocked">
                A new link stops this one at once. Any calendar using it stops getting games until you add the new
                link.
              </p>
              <div className="soccer-confirm">
                <button type="button" className="quiet-button" onClick={() => void replace()} disabled={working !== null}>
                  {working === "replace" ? "Replacing…" : "Replace link"}
                </button>
                <button type="button" className="quiet-button" onClick={() => setConfirming(false)} disabled={working !== null}>
                  Keep this link
                </button>
              </div>
            </>
          ) : (
            <>
              <button
                type="button"
                className="quiet-button soccer-also-action"
                onClick={() => {
                  setOutcome(null);
                  setConfirming(true);
                }}
              >
                Replace link
              </button>
              <p className="soccer-hint">If someone has this link who shouldn’t, a new one stops the old address.</p>
            </>
          )}
        </div>
      )}
    </section>
  );
}
