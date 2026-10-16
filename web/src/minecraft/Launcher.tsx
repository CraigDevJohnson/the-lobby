import { useCallback, useEffect, useRef, useState, useSyncExternalStore, type KeyboardEvent } from "react";
import { Robot } from "../Robot";
import { SignOut } from "../SignOut";
import { getStatus, startServer, stopServer, type StatusAnswer } from "./api";
import { ServerBlock, StatusGlyph, Terrace } from "./art";
import { Join } from "./Join";
import {
  announcement,
  canStart,
  canStop,
  describe,
  hasDetails,
  lead,
  pageTitle,
  pollDelay,
  RECHECKED,
  refusalMessage,
  SLOW_START,
  SLOW_START_MS,
  STILL_NO_ACCESS,
  unconfirmed,
  type Description,
  type ServerStatus,
} from "./logic";

// The Minecraft Launcher (docs/design/minecraft-brief.md, "Status beside
// setup", in the block motif of DESIGN.md): can I join, do I need to start
// the server, how do I connect on my device. The page arrives as a checking
// state with nothing private in it; the server's name, status and address
// appear only once the site has confirmed the Member's minecraft grant.

type View =
  | { kind: "checking" }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed"; retrying: boolean; retried?: boolean }
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

// Stop server sits under the server card on wide screens and after the
// joining help on phones, so Copy address comes first there. It is placed
// in the markup by width, so keyboard order matches what is seen. The
// prerendered page has no server on it, so it can assume wide.
const WIDE = "(min-width: 45.0625rem)";
const isWide = () => window.matchMedia(WIDE).matches;

// Focus moves the page makes don't scroll it, unless the control the Member
// was using went while they looked at it, or moved with the layout, and its
// replacement is out of view.
const focusHere = (el: HTMLElement | null | undefined, scroll = false) => el?.focus({ preventScroll: !scroll });

const inView = (el: Element) => {
  const r = el.getBoundingClientRect();
  return r.bottom > 0 && r.top < window.innerHeight;
};

