import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { DeviceGlyph } from "./art";
import { javaAddress, segments, type World } from "./logic";

// The joining help (docs/design/minecraft-brief.md): an explicit, reversible
// device choice, the connection values for it and the steps in Minecraft.
// Each world has its own address, so the values come in one group per world,
// each saying whether that world is ready. Choosing a device never touches a
// server.

type Device = "java" | "bedrock" | "console";

// Each choice says where it plays, so nobody needs to know their edition's
// name first.
const DEVICES: readonly { id: Device; label: string; hint: string }[] = [
  { id: "java", label: "Java Edition", hint: "Windows, Mac, Linux" },
  { id: "bedrock", label: "Bedrock", hint: "Computer, phone, tablet" },
  { id: "console", label: "Console help", hint: "Xbox, PlayStation, Switch" },
];

// The choice is remembered on this browser only, as a convenience.
const REMEMBERED = "minecraft-device";

function remembered(): Device | null {
  try {
    const v = window.localStorage.getItem(REMEMBERED);
    return DEVICES.some((d) => d.id === v) ? (v as Device) : null;
  } catch {
    return null;
  }
}

export function Join({ worlds }: { worlds: readonly World[] }) {
  const [device, setDevice] = useState<Device>("java");
  const tabs = useRef<Record<Device, HTMLButtonElement | null>>({ java: null, bedrock: null, console: null });

  // Read after the first paint, so the prerendered page and the browser agree.
  useEffect(() => {
    const d = remembered();
    if (d) setDevice(d);
  }, []);

  const choose = (d: Device, focus = false) => {
    setDevice(d);
    try {
      window.localStorage.setItem(REMEMBERED, d);
    } catch {
      // Not remembered; the choice still holds on this page.
    }
    if (focus) tabs.current[d]?.focus();
  };

  // Arrow keys move along the tabs, as tabs do everywhere else.
  const onKey = (e: KeyboardEvent) => {
    const i = DEVICES.findIndex((d) => d.id === device);
    const to =
      e.key === "ArrowRight"
        ? (i + 1) % DEVICES.length
        : e.key === "ArrowLeft"
          ? (i - 1 + DEVICES.length) % DEVICES.length
          : e.key === "Home"
            ? 0
            : e.key === "End"
              ? DEVICES.length - 1
              : -1;
    if (to < 0) return;
    e.preventDefault();
    choose(DEVICES[to].id, true);
  };

  // A backend that lists no worlds has one server, which the help calls
  // "the server", as it did before there were worlds.
  const named = worlds.some((w) => w.id);
  const anyReady = worlds.some((w) => w.state === "ready");
  // The name the Bedrock steps offer as an example: a world that is up, or
  // the first.
  const example = (worlds.find((w) => w.state === "ready") ?? worlds[0])?.name ?? "";

  // Once a world is ready, copying its address is the job, so its Copy takes
  // the custard face. The port's stays quiet.
  const groups = (values: (w: World) => Copyable[]): Group[] =>
    worlds.map((w) => ({
      key: w.id ?? "",
      title: named ? w.name : undefined,
      ready: w.state === "ready",
      values: values(w),
    }));

  const java = groups((w) => [
    {
      label: named ? "Address" : "Server address",
      value: w.connection.java ? javaAddress(w.connection.java) : undefined,
      copy: "Copy address",
      copyFor: named ? `Copy address for ${w.name}` : undefined,
      copied: named ? `${w.name}’s address copied.` : "Address copied.",
      press: w.state === "ready",
    },
  ]);
  const bedrock = groups((w) => [
    {
      label: named ? "Address" : "Server address",
      value: w.connection.bedrock?.address,
      copy: "Copy address",
      copyFor: named ? `Copy address for ${w.name}` : undefined,
      copied: named ? `${w.name}’s address copied.` : "Address copied.",
      press: w.state === "ready",
    },
    {
      label: "Port",
      value: w.connection.bedrock ? String(w.connection.bedrock.port) : undefined,
      copy: "Copy port",
      copyFor: named ? `Copy port for ${w.name}` : undefined,
      copied: named ? `${w.name}’s port copied.` : "Port copied.",
      press: false,
    },
  ]);

  return (
    <section className="mc-guide" aria-labelledby="join-title">
      <h2 id="join-title" className="mc-guide-title">
        Join from your device
      </h2>
      <div className="mc-tabs" role="tablist" aria-label="Where you play Minecraft" onKeyDown={onKey}>
        {DEVICES.map((d) => (
          <button
            key={d.id}
            ref={(el) => {
              tabs.current[d.id] = el;
            }}
            id={`tab-${d.id}`}
            className="mc-tab"
            type="button"
            role="tab"
            aria-label={d.label}
            aria-describedby={`hint-${d.id}`}
            aria-selected={device === d.id}
            aria-controls={`panel-${d.id}`}
            tabIndex={device === d.id ? 0 : -1}
            onClick={() => choose(d.id)}
          >
            <DeviceGlyph device={d.id} />
            <span className="mc-tab-text">
              <span className="mc-tab-label">{d.label}</span>
              <span id={`hint-${d.id}`} className="mc-tab-hint">
                {d.hint}
              </span>
            </span>
          </button>
        ))}
      </div>

      <Panel id="java" shown={device}>
        <Slot groups={java} named={named} />
        <p className="mc-guide-lead">
          {anyReady
            ? "Open Minecraft Java Edition."
            : `When ${named ? "a world" : "the server"} is ready, open Minecraft Java Edition.`}
        </p>
        <Steps>
          <li>
            <span>
              Choose <strong>Multiplayer → Add Server</strong>.
            </span>
          </li>
          <li>
            {named ? (
              <span>
                Name it after the world, such as {example}, and paste that world’s address into{" "}
                <strong>Server Address</strong>. Add each world you play the same way, once.
              </span>
            ) : (
              <span>
                Type or paste the address into <strong>Server Address</strong>.
              </span>
            )}
          </li>
          <li>
            <span>
              Choose <strong>Done</strong>, then <strong>Join Server</strong>.
            </span>
          </li>
        </Steps>
      </Panel>

      <Panel id="bedrock" shown={device}>
        <Slot groups={bedrock} named={named} />
        <p className="mc-guide-lead">
          {anyReady
            ? "Open Minecraft on your computer, phone or tablet."
            : `When ${named ? "a world" : "the server"} is ready, open Minecraft on your computer, phone or tablet.`}
        </p>
        <Steps>
          <li>
            <span>
              Choose <strong>Play → Servers → Add Server</strong>.
            </span>
          </li>
          <li>
            {named ? (
              <span>
                Give it the world’s name, such as {example}, then enter that world’s address and port. Add each world
                you play the same way, once.
              </span>
            ) : (
              <span>Give it a name, such as {example}, then enter the address and port shown here.</span>
            )}
          </li>
          <li>
            <span>
              Choose <strong>Save</strong>, then pick the server to join.
            </span>
          </li>
        </Steps>
      </Panel>

      {/* Nothing in Console help can take focus, so the panel itself does,
          and Tab reaches it from the tabs instead of skipping to Privacy. */}
      <Panel id="console" shown={device} focusable>
        <div className="mc-console">
          <p className="mc-guide-lead">Playing on Xbox, PlayStation or Switch? Console setup needs extra help.</p>
          <p>
            Consoles don’t have the <strong>Add Server</strong> option that Bedrock has on computers, phones and
            tablets, so the steps above don’t work there.
          </p>
          <p className="mc-unconfirmed">
            Joining {named ? "these worlds" : "this server"} from your console hasn’t been confirmed yet.
          </p>
          <p>
            For now, play from a computer, phone or tablet with the Java Edition or Bedrock steps. Your console may also
            need its own online membership for multiplayer.
          </p>
        </div>
      </Panel>

      <div className="mc-before">
        <p>
          <strong>Before you join:</strong> sign in to Minecraft with the account Craig added to the{" "}
          {named ? "worlds" : "server"}, and make sure it’s allowed to play multiplayer. Your Lobby sign-in doesn’t add
          your Minecraft account.
        </p>
        {device !== "console" && (
          <p>
            <strong>Xbox, PlayStation or Switch?</strong>{" "}
            <button type="button" className="mc-text-button" onClick={() => choose("console", true)}>
              See console help
            </button>
          </p>
        )}
      </div>
    </section>
  );
}

