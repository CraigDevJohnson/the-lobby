import { useEffect, useRef, useState, type ReactNode } from "react";
import { createLink, downloadCalendar, ToolError, type FailureKind } from "./api";
import { allGames, downloadChoice, listNames, plural, teamsKey, webcal, type Picks } from "./logic";
import { Dots, Icon, LeagueDown, ToolDown } from "./parts";

type Calendar = "apple" | "google" | "outlook";
type Method = "link" | "file";

const CALENDARS: { id: Calendar; name: string }[] = [
  { id: "apple", name: "Apple Calendar" },
  { id: "google", name: "Google Calendar" },
  { id: "outlook", name: "Outlook" },
];

// What must be true before starting, shown above the action. These are the
// limits each calendar's own instructions state; none of it has been proved
// on real devices for this site yet (docs/design/soccer-brief.md).
const REQUIRES: Record<Calendar, { title: string; body: string } | null> = {
  apple: null,
  google: {
    title: "Setup needs a computer",
    body: "The Google Calendar phone app cannot add a calendar from a link or import this file.",
  },
  outlook: {
    title: "Use Outlook on the web on a computer",
    body: "That is the setup Microsoft documents for adding a calendar from a link or a file.",
  },
};

const LINK_STEPS: Record<Calendar, ReactNode[]> = {
  apple: [
    "Copy the Session link.",
    <>
      In Calendar, open <b>Calendars</b> → <b>Add Calendar</b> → <b>Add Subscription Calendar</b>.
    </>,
    <>
      Paste the link, then tap <b>Find</b>—or <b>Subscribe</b> on older iOS versions—and finish adding it.
    </>,
  ],
  google: [
    "Copy the Session link.",
    "On a computer, open Google Calendar.",
    <>
      Choose <b>+</b> beside <b>Other calendars</b> → <b>From URL</b>.
    </>,
    "Paste the Session link and add the calendar.",
    "On your phone, make sure that calendar is selected in the app’s menu.",
  ],
  outlook: [
    "Copy the Session link.",
    <>
      In Outlook on the web, open <b>Calendar</b> → <b>Add calendar</b> → <b>Subscribe from web</b>.
    </>,
    "Paste the Session link and finish adding it.",
  ],
};

const FILE_STEPS: Record<Calendar, ReactNode[]> = {
  apple: [
    "Download the .ics file.",
    <>
      On a Mac, open Calendar → <b>File</b> → <b>Import</b>, choose your downloaded file, and select a calendar.
    </>,
  ],
  google: [
    "Download the .ics file.",
    <>
      On a computer, open Google Calendar → <b>Settings</b> → <b>Import &amp; Export</b>.
    </>,
    "Select your downloaded .ics file, choose a calendar, and import it.",
  ],
  outlook: [
    "Download the .ics file.",
    <>
      In Outlook on the web, open <b>Calendar</b> → <b>Add calendar</b> → <b>Upload from file</b>.
    </>,
    "Choose your downloaded .ics file, select a calendar, and import it.",
  ],
};

const LINK_NOTES: Record<Calendar, string> = {
  apple: "Your calendar checks for updates, so changes may take time to appear.",
  google:
    "On a phone, keep this link somewhere you can open on your computer; copying it doesn’t move it there. Changes may take time to appear.",
  outlook: "Schedule changes can take more than 24 hours to appear.",
};

const FILE_NOTES: Record<Calendar, string> = {
  apple:
    "For a file import on iPhone, Apple supports opening an .ics attachment in Mail. For the simpler phone setup, use the Session link.",
  google:
    "On a phone, the file has to end up somewhere your computer can open it. Downloading doesn’t add games to the Google Calendar phone app.",
  outlook: "This is a one-time copy. It won’t receive later schedule changes.",
};

type Working = "link" | "open" | "file" | null;
type Outcome =
  | { kind: "copied" }
  | { kind: "manual" } // the link exists but the clipboard refused it
  | { kind: "started" } // the browser was handed the file
  | { kind: "opened" } // Apple Calendar was asked to open
  | { kind: "failed"; what: Method; why: FailureKind };

