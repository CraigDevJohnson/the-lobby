import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { SignOut } from "../SignOut";
import { ToolFooter } from "../ToolChrome";
import { getStatus, startWorld, stopWorld, type Problem, type StatusAnswer, type WorldAnswer } from "./api";
import { ServerBlock, StatusGlyph } from "./art";
import { Join } from "./Join";
import {
  allUnconfirmed,
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
  UNNAMED,
  worldKey,
  type Description,
  type Refusal,
  type World,
} from "./logic";

// The Minecraft Launcher (docs/design/minecraft-brief.md, "Status beside
// setup", in the block motif of DESIGN.md): which worlds are up, start the
// one I want, how do I connect on my device. Each world runs on its own
// server and gets its own card; the joining help beside them is shared. The
// page arrives as a checking state with nothing private in it; the worlds,
// their status and addresses appear only once the site has confirmed the
// Member's minecraft grant.

type View =
  | { kind: "checking" }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed"; retrying: boolean; retried?: boolean }
  | { kind: "not-configured" }
  | { kind: "worlds"; worlds: World[] };

// What the page is doing about a world's Start or Stop, apart from what the
// world says. "confirming": Stop was pressed and the card is asking whether
// they mean it; nothing has been sent. "asking": the request is on its way.
// "refused": the backend would not stop it, for the reason given, and
// nothing changed. "unsure": the request may or may not have reached the
// server, so the card offers a status check, never the same request again.
type Action = "start" | "stop";
type Request =
  | { kind: "idle" }
  | { kind: "confirming" }
  | { kind: "asking"; of: Action }
  | { kind: "refused"; message: string }
  | { kind: "unsure"; of: Action };

const IDLE: Request = { kind: "idle" };

// Focus moves the page makes don't scroll it, unless the control the Member
// was using went while they looked at it and its replacement is out of view.
const focusHere = (el: HTMLElement | null | undefined, scroll = false) => el?.focus({ preventScroll: !scroll });

const inView = (el: Element) => {
  const r = el.getBoundingClientRect();
  return r.bottom > 0 && r.top < window.innerHeight;
};

// The world a page element belongs to, read from its card.
const cardOf = (el: Element | null) => el?.closest("[data-world]")?.getAttribute("data-world") ?? null;

// A world's name in the page's sentences, or none for the one server of a
// backend that lists no worlds, which the page calls "the server".
const named = (w: World) => (w.id ? w.name : undefined);