export function Launcher() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [request, setRequest] = useState<Request>(IDLE);
  const [refreshing, setRefreshing] = useState(false);
  // Check status again came back with the same unconfirmed answer.
  const [rechecked, setRechecked] = useState(false);
  const [said, setSaid] = useState("");
  // When the page first saw the server starting, for the slow-start note.
  const [startingSince, setStartingSince] = useState<number | null>(null);
  const [slow, setSlow] = useState(false);
  // The host waves once each time the server becomes ready while the page
  // is open, never on arrival at a server that was already ready.
  const [waves, setWaves] = useState(0);

  const current = useRef<ServerStatus | null>(null);
  const latest = useRef(0);
  // Stop server at rest, or the confirmation's Stop: never both at once.
  const stopButton = useRef<HTMLButtonElement>(null);
  const keepButton = useRef<HTMLButtonElement>(null);
  const question = useRef<HTMLParagraphElement>(null);
  const statusLine = useRef<HTMLParagraphElement>(null);
  const problemTitle = useRef<HTMLParagraphElement>(null);
  // What had focus as an answer was applied, and whether it was in view.
  const hadFocus = useRef<{ el: Element; seen: boolean } | null>(null);
  // Where focus goes when the Stop control changes shape, if anywhere.
  const focusNext = useRef<"question" | "stop" | null>(null);
  const stopArea = useRef<HTMLDivElement>(null);
  // Where focus was in the Stop controls when they moved across the
  // breakpoint, so it can follow them.
  const carried = useRef<"question" | "stop" | "keep" | null>(null);

  const watchWide = useCallback((on: () => void) => {
    const m = window.matchMedia(WIDE);
    const change = () => {
      const a = document.activeElement;
      carried.current =
        !a || !stopArea.current?.contains(a)
          ? null
          : a === stopButton.current
            ? "stop"
            : a === keepButton.current
              ? "keep"
              : "question";
      on();
    };
    m.addEventListener("change", change);
    return () => m.removeEventListener("change", change);
  }, []);
  const wide = useSyncExternalStore(watchWide, isWide, () => true);

  useEffect(() => {
    const to = carried.current;
    carried.current = null;
    if (to) focusHere({ question, stop: stopButton, keep: keepButton }[to].current, true);
  }, [wide]);

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
      // If what has focus is about to go (Start server, Stop, anything in the
      // confirmation, or the whole server panel when access is gone), focus
      // moves to the status it changed, or to the problem that replaced it,
      // rather than falling to the page.
      const active = document.activeElement;
      hadFocus.current = active && active !== document.body ? { el: active, seen: inView(active) } : null;
      setRechecked(false);
      if (answer.kind !== "status" && answer.kind !== "unavailable") {
        // Signed out, no access, or no server: nothing private stays on screen.
        current.current = null;
        setView(answer.kind === "access-failed" ? { kind: "access-failed", retrying: false } : { kind: answer.kind });
        return;
      }
      const before = current.current?.state;
      const status = answer.kind === "status" ? answer.status : unconfirmed(current.current);
      current.current = status;
      if (before && before !== "ready" && status.state === "ready") setWaves((n) => n + 1);
      setView({ kind: "server", status });
      // A refusal was true when the stop was refused; a later answer is newer
      // than it, so it goes and Stop server stands alone again.
      setRequest((r) => (r.kind === "refused" ? IDLE : r));
      say(announcement(before, status));
    },
    [say],
  );

  // check also says whether its answer is the one on screen, as a newer call
  // may have overtaken it.
  const check = useCallback(async () => {
    const id = ++latest.current;
    const answer = await getStatus();
    apply(answer, id);
    return { answer, shown: id === latest.current };
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
  // It pauses while Start, Stop or Check status again is on its way: a check
  // that crossed the request would be the newer call and replace the
  // request's own answer, perhaps with the state from before it.
  const busy = request.kind === "asking" || refreshing;
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

  // The slow-start note: measured from when this page first saw "starting",
  // and read out once when it appears.
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
  useEffect(() => {
    if (slow) say(SLOW_START);
  }, [slow, say]);

  // A confirmation or a refusal belongs to a ready server. When the server
  // moves on (another Member stopped it, say), neither lingers.
  useEffect(() => {
    if (state && canStop(state)) return;
    setRequest((r) => (r.kind === "confirming" || r.kind === "refused" ? IDLE : r));
  }, [state]);

  useEffect(() => {
    const had = hadFocus.current;
    hadFocus.current = null;
    if (had && !had.el.isConnected) focusHere(statusLine.current ?? problemTitle.current, had.seen);
  });

  // Opening the confirmation puts focus on its question; leaving it without
  // a stop puts focus back on Stop server.
  useEffect(() => {
    const target =
      focusNext.current === "question" ? question.current : focusNext.current === "stop" ? stopButton.current : null;
    focusNext.current = null;
    focusHere(target);
  });

  // The tab's title leads with the state, for a Member waiting in another tab.
  useEffect(() => {
    document.title = pageTitle(status);
  }, [status]);

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

  // Check status again stays where it is while it asks, so focus stays on it,
  // and an answer that changed nothing is still said and shown.
  const refresh = useCallback(async () => {
    if (refreshing) return;
    setRefreshing(true);
    const before = current.current?.state;
    const { answer, shown } = await check();
    setRefreshing(false);
    // An answer that was overtaken says nothing about what is on screen.
    if (!shown) return;
    if (answer.kind === "status") setRequest(IDLE);
    if ((answer.kind === "status" || answer.kind === "unavailable") && current.current?.state === before) {
      setRechecked(true);
      say(RECHECKED);
    }
  }, [refreshing, check, say]);

  // Retry likewise keeps focus, and says and shows it when access still
  // can't be checked.
  const retryAccess = useCallback(async () => {
    if (view.kind === "access-failed" && view.retrying) return;
    setView({ kind: "access-failed", retrying: true });
    const { answer, shown } = await check();
    if (shown && answer.kind === "access-failed") {
      setView((v) => (v.kind === "access-failed" ? { ...v, retried: true } : v));
      say(STILL_NO_ACCESS);
    }
  }, [view, check, say]);

  const member = view.kind !== "checking" && view.kind !== "signed-out";
  // The lead says what to do now. A Member who can't use the Launcher yet
  // isn't told to start anything; the problem says what's wrong.
  const showLead = view.kind === "checking" || view.kind === "signed-out" || view.kind === "server";

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
            {member && <SignOut />}
          </nav>
        </div>
      </header>

      <main className="mc-main" aria-busy={view.kind === "checking" ? true : undefined}>
        <h1 className="mc-title">Minecraft Launcher</h1>
        {showLead && <p className="mc-lead">{lead(status)}</p>}

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
          <Problem
            titleRef={problemTitle}
            title="Sign in to use the Minecraft Launcher."
            detail="It’s for Members Craig has invited. You’ll come back here after signing in."
          >
            <a className="tactile tactile-small" href="/signin?next=/minecraft">
              Sign in
            </a>
          </Problem>
        )}

        {view.kind === "denied" && (
          <Problem titleRef={problemTitle} title="You don’t have access to this Tool.">
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {view.kind === "access-failed" && (
          <Problem
            titleRef={problemTitle}
            title="We couldn’t check your access."
            detail="This doesn’t mean your access has changed. Please try again in a moment."
          >
            <button
              className="tactile tactile-small"
              type="button"
              aria-disabled={view.retrying ? true : undefined}
              onClick={retryAccess}
            >
              {view.retrying ? "Checking…" : "Retry"}
            </button>
            {view.retried && !view.retrying && <p className="mc-rechecked">{STILL_NO_ACCESS}</p>}
          </Problem>
        )}

        {view.kind === "not-configured" && (
          <Problem
            titleRef={problemTitle}
            title="The Minecraft server isn’t available here yet."
            detail="No server is set up for the Launcher, so there’s nothing to start or join from this page."
          >
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
              rechecked={rechecked}
              slow={slow}
              onStart={start}
              onRefresh={refresh}
              statusRef={statusLine}
            />
            {wide && (
              <StopControls
                status={status}
                request={request}
                onConfirmStop={confirmStop}
                onStop={stop}
                onKeepPlaying={keepPlaying}
                stopRef={stopButton}
                keepRef={keepButton}
                questionRef={question}
                areaRef={stopArea}
              />
            )}
            <Join connection={status.connection} state={status.state} serverName={status.name} />
            {!wide && (
              <StopControls
                status={status}
                request={request}
                onConfirmStop={confirmStop}
                onStop={stop}
                onKeepPlaying={keepPlaying}
                stopRef={stopButton}
                keepRef={keepButton}
                questionRef={question}
                areaRef={stopArea}
              />
            )}
          </div>
        )}

        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      {/* The host stands on the terrace's landing, under the server card. */}
      <footer className="mc-footer">
        <Terrace>
          <Robot key={waves} className="mc-host" wave={waves > 0} />
        </Terrace>
        <div className="mc-footer-bar">
          <a href="/privacy">Privacy</a>
        </div>
      </footer>
    </div>
  );
}

