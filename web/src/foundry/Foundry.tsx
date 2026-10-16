import { useCallback, useEffect, useRef, useState } from "react";
import { ToolFooter } from "../ToolChrome";
import { getStatus } from "./api";
import { Tabletop } from "./art";
import {
  address,
  announcement,
  availability,
  caveat,
  describe,
  EMPTY,
  forget,
  key,
  noLink,
  openIsPrimary,
  pageTitle,
  pollDelay,
  rechecked,
  segments,
  STALE_MS,
  STILL_NO_ACCESS,
  take,
  when,
  type Answer,
  type Availability,
  type Picture,
} from "./logic";
import { StatusMark } from "./marks";

// Foundry (docs/design/foundry-brief.md): invited players check whether
// Foundry is up, then open it and finish joining inside Foundry. Check
// availability sits on the left and Join in Foundry on the right, stacked in
// that order on phones. The page arrives with both sections in place and the
// status card checking, with nothing private in it; the status and the join
// link appear only once the site has confirmed the Member's foundry grant.
// It never changes the server.

type View =
  | { kind: "checking" }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed"; retrying: boolean; retried?: boolean }
  | { kind: "not-configured" }
  | { kind: "foundry" };

function problem(a: Answer): View {
  switch (a.kind) {
    case "signed-out":
    case "denied":
    case "not-configured":
      return { kind: a.kind };
    default:
      return { kind: "access-failed", retrying: false };
  }
}

// Focus moves the page makes don't scroll it, unless the control the Member
// was using went while they looked at it and its replacement is out of view.
const focusHere = (el: HTMLElement | null | undefined, scroll = false) => el?.focus({ preventScroll: !scroll });

const inView = (el: Element) => {
  const r = el.getBoundingClientRect();
  return r.bottom > 0 && r.top < window.innerHeight;
};

// How long Check again's "Checked just now" stays before it would be untrue.
const RECHECKED_MS = 10_000;