export function Launcher() {
  const [view, setView] = useState<View>({ kind: "checking" });
  // Each world's request, by its key; a world not listed is idle.
  const [requests, setRequests] = useState<Record<string, Request>>({});
  // The card whose Check status again is on its way, and the card where it
  // came back with the same unconfirmed answer.
  const [refreshing, setRefreshing] = useState<string | null>(null);
  const [rechecked, setRechecked] = useState<string | null>(null);
  const [said, setSaid] = useState("");
  // The host waves once each time a world becomes ready while the page is
  // open, never on arrival at a world that was already ready.
  const [waves, setWaves] = useState(0);

  const current = useRef<World[] | null>(null);
  const pending = useRef(requests);
  pending.current = requests;
  // Every call gets the next number. A status check covers every world; a
  // start or stop covers one. An answer is shown only if no newer call has
  // covered its world since, so an older answer arriving late never undoes
  // a newer one.
  const seq = useRef(0);
  const lastCheck = useRef(0);
  const lastFor = useRef<Record<string, number>>({});
  // Starts and stops on their way. A status check waits for them, so it
  // never reads a world from before its request and replaces the answer.
  const sending = useRef(new Set<Promise<unknown>>());
  const send = useCallback(<T,>(request: Promise<T>) => {
    sending.current.add(request);
    void request.finally(() => sending.current.delete(request));
    return request;
  }, []);

  const statusLines = useRef(new Map<string, HTMLParagraphElement>());
  const stopButtons = useRef(new Map<string, HTMLButtonElement>());
  const questions = useRef(new Map<string, HTMLParagraphElement>());
  const problemTitle = useRef<HTMLParagraphElement>(null);
  // What had focus as an answer was applied, whether it was in view, and
  // which card it was in.
  const hadFocus = useRef<{ el: Element; seen: boolean; key: string | null } | null>(null);
  // Where focus goes when a card's Stop control changes shape, if anywhere.
  const focusNext = useRef<{ key: string; to: "question" | "stop" } | null>(null);

  const say = useCallback((text: string) => {
    if (!text) return;
    setSaid("");
    requestAnimationFrame(() => setSaid(text));
  }, []);

  const setRequest = useCallback((key: string, r: Request) => setRequests((all) => ({ ...all, [key]: r })), []);

  // If what has focus is about to go (Start, Stop, anything in a stop
  // question, or the whole page of worlds when access is gone), focus moves
  // to the state it changed, or to the problem that replaced it, rather than
  // falling to the page.
  const rememberFocus = (key: string | null) => {
    const active = document.activeElement;
    hadFocus.current =
      active && active !== document.body ? { el: active, seen: inView(active), key: cardOf(active) ?? key } : null;
  };

  // Signed out, no access, or no server: nothing private stays on screen,
  // and no answer still on its way can put it back.
  const showProblem = (answer: Exclude<Problem, { kind: "unavailable" }>) => {
    const id = ++seq.current;
    lastCheck.current = id;
    for (const key of Object.keys(lastFor.current)) lastFor.current[key] = id;
    current.current = null;
    setView(answer.kind === "access-failed" ? { kind: "access-failed", retrying: false } : { kind: answer.kind });
  };

  // show puts worlds on screen. A refusal was true when it was given; a
  // later answer about its world is newer, so it goes, and a stop question
  // goes with a world that is no longer ready.
  const show = useCallback(
    (worlds: World[], changed: (key: string) => boolean) => {
      const before = current.current;
      current.current = worlds;
      setView({ kind: "worlds", worlds });
      setRequests((all) => {
        let next = all;
        for (const w of worlds) {
          const key = worldKey(w);
          const r = all[key];
          if (!r || !changed(key)) continue;
          if (r.kind === "refused" || (r.kind === "confirming" && !canStop(w.state))) next = { ...next, [key]: IDLE };
        }
        return next;
      });
      const was = (w: World) => before?.find((b) => worldKey(b) === worldKey(w))?.state;
      if (worlds.some((w) => changed(worldKey(w)) && was(w) && was(w) !== "ready" && w.state === "ready"))
        setWaves((n) => n + 1);
      say(
        worlds
          .filter((w) => changed(worldKey(w)))
          .map((w) => announcement(was(w), w, named(w)))
          .filter(Boolean)
          .join(" "),
      );
    },
    [say],
  );

  // applyCheck shows a status check's answer: every world, except one that
  // a start or stop has covered since the check was sent. It says whether
  // the answer was shown, as a newer call may have overtaken it.
  const applyCheck = useCallback(
    (answer: StatusAnswer, id: number) => {
      if (id !== lastCheck.current) return false;
      rememberFocus(null);
      setRechecked(null);
      if (answer.kind !== "worlds" && answer.kind !== "unavailable") {
        showProblem(answer);
        return true;
      }
      const before = current.current;
      const newer = (key: string) => (lastFor.current[key] ?? 0) > id;
      const fresh = answer.kind === "worlds" ? answer.worlds : allUnconfirmed(before);
      const worlds = fresh.map((w) => (newer(worldKey(w)) && before?.find((b) => worldKey(b) === worldKey(w))) || w);
      show(worlds, (key) => !newer(key));
      return true;
    },
    [show],
  );

  // applyWorld shows a start's or stop's answer in the card of the world it
  // was sent for, unless a newer call has covered that world.
  const applyWorld = useCallback(
    (key: string, answer: Exclude<WorldAnswer, { kind: "refused" }>, id: number) => {
      if (lastFor.current[key] !== id || lastCheck.current > id) return;
      rememberFocus(key);
      setRechecked(null);
      if (answer.kind !== "status" && answer.kind !== "unavailable") return showProblem(answer);
      const before = current.current ?? [];
      const prev = before.find((w) => worldKey(w) === key) ?? null;
      const status = answer.kind === "status" ? answer.status : prev ? unconfirmed(prev) : unconfirmed(null);
      // The answer is that one world's status. It keeps the world's id, and
      // its name when the answer gives none.
      const world: World = {
        ...status,
        name: status.name === UNNAMED && prev ? prev.name : status.name,
        ...(key ? { id: key } : {}),
      };
      const worlds = prev ? before.map((w) => (worldKey(w) === key ? world : w)) : [...before, world];
      show(worlds, (k) => k === key);
    },
    [show],
  );

  // check also says whether its answer was shown. It is sent once no start
  // or stop is on its way.
  const check = useCallback(async () => {
    while (sending.current.size) await Promise.allSettled(sending.current);
    const id = ++seq.current;
    lastCheck.current = id;
    const answer = await getStatus();
    return { answer, shown: applyCheck(answer, id) };
  }, [applyCheck]);

  // First load.
  useEffect(() => {
    void check();
  }, [check]);

  const worlds = view.kind === "worlds" ? view.worlds : null;

  // Keep asking while the page is visible: often while any world is on its
  // way somewhere, rarely otherwise. A hidden page stops asking and checks
  // again when it is shown. Polling never moves focus or the device choice.
  // It pauses while a Start, Stop or Check status again is on its way: a
  // check that crossed the request would be the newer call and replace the
  // request's own answer, perhaps with the state from before it.
  const busy = Object.values(requests).some((r) => r.kind === "asking") || refreshing !== null;
  useEffect(() => {
    if (!worlds || busy) return;
    let timer = 0;
    const schedule = () => {
      window.clearTimeout(timer);
      if (document.visibilityState === "visible") timer = window.setTimeout(() => void check(), pollDelay(worlds));
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
  }, [worlds, busy, check]);

  useEffect(() => {
    const had = hadFocus.current;
    hadFocus.current = null;
    if (!had || had.el.isConnected) return;
    const line = (had.key !== null && statusLines.current.get(had.key)) || statusLines.current.values().next().value;
    focusHere(line ?? problemTitle.current, had.seen);
  });

  // Opening a stop question puts focus on it; leaving it without a stop puts
  // focus back on that world's Stop.
  useEffect(() => {
    const next = focusNext.current;
    focusNext.current = null;
    if (next) focusHere((next.to === "question" ? questions : stopButtons).current.get(next.key));
  });

  // The tab's title leads with the state, for a Member waiting in another tab.
  useEffect(() => {
    document.title = pageTitle(worlds);
  }, [worlds]);

  // A world the backend no longer has: the page says so and reads the list
  // again, which takes its card away.
  const gone = useCallback(
    (key: string, refusal: Refusal) => {
      setRequest(key, IDLE);
      say(refusalMessage(refusal));
      void check();
    },
    [setRequest, say, check],
  );

  const start = useCallback(
    async (w: World) => {
      const key = worldKey(w);
      if (pending.current[key]?.kind === "asking") return;
      setRequest(key, { kind: "asking", of: "start" });
      say(w.id ? `Asking ${w.name} to start.` : "Asking the server to start.");
      const id = ++seq.current;
      lastFor.current[key] = id;
      const answer = await send(startWorld(w.id));
      if (answer.kind === "refused") return gone(key, answer);
      // The start may have reached the server even though no answer came back.
      setRequest(key, answer.kind === "unavailable" ? { kind: "unsure", of: "start" } : IDLE);
      applyWorld(key, answer, id);
    },
    [setRequest, say, gone, applyWorld, send],
  );

  // Stop asks first; nothing is sent until the question's Stop is pressed.
  const confirmStop = useCallback(
    (key: string) => {
      focusNext.current = { key, to: "question" };
      setRequest(key, { kind: "confirming" });
    },
    [setRequest],
  );

  const keepPlaying = useCallback(
    (key: string) => {
      focusNext.current = { key, to: "stop" };
      setRequest(key, IDLE);
    },
    [setRequest],
  );

  const stop = useCallback(
    async (w: World) => {
      const key = worldKey(w);
      if (pending.current[key]?.kind === "asking") return;
      setRequest(key, { kind: "asking", of: "stop" });
      say(w.id ? `Asking ${w.name} to stop.` : "Asking the server to stop.");
      const id = ++seq.current;
      lastFor.current[key] = id;
      const answer = await send(stopWorld(w.id));
      if (answer.kind === "refused") {
        if (answer.code === "unknown_world") return gone(key, answer);
        // Nothing changed. The reason takes the question's place, and Stop
        // is back for when it no longer applies.
        const message = refusalMessage(answer, named(w));
        focusNext.current = { key, to: "stop" };
        setRequest(key, { kind: "refused", message });
        say(message);
        return;
      }
      // The stop may have reached the server even though no answer came back.
      setRequest(key, answer.kind === "unavailable" ? { kind: "unsure", of: "stop" } : IDLE);
      applyWorld(key, answer, id);
    },
    [setRequest, say, gone, applyWorld, send],
  );

  // Check status again stays where it is while it asks, so focus stays on it,
  // and an answer that changed nothing is still said and shown in its card.
  const refresh = useCallback(
    async (key: string) => {
      if (refreshing !== null) return;
      setRefreshing(key);
      const state = () => current.current?.find((w) => worldKey(w) === key)?.state;
      const before = state();
      const { answer, shown } = await check();
      setRefreshing(null);
      // An answer that was overtaken says nothing about what is on screen.
      if (!shown) return;
      // Confirmed again: no card is left unsure of its last request.
      if (answer.kind === "worlds")
        setRequests((all) =>
          Object.fromEntries(Object.entries(all).map(([k, r]) => [k, r.kind === "unsure" ? IDLE : r])),
        );
      if ((answer.kind === "worlds" || answer.kind === "unavailable") && state() === before) {
        setRechecked(key);
        say(RECHECKED);
      }
    },
    [refreshing, check, say],
  );

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
  const showLead = view.kind === "checking" || view.kind === "signed-out" || view.kind === "worlds";

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
        {showLead && <p className="mc-lead">{lead(worlds)}</p>}

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
          <ProblemPanel
            titleRef={problemTitle}
            title="Sign in to use the Minecraft Launcher."
            detail="It’s for Members Craig has invited. You’ll come back here after signing in."
          >
            <a className="tactile tactile-small" href="/signin?next=/minecraft">
              Sign in
            </a>
          </ProblemPanel>
        )}

        {view.kind === "denied" && (
          <ProblemPanel titleRef={problemTitle} title="You don’t have access to this Tool.">
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </ProblemPanel>
        )}

        {view.kind === "access-failed" && (
          <ProblemPanel
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
          </ProblemPanel>
        )}

        {view.kind === "not-configured" && (
          <ProblemPanel
            titleRef={problemTitle}
            title="The Minecraft server isn’t available here yet."
            detail="No server is set up for the Launcher, so there’s nothing to start or join from this page."
          >
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </ProblemPanel>
        )}

        {worlds && (
          <div className="mc-workspace">
            <div className="mc-worlds">
              {worlds.map((w) => {
                const key = worldKey(w);
                return (
                  <WorldCard
                    key={key}
                    world={w}
                    request={requests[key] ?? IDLE}
                    refreshing={refreshing === key}
                    rechecked={rechecked === key}
                    say={say}
                    onStart={() => void start(w)}
                    onConfirmStop={() => confirmStop(key)}
                    onStop={() => void stop(w)}
                    onKeepPlaying={() => keepPlaying(key)}
                    onRefresh={() => void refresh(key)}
                    statusRef={register(statusLines.current, key)}
                    stopRef={register(stopButtons.current, key)}
                    questionRef={register(questions.current, key)}
                  />
                );
              })}
            </div>
            <Join worlds={worlds} />
          </div>
        )}

        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      {/* The host stands on the terrace's landing, under the world cards. */}
      <ToolFooter waveKey={waves} />
    </div>
  );
}

