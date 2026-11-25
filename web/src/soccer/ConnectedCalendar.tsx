import { useState } from "react";
import {
  addToGoogle,
  addToICloud,
  connectGoogle,
  disconnectGoogle,
  disconnectICloud,
  setCalendarDaily,
  ToolError,
  type CalendarProvider,
  type CalendarStatus,
  type FailureKind,
  type CalendarAdded,
} from "./api";
import { ICloudConnect } from "./ICloudConnect";
import { addedText, downloadChoice, plural, type Picks } from "./logic";
import { Icon, LeagueDown, ToolDown } from "./parts";

const GOOGLE_BACK: Record<string, string> = {
  connected: "Google Calendar is connected. Choose a calendar below.",
  denied: "Google Calendar wasn’t connected. Connect again and allow The Lobby to add events.",
  failed: "Connecting Google Calendar didn’t work. Please try again.",
  unavailable: "Add to Google Calendar isn’t available right now.",
};

export type CalendarAction = "connect" | "daily" | "add" | "disconnect" | null;

type Outcome =
  | { kind: "notice"; text: string }
  | { kind: "added"; result: CalendarAdded; calendar: string }
  | { kind: "failed"; why: FailureKind; what: "add" | "daily" | "disconnect" };

export function ConnectedCalendar({
  provider,
  status,
  onStatus,
  picks,
  count,
  myTeams,
  googleBack,
  blocked,
  working,
  setWorking,
}: {
  provider: CalendarProvider;
  status: CalendarStatus;
  onStatus: (s: CalendarStatus) => void;
  picks: Picks;
  count: number;
  myTeams?: number;
  googleBack?: string | null;
  blocked?: string;
  working: CalendarAction;
  setWorking: (action: CalendarAction) => void;
}) {
  const google = provider === "google";
  const name = google ? "Google Calendar" : "iCloud Calendar";
  const calendars = status.calendars ?? [];
  const saved = calendars.find((cal) => cal.id === status.calendar_id);
  const chosen = saved ?? calendars[0];
  const daily = status.daily === true && saved !== undefined;
  const paused = status.daily_unavailable === true;
  const [outcome, setOutcome] = useState<Outcome | null>(() => {
    const message = google && googleBack && GOOGLE_BACK[googleBack];
    return message && (googleBack !== "connected" || status.connected) ? { kind: "notice", text: message } : null;
  });

  function failed(err: unknown, what: "add" | "daily" | "disconnect") {
    const why = err instanceof ToolError ? err.kind : "tool";
    if (why === "expired") onStatus({ available: true, connected: false, expired: true });
    else setOutcome({ kind: "failed", why, what });
  }

  async function add() {
    if (working || !chosen || blocked || count === 0) return;
    setWorking("add");
    setOutcome(null);
    try {
      const write = google ? addToGoogle : addToICloud;
      const result = await write(downloadChoice(picks), chosen.id);
      setOutcome({ kind: "added", result, calendar: chosen.name || "your calendar" });
    } catch (err) {
      failed(err, "add");
    } finally {
      setWorking(null);
    }
  }

  async function setDaily(on: boolean, id: string) {
    if (working) return;
    setWorking("daily");
    setOutcome(null);
    try {
      const now = await setCalendarDaily(provider, on, id);
      onStatus({ ...status, daily: now.daily, daily_unavailable: false, calendar_id: on ? id : status.calendar_id });
      if (paused && !on) setOutcome({ kind: "notice", text: "Daily updates are off." });
    } catch (err) {
      failed(err, "daily");
    } finally {
      setWorking(null);
    }
  }

  async function disconnect() {
    if (working) return;
    setWorking("disconnect");
    setOutcome(null);
    try {
      onStatus(await (google ? disconnectGoogle() : disconnectICloud()));
      setOutcome({
        kind: "notice",
        text: google
          ? "Disconnected. Games already added stay in your calendar."
          : "Disconnected. Games already added stay in your calendar. Remove the app-specific password at account.apple.com to end it at Apple.",
      });
    } catch (err) {
      failed(err, "disconnect");
    } finally {
      setWorking(null);
    }
  }

  const notice = (
    <div className="soccer-outcome" role="status">
      {outcome?.kind === "notice" && <p className="soccer-done">{outcome.text}</p>}
      {outcome?.kind === "added" && <p className="soccer-done">{addedText(outcome.result, outcome.calendar)}</p>}
    </div>
  );

  if (!status.connected) {
    return (
      <>
        {status.expired && (
          <p className="soccer-blocked" role="alert">
            {google
              ? "Your Google access has ended. Connect again to add games."
              : "Apple no longer takes the password The Lobby had, so it was deleted. Connect again to add games."}
          </p>
        )}
        {google ? (
          <div className="soccer-act">
            <button
              type="button"
              className="tactile soccer-action"
              onClick={() => connectGoogle(picks.teams, picks.checked)}
            >
              <span>Connect Google Calendar</span>
              <Icon name="arrow" />
            </button>
            <p className="soccer-hint">
              Choose a Google account and allow The Lobby to add events. Google may warn that the app isn’t verified
              yet.
            </p>
          </div>
        ) : (
          <ICloudConnect
            working={working === "connect"}
            onWorking={(busy) => setWorking(busy ? "connect" : null)}
            onStatus={(s) => {
              onStatus(s);
              setOutcome({ kind: "notice", text: "iCloud Calendar is connected. Choose a calendar below." });
            }}
          />
        )}
        {notice}
      </>
    );
  }

  const switchBlocked = !daily && !myTeams;
  return (
    <div className="soccer-act">
      <p className="soccer-hint">
        Connected as <b>{status.account}</b>.
      </p>
      {paused && (
        <div className="soccer-notice">
          <p className="soccer-notice-title">Daily updates are paused.</p>
          <p>
            {chosen
              ? "Your saved calendar is unavailable. Choose a writable calendar and turn on daily updates to use it instead."
              : "Your saved calendar is unavailable."}{" "}
            Updates resume if the saved calendar becomes writable again.
          </p>
          <button
            type="button"
            className="quiet-button"
            disabled={working !== null}
            onClick={() => void setDaily(false, "")}
          >
            Turn off paused daily update
          </button>
        </div>
      )}
      {chosen ? (
        <>
          <label className="soccer-label" htmlFor={`${provider}-calendar`}>
            Add to
          </label>
          <select
            id={`${provider}-calendar`}
            className="soccer-input soccer-select soccer-google-calendar"
            value={chosen.id}
            disabled={working !== null}
            onChange={(e) => {
              if (daily) void setDaily(true, e.target.value);
              else {
                onStatus({ ...status, calendar_id: e.target.value });
                setOutcome(null);
              }
            }}
          >
            {calendars.map((cal) => (
              <option key={cal.id} value={cal.id}>
                {cal.name || "Untitled calendar"}
                {cal.primary ? " (main calendar)" : ""}
              </option>
            ))}
          </select>
          {picks.teams.length > 0 && (
            <>
              <button
                type="button"
                className="tactile soccer-action"
                onClick={() => void add()}
                disabled={working !== null || !!blocked || count === 0}
              >
                <Icon name="plus" />
                <span>{working === "add" ? "Adding your games…" : `Add ${plural(count, "game")}`}</span>
              </button>
              {(blocked || count === 0) && (
                <p className="soccer-blocked">{blocked ?? "Choose at least one game to add now."}</p>
              )}
            </>
          )}
          <div className="soccer-also">
            <div className="soccer-daily">
              <label className="soccer-switch" data-busy={working === "daily" || undefined}>
                <span className="soccer-switch-name">Add new games every day</span>
                <input
                  type="checkbox"
                  role="switch"
                  checked={daily}
                  aria-disabled={working !== null || switchBlocked || undefined}
                  aria-describedby={switchBlocked ? "daily-about daily-teams" : "daily-about"}
                  onChange={() => {
                    if (!working && !switchBlocked) void setDaily(!daily, chosen.id);
                  }}
                />
                <span className="soccer-switch-track" aria-hidden="true">
                  <span className="soccer-switch-knob" />
                </span>
                <span className="soccer-switch-state" aria-hidden="true">
                  {daily ? "On" : "Off"}
                </span>
              </label>
              <span className="soccer-switch-note" role="status">
                {working === "daily" && "Saving…"}
              </span>
            </div>
            <p id="daily-about" className="soccer-hint">
              Once a day, The Lobby adds upcoming games for your remembered teams to this calendar. When the league
              moves a game, The Lobby updates it and emails you what changed. Teams carry over when the league lists a
              clear match for the next Session.
            </p>
            {myTeams === 0 && (
              <p id="daily-teams" className="soccer-blocked">
                <a href="#remembered-teams">Remember a team</a> for the daily update.
              </p>
            )}
            {myTeams === undefined && (
              <p id="daily-teams" className="soccer-blocked">
                Your remembered teams haven’t loaded. Refresh the page to try again.
              </p>
            )}
            <p className="soccer-hint">
              The daily update leaves your edits alone until the league moves a game. Updating a moved game can replace
              your notes and alerts. After the daily update has seen a game, you can delete it and it stays deleted.
            </p>
          </div>
        </>
      ) : (
        <p className="soccer-blocked" role="status">
          This account has no writable calendars. Create one in {name}, then refresh this page.
        </p>
      )}
      {notice}
      {outcome?.kind === "failed" &&
        (outcome.why === "calendar" ? (
          <p className="soccer-blocked" role="alert">
            That calendar can’t take new events. Pick another calendar, then try again.
          </p>
        ) : outcome.what === "add" && outcome.why === "league" ? (
          <LeagueDown what="Your games couldn’t be added." onRetry={() => void add()} />
        ) : (
          <ToolDown
            what={
              outcome.what === "daily"
                ? "The daily update wasn’t changed."
                : outcome.what === "disconnect"
                  ? `${name} is still connected.`
                  : "Your games weren’t added."
            }
            onRetry={outcome.what === "add" ? () => void add() : undefined}
          />
        ))}
      <div className="soccer-also">
        <button
          type="button"
          className="quiet-button soccer-also-action"
          onClick={() => void disconnect()}
          disabled={working !== null}
        >
          {working === "disconnect" ? "Disconnecting…" : `Disconnect ${name}`}
        </button>
        <p className="soccer-hint">
          {google
            ? "The Lobby gives up its access."
            : "To end the app-specific password at Apple, remove it at account.apple.com."}{" "}
          Games already added stay in your calendar.
        </p>
      </div>
    </div>
  );
}