// The server card: the server block, its name, the state in large letters
// and the one action that fits it. It keeps its place as the server moves
// from stopped to starting to ready.
function Server({
  status,
  described,
  request,
  refreshing,
  rechecked,
  slow,
  onStart,
  onRefresh,
  statusRef,
}: {
  status: ServerStatus;
  described: Description;
  request: Request;
  refreshing: boolean;
  rechecked: boolean;
  slow: boolean;
  onStart: () => void;
  onRefresh: () => void;
  statusRef: React.RefObject<HTMLParagraphElement | null>;
}) {
  const asking = request.kind === "asking" && request.of === "start";
  // The request the page could not confirm, once the status is shown as
  // unconfirmed.
  const unsure = request.kind === "unsure" && status.state === "unknown" ? request.of : null;
  const starting = status.state === "starting" || asking;
  // Start server stays in place, unavailable, while a start is on its way,
  // so a second tap does nothing and focus is not lost.
  const showStart = !unsure && (canStart(status.state) || starting);
  // While the start is on its way the state already reads as starting, so
  // the words never contradict the button beside them.
  const shown: Pick<Description, "label" | "mark"> =
    asking && status.state !== "starting" ? { label: "Starting", mark: "busy" } : described;
  const message = unsure
    ? `We couldn’t tell whether the server got the ${unsure} request. Check its status before trying again.`
    : asking && status.state !== "starting"
      ? "Asking the server to start…"
      : described.message;
  // At Ready the lead under the title already says what to do, unless there
  // is nothing to join with yet.
  const showMessage = unsure !== null || status.state !== "ready" || !hasDetails(status.connection);

  return (
    <section className="mc-server" aria-labelledby="server-title">
      <ServerBlock lit={status.state === "ready"} />
      <h2 id="server-title" className="mc-server-name">
        {status.name}
      </h2>
      <p ref={statusRef} className="mc-state" tabIndex={-1}>
        <StatusGlyph mark={shown.mark} />
        {shown.label}
      </p>
      {showMessage && <p className="mc-message">{message}</p>}

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

      {(status.state === "unknown" || unsure) && (
        <>
          <button
            className="quiet-button mc-refresh"
            type="button"
            aria-disabled={refreshing ? true : undefined}
            onClick={onRefresh}
          >
            {refreshing ? "Checking server status…" : "Check status again"}
          </button>
          {rechecked && !refreshing && <p className="mc-rechecked">{RECHECKED}</p>}
        </>
      )}

      {status.state === "starting" && slow && <p className="mc-note">{SLOW_START}</p>}

      {(status.state === "stopped" || starting) && (
        <p className="mc-note">You can get your connection details ready while it starts.</p>
      )}
    </section>
  );
}

