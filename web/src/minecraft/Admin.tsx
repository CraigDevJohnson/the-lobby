import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { SignOut } from "../SignOut";
import { ToolFooter } from "../ToolChrome";
import {
  changedSince,
  countLine,
  findMods,
  firstSeen,
  getMods,
  launcherCue,
  LISTED,
  modGrid,
  NEW_TAB,
  NO_MODS,
  NO_WORLDS,
  NONE_FOUND,
  notOffered,
  stackUnder,
  WAITING,
  switched,
  switchMod,
  unswitched,
  type Mod,
  type Settings,
  type ModWorld,
  type ModsAnswer,
} from "./mods";
import { STILL_NO_ACCESS } from "./logic";

// Minecraft Admin (docs/design/minecraft-brief.md, "Mods, 2026-11-10"): each
// world's mods, each switched on or off for that world from its next start.
// It is for Members with the minecraft-admin grant, which doesn't include the
// Launcher's. The page arrives as a checking state with nothing private in
// it; the worlds and their mods appear only once the site has confirmed the
// grant. It never says which world is running: the Launcher does that.

type View =
  | { kind: "checking" }
  | { kind: "signed-out" }
  | { kind: "denied" }
  | { kind: "access-failed"; retrying: boolean; retried?: boolean }
  | { kind: "not-configured" }
  // note: a switch the page couldn't confirm, kept until the mods can be
  // read again.
  | { kind: "unavailable"; retrying: boolean; retried?: boolean; note?: string }
  | { kind: "mods"; worlds: ModWorld[] };

// A switch on its way: which world and mod, and the setting asked for.
type Pending = { world: string; mod: string; enabled: boolean };

// What the last switch did, and which switch it was, so dismissing the note
// can hand focus back to it. went: the switch went through, so the note ends
// with where a world's next start happens.
type Outcome = { world: string; mod: string; text: string; went?: boolean };

// A switch's element id. An id can't hold ".", so two cells never share one.
const switchId = (world: string, mod: string) => `mod-${world}.${mod}`;

const STILL_UNAVAILABLE = "Checked just now. We still can’t load the mods.";

