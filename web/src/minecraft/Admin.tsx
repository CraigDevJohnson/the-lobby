import { useCallback, useEffect, useRef, useState } from "react";
import { SignOut } from "../SignOut";
import { ToolFooter } from "../ToolChrome";
import { getMods, switched, switchMod, unswitched, type Mod, type ModWorld, type ModsAnswer } from "./mods";
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
      setPending(null);
      if (answer.kind === "mods") {
        show(answer, id);
        const now = answer.worlds.find((w) => w.id === world.id);
        const after = now?.mods.find((m) => m.id === mod.id);
        const text = now && after ? switched(now, after) : unswitched("unknown_mod", mod.name);
        setOutcome({ world: world.id, text });
        say(text);
        return;
      }
      if (answer.kind !== "refused" && answer.kind !== "unavailable") {
        show(answer, id);
        return;
      }
      const text = unswitched(answer.kind === "refused" ? answer.code : "unavailable", mod.name);
      const { answer: reread, shown } = await load();
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
          <div className="mca-worlds">
            {/* A switch whose world is no longer listed says what happened here. */}
            {outcome && !view.worlds.some((w) => w.id === outcome.world) && (
              <p className="mca-outcome">{outcome.text}</p>
            )}
            {view.worlds.length === 0 && <p className="mc-problem-inline">The server lists no worlds yet.</p>}
            {view.worlds.map((w) => (
              <section key={w.id} className="mca-world" aria-labelledby={`mods-${w.id}`}>
                <h2 id={`mods-${w.id}`} className="mca-world-name">
                  {w.name}
                </h2>
                {outcome?.world === w.id && <p className="mca-outcome">{outcome.text}</p>}
                {w.mods.length === 0 ? (
                  <p className="mca-none">No mods are offered for {w.name}.</p>
                ) : (
                  <ul className="mca-mods">
                    {w.mods.map((m) => (
                      <ModRow
                        key={m.id}
                        world={w}
                        mod={m}
                        pending={pending}
                        onToggle={() => void toggle(w, m)}
                      />
                    ))}
                  </ul>
                )}
              </section>
            ))}
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

// A mod: its name and version, what it does, how it behaves on Bedrock, and
// its switch. The switch is a native checkbox with the switch role, so it is
// reached and changed like any other; On or Off is written beside it, never
// said by colour alone. While its own switch is on its way it shows the
// setting asked for and takes no presses; every other switch waits too.
function ModRow({
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
  const base = `mod-${world.id}-${mod.id}`;
  return (
    <li className="mca-mod">
      <div className="mca-mod-text">
        <p className="mca-mod-name">
          {mod.name}
          {mod.version && <span className="mca-mod-version">{mod.version}</span>}
        </p>
        {mod.summary && <p className="mca-mod-summary">{mod.summary}</p>}
        {mod.bedrock && (
          <p className="mca-mod-bedrock">
            <strong>On Bedrock:</strong> {mod.bedrock}
          </p>
        )}
        <p id={`${base}-default`} className="mca-mod-default">
          {mod.default ? "On" : "Off"} by default in {world.name}
        </p>
      </div>
      <label className="mca-switch" data-busy={mine ? true : undefined}>
        <input
          type="checkbox"
          role="switch"
          checked={on}
          aria-label={`${mod.name} in ${world.name}`}
          aria-describedby={`${base}-default`}
          aria-disabled={pending ? true : undefined}
          onChange={() => {
            if (!pending) onToggle();
          }}
        />
        <span className="mca-switch-track" aria-hidden="true">
          <span className="mca-switch-knob" />
        </span>
        <span className="mca-switch-state" aria-hidden="true">
          {mine ? "Switching…" : on ? "On" : "Off"}
        </span>
      </label>
    </li>
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
