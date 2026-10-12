import { useCallback, useEffect, useRef, useState } from "react";
import { ToolGlyph } from "../glyphs";
import { Robot } from "../Robot";
import { getStatus, startServer, stopServer, type StatusAnswer } from "./api";
import { Join } from "./Join";
import {
  announcement,
  canStart,
  canStop,
  describe,
  pollDelay,
  refusalMessage,
  SLOW_START_MS,
  unconfirmed,
  type Description,
  type ServerStatus,
} from "./logic";

// The Minecraft Launcher (docs/design/minecraft-brief.md, "Status beside
// setup"): can I join, do I need to start the server, how do I connect on my
// device. The page arrives as a checking state with nothing private in it;
// the server's name, status and address appear only once the site has
// confirmed the Member's minecraft grant.

type View =
  | { kind: "checking" }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed"; retrying: boolean }
  | { kind: "not-configured" }
  | { kind: "server"; status: ServerStatus };

// What the page is doing about Start server or Stop server, apart from what
// the server says. "confirming": Stop server was pressed and the page is
// asking whether they mean it; nothing has been sent. "asking": the request
// is on its way. "refused": the backend would not stop the server, for the
// reason given, and nothing changed. "unsure": the request may or may not
// have reached the server, so the page offers a status check, never the
// same request again.
type Action = "start" | "stop";
type Request =
  | { kind: "idle" }
  | { kind: "confirming" }
  | { kind: "asking"; of: Action }
  | { kind: "refused"; message: string }
  | { kind: "unsure"; of: Action };

const IDLE: Request = { kind: "idle" };