// Stop server, offered only while the server is ready. It asks before it
// acts: the question and its two answers take the button's place, and a
// refusal sits above the button as a notice. The confirmation's Stop is
// quiet, so the only custard control in view stays Copy address.
function StopControls({
  status,
  request,
  onConfirmStop,
  onStop,
  onKeepPlaying,
  stopRef,
  keepRef,
  questionRef,
  areaRef,
}: {
  status: ServerStatus;
  request: Request;
  onConfirmStop: () => void;
  onStop: () => void;
  onKeepPlaying: () => void;
  stopRef: React.RefObject<HTMLButtonElement | null>;
  keepRef: React.RefObject<HTMLButtonElement | null>;
  questionRef: React.RefObject<HTMLParagraphElement | null>;
  areaRef: React.RefObject<HTMLDivElement | null>;
}) {
  const unsure = request.kind === "unsure" && status.state === "unknown";
  const stopping = request.kind === "asking" && request.of === "stop";
  const showStop = !unsure && canStop(status.state);
  const confirming = showStop && (request.kind === "confirming" || stopping);
  if (!showStop) return null;

  // Escape answers the question the same way Keep playing does.
  const onKey = (e: KeyboardEvent) => {
    if (e.key !== "Escape" || stopping) return;
    e.preventDefault();
    onKeepPlaying();
  };

  return (
    <div ref={areaRef} className="mc-stop-area">
      {!confirming && request.kind === "refused" && <p className="mc-refused">{request.message}</p>}
      {!confirming && (
        <button ref={stopRef} className="quiet-button mc-stop" type="button" onClick={onConfirmStop}>
          Stop server
        </button>
      )}
      {confirming && (
        <div className="mc-confirm" onKeyDown={onKey}>
          <p ref={questionRef} className="mc-confirm-question" tabIndex={-1}>
            Stop the server? Everyone’s progress is saved first.
          </p>
          <div className="mc-confirm-actions">
            <button
              ref={stopRef}
              className="quiet-button mc-confirm-stop"
              type="button"
              aria-disabled={stopping ? true : undefined}
              onClick={stopping ? undefined : onStop}
            >
              {stopping ? "Stopping…" : "Stop"}
            </button>
            <button
              ref={keepRef}
              className="quiet-button mc-keep"
              type="button"
              onClick={onKeepPlaying}
              disabled={stopping}
            >
              Keep playing
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// A problem in place of the server. Only what went wrong is the alert, so a
// control below it changing its label is never read out as a new alert.
function Problem({
  title,
  detail,
  titleRef,
  children,
}: {
  title: string;
  detail?: string;
  titleRef: React.RefObject<HTMLParagraphElement | null>;
  children: React.ReactNode;
}) {
  return (
    <div className="lobby-problem">
      <div className="mc-problem-says" role="alert">
        <p ref={titleRef} className="lobby-problem-title" tabIndex={-1}>
          {title}
        </p>
        {detail && <p>{detail}</p>}
      </div>
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