// Choose the calendar and how the games get there, then hand off. The page
// only ever says what it saw happen: a download started, or a link copied.
export function Delivery({ picks }: { picks: Picks }) {
  const [calendar, setCalendar] = useState<Calendar | null>(null);
  const [method, setMethod] = useState<Method | null>(null);
  const [link, setLink] = useState<{ key: string; url: string } | null>(null);
  const [working, setWorking] = useState<Working>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const linkField = useRef<HTMLInputElement>(null);

  const teams = picks.teams;
  const key = teamsKey(teams);
  const pending = teams.find((t) => t.state === "loading");
  const failed = teams.find((t) => t.state === "failed");
  const games = allGames(teams);
  const count = games.filter((g) => picks.checked.has(g.id)).length;
  const names = listNames(teams.map((t) => t.name));
  const current = link?.key === key ? link.url : null;
  // A link made for other Teams is still good for those Teams, not these.
  const stale = link !== null && link.key !== key;

  // A link belongs to the Teams it was made for. Changing Teams does not
  // edit a link already handed out; it only means the next one is new.
  useEffect(() => {
    setOutcome(null);
  }, [key]);

  // When copying fails, the link is put in reach for copying by hand.
  useEffect(() => {
    if (outcome?.kind === "manual") linkField.current?.select();
  }, [outcome]);

  function fail(what: Method, err: unknown) {
    setOutcome({ kind: "failed", what, why: err instanceof ToolError ? err.kind : "tool" });
  }

  async function copy() {
    if (working) return;
    setOutcome(null);
    if (current) {
      try {
        await navigator.clipboard.writeText(current);
        setOutcome({ kind: "copied" });
      } catch {
        setOutcome({ kind: "manual" });
      }
      return;
    }
    setWorking("link");
    const making = createLink(teams.map((t) => t.id));
    // Start the clipboard write inside the tap, before the link exists:
    // some browsers refuse a write that begins after waiting on the network.
    let written: Promise<void> | null = null;
    try {
      const text = making.then((url) => new Blob([url], { type: "text/plain" }));
      written = navigator.clipboard.write([new ClipboardItem({ "text/plain": text })]);
      written.catch(() => {});
    } catch {
      written = null;
    }
    try {
      const url = await making;
      setLink({ key, url });
      try {
        if (!written) throw new Error("no clipboard");
        await written;
        setOutcome({ kind: "copied" });
      } catch {
        setOutcome({ kind: "manual" });
      }
    } catch (err) {
      fail("link", err);
    } finally {
      setWorking(null);
    }
  }

  async function openInApple() {
    if (working) return;
    setOutcome(null);
    let url = current;
    if (!url) {
      setWorking("open");
      try {
        url = await createLink(teams.map((t) => t.id));
        setLink({ key, url });
      } catch (err) {
        fail("link", err);
        return;
      } finally {
        setWorking(null);
      }
    }
    setOutcome({ kind: "opened" });
    window.location.href = webcal(url);
  }

  async function download() {
    if (working || count === 0) return;
    setOutcome(null);
    setWorking("file");
    try {
      const file = await downloadCalendar(downloadChoice(picks));
      const href = URL.createObjectURL(file.blob);
      const a = document.createElement("a");
      a.href = href;
      a.download = file.filename;
      document.body.append(a);
      a.click();
      a.remove();
      window.setTimeout(() => URL.revokeObjectURL(href), 60_000);
      setOutcome({ kind: "started" });
    } catch (err) {
      fail("file", err);
    } finally {
      setWorking(null);
    }
  }

  const blocked = failed
    ? `Games for ${failed.name} didn’t load. Retry or remove that Team first, so your calendar isn’t missing its games.`
    : pending
      ? null
      : undefined;

  // What happened, said right under the action that caused it.
  const result = (
    <>
      <div className="soccer-outcome" role="status">
        {outcome?.kind === "copied" && (
          <p className="soccer-done">Session link copied. Follow the steps above to add it to your calendar.</p>
        )}
        {outcome?.kind === "started" && (
          <p className="soccer-done">Download started. Follow the steps above to import the file.</p>
        )}
        {outcome?.kind === "opened" && (
          <p className="soccer-done">
            Asked Apple Calendar to open. If nothing happened, copy the Session link and follow the steps above.
          </p>
        )}
        {outcome?.kind === "manual" && (
          <p className="soccer-done">
            Copying didn’t work in this browser. The link is selected below: copy it yourself, then follow the steps above.
          </p>
        )}
      </div>

      {outcome?.kind === "failed" &&
        (outcome.why === "gone" ? (
          <div className="soccer-notice" role="alert">
            <p className="soccer-notice-title">The league no longer lists one of your Teams.</p>
            <p>Its Team ID may be from an earlier Session. Remove it, find the Team again, then retry.</p>
          </div>
        ) : outcome.why === "league" ? (
          <LeagueDown
            what={`${outcome.what === "link" ? "Your Session link" : "Your download"} can’t be made until it’s back. Your Teams and choices are still here.`}
            onRetry={outcome.what === "link" ? copy : download}
          />
        ) : (
          <ToolDown
            what={`${outcome.what === "link" ? "Your Session link wasn’t made" : "Your download didn’t start"}. Your Teams and choices are still here.`}
            onRetry={outcome.what === "link" ? copy : download}
          />
        ))}
    </>
  );

  return (
    <section id="get-your-calendar" className="soccer-card soccer-deliver" aria-labelledby="deliver-title">
      <h2 id="deliver-title" className="soccer-card-title">
        Get your calendar
      </h2>
      <p className="soccer-help">No Lobby sign-in needed. Your calendar provider may ask you to sign in.</p>

      <fieldset className="soccer-choice">
        <legend className="soccer-label">Your calendar</legend>
        <div className="soccer-options soccer-options-calendars">
          {CALENDARS.map((c) => (
            <label key={c.id} className="soccer-option">
              <input
                type="radio"
                name="calendar"
                value={c.id}
                checked={calendar === c.id}
                onChange={() => {
                  setCalendar(c.id);
                  setOutcome(null);
                }}
              />
              <span className="soccer-option-name">{c.name}</span>
            </label>
          ))}
        </div>
      </fieldset>

      <fieldset className="soccer-choice">
        <legend className="soccer-label">How you want it</legend>
        <div className="soccer-options">
          <label className="soccer-option soccer-option-tall">
            <input
              type="radio"
              name="method"
              value="link"
              checked={method === "link"}
              onChange={() => {
                setMethod("link");
                setOutcome(null);
              }}
              aria-describedby="link-about"
            />
            <span>
              <span className="soccer-option-name">Session link</span>
              <span id="link-about" className="soccer-option-about">
                Subscribe to all games for your chosen Teams in this league Session, including games posted later. Use
                a new link for the next Session.
              </span>
            </span>
          </label>
          <label className="soccer-option soccer-option-tall">
            <input
              type="radio"
              name="method"
              value="file"
              checked={method === "file"}
              onChange={() => {
                setMethod("file");
                setOutcome(null);
              }}
              aria-describedby="file-about"
            />
            <span>
              <span className="soccer-option-name">Download .ics</span>
              <span id="file-about" className="soccer-option-about">
                Save a one-time copy of your selected games. Imported events won’t receive later schedule changes.
                Importing again may create duplicates.
              </span>
            </span>
          </label>
        </div>
        <p className="soccer-hint">
          A Session is one run of league play. The games you check above change only the download.
        </p>
      </fieldset>

      {(!calendar || !method) && (
        <p className="soccer-next">
          {!calendar && !method
            ? "Choose your calendar, then a Session link or a download, to see the steps."
            : !calendar
              ? "Choose your calendar to see the steps."
              : "Choose a Session link or a download to see the steps."}
        </p>
      )}

      {calendar && method && (
        <div className="soccer-handoff">
          <div className="soccer-scope">
            <p className="soccer-scope-what">
              {method === "link" ? `${plural(teams.length, "Team")} · this Session` : `${plural(count, "game")} · one-time copy`}
            </p>
            <p className="soccer-scope-who">{names}</p>
          </div>

          {REQUIRES[calendar] && (
            <div className="soccer-requires">
              <p className="soccer-requires-title">{REQUIRES[calendar].title}</p>
              <p>{REQUIRES[calendar].body}</p>
            </div>
          )}

          <ol className="soccer-steps">
            {(method === "link" ? LINK_STEPS : FILE_STEPS)[calendar].map((step, i) => (
              <li key={i}>{step}</li>
            ))}
          </ol>
          <p className="soccer-hint">{(method === "link" ? LINK_NOTES : FILE_NOTES)[calendar]}</p>
          {calendar === "google" && (
            <p className="soccer-hint">
              On a computer now? <a href="https://calendar.google.com/">Open Google Calendar</a>, then follow the steps.
            </p>
          )}

          {blocked ? (
            <p className="soccer-blocked" role="status">
              {blocked}
            </p>
          ) : blocked === null ? (
            <p className="soccer-status" role="status">
              <Dots />
              Waiting for games for {pending?.name}…
            </p>
          ) : method === "link" ? (
            <div className="soccer-act">
              <button type="button" className="tactile soccer-action" onClick={copy} disabled={working !== null}>
                <Icon name="link" />
                <span>{working === "link" ? "Getting your link…" : "Copy Session link"}</span>
              </button>
              <p className="soccer-hint">Anyone with this link can use it.</p>
              {stale && (
                <p className="soccer-hint">
                  Your Teams changed since your last Session link. That link still follows the Teams it was made for;
                  copy a new one for these.
                </p>
              )}
              {result}

              {current && (
                <div className="soccer-linkbox">
                  <label className="soccer-label" htmlFor="session-link">
                    Session link for {names}
                  </label>
                  <input
                    ref={linkField}
                    id="session-link"
                    className="soccer-input soccer-linkfield"
                    type="text"
                    readOnly
                    value={current}
                    onFocus={(e) => e.currentTarget.select()}
                  />
                </div>
              )}

              {calendar === "apple" && (
                <div className="soccer-also">
                  <button type="button" className="quiet-button soccer-also-action" onClick={openInApple} disabled={working !== null}>
                    <span>{working === "open" ? "Getting your link…" : "Open in Apple Calendar"}</span>
                    <Icon name="arrow" />
                  </button>
                  <p className="soccer-hint">
                    On an iPhone, iPad or Mac you can try this instead of pasting. Apple Calendar still asks you to
                    confirm the subscription before it’s added.
                  </p>
                </div>
              )}
            </div>
          ) : (
            <div className="soccer-act">
              <button
                type="button"
                className="tactile soccer-action"
                onClick={download}
                disabled={working !== null}
                aria-disabled={count === 0 || undefined}
                aria-describedby={count === 0 ? "no-games" : undefined}
              >
                <Icon name="download" />
                <span>{working === "file" ? "Preparing your file…" : "Download .ics"}</span>
              </button>
              {count === 0 && (
                <p id="no-games" className="soccer-blocked">
                  Choose at least one game.
                </p>
              )}
              {result}
            </div>
          )}

        </div>
      )}
    </section>
  );
}
