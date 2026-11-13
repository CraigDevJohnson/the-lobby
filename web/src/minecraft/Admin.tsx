import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { SignOut } from "../SignOut";
import { ToolFooter } from "../ToolChrome";
import {
  findMods,
  getMods,
  modGrid,
  switched,
  switchMod,
  unswitched,
  type Mod,
  type Showing,
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

const STILL_UNAVAILABLE = "Checked just now. We still can’t load the mods.";

export function MinecraftAdmin() {
  const [view, setView] = useState<View>({ kind: "checking" });
  const [pending, setPending] = useState<Pending | null>(null);
  // What the last switch did, shown in its world's section.
  const [outcome, setOutcome] = useState<{ world: string; text: string } | null>(null);
  const [said, setSaid] = useState("");
  const latest = useRef(0);
  const problemTitle = useRef<HTMLParagraphElement>(null);

  const say = useCallback((text: string) => {
    setSaid("");
    requestAnimationFrame(() => setSaid(text));
  }, []);

  // show puts the newest answer on screen; an older one arriving late is
  // dropped.
  const show = useCallback((answer: ModsAnswer, id: number) => {
    if (id !== latest.current) return false;
    if (answer.kind === "mods") setView({ kind: "mods", worlds: answer.worlds });
    else if (answer.kind === "access-failed" || answer.kind === "unavailable")
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
    }
  }, [view, load, say]);

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
        const text = now && after ? switched(now, after) : unswitched("unknown_mod", mod.name);
        setOutcome({ world: world.id, text });
        say(text);
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
        setOutcome({ world: world.id, text });
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
        <h1 className="mc-title">Minecraft Admin</h1>
        {(view.kind === "checking" || view.kind === "signed-out" || view.kind === "mods") && (
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
                ? "This doesn’t mean your access has changed. Please try again in a moment."
                : view.note
                  ? `${view.note} Please try again in a moment to see what the server has.`
                  : "Nothing has been switched. Please try again in a moment."
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

        {view.kind === "mods" && (
          <ModGrid
            worlds={view.worlds}
            pending={pending}
            outcome={outcome?.text}
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

// Past this many mods the grid offers a search and the on or off filter.
const FIND_FROM = 6;

const SHOWING: { value: Showing; label: string }[] = [
  { value: "all", label: "All" },
  { value: "on", label: "On" },
  { value: "off", label: "Off" },
];

// The mods as one grid: a row per mod by name, a column per world, and a
// switch where that world offers the mod. A mod several worlds share is listed
// once, so its settings can be compared across worlds at a glance. The
// world headers stay in view while a long list scrolls. On a phone each row
// becomes the mod's name over a line per world.
function ModGrid({
  worlds,
  pending,
  outcome,
  onToggle,
}: {
  worlds: ModWorld[];
  pending: Pending | null;
  outcome?: string;
  onToggle: (world: ModWorld, mod: Mod) => void;
}) {
  const [query, setQuery] = useState("");
  const [showing, setShowing] = useState<Showing>("all");
  // Rows switched since the search or filter last changed stay listed, so a switch
  // never makes its own row vanish.
  const [keep, setKeep] = useState<Set<string>>(() => new Set());
  const lines = useMemo(() => modGrid(worlds), [worlds]);
  const shown = useMemo(() => findMods(lines, query, showing, keep), [lines, query, showing, keep]);
  const finding = lines.length >= FIND_FROM;
  const narrowed = query.trim() !== "" || showing !== "all";

  const refine = (q: string, s: Showing) => {
    setQuery(q);
    setShowing(s);
    setKeep(new Set());
  };

  if (worlds.length === 0) return <p className="mc-problem-inline">The server lists no worlds yet.</p>;

  return (
    <div className="mca">
      {finding && (
        <div className="mca-find" role="search">
          <label className="mca-search">
            <span className="mca-find-label">Find a mod</span>
            <input
              type="search"
              value={query}
              placeholder="Name or what it does"
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => refine(e.target.value, showing)}
            />
          </label>
          <div className="mca-show">
            <span id="mca-show-label" className="mca-find-label">
              Show
            </span>
            <div className="mca-show-options" role="radiogroup" aria-labelledby="mca-show-label">
              {SHOWING.map((o) => (
                <label key={o.value} className="mca-show-option">
                  <input
                    type="radio"
                    name="mca-show"
                    checked={showing === o.value}
                    onChange={() => refine(query, o.value)}
                  />
                  {o.label}
                </label>
              ))}
            </div>
          </div>
          <p className="mca-count" aria-live="polite">
            {narrowed ? `${shown.length} of ${lines.length} mods` : `${lines.length} mods`}
          </p>
        </div>
      )}

      {lines.length === 0 ? (
        <p className="mca-none">No mods are offered for any world yet.</p>
      ) : (
        <div
          className="mca-grid"
          role="table"
          aria-label="Mods by world"
          style={{ "--worlds": worlds.length } as React.CSSProperties}
        >
          <div className="mca-head" role="row">
            <span className="mca-head-mod" role="columnheader">
              Mod
            </span>
            {worlds.map((w) => {
              const on = w.mods.filter((m) => m.enabled).length;
              return (
                <span key={w.id} className="mca-head-world" role="columnheader">
                  <span className="mca-head-name">{w.name}</span>
                  <span className="mca-head-count">
                    {w.mods.length === 0 ? "No mods" : `${on} of ${w.mods.length} on`}
                  </span>
                </span>
              );
            })}
          </div>
          {shown.map((line) => (
            <div key={line.id} className="mca-row" role="row">
              <div className="mca-mod" role="rowheader">
                <p className="mca-mod-name">
                  {line.source ? (
                    <a href={line.source} target="_blank" rel="noopener noreferrer">
                      {line.name}
                    </a>
                  ) : (
                    line.name
                  )}
                  {line.version && <span className="mca-mod-version">{line.version}</span>}
                </p>
                {line.summary && <p className="mca-mod-summary">{line.summary}</p>}
              </div>
              {worlds.map((w, i) => {
                const m = line.cells[i];
                return (
                  <div key={w.id} className="mca-cell" role="cell">
                    <span className="mca-cell-world" aria-hidden="true">
                      {w.name}
                    </span>
                    {m ? (
                      <Switch
                        world={w}
                        mod={m}
                        pending={pending}
                        onToggle={() => {
                          if (narrowed) setKeep((k) => new Set(k).add(line.id));
                          onToggle(w, m);
                        }}
                      />
                    ) : (
                      <span className="mca-absent">Not offered</span>
                    )}
                  </div>
                );
              })}
            </div>
          ))}
          {shown.length === 0 && (
            <div className="mca-row mca-row-empty" role="row">
              <div role="cell">
                <p>{query.trim() || showing === "all" ? "No mods match that." : `No mods are ${showing}.`}</p>
                <button className="quiet-button" type="button" onClick={() => refine("", "all")}>
                  Show every mod
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* What the last switch did. It sits under the grid and stays at the
          foot of the view while a long list scrolls, so it never pushes the
          rows and is in sight wherever the switch was. */}
      {outcome && <p className="mca-outcome">{outcome}</p>}
    </div>
  );
}

// A mod's switch in one world. It is a native checkbox with the switch role,
// so it is reached and changed like any other; On or Off is written beside
// it, never said by colour alone. While its own switch is on its way it shows
// the setting asked for, says "Switching…" under it and takes no presses;
// every other switch waits too.
function Switch({
  world,
  mod,
  pending,
  onToggle,
}: {
  world: ModWorld;
  mod: Mod;
  pending: Pending | null;
  onToggle: () => void;
}) {
  const mine = pending?.world === world.id && pending.mod === mod.id;
  const on = mine ? pending.enabled : mod.enabled;
  const note = `mod-${world.id}-${mod.id}-note`;
  return (
    <span className="mca-setting">
      <label className="mca-switch" data-busy={mine ? true : undefined}>
        <input
          type="checkbox"
          role="switch"
          checked={on}
          aria-label={`${mod.name} in ${world.name}`}
          aria-describedby={note}
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
      <span id={note} className="mca-note">
        {mine ? "Switching…" : ""}
      </span>
    </span>
  );
}

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