export function Launcher() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [request, setRequest] = useState<Request>(IDLE);
  const [refreshing, setRefreshing] = useState(false);
  const [said, setSaid] = useState("");
  // When the page first saw the server starting, for the slow-start note.
  const [startingSince, setStartingSince] = useState<number | null>(null);
  const [slow, setSlow] = useState(false);

  const current = useRef<ServerStatus | null>(null);
  const latest = useRef(0);
  // Stop server at rest, or the confirmation's Stop: never both at once.
  const stopButton = useRef<HTMLButtonElement>(null);
  const question = useRef<HTMLParagraphElement>(null);
  const statusLine = useRef<HTMLParagraphElement>(null);
  const serverColumn = useRef<HTMLElement>(null);
  // What had focus in the server column as an answer was applied.
  const hadFocus = useRef<Element | null>(null);
  // Where focus goes when the Stop control changes shape, if anywhere.
  const focusNext = useRef<"question" | "stop" | null>(null);

  const say = useCallback((text: string) => {
    if (!text) return;
    setSaid("");
    requestAnimationFrame(() => setSaid(text));
  }, []);

  // apply puts an answer on screen. Only the newest call may: an older answer
  // arriving late must not undo a newer one.
  const apply = useCallback(
    (answer: StatusAnswer, id: number) => {
      if (id !== latest.current) return;
      // If a control in the server column had focus and is about to go (Start
      // server, Stop, or anything in the confirmation), focus moves to the
      // status it changed rather than falling to the page.
      const active = document.activeElement;
      hadFocus.current = active && serverColumn.current?.contains(active) ? active : null;
      if (answer.kind !== "status" && answer.kind !== "unavailable") {
        // Signed out, no access, or no server: nothing private stays on screen.
        current.current = null;
        setView(answer.kind === "access-failed" ? { kind: "access-failed", retrying: false } : { kind: answer.kind });
        return;
      }
      const before = current.current?.state;
      const status = answer.kind === "status" ? answer.status : unconfirmed(current.current);
      current.current = status;
      setView({ kind: "server", status });
      // A refusal was true when the stop was refused; a later answer is newer
      // than it, so it goes and Stop server stands alone again.
      setRequest((r) => (r.kind === "refused" ? IDLE : r));
      say(announcement(before, status));
    },
    [say],
  );

  const check = useCallback(async () => {
    const id = ++latest.current;
    const answer = await getStatus();
    apply(answer, id);
    return answer;
  }, [apply]);

  // First load.
  useEffect(() => {
    void check();
  }, [check]);

  const status = view.kind === "server" ? view.status : null;
  const state = status?.state;

  // Keep asking while the page is visible: often while the server is on its
  // way somewhere, rarely otherwise. A hidden page stops asking and checks
  // again when it is shown. Polling never moves focus or the device choice.
  // It pauses while Start or Stop is on its way: a check that crossed the
  // request could answer with the state from before it, and as the newer
  // call it would replace the request's own answer.
  const busy = request.kind === "asking";
  useEffect(() => {
    if (!state || busy) return;
    let timer = 0;
    const schedule = () => {
      window.clearTimeout(timer);
      if (document.visibilityState === "visible") timer = window.setTimeout(() => void check(), pollDelay(state));
    };
    const onVisible = () => {
      if (document.visibilityState === "visible") void check();
      else window.clearTimeout(timer);
    };
    schedule();
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [state, status, busy, check]);

  // The slow-start note: measured from when this page first saw "starting".
  useEffect(() => {
    if (state !== "starting") {
      setStartingSince(null);
      setSlow(false);
      return;
    }
    setStartingSince((since) => since ?? Date.now());
  }, [state]);
  useEffect(() => {
    if (startingSince === null) return;
    const t = window.setTimeout(() => setSlow(true), Math.max(0, startingSince + SLOW_START_MS - Date.now()));
    return () => window.clearTimeout(t);
  }, [startingSince]);

  // A confirmation or a refusal belongs to a ready server. When the server
  // moves on (another Member stopped it, say), neither lingers.
  useEffect(() => {
    if (state && canStop(state)) return;
    setRequest((r) => (r.kind === "confirming" || r.kind === "refused" ? IDLE : r));
  }, [state]);

  useEffect(() => {
    if (hadFocus.current && !hadFocus.current.isConnected) statusLine.current?.focus();
    hadFocus.current = null;
  });

  // Opening the confirmation puts focus on its question; leaving it without
  // a stop puts focus back on Stop server.
  useEffect(() => {
    const target =
      focusNext.current === "question" ? question.current : focusNext.current === "stop" ? stopButton.current : null;
    focusNext.current = null;
    target?.focus();
  });

  const start = useCallback(async () => {
    if (request.kind === "asking") return;
    setRequest({ kind: "asking", of: "start" });
    say("Asking the server to start.");
    const id = ++latest.current;
    const answer = await startServer();
    // The start may have reached the server even though no answer came back.
    setRequest(answer.kind === "unavailable" ? { kind: "unsure", of: "start" } : IDLE);
    apply(answer, id);
  }, [request, apply, say]);

  // Stop server asks first; nothing is sent until Stop is pressed.
  const confirmStop = useCallback(() => {
    focusNext.current = "question";
    setRequest({ kind: "confirming" });
  }, []);

  const keepPlaying = useCallback(() => {
    focusNext.current = "stop";
    setRequest(IDLE);
  }, []);

  const stop = useCallback(async () => {
    if (request.kind === "asking") return;
    setRequest({ kind: "asking", of: "stop" });
    say("Asking the server to stop.");
    const id = ++latest.current;
    const answer = await stopServer();
    if (answer.kind === "refused") {
      // Nothing changed. The reason takes the question's place, and Stop
      // server is back for when it no longer applies.
      const message = refusalMessage(answer);
      focusNext.current = "stop";
      setRequest({ kind: "refused", message });
      say(message);
      return;
    }
    // The stop may have reached the server even though no answer came back.
    setRequest(answer.kind === "unavailable" ? { kind: "unsure", of: "stop" } : IDLE);
    apply(answer, id);
  }, [request, apply, say]);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    const answer = await check();
    setRefreshing(false);
    if (answer.kind === "status") setRequest(IDLE);
  }, [check]);

  const retryAccess = useCallback(() => {
    setView({ kind: "access-failed", retrying: true });
    void check();
  }, [check]);

  useEffect(() => {
    document.title = "Minecraft Launcher · The Lobby";
  }, []);

  const member = view.kind !== "checking" && view.kind !== "signed-out";

  return (
    <div className="page minecraft">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <nav className="mc-header-nav" aria-label="Account and Tools">
            {view.kind !== "signed-out" && (
              <a className="page-back" href="/">
                Back to Tools
              </a>
            )}
            {member && (
              <form method="post" action="/signout">
                <button className="quiet-button" type="submit">
                  Sign out
                </button>
              </form>
            )}
          </nav>
        </div>
      </header>

      <main className="mc-main" aria-busy={view.kind === "checking" ? true : undefined}>
        <h1 className="mc-title">Minecraft Launcher</h1>
        <p className="mc-lead">Start the server, then join from Minecraft.</p>

        <noscript>
          <p className="mc-problem-inline">The Minecraft Launcher needs JavaScript to check the server and start it.</p>
        </noscript>

        {view.kind === "checking" && (
          <p className="lobby-status mc-checking" role="status">
            <Dots />
            Checking your access…
          </p>
        )}

        {view.kind === "signed-out" && (
          <Problem title="Sign in to use the Minecraft Launcher.">
            <p>It’s for Members Craig has invited. You’ll come back here after signing in.</p>
            <a className="tactile tactile-small" href="/signin?next=/minecraft">
              Sign in
            </a>
          </Problem>
        )}

        {view.kind === "denied" && (
          <Problem title="You don’t have access to this Tool.">
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {view.kind === "access-failed" && (
          <Problem title="We couldn’t check your access.">
            <p>This doesn’t mean your access has changed. Please try again in a moment.</p>
            <button className="tactile tactile-small" type="button" onClick={retryAccess} disabled={view.retrying}>
              {view.retrying ? "Checking…" : "Retry"}
            </button>
          </Problem>
        )}

        {view.kind === "not-configured" && (
          <Problem title="The Minecraft server isn’t available here yet.">
            <p>No server is set up for the Launcher, so there’s nothing to start or join from this page.</p>
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {status && (
          <div className="mc-workspace">
            <Server
              status={status}
              described={describe(status)}
              request={request}
              refreshing={refreshing}
              slow={slow}
              onStart={start}
              onConfirmStop={confirmStop}
              onStop={stop}
              onKeepPlaying={keepPlaying}
              onRefresh={refresh}
              stopRef={stopButton}
              questionRef={question}
              statusRef={statusLine}
              columnRef={serverColumn}
            />
            <Join connection={status.connection} state={status.state} serverName={status.name} />
          </div>
        )}

        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      <footer className="page-footer">
        <div className="page-footer-inner">
          <Robot className="lobby-robot" />
          <a href="/privacy">Privacy</a>
        </div>
      </footer>
    </div>
  );
}

// The server column: identity, state and the one action that fits it. It
// keeps its place as the server moves from stopped to starting to ready.
function Server({
  status,
  described,
  request,
  refreshing,
  slow,
  onStart,
  onConfirmStop,
  onStop,
  onKeepPlaying,
  onRefresh,
  stopRef,
  questionRef,
  statusRef,
  columnRef,
}: {
  status: ServerStatus;
  described: Description;
  request: Request;
  refreshing: boolean;
  slow: boolean;
  onStart: () => void;
  onConfirmStop: () => void;
  onStop: () => void;
  onKeepPlaying: () => void;
  onRefresh: () => void;
  stopRef: React.RefObject<HTMLButtonElement | null>;
  questionRef: React.RefObject<HTMLParagraphElement | null>;
  statusRef: React.RefObject<HTMLParagraphElement | null>;
  columnRef: React.RefObject<HTMLElement | null>;
}) {
  const asking = request.kind === "asking" && request.of === "start";
  // The request the page could not confirm, once the status is shown as
  // unconfirmed.
  const unsure = request.kind === "unsure" && status.state === "unknown" ? request.of : null;
  const starting = status.state === "starting" || asking;
  // Start server stays in place, unavailable, while a start is on its way,
  // so a second tap does nothing and focus is not lost.
  const showStart = !unsure && (canStart(status.state) || starting);
  // Stop server is offered only while the server is ready. Its confirmation
  // stays in place, unavailable, while the stop is on its way, likewise.
  const stopping = request.kind === "asking" && request.of === "stop";
  const showStop = !unsure && canStop(status.state);
  const confirming = showStop && (request.kind === "confirming" || stopping);
  const message = unsure
    ? `We couldn’t tell whether the server got the ${unsure} request. Check its status before trying again.`
    : asking && status.state !== "starting"
      ? "Asking the server to start…"
      : described.message;

  return (
    <section ref={columnRef} className="mc-server" aria-labelledby="server-title">
      <div className="mc-server-name">
        <ToolGlyph glyph="cube" />
        <h2 id="server-title">{status.name}</h2>
      </div>
      <p ref={statusRef} className="mc-state" tabIndex={-1}>
        <span className="mc-mark" data-mark={described.mark} aria-hidden="true" />
        {described.label}
      </p>
      <p className="mc-message">{message}</p>

      {showStart && (
        <button
          className="tactile mc-start"
          type="button"
          aria-disabled={starting ? true : undefined}
          onClick={starting ? undefined : onStart}
        >
          {starting ? "Starting…" : status.state === "failed" ? "Try starting again" : "Start server"}
        </button>
      )}

      {showStop && !confirming && request.kind === "refused" && <p className="mc-refused">{request.message}</p>}

      {showStop && !confirming && (
        <button ref={stopRef} className="quiet-button mc-stop" type="button" onClick={onConfirmStop}>
          Stop server
        </button>
      )}

      {confirming && (
        <div className="mc-confirm">
          <p ref={questionRef} className="mc-confirm-question" tabIndex={-1}>
            Stop the server? Everyone’s progress is saved first.
          </p>
          <div className="mc-confirm-actions">
            <button
              ref={stopRef}
              className="tactile tactile-small"
              type="button"
              aria-disabled={stopping ? true : undefined}
              onClick={stopping ? undefined : onStop}
            >
              {stopping ? "Stopping…" : "Stop"}
            </button>
            <button className="quiet-button mc-keep" type="button" onClick={onKeepPlaying} disabled={stopping}>
              Keep playing
            </button>
          </div>
        </div>
      )}

      {(status.state === "unknown" || unsure) && (
        <button className="quiet-button mc-refresh" type="button" onClick={onRefresh} disabled={refreshing}>
          {refreshing ? "Checking server status…" : "Check status again"}
        </button>
      )}

      {status.state === "starting" && slow && (
        <p className="mc-note">
          This is taking longer than usual. It’s still starting, and there’s nothing to press. If it isn’t ready in a
          few more minutes, let Craig know.
        </p>
      )}

      {(status.state === "stopped" || starting) && (
        <p className="mc-note">You can get your connection details ready while it starts.</p>
      )}
    </section>
  );
}

function Problem({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="lobby-problem" role="alert">
      <p className="lobby-problem-title">{title}</p>
      {children}
    </div>
  );
}

function Dots() {
  return (
    <span className="lobby-status-dots" aria-hidden="true">
      <span />
      <span />
      <span />
    </span>
  );
}