export function MinecraftAdmin() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [pending, setPending] = useState<Pending | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // The setting each cell had when the page first showed it. A cell that
  // differs from its own is marked "From next start" until it is switched
  // back or the page reloads.
  const [first, setFirst] = useState<Settings | null>(null);
  const [said, setSaid] = useState("");
  // Retry on the "no worlds" panel: on its way, or back with the same answer.
  const [reread, setReread] = useState<"idle" | "checking" | "same">("idle");
  const latest = useRef(0);
  const problemTitle = useRef<HTMLParagraphElement>(null);
  const title = useRef<HTMLHeadingElement>(null);

  const say = useCallback((text: string) => {
    setSaid("");
    requestAnimationFrame(() => setSaid(text));
  }, []);

  // show puts the newest answer on screen; an older one arriving late is
  // dropped.
  const show = useCallback((answer: ModsAnswer, id: number) => {
    if (id !== latest.current) return false;
    if (answer.kind === "mods") {
      setView({ kind: "mods", worlds: answer.worlds });
      setFirst((f) => firstSeen(f, answer.worlds));
    } else if (answer.kind === "access-failed" || answer.kind === "unavailable")
      setView({ kind: answer.kind, retrying: false });
    else setView({ kind: answer.kind });
    return true;
  }, []);

  const load = useCallback(async () => {
    const id = ++latest.current;
    const answer = await getMods();
    return { answer, shown: show(answer, id) };
  }, [show]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    document.title = "Minecraft Admin · The Lobby";
  }, []);

  // A Retry that brings something else takes its button away with its
  // panel, so focus moves instead of dropping to the page: to the title of
  // the panel that replaced it, or, when the mods are listed, to the page
  // title, and the page says so. A late answer that was dropped moves nothing.
  const land = useCallback(
    (answer: ModsAnswer, shown: boolean) => {
      if (!shown) return;
      setSaid("");
      requestAnimationFrame(() => {
        if (problemTitle.current) problemTitle.current.focus();
        else if (answer.kind === "mods") {
          title.current?.focus();
          say(LISTED);
        }
      });
    },
    [say],
  );

  // Retry keeps focus where it is, and says so when nothing changed.
  const retry = useCallback(async () => {
    if ((view.kind !== "access-failed" && view.kind !== "unavailable") || view.retrying) return;
    const kind = view.kind;
    const note = view.kind === "unavailable" ? view.note : undefined;
    setView({ kind, retrying: true, note });
    const { answer, shown } = await load();
    if (shown && answer.kind === kind) {
      setView({ kind, retrying: false, retried: true, note });
      say(kind === "access-failed" ? STILL_NO_ACCESS : STILL_UNAVAILABLE);
    } else land(answer, shown);
  }, [view, load, say, land]);

  // Retry where the server lists no worlds reads the list again. Like the
  // other Retry it keeps focus where it is and says so when nothing changed.
  // If another panel replaces this one, focus goes to its title; if the mods
  // are listed, focus goes to the page title and the page says so.
  const readAgain = useCallback(async () => {
    if (reread === "checking") return;
    setReread("checking");
    const { answer, shown } = await load();
    const same = shown && answer.kind === "mods" && answer.worlds.length === 0;
    setReread(same ? "same" : "idle");
    if (same) say(NO_WORLDS.still);
    else land(answer, shown);
  }, [reread, load, say, land]);

  // One switch at a time. The answer is every world's mods as the backend
  // now has them, so the page shows that rather than what was asked. When a
  // switch doesn't go through, the page reads the list again, so it never
  // shows a setting the backend doesn't have. If that read fails too, the
  // page says it can't tell whether the switch went through.
  const toggle = useCallback(
    async (world: ModWorld, mod: Mod) => {
      if (pending) return;
      const enabled = !mod.enabled;
      setPending({ world: world.id, mod: mod.id, enabled });
      setOutcome(null);
      const id = ++latest.current;
      const answer = await switchMod(world.id, mod.id, enabled);
      if (answer.kind === "mods") {
        setPending(null);
        show(answer, id);
        const now = answer.worlds.find((w) => w.id === world.id);
        const after = now?.mods.find((m) => m.id === mod.id);
        const went = Boolean(now && after);
        const text = now && after ? switched(now, after) : unswitched("unknown_mod", mod.name);
        setOutcome({ world: world.id, mod: mod.id, text, went });
        const cue = launcherCue();
        say(went ? `${text} ${cue.before}${cue.link}${cue.after}` : text);
        return;
      }
      if (answer.kind !== "refused" && answer.kind !== "unavailable") {
        setPending(null);
        show(answer, id);
        return;
      }
      // Every switch stays locked until the list has been read again, so
      // none is pressed against a setting that may be out of date.
      const text = unswitched(answer.kind === "refused" ? answer.code : "unavailable", mod.name);
      const { answer: reread, shown } = await load();
      setPending(null);
      if (!shown) return;
      if (reread.kind === "mods") {
        setOutcome({ world: world.id, mod: mod.id, text });
        say(text);
      } else if (reread.kind === "unavailable" && answer.kind === "unavailable") {
        const note = `We couldn’t tell whether ${mod.name} was switched in ${world.name}.`;
        setView({ kind: "unavailable", retrying: false, note });
        say(note);
      }
    },
    [pending, show, load, say],
  );

  const member = view.kind !== "checking" && view.kind !== "signed-out";
  // A list with nothing to put in the grid gets a panel that says which
  // thing is missing, instead of an empty card.
  const empty =
    view.kind !== "mods"
      ? null
      : view.worlds.length === 0
        ? "worlds"
        : view.worlds.every((w) => w.mods.length === 0)
          ? "mods"
          : null;

  return (
    <div className="page minecraft mc-admin">
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
        <h1 ref={title} className="mc-title" tabIndex={-1}>
          Minecraft Admin
        </h1>
        {(view.kind === "checking" || view.kind === "signed-out" || (view.kind === "mods" && !empty)) && (
          <p className="mc-lead">Turn mods on or off for each world. A change applies the next time that world starts.</p>
        )}

        <noscript>
          <p className="mc-problem-inline">Minecraft Admin needs JavaScript to load and switch the mods.</p>
        </noscript>

        {view.kind === "checking" && (
          <p className="lobby-status mc-checking" role="status">
            <span className="lobby-status-dots" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            Checking your access…
          </p>
        )}

        {view.kind === "signed-out" && (
          <Problem
            titleRef={problemTitle}
            title="Sign in to use Minecraft Admin."
            detail="It’s for Members Craig has invited. You’ll come back here after signing in."
          >
            <a className="tactile tactile-small" href="/signin?next=/minecraft/admin">
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

        {view.kind === "not-configured" && (
          <Problem
            titleRef={problemTitle}
            title="The Minecraft server isn’t available here yet."
            detail="No server is set up for the Launcher, so there are no mods to switch from this page."
          >
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {(view.kind === "access-failed" || view.kind === "unavailable") && (
          <Problem
            titleRef={problemTitle}
            title={view.kind === "access-failed" ? "We couldn’t check your access." : "We can’t load the mods right now."}
            detail={
              view.kind === "access-failed"
                ? "This doesn’t mean your access has changed."
                : view.note
                  ? `${view.note} Retry to see what the server has.`
                  : "Nothing has been switched."
            }
          >
            <button
              className="tactile tactile-small"
              type="button"
              aria-disabled={view.retrying ? true : undefined}
              onClick={retry}
            >
              {view.retrying ? "Checking…" : "Retry"}
            </button>
            {view.retried && !view.retrying && (
              <p className="mc-rechecked">{view.kind === "access-failed" ? STILL_NO_ACCESS : STILL_UNAVAILABLE}</p>
            )}
          </Problem>
        )}

        {empty === "worlds" && (
          <Problem titleRef={problemTitle} title={NO_WORLDS.title} detail={NO_WORLDS.detail} calm>
            <div className="mca-acts">
              <button
                className="tactile tactile-small"
                type="button"
                aria-disabled={reread === "checking" ? true : undefined}
                onClick={() => void readAgain()}
              >
                {reread === "checking" ? "Checking…" : "Retry"}
              </button>
              <a className="quiet-button" href="/">
                Back to Tools
              </a>
            </div>
            {reread === "same" && <p className="mc-rechecked">{NO_WORLDS.still}</p>}
          </Problem>
        )}

        {empty === "mods" && (
          <Problem titleRef={problemTitle} title={NO_MODS.title} detail={NO_MODS.detail} calm>
            <a className="tactile tactile-small" href="/">
              Back to Tools
            </a>
          </Problem>
        )}

        {view.kind === "mods" && !empty && (
          <ModGrid
            worlds={view.worlds}
            first={first}
            pending={pending}
            outcome={outcome}
            onDismiss={() => setOutcome(null)}
            onToggle={(w, m) => void toggle(w, m)}
          />
        )}

        <p className="visually-hidden" role="status">
          {said}
        </p>
      </main>

      <ToolFooter />
    </div>
  );
}

// Past this many mods the grid offers a search.
const FIND_FROM = 6;

// The mods as one grid: a row per mod by name, a column per world, and a
// switch where that world offers the mod. A mod several worlds share is listed
// once, so its settings can be compared across worlds at a glance. The
// world headers stay in view while a long list scrolls. Where the window is
// too narrow for the columns each row becomes the mod's name over a line per
// world that offers it.
function ModGrid({
  worlds,
  first,
  pending,
  outcome,
  onDismiss,
  onToggle,
}: {
  worlds: ModWorld[];
  first: Settings | null;
  pending: Pending | null;
  outcome: Outcome | null;
  onDismiss: () => void;
  onToggle: (world: ModWorld, mod: Mod) => void;
}) {
  const [query, setQuery] = useState("");
  const lines = useMemo(() => modGrid(worlds), [worlds]);
  const shown = useMemo(() => findMods(lines, query), [lines, query]);
  const finding = lines.length >= FIND_FROM;

  const stacked = useStacked(worlds.length);

  const search = useRef<HTMLInputElement>(null);
  const grid = useRef<HTMLDivElement>(null);
  const cue = launcherCue();

  // A new search is a new question, so the last switch's note goes rather
  // than sit over rows it has nothing to do with.
  const refine = (q: string) => {
    setQuery(q);
    onDismiss();
  };

  // Dismissing the note hands focus back to the switch it was about. If that
  // switch is gone, the search takes it, or failing that the first switch.
  const dismiss = () => {
    const back =
      (outcome && document.getElementById(switchId(outcome.world, outcome.mod))) ??
      search.current ??
      grid.current?.querySelector<HTMLElement>("input");
    onDismiss();
    back?.focus();
  };

  return (
    <div className="mca">
      {finding && (
        <div className="mca-find" role="search">
          <label className="mca-search">
            <span className="mca-find-label">Find a mod</span>
            <input
              ref={search}
              type="search"
              value={query}
              placeholder="Name or what it does"
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => refine(e.target.value)}
            />
          </label>
          <p className="mca-count" aria-live="polite">
            {countLine({ total: lines.length, matching: shown.length, query })}
          </p>
        </div>
      )}

      {/* The table is built from divs so each row can share the card's
          columns. The two row groups are there for assistive technology only
          and make no box of their own, so the rows stay the grid's own. */}
      <div
        ref={grid}
        className="mca-grid"
        role="table"
        aria-label="Mods by world"
        data-stacked={stacked ? true : undefined}
        style={{ "--worlds": worlds.length } as React.CSSProperties}
      >
        <div className="mca-group" role="rowgroup">
          <div className="mca-head" role="row">
            <span className="mca-head-mod" role="columnheader">
              Mod
            </span>
            {worlds.map((w) => {
              const on = w.mods.filter((m) => m.enabled).length;
              return (
                <span key={w.id} className="mca-head-world" role="columnheader">
                  {/* A long name keeps to two lines so the header stays
                      short; the whole name is still there to read. */}
                  <span className="mca-head-name" title={w.name}>
                    {w.name}
                  </span>
                  <span className="mca-head-count">
                    {w.mods.length === 0 ? "No mods" : `${on} of ${w.mods.length} on`}
                  </span>
                </span>
              );
            })}
          </div>
        </div>
        <div className="mca-group" role="rowgroup">
          {shown.map((line) => {
            const missing = notOffered(worlds.filter((_, i) => !line.cells[i]).map((w) => w.name));
            return (
              <div key={line.id} className="mca-row" role="row">
                <div className="mca-mod" role="rowheader">
                  <p className="mca-mod-name">
                    {line.source ? (
                      <a href={line.source} target="_blank" rel="noopener noreferrer">
                        {line.name}
                        <span className="visually-hidden"> ({NEW_TAB})</span>
                        {/* A word joiner, so the glyph never wraps away from the name. */}
                        {"\u2060"}
                        <NewTabGlyph />
                      </a>
                    ) : (
                      line.name
                    )}
                    {line.version && <span className="mca-mod-version">{line.version}</span>}
                  </p>
                  {line.summary && <Summary name={line.name} text={line.summary} />}
                </div>
                {worlds.map((w, i) => {
                  const m = line.cells[i];
                  return (
                    <div key={w.id} className={m ? "mca-cell" : "mca-cell mca-cell-absent"} role="cell">
                      <span className="mca-cell-world" aria-hidden="true">
                        {w.name}
                      </span>
                      {m ? (
                        <Switch
                          world={w}
                          mod={m}
                          changed={changedSince(first, w.id, m)}
                          pending={pending}
                          onToggle={() => onToggle(w, m)}
                        />
                      ) : (
                        <span className="mca-absent">
                          Not offered<span className="visually-hidden"> in {w.name}</span>
                        </span>
                      )}
                    </div>
                  );
                })}
                {/* Stacked, the worlds without this mod share one line in
                    place of a line each. The cells above still say it to
                    assistive technology, so this copy is for the eye only. */}
                {missing && (
                  <p className="mca-unoffered" aria-hidden="true">
                    {missing}
                  </p>
                )}
              </div>
            );
          })}
          {/* With no rows left, one cell across every column says so. */}
          {shown.length === 0 && (
            <div className="mca-row mca-row-empty" role="row">
              <div role="cell" aria-colspan={worlds.length + 1}>
                <p>{NONE_FOUND}</p>
                {/* The button goes with its row, so the search takes focus
                    rather than the page. */}
                <button
                  className="quiet-button"
                  type="button"
                  onClick={() => {
                    refine("");
                    search.current?.focus();
                  }}
                >
                  Show every mod
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Why a switch takes no presses while another is on its way. Each
          waiting switch is described by it; it is never shown. */}
      <span id="mca-waiting" hidden>
        {WAITING}
      </span>

      {/* What the last switch did. It sits under the grid and stays at the
          foot of the view while a long list scrolls, so it never pushes the
          rows and is in sight wherever the switch was. It goes at the next
          switch, at a new search, or when dismissed. */}
      {outcome && (
        <div className="mca-outcome">
          <p>
            {outcome.text}
            {outcome.went && (
              <>
                {" "}
                {cue.before}
                <a href={cue.href}>{cue.link}</a>
                {cue.after}
              </>
            )}
          </p>
          <button className="mca-dismiss" type="button" aria-label="Dismiss this note" onClick={dismiss}>
            <svg viewBox="0 0 32 32" aria-hidden="true" focusable="false">
              <path d="M9 9l14 14M23 9 9 23" />
            </svg>
          </button>
        </div>
      )}
    </div>
  );
}

// A mod's switch in one world. It is a native checkbox with the switch role,
// so it is reached and changed like any other; On or Off is written beside
// it, never said by colour alone. While its own switch is on its way it shows
// the setting asked for, says "Switching…" under it and takes no presses;
// every other switch waits too. A switch whose setting differs from the one
// first loaded says "From next start" in the same place.
function Switch({
  world,
  mod,
  changed,
  pending,
  onToggle,
}: {
  world: ModWorld;
  mod: Mod;
  changed: boolean;
  pending: Pending | null;
  onToggle: () => void;
}) {
  const mine = pending?.world === world.id && pending.mod === mod.id;
  const on = mine ? pending.enabled : mod.enabled;
  const id = switchId(world.id, mod.id);
  const note = mine ? "Switching…" : changed ? "From next start" : "";
  // A waiting switch says why it takes no presses. The description never
  // points at an element with nothing in it.
  const described = [note && `${id}-note`, pending && !mine && "mca-waiting"].filter(Boolean).join(" ");
  return (
    <span className="mca-setting">
      <label className="mca-switch" data-busy={mine ? true : undefined}>
        <input
          type="checkbox"
          role="switch"
          id={id}
          checked={on}
          aria-label={`${mod.name} in ${world.name}`}
          aria-describedby={described || undefined}
          aria-disabled={pending ? true : undefined}
          onChange={() => {
            if (!pending) onToggle();
          }}
        />
        <span className="mca-switch-track" aria-hidden="true">
          <span className="mca-switch-knob" />
        </span>
        <span className="mca-switch-state" aria-hidden="true">
          {on ? "On" : "Off"}
        </span>
      </label>
      <span id={`${id}-note`} className="mca-note">
        {note}
      </span>
    </span>
  );
}

// What a mod does. In the stacked layout a summary longer than two lines is
// cut to two, with More to read the rest. It is measured when its box changes
// size, not on every render, and until it has been measured it shows whole.
function Summary({ name, text }: { name: string; text: string }) {
  const words = useRef<HTMLParagraphElement>(null);
  const [long, setLong] = useState(false);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const el = words.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    // scrollHeight is the whole text's height whether or not it is cut.
    const seen = new ResizeObserver(() => {
      setLong(el.scrollHeight > 2 * parseFloat(getComputedStyle(el).lineHeight) + 1);
    });
    seen.observe(el);
    return () => seen.disconnect();
  }, []);

  return (
    <>
      <p ref={words} className="mca-mod-summary" data-long={long ? true : undefined} data-open={open ? true : undefined}>
        {text}
      </p>
      {long && (
        <button className="mca-more" type="button" aria-expanded={open} onClick={() => setOpen((o) => !o)}>
          {open ? "Less" : "More"}
          <span className="visually-hidden"> about {name}</span>
        </button>
      )}
    </>
  );
}

// Whether the window is too narrow for a column per world, which depends on
// how many worlds there are.
function useStacked(worlds: number) {
  const query = `(max-width: ${stackUnder(worlds)}rem)`;
  const watch = useCallback(
    (changed: () => void) => {
      const media = window.matchMedia(query);
      media.addEventListener("change", changed);
      return () => media.removeEventListener("change", changed);
    },
    [query],
  );
  return useSyncExternalStore(
    watch,
    () => window.matchMedia(query).matches,
    () => false,
  );
}

// The glyph on a mod's link: an arrow leaving a box, in the Tool glyphs'
// stroke. The link's own words say it opens a new tab.
function NewTabGlyph() {
  return (
    <svg className="mca-newtab" viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      <path d="M14 8H9a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h13a2 2 0 0 0 2-2v-5M19 6h7v7M26 6 15 17" />
    </svg>
  );
}

// calm: the panel isn't a failure (a list with nothing in it yet), so it is
// read in its turn instead of interrupting as an alert.
function Problem({
  title,
  detail,
  titleRef,
  calm,
  children,
}: {
  title: string;
  detail?: string;
  titleRef: React.RefObject<HTMLParagraphElement | null>;
  calm?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="lobby-problem">
      <div className="mc-problem-says" role={calm ? undefined : "alert"}>
        <p ref={titleRef} className="lobby-problem-title" tabIndex={-1}>
          {title}
        </p>
        {detail && <p>{detail}</p>}
      </div>
      {children}
    </div>
  );
}
