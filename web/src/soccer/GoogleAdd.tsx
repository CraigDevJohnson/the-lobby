import { useState } from "react";
import { addToGoogle, connectGoogle, disconnectGoogle, ToolError, type FailureKind, type GoogleAdded, type GoogleStatus } from "./api";
import { addedText, downloadChoice, plural, type Picks } from "./logic";
import { Icon, LeagueDown, ToolDown } from "./parts";

// How connecting went, as Google's return to /soccer?google= says.
const BACK: Record<string, string> = {
  connected: "Google Calendar is connected. Choose a calendar, then add your games.",
  denied: "Google Calendar wasn’t connected: The Lobby needs permission to add events. Connect again and allow it.",
  failed: "Connecting Google Calendar didn’t work. Please try again.",
  unavailable: "Add to Google Calendar isn’t available right now.",
};

type Outcome =
  | { kind: "back"; text: string }
  | { kind: "added"; result: GoogleAdded; calendar: string }
  | { kind: "disconnected" }
  | { kind: "failed"; why: FailureKind; what: "add" | "disconnect" };

// Add to Google Calendar (ADR 0007): a Member with Let's Plan Soccer access
// connects Google once, then each press writes the checked games into the
// calendar they pick. Pressing again rewrites them and adds games posted
// since; nothing happens between presses.
export function GoogleAdd({
  status,
  onStatus,
  picks,
  count,
  back,
}: {
  status: GoogleStatus;
  onStatus: (s: GoogleStatus) => void;
  picks: Picks;
  count: number;
  back?: string | null;
}) {
  const calendars = status.calendars ?? [];
  const [calendarID, setCalendarID] = useState(status.calendar_id ?? calendars[0]?.id ?? "");
  const [working, setWorking] = useState<"add" | "disconnect" | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(back && BACK[back] ? { kind: "back", text: BACK[back] } : null);
  const chosen = calendars.find((c) => c.id === calendarID);

  async function add() {
    if (working || count === 0) return;
    setOutcome(null);
    setWorking("add");
    try {
      const result = await addToGoogle(downloadChoice(picks), calendarID);
      setOutcome({ kind: "added", result, calendar: chosen?.name ?? "your calendar" });
    } catch (err) {
      const why = err instanceof ToolError ? err.kind : "tool";
      if (why === "expired") onStatus({ available: true, connected: false, expired: true });
      else setOutcome({ kind: "failed", why, what: "add" });
    } finally {
      setWorking(null);
    }
  }

  async function disconnect() {
    if (working) return;
    setOutcome(null);
    setWorking("disconnect");
    try {
      const now = await disconnectGoogle();
      setOutcome({ kind: "disconnected" });
      onStatus(now);
    } catch {
      setOutcome({ kind: "failed", why: "tool", what: "disconnect" });
    } finally {
      setWorking(null);
    }
  }

  if (!status.connected) {
    return (
      <div className="soccer-act">
        {status.expired && (
          <p className="soccer-blocked">Your Google access has ended. Connect again to add games.</p>
        )}
        <button type="button" className="tactile soccer-action" onClick={() => connectGoogle(picks.teams)}>
          <span>Connect Google Calendar</span>
          <Icon name="arrow" />
        </button>
        <p className="soccer-hint">
          Google asks you to choose an account and allow The Lobby to add events to your calendars. It may warn that
          the app isn’t verified yet.
        </p>
        <Said outcome={outcome} />
      </div>
    );
  }

  return (
    <div className="soccer-act">
      <p className="soccer-hint">
        Connected as <b>{status.account}</b>.
      </p>
      <label className="soccer-label" htmlFor="google-calendar">
        Add to
      </label>
      <select
        id="google-calendar"
        className="soccer-input soccer-select soccer-google-calendar"
        value={calendarID}
        onChange={(e) => {
          setCalendarID(e.target.value);
          setOutcome(null);
        }}
      >
        {calendars.map((c) => (
          <option key={c.id} value={c.id}>
            {c.primary ? `${c.name} (main calendar)` : c.name}
          </option>
        ))}
      </select>
      <button
        type="button"
        className="tactile soccer-action"
        onClick={() => void add()}
        disabled={working !== null}
        aria-disabled={count === 0 || undefined}
        aria-describedby={count === 0 ? "no-google-games" : undefined}
      >
        <Icon name="plus" />
        <span>{working === "add" ? "Adding your games…" : `Add ${plural(count, "game")}`}</span>
      </button>
      {count === 0 && (
        <p id="no-google-games" className="soccer-blocked">
          Choose at least one game.
        </p>
      )}
      <p className="soccer-hint">Press again any time to update these games and add any posted since. Nothing is added twice.</p>
      <Said outcome={outcome} onRetry={() => void add()} />

      <div className="soccer-also">
        <button type="button" className="quiet-button soccer-also-action" onClick={() => void disconnect()} disabled={working !== null}>
          {working === "disconnect" ? "Disconnecting…" : "Disconnect Google Calendar"}
        </button>
        <p className="soccer-hint">The Lobby gives up its access. Games already added stay in your calendar.</p>
      </div>
    </div>
  );
}

function Said({ outcome, onRetry }: { outcome: Outcome | null; onRetry?: () => void }) {
  return (
    <>
      <div className="soccer-outcome" role="status">
        {outcome?.kind === "back" && <p className="soccer-done">{outcome.text}</p>}
        {outcome?.kind === "disconnected" && (
          <p className="soccer-done">Disconnected. Games already added stay in your calendar.</p>
        )}
        {outcome?.kind === "added" && <p className="soccer-done">{addedText(outcome.result, outcome.calendar)}</p>}
      </div>
      {outcome?.kind === "failed" &&
        (outcome.why === "calendar" ? (
          <div className="soccer-notice" role="alert">
            <p className="soccer-notice-title">That calendar can’t take new events.</p>
            <p>It may be shared with you to view only, or gone. Pick another calendar, then try again.</p>
          </div>
        ) : outcome.why === "league" ? (
          <LeagueDown what="Your games can’t be added until it’s back. Your Teams and choices are still here." onRetry={onRetry} />
        ) : outcome.what === "disconnect" ? (
          <ToolDown what="Google Calendar is still connected." />
        ) : (
          <ToolDown what="Your games weren’t added. Your Teams and choices are still here." onRetry={onRetry} />
        ))}
    </>
  );
}