export function Foundry() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [picture, setPicture] = useState<Picture>(EMPTY);
  // This browser's clock as of the newest answer, or of the moment a
  // confirmed state went stale; nothing reads it before the first answer.
  const [now, setNow] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  // Check again came back with the status already on screen.
  const [outcome, setOutcome] = useState("");
  const [said, setSaid] = useState("");

  const current = useRef<Picture>(EMPTY);
  const shown = useRef<Availability | null>(null);
  const latest = useRef(0);
  const statusLine = useRef<HTMLParagraphElement>(null);
  const problemTitle = useRef<HTMLParagraphElement>(null);
  // What had focus as an answer was applied, and whether it was in view.
  const hadFocus = useRef<{ el: Element; seen: boolean } | null>(null);

  const say = useCallback((text: string) => {
    if (!text) return;
    setSaid("");
    requestAnimationFrame(() => setSaid(text));
  }, []);

  // apply puts an answer on screen. Only the newest call may: an older answer
  // arriving late must not undo a newer one.
  const apply = useCallback(
    (answer: Answer, id: number) => {
      if (id !== latest.current) return;
      // If what has focus is about to go (the whole workspace, when access is
      // gone), focus moves to the problem that replaced it, or to the status.
      const active = document.activeElement;
      hadFocus.current = active && active !== document.body ? { el: active, seen: inView(active) } : null;
      setOutcome("");
      const at = Date.now();
      const next = take(current.current, answer, at);
      if (!next) {
        // Signed out, no access, or no backend: nothing about Foundry stays.
        // An access check that failed only hides it until Retry's answer.
        current.current = forget(current.current, answer);
        shown.current = null;
        setPicture(current.current);
        setView(problem(answer));
        return;
      }
      const before = shown.current;
      const after = availability(next, at);
      current.current = next;
      shown.current = after;
      setPicture(next);
      setNow(at);
      setView({ kind: "foundry" });
      say(announcement(before, after));
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

  // age shows what the page can honestly say as of now, and says it if that
  // changed: a confirmed state nothing has confirmed again for a while reads
  // as possibly out of date.
  const age = useCallback(() => {
    const at = Date.now();
    const after = availability(current.current, at);
    say(announcement(shown.current, after));
    shown.current = after;
    setNow(at);
  }, [say]);

  // First load.
  useEffect(() => {
    void check();
  }, [check]);

  const avail = view.kind === "foundry" ? availability(picture, now) : null;
  const delay = avail ? pollDelay(avail) : 0;
  const hasLink = picture.link !== null;

  // Keep asking while the page is visible: often while Foundry is on its way
  // up or down, about once a minute otherwise. A hidden page stops asking and
  // checks again as soon as it is shown. Asking pauses while Check again is
  // on its way, so its own answer is the one that lands.
  useEffect(() => {
    if (view.kind !== "foundry" || refreshing) return;
    let timer = 0;
    if (document.visibilityState === "visible") timer = window.setTimeout(() => void check(), delay);
    const onVisible = () => {
      window.clearTimeout(timer);
      if (document.visibilityState !== "visible") return;
      // A state confirmed long ago reads as possibly out of date until the
      // new answer lands, and is said so, as when it ages on screen.
      age();
      void check();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [view.kind, picture, refreshing, delay, check, age]);

  // A confirmed state that nothing has confirmed again for a while is shown
  // as possibly out of date, and said so once.
  useEffect(() => {
    const last = picture.last;
    if (view.kind !== "foundry" || !last || picture.failedAt !== null) return;
    const due = last.receivedAt + STALE_MS - Date.now();
    if (due < 0) return;
    const t = window.setTimeout(age, due + 1);
    return () => window.clearTimeout(t);
  }, [view.kind, picture, age]);

  useEffect(() => {
    if (!outcome) return;
    const t = window.setTimeout(() => setOutcome(""), RECHECKED_MS);
    return () => window.clearTimeout(t);
  }, [outcome]);

  // Retry's "Checked just now" goes when it would no longer be true.
  const retried = view.kind === "access-failed" && view.retried === true && !view.retrying;
  useEffect(() => {
    if (!retried) return;
    const t = window.setTimeout(
      () => setView((v) => (v.kind === "access-failed" ? { ...v, retried: false } : v)),
      RECHECKED_MS,
    );
    return () => window.clearTimeout(t);
  }, [retried]);

  useEffect(() => {
    const had = hadFocus.current;
    hadFocus.current = null;
    if (had && !had.el.isConnected) focusHere(statusLine.current ?? problemTitle.current, had.seen);
  });

  // The tab's title leads with the state, for a player waiting in another tab.
  useEffect(() => {
    document.title = pageTitle(avail);
  }, [avail]);

  // Check again stays where it is while it asks, so focus stays on it, and
  // an answer that left the status as it was is still said and shown.
  const refresh = useCallback(async () => {
    if (refreshing) return;
    setRefreshing(true);
    // A line from the last press goes now, so this answer's line gets its
    // own full time on screen.
    setOutcome("");
    const before = key(shown.current);
    const { shown: landed } = await check();
    setRefreshing(false);
    // An answer that was overtaken says nothing about what is on screen.
    if (!landed || !shown.current || key(shown.current) !== before) return;
    const text = rechecked(shown.current);
    setOutcome(text);
    say(text);
  }, [refreshing, check, say]);

  // Retry likewise keeps focus, and says and shows it when access still
  // can't be checked.
  const retryAccess = useCallback(async () => {
    if (view.kind === "access-failed" && view.retrying) return;
    setView({ kind: "access-failed", retrying: true });
    const { answer, shown: landed } = await check();
    if (landed && answer.kind === "access-failed") {
      setView((v) => (v.kind === "access-failed" ? { ...v, retried: true } : v));
      say(STILL_NO_ACCESS);
    }
  }, [view, check, say]);

  // Sign out is there from the start, so the header doesn't shift when the
  // first answer lands; only a signed-out answer takes it away.
  const member = view.kind !== "signed-out";
  // The lead says what the page is for. A Member who can't use Foundry yet
  // isn't told to open anything; the problem says what's wrong.
  const showLead = view.kind === "checking" || view.kind === "foundry";

  return (
    <div className="page foundry">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <nav className="fd-header-nav" aria-label="Account and Tools">
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

      <main className="fd-main" aria-busy={view.kind === "checking" ? true : undefined}>
        <h1 className="fd-title">Foundry</h1>
        {showLead && <p className="fd-lead">See whether Foundry is up, then open it to join your game.</p>}

        <noscript>
          <p className="fd-problem-inline">Foundry’s page needs JavaScript to check whether Foundry is up.</p>
        </noscript>

        {view.kind === "signed-out" && (
          <Problem
            titleRef={problemTitle}
            title="Sign in to use Foundry."
            detail="It’s for players Craig has invited. You’ll come back here after signing in."
          >
            <a className="tactile tactile-small" href="/signin?next=/foundry">
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
            {view.retried && !view.retrying && <p className="fd-rechecked">{STILL_NO_ACCESS}</p>}
          </Problem>
        )}

        {view.kind === "not-configured" && (
          <Problem
            titleRef={problemTitle}
            title="Foundry isn’t available here yet."
            detail="No Foundry server is set up for this site, so there’s nothing to check or open from this page."
          >
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {(view.kind === "checking" || avail) && (
          <div className="fd-workspace">
            <CheckAvailability
              avail={avail}
              hasLink={hasLink}
              now={now}
              refreshing={refreshing}
              outcome={outcome}
              onRefresh={refresh}
              statusRef={statusLine}
            />
            <Join avail={avail} link={avail ? picture.link : null} />
          </div>
        )}

        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      <ToolFooter />
    </div>
  );
}

// Check availability: the state in large letters beside its mark, when it
// was checked, what it means, and Check again. It keeps its place whatever
// the state, and until the first answer it says it is checking.
function CheckAvailability({
  avail,
  hasLink,
  now,
  refreshing,
  outcome,
  onRefresh,
  statusRef,
}: {
  avail: Availability | null;
  hasLink: boolean;
  now: number;
  refreshing: boolean;
  outcome: string;
  onRefresh: () => void;
  statusRef: React.RefObject<HTMLParagraphElement | null>;
}) {
  const d = avail && describe(avail, hasLink, (at) => when(at, now));
  const asking = refreshing || !d;
  return (
    <section className="fd-check" aria-labelledby="check-title">
      <h2 id="check-title" className="fd-section-title">
        <span className="tool-step" aria-hidden="true">1</span>
        Check availability
      </h2>
      <div className="fd-card">
        <div className="fd-tabletop">
          <Tabletop lit={avail?.kind === "confirmed" && avail.reading.state === "ready"} />
        </div>
        {d ? (
          <>
            <p ref={statusRef} className="fd-state" tabIndex={-1}>
              <StatusMark mark={d.mark} stale={d.stale} />
              {d.label}
            </p>
            <p className="fd-when">
              <time dateTime={new Date(d.at).toISOString()}>{d.when}</time>
            </p>
            <p className="fd-message">{d.message}</p>
          </>
        ) : (
          <p className="fd-state" role="status">
            <span className="fd-mark fd-mark-checking">
              <Dots />
            </span>
            Checking Foundry…
          </p>
        )}
        <button
          className="quiet-button fd-again"
          type="button"
          aria-disabled={asking ? true : undefined}
          onClick={d ? onRefresh : undefined}
        >
          {asking ? "Checking…" : "Check again"}
        </button>
        {outcome && !refreshing && <p className="fd-rechecked">{outcome}</p>}
      </div>
    </section>
  );
}

// Join in Foundry: the way in, which a failed check never takes away, and
// what happens next inside Foundry. Choosing it never touches the server.
// Until the first answer, Open Foundry's place is kept empty.
function Join({ avail, link }: { avail: Availability | null; link: string | null }) {
  const primary = avail ? openIsPrimary(avail) : false;
  const note = avail && link ? caveat(avail) : "";
  return (
    <section className="fd-join" aria-labelledby="join-title">
      <h2 id="join-title" className="fd-section-title">
        <span className="tool-step" aria-hidden="true">2</span>
        Join in Foundry
      </h2>

      {!avail ? (
        <div className="fd-open-area fd-open-pending" />
      ) : link ? (
        <div className="fd-open-area">
          <a className={primary ? "tactile fd-open" : "quiet-button fd-open"} href={link}>
            Open Foundry
          </a>
          <p className="fd-address">
            Opens{" "}
            <span className="fd-host">
              {segments(address(link)).map((part, i) => (
                <span key={i} className="fd-seg">
                  {part}
                </span>
              ))}
            </span>
          </p>
          {note && <p className="fd-caveat">{note}</p>}
        </div>
      ) : (
        <p className="fd-no-link">{noLink(avail)}</p>
      )}

      <h3 className="fd-steps-title">In Foundry</h3>
      <ol className="fd-steps" role="list">
        <li>Choose your name from Foundry’s list of players.</li>
        <li>Enter your password, if you have one.</li>
        <li>Join the game.</li>
      </ol>
      <div className="fd-boundary">
        <p>Your Foundry name and password come from Craig, not from The Lobby.</p>
        {/* "Let Craig know" stays together, so the note never ends on two stray words. */}
        <p>
          If Foundry shows its setup screen instead of a list of players, no game is open yet. Let&nbsp;Craig&nbsp;know.
        </p>
      </div>
    </section>
  );
}

// A problem in place of Foundry. Only what went wrong is the alert, so a
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
      <div className="fd-problem-says" role="alert">
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