function Panel({
  id,
  shown,
  focusable = false,
  children,
}: {
  id: Device;
  shown: Device;
  focusable?: boolean;
  children: ReactNode;
}) {
  return (
    <div
      id={`panel-${id}`}
      className="mc-panel"
      role="tabpanel"
      aria-labelledby={`tab-${id}`}
      tabIndex={focusable ? 0 : undefined}
      hidden={id !== shown}
    >
      {children}
    </div>
  );
}

function Steps({ children }: { children: ReactNode }) {
  return <ol className="mc-steps">{children}</ol>;
}

type Copyable = {
  label: string;
  value: string | undefined;
  copy: string; // the button's words
  copyFor?: string; // the button's name when several worlds share the slot
  copied: string;
  press: boolean;
};

// One world's values in the slot. Without a title the slot holds the one
// server's, as it did before there were worlds.
type Group = { key: string; title?: string; ready: boolean; values: Copyable[] };

// Copied says so for a few seconds; a copy that had to be done by hand
// keeps its instruction until the next copy.
const COPIED_FOR_MS = 4000;
type Outcome = { text: string; lasts: boolean } | null;

// The slot frames the connection values, the thing a Member came for, one
// group per world. Its values share one feedback line just under the frame,
// so feedback never stacks; the line keeps its space, so feedback never
// covers the steps below.
function Slot({ groups, named }: { groups: Group[]; named: boolean }) {
  const [outcome, setOutcome] = useState<Outcome>(null);
  const texts = useRef(new Map<string, HTMLSpanElement>());

  // New values (a new device, or a world that changed) start without old
  // feedback.
  const shown = groups.map((g) => g.values.map((v) => v.value ?? "").join(" ")).join(" | ");
  useEffect(() => setOutcome(null), [shown]);

  useEffect(() => {
    if (!outcome || outcome.lasts) return;
    const t = window.setTimeout(() => setOutcome(null), COPIED_FOR_MS);
    return () => window.clearTimeout(t);
  }, [outcome]);

  // Copying never claims to reach another device. When the clipboard
  // refuses, the value is selected so it can be copied by hand.
  const copy = async (where: string, { value, copied }: Copyable) => {
    if (!value) return;
    setOutcome(null);
    try {
      await navigator.clipboard.writeText(value);
      setOutcome({ text: copied, lasts: false });
    } catch {
      const el = texts.current.get(where);
      const sel = window.getSelection();
      if (el && sel) {
        const range = document.createRange();
        range.selectNodeContents(el);
        sel.removeAllRanges();
        sel.addRange(range);
      }
      setOutcome({ text: "Copying didn’t work here. It’s selected above: copy it yourself.", lasts: true });
    }
  };

  return (
    <>
      <div className={named ? "mc-slot mc-slot-worlds" : "mc-slot"}>
        {groups.map((g) => {
          const has = g.values.some((v) => v.value);
          return (
            <div key={g.key} className="mc-slot-group">
              {g.title && (
                <p className="mc-slot-world">
                  {g.title}
                  {has && <span className="mc-value-when">{g.ready ? "Ready to join" : "Join after it’s ready"}</span>}
                </p>
              )}
              {g.values.map((v) => {
                const where = `${g.key}:${v.label}`;
                return (
                  <div key={v.label} className="mc-value">
                    {/* With several worlds, Java's one value sits under its
                      world's name, which says enough on screen. */}
                    <p className={g.title && g.values.length === 1 ? "visually-hidden" : "mc-value-label"}>
                      {v.label}
                      {!g.title && v.value && !g.ready && (
                        <span className="mc-value-when">Join after the server is ready</span>
                      )}
                    </p>
                    {v.value ? (
                      <div className="mc-value-row">
                        {/* Shown in parts that wrap after a dot or colon (a part too
                          long for the line breaks inside it); copied whole, exactly
                          as sent. The parts are only for wrapping, so assistive
                          technology reads the value as one. */}
                        <span
                          ref={(el) => {
                            if (el) texts.current.set(where, el);
                            else texts.current.delete(where);
                          }}
                          className="mc-value-text"
                        >
                          {segments(v.value).map((part, j) => (
                            <span key={j} className="mc-seg" role="none">
                              {part}
                            </span>
                          ))}
                        </span>
                        <button
                          type="button"
                          className={v.press ? "tactile tactile-small mc-copy" : "quiet-button mc-copy"}
                          aria-label={v.copyFor}
                          onClick={() => void copy(where, v)}
                        >
                          {v.copy}
                        </button>
                      </div>
                    ) : (
                      <p className="mc-value-missing">Connection details unavailable</p>
                    )}
                  </div>
                );
              })}
            </div>
          );
        })}
      </div>
      <p className="mc-copy-outcome" role="status">
        {outcome?.text}
      </p>
    </>
  );
}
