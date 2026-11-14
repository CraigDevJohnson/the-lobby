import { useState, type FormEvent } from "react";
import { addToICloud, connectICloud, disconnectICloud, ToolError, type FailureKind, type GoogleAdded, type ICloudStatus } from "./api";
import { addedText, downloadChoice, plural, type Picks } from "./logic";
import { Icon, LeagueDown, ToolDown } from "./parts";

type Outcome =
  | { kind: "connected" }
  | { kind: "added"; result: GoogleAdded; calendar: string }
  | { kind: "disconnected" }
  | { kind: "failed"; why: FailureKind; what: "connect" | "add" | "disconnect" };

// Add to iCloud Calendar (ADR 0012): a Member with Let's Plan Soccer access
// connects iCloud once, then each press writes the checked games into the
// calendar they pick. Pressing again rewrites them and adds games posted
// since; nothing happens between presses. Apple has no consent screen for
// this, so connecting takes an app-specific password the Member makes at
// Apple; it is sent once and never kept on the page.
export function ICloudAdd({
  status,
  onStatus,
  picks,
  count,
}: {
  status: ICloudStatus;
  onStatus: (s: ICloudStatus) => void;
  picks: Picks;
  count: number;
}) {
  const calendars = status.calendars ?? [];
  const calendarID = calendars.some((c) => c.id === status.calendar_id) ? status.calendar_id! : (calendars[0]?.id ?? "");
  const [working, setWorking] = useState<"connect" | "add" | "disconnect" | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [appleID, setAppleID] = useState("");
  const [password, setPassword] = useState("");
  const chosen = calendars.find((c) => c.id === calendarID);

  async function connect(e: FormEvent) {
    e.preventDefault();
    if (working || !appleID.trim() || !password.trim()) return;
    setOutcome(null);
    setWorking("connect");
    try {
      const now = await connectICloud(appleID.trim(), password.trim());
      setPassword("");
      setOutcome({ kind: "connected" });
      onStatus(now);
    } catch (err) {
      setOutcome({ kind: "failed", why: err instanceof ToolError ? err.kind : "tool", what: "connect" });
    } finally {
      setWorking(null);
    }
  }

  async function add() {
    if (working || count === 0 || !calendarID) return;
    setOutcome(null);
    setWorking("add");
    try {
      const result = await addToICloud(downloadChoice(picks), calendarID);
      setOutcome({ kind: "added", result, calendar: chosen?.name || "your calendar" });
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
      const now = await disconnectICloud();
      setOutcome({ kind: "disconnected" });
      onStatus(now);
    } catch {
      setOutcome({ kind: "failed", why: "tool", what: "disconnect" });
    } finally {
      setWorking(null);
    }
  }

  if (!status.connected) {
    const denied = outcome?.kind === "failed" && outcome.why === "denied";
    return (
      <form className="soccer-act" onSubmit={(e) => void connect(e)}>
        {status.expired && (
          <p className="soccer-blocked" role="alert">
            Apple no longer takes the password The Lobby had, so it was deleted. Connect again to add games.
          </p>
        )}
        <div className="soccer-requires">
          <p className="soccer-requires-title">This needs an app-specific password</p>
          <p>
            Apple has no sign-in screen for this. You make a separate password in your Apple Account and give it to The
            Lobby. Apple doesn’t limit it to calendars: it also opens the other iCloud data outside apps can reach,
            such as mail and contacts. The Lobby uses it only to list your calendars and add the games you add.{" "}
            <a href="/privacy#icloud-calendar">What The Lobby keeps</a>
          </p>
        </div>
        <ol className="soccer-steps" role="list">
          <li>
            Sign in at{" "}
            <a href="https://account.apple.com/" target="_blank" rel="noreferrer">
              account.apple.com
            </a>
            . Your account needs two-factor authentication.
          </li>
          <li>
            Open <b>Sign-In and Security</b> → <b>App-Specific Passwords</b> and make one named <b>The Lobby</b>.
          </li>
          <li>Enter it below with your Apple Account email. Never enter your Apple Account’s own password here.</li>
        </ol>

        <div className="soccer-icloud-field">
          <label className="soccer-label" htmlFor="icloud-apple-id">
            Apple Account email
          </label>
          <input
            id="icloud-apple-id"
            className="soccer-input"
            type="email"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            required
            value={appleID}
            onChange={(e) => setAppleID(e.target.value)}
          />
        </div>
        <div className="soccer-icloud-field">
          <label className="soccer-label" htmlFor="icloud-password">
            App-specific password
          </label>
          <input
            id="icloud-password"
            className="soccer-input"
            type="password"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            required
            aria-invalid={denied || undefined}
            aria-describedby={denied ? "icloud-denied" : undefined}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        {denied && (
          <p id="icloud-denied" className="soccer-blocked" role="alert">
            Apple didn’t accept that email and password. Check both, and that it’s an app-specific password, not your
            Apple Account’s own.
          </p>
        )}
        <button type="submit" className="tactile soccer-action soccer-icloud-connect" disabled={working !== null}>
          <span>{working === "connect" ? "Checking with Apple…" : "Connect iCloud Calendar"}</span>
          <Icon name="arrow" />
        </button>
        <Said outcome={denied ? null : outcome} />
      </form>
    );
  }

  return (
    <div className="soccer-act">
      <p className="soccer-hint">
        Connected as <b>{status.account}</b>.
      </p>
      {calendars.length === 0 ? (
        <p className="soccer-blocked" role="status">
          This iCloud account has no calendar The Lobby can add to. Make one in Apple Calendar, then come back.
        </p>
      ) : (
        <>
          <label className="soccer-label" htmlFor="icloud-calendar">
            Add to
          </label>
          <select
            id="icloud-calendar"
            className="soccer-input soccer-select soccer-google-calendar"
            value={calendarID}
            onChange={(e) => {
              onStatus({ ...status, calendar_id: e.target.value });
              setOutcome(null);
            }}
          >
            {calendars.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name || "Untitled calendar"}
              </option>
            ))}
          </select>
          <button
            type="button"
            className="tactile soccer-action"
            onClick={() => void add()}
            disabled={working !== null}
            aria-disabled={count === 0 || undefined}
            aria-describedby={count === 0 ? "no-icloud-games" : undefined}
          >
            <Icon name="plus" />
            <span>{working === "add" ? "Adding your games…" : `Add ${plural(count, "game")}`}</span>
          </button>
          {count === 0 && (
            <p id="no-icloud-games" className="soccer-blocked">
              Choose at least one game.
            </p>
          )}
          <p className="soccer-hint">
            Press again any time to update these games and add any posted since. Nothing is added twice.
          </p>
        </>
      )}
      <Said outcome={outcome} onRetry={() => void add()} />

      <div className="soccer-also">
        <button type="button" className="quiet-button soccer-also-action" onClick={() => void disconnect()} disabled={working !== null}>
          {working === "disconnect" ? "Disconnecting…" : "Disconnect iCloud Calendar"}
        </button>
        <p className="soccer-hint">
          The Lobby deletes the password it kept. To end the password itself, remove it at account.apple.com. Games
          already added stay in your calendar.
        </p>
      </div>
    </div>
  );
}

function Said({ outcome, onRetry }: { outcome: Outcome | null; onRetry?: () => void }) {
  return (
    <>
      <div className="soccer-outcome" role="status">
        {outcome?.kind === "connected" && (
          <p className="soccer-done">iCloud Calendar is connected. Choose a calendar, then add your games.</p>
        )}
        {outcome?.kind === "disconnected" && (
          <p className="soccer-done">
            Disconnected. Games already added stay in your calendar. The password still works at Apple until you remove
            it at account.apple.com.
          </p>
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
        ) : outcome.what === "connect" ? (
          <ToolDown what="iCloud Calendar wasn’t connected, and nothing was kept. Your Teams and choices are still here." />
        ) : outcome.what === "disconnect" ? (
          <ToolDown what="iCloud Calendar is still connected." />
        ) : (
          <ToolDown what="Your games weren’t added. Your Teams and choices are still here." onRetry={onRetry} />
        ))}
    </>
  );
}
