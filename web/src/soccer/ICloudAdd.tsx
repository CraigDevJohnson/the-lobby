import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  addToICloud,
  connectICloud,
  disconnectICloud,
  MAX_MINE,
  setICloudDaily,
  ToolError,
  type FailureKind,
  type GoogleAdded,
  type ICloudStatus,
} from "./api";
import { addedText, downloadChoice, plural, type Picks } from "./logic";
import { Icon, LeagueDown, ToolDown } from "./parts";

type Outcome =
  | { kind: "connected" }
  | { kind: "added"; result: GoogleAdded; calendar: string }
  | { kind: "disconnected" }
  | { kind: "teams" } // the Teams on the page joined My calendar
  | { kind: "failed"; why: FailureKind; what: "connect" | "add" | "disconnect" | "daily" | "teams" };

// Add to iCloud Calendar (ADR 0012): a Member with Let's Plan Soccer access
// connects iCloud once, then each press writes the checked games into the
// calendar they pick. Pressing again rewrites them and adds games posted
// since. Between presses nothing happens unless the Member turns the daily
// update on (ADR 0015): then, once a day, the site adds the upcoming games of
// their Remembered teams that it hasn't added to that calendar yet, and moves
// one the league has moved, with an email to the Member (ADR 0017). Apple has
// no consent screen for this, so connecting takes an app-specific password
// the Member makes at Apple; it is sent once and never kept on the page.
// myTeams is how many Teams My calendar holds, once that has been read, and
// onAddTeams puts the Teams on the page into it.
export function ICloudAdd({
  status,
  onStatus,
  picks,
  count,
  myTeams,
  onAddTeams,
}: {
  status: ICloudStatus;
  onStatus: (s: ICloudStatus) => void;
  picks: Picks;
  count: number;
  myTeams?: number;
  onAddTeams: () => Promise<void>;
}) {
  const calendars = status.calendars ?? [];
  const calendarID = calendars.some((c) => c.id === status.calendar_id) ? status.calendar_id! : (calendars[0]?.id ?? "");
  const daily = status.daily === true;
  // The daily update adds the games of the Teams in My calendar, so with
  // none there it can't be turned on. It can always be turned off. The card
  // then offers to add the Teams on the page, so nobody has to go by way of
  // the Member link to use the switch.
  const noTeams = myTeams === 0;
  const dailySwitch = useRef<HTMLInputElement>(null);
  const [working, setWorking] = useState<"connect" | "add" | "disconnect" | "daily" | "teams" | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [appleID, setAppleID] = useState("");
  const [password, setPassword] = useState("");
  const chosen = calendars.find((c) => c.id === calendarID);

  // The button goes once the Teams are in My calendar, so the switch it was
  // in the way of takes the focus, after the change has rendered. On a phone
  // the Teams in my calendar card arrives above and pushes this one down, so
  // the switch is also brought back into view.
  useEffect(() => {
    if (outcome?.kind !== "teams") return;
    dailySwitch.current?.focus();
    dailySwitch.current?.closest(".soccer-also")?.scrollIntoView({ block: "nearest" });
  }, [outcome]);

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

  // Turns the daily update on for a calendar, or off. The switch and the
  // calendar show what was asked for while it is on its way, as Minecraft
  // Admin's switches do, and go back to how they were if it fails.
  async function setDaily(on: boolean, id: string) {
    if (working) return;
    const asked = { ...status, daily: on, calendar_id: on ? id : status.calendar_id };
    setOutcome(null);
    setWorking("daily");
    onStatus(asked);
    try {
      const now = await setICloudDaily(on, on ? id : "");
      onStatus({ ...asked, daily: now.daily });
    } catch (err) {
      const why = err instanceof ToolError ? err.kind : "tool";
      if (why === "expired") onStatus({ available: true, connected: false, expired: true });
      else {
        onStatus(status);
        setOutcome({ kind: "failed", why, what: "daily" });
      }
    } finally {
      setWorking(null);
    }
  }

  // Puts the Teams on the page into My calendar.
  async function addTeams() {
    if (working) return;
    setOutcome(null);
    setWorking("teams");
    try {
      await onAddTeams();
      setOutcome({ kind: "teams" });
    } catch (err) {
      setOutcome({ kind: "failed", why: err instanceof ToolError ? err.kind : "tool", what: "teams" });
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
            such as mail and contacts. The Lobby uses it only to list your calendars and add games: the ones you add,
            and new ones once a day if you turn the daily update on, which also moves a game the league has moved.{" "}
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

  const dailyFailed = outcome?.kind === "failed" && outcome.what === "daily";
  const teamsFailed = outcome?.kind === "failed" && outcome.what === "teams" ? outcome.why : null;
  const dailyNote = working === "daily" ? "Saving…" : "";
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
              // With the daily update on, the new calendar is saved at once,
              // so the one showing is always the one it adds to.
              if (daily) {
                void setDaily(true, e.target.value);
                return;
              }
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
      <Said outcome={dailyFailed || teamsFailed ? null : outcome} onRetry={() => void add()} />

      {calendars.length > 0 && (
        <div className="soccer-also">
          <div className="soccer-daily">
            <label className="soccer-switch" data-busy={working === "daily" || undefined}>
              <span className="soccer-switch-name">Add new games every day</span>
              <input
                ref={dailySwitch}
                type="checkbox"
                role="switch"
                checked={daily}
                aria-describedby={[dailyNote && "icloud-daily-note", noTeams && "icloud-daily-teams", "icloud-daily-about"]
                  .filter(Boolean)
                  .join(" ")}
                aria-disabled={working !== null || (noTeams && !daily) || undefined}
                onChange={() => {
                  if (!noTeams || daily) void setDaily(!daily, calendarID);
                }}
              />
              <span className="soccer-switch-track" aria-hidden="true">
                <span className="soccer-switch-knob" />
              </span>
              <span className="soccer-switch-state" aria-hidden="true">
                {daily ? "On" : "Off"}
              </span>
            </label>
            <span id="icloud-daily-note" className="soccer-switch-note">
              {dailyNote}
            </span>
          </div>
          {noTeams && (
            <>
              <p id="icloud-daily-teams" className="soccer-blocked">
                {daily ? "Nothing is added until My calendar has Teams." : "Add Teams to My calendar first."}
              </p>
              <button
                type="button"
                className="quiet-button soccer-also-action soccer-daily-teams"
                onClick={() => void addTeams()}
                disabled={working !== null}
              >
                {working === "teams" ? "Adding your Teams…" : `Add ${plural(picks.teams.length, "Team")} to My calendar`}
              </button>
            </>
          )}
          <div className="soccer-outcome" role="status">
            {outcome?.kind === "teams" && (
              <p className="soccer-done">
                Added to My calendar.{daily ? "" : " You can turn the daily update on now."}
              </p>
            )}
          </div>
          {teamsFailed &&
            (teamsFailed === "full" ? (
              <div className="soccer-notice" role="alert">
                <p className="soccer-notice-title">My calendar holds up to {MAX_MINE} Teams.</p>
                <p>Remove some Teams from this page, then try again.</p>
              </div>
            ) : teamsFailed === "league" ? (
              <LeagueDown what="Your Teams can’t be added to My calendar until it’s back." onRetry={() => void addTeams()} />
            ) : (
              <ToolDown what="Your Teams weren’t added to My calendar." onRetry={() => void addTeams()} />
            ))}
          <p id="icloud-daily-about" className="soccer-hint">
            Once a day, The Lobby’s daily update adds every upcoming game of the Teams in My calendar that isn’t in the
            calendar above yet, whatever you’ve checked. When the league moves a game after that, the daily update
            moves it in your calendar and emails you what changed.
          </p>
          <p className="soccer-hint">
            Games already there are left as they are until the league moves them. Anything you changed on a moved game
            in iCloud, such as notes or alerts, is replaced. A game you delete after the daily update has added it, or
            found it there, stays deleted, even if the league moves it. One you deleted before that comes back once. If
            your Member link is also in Apple Calendar, each game shows twice.
          </p>
          {dailyFailed && <ToolDown what="The daily update wasn’t changed." />}
        </div>
      )}

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