// register keeps a card's element in a map by the card's key, for the focus
// moves above.
function register<E extends HTMLElement>(map: Map<string, E>, key: string) {
  return (el: E | null) => {
    if (el) map.set(key, el);
    else map.delete(key);
  };
}

// A world's card: its server block, its name, the state in large letters
// and the one action that fits it. It keeps its place as the world moves
// from stopped to starting to ready. Stop asks before it acts: the question
// and its two answers take its place in the card, and a refusal sits above
// it as a notice. Stop and the question's answers are quiet, so the only
// custard control in a card is Start.
function WorldCard({
  world,
  request,
  refreshing,
  rechecked,
  say,
  onStart,
  onConfirmStop,
  onStop,
  onKeepPlaying,
  onRefresh,
  statusRef,
  stopRef,
  questionRef,
}: {
  world: World;
  request: Request;
  refreshing: boolean;
  rechecked: boolean;
  say: (text: string) => void;
  onStart: () => void;
  onConfirmStop: () => void;
  onStop: () => void;
  onKeepPlaying: () => void;
  onRefresh: () => void;
  statusRef: (el: HTMLParagraphElement | null) => void;
  stopRef: (el: HTMLButtonElement | null) => void;
  questionRef: (el: HTMLParagraphElement | null) => void;
}) {
  const name = named(world);
  const state = world.state;
  const described = describe(world, name);
  const slow = useSlowStart(state === "starting", say);

  const asking = request.kind === "asking" && request.of === "start";
  const stopping = request.kind === "asking" && request.of === "stop";
  // The request the card could not confirm, once the state is shown as
  // unconfirmed.
  const unsure = request.kind === "unsure" && state === "unknown" ? request.of : null;
  const starting = state === "starting" || asking;
  // Start stays in place, unavailable, while a start is on its way, so a
  // second tap does nothing and focus is not lost.
  const showStart = !unsure && (canStart(state) || starting);
  const showStop = !unsure && canStop(state);
  const confirming = showStop && (request.kind === "confirming" || stopping);
  // While the start is on its way the state already reads as starting, so
  // the words never contradict the button beside them.
  const shown: Pick<Description, "label" | "mark"> =
    asking && state !== "starting" ? { label: "Starting", mark: "busy" } : described;
  const message = unsure
    ? `We couldn’t tell whether ${name ?? "the server"} got the ${unsure} request. Check its status before trying again.`
    : asking && state !== "starting"
      ? `Asking ${name ?? "the server"} to start…`
      : described.message;
  // At Ready the joining help says what to do, unless there is nothing to
  // join with yet.
  const showMessage = unsure !== null || state !== "ready" || !hasDetails(world.connection);
  const id = `world-${world.id ?? "server"}`;

  // Escape answers the stop question the same way Keep playing does.
  const onKey = (e: KeyboardEvent) => {
    if (e.key !== "Escape" || stopping) return;
    e.preventDefault();
    onKeepPlaying();
  };

  return (
    <section className="mc-server" data-world={worldKey(world)} aria-labelledby={`${id}-title`}>
      <ServerBlock lit={state === "ready"} />
      <h2 id={`${id}-title`} className="mc-server-name">
        {world.name}
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
          {starting
            ? "Starting…"
            : state === "failed"
              ? name
                ? `Try starting ${name} again`
                : "Try starting again"
              : name
                ? `Start ${name}`
                : "Start server"}
        </button>
      )}

      {showStop && !confirming && request.kind === "refused" && <p className="mc-refused">{request.message}</p>}
      {showStop && !confirming && (
        <button ref={stopRef} className="quiet-button mc-stop" type="button" onClick={onConfirmStop}>
          {name ? `Stop ${name}` : "Stop server"}
        </button>
      )}
      {confirming && (
        <div className="mc-confirm" onKeyDown={onKey}>
          <p ref={questionRef} className="mc-confirm-question" tabIndex={-1}>
            {name ? `Stop ${name}?` : "Stop the server?"} Everyone’s progress is saved first.
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
            <button className="quiet-button mc-keep" type="button" onClick={onKeepPlaying} disabled={stopping}>
              Keep playing
            </button>
          </div>
        </div>
      )}

      {(state === "unknown" || unsure) && (
        <>
          <button
            className="quiet-button mc-refresh"
            type="button"
            aria-disabled={refreshing ? true : undefined}
            onClick={onRefresh}
          >
            {refreshing ? "Checking status…" : "Check status again"}
          </button>
          {rechecked && !refreshing && <p className="mc-rechecked">{RECHECKED}</p>}
        </>
      )}

      {state === "starting" && slow && <p className="mc-note">{SLOW_START}</p>}

      {starting && <p className="mc-note">You can get your connection details ready while it starts.</p>}
    </section>
  );
}

// useSlowStart measures from when this card first saw its world starting,
// and the note is read out once when it appears.
function useSlowStart(starting: boolean, say: (text: string) => void) {
  const [since, setSince] = useState<number | null>(null);
  const [slow, setSlow] = useState(false);
  useEffect(() => {
    if (!starting) {
      setSince(null);
      setSlow(false);
      return;
    }
    setSince((s) => s ?? Date.now());
  }, [starting]);
  useEffect(() => {
    if (since === null) return;
    const t = window.setTimeout(() => setSlow(true), Math.max(0, since + SLOW_START_MS - Date.now()));
    return () => window.clearTimeout(t);
  }, [since]);
  useEffect(() => {
    if (slow) say(SLOW_START);
  }, [slow, say]);
  return slow;
}

// A problem in place of the worlds. Only what went wrong is the alert, so a
// control below it changing its label is never read out as a new alert.
function ProblemPanel({
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
