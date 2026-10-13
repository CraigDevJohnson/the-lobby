import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { DeviceGlyph } from "./art";
import { javaAddress, segments, type Connection, type ServerState } from "./logic";

// The joining help (docs/design/minecraft-brief.md): an explicit, reversible
// device choice, the connection values for it and the steps in Minecraft.
// Choosing a device never touches the server.

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

export function Join({
  connection,
  state,
  serverName,
}: {
  connection: Connection;
  state: ServerState;
  serverName: string;
}) {
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

  // Once the server is ready, copying the address is the job, so its Copy
  // takes the custard face. The port's stays quiet: one custard at a time.
  const ready = state === "ready";

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
        <Slot
          ready={ready}
          values={[
            {
              label: "Server address",
              value: connection.java ? javaAddress(connection.java) : undefined,
              copy: "Copy address",
              copied: "Address copied.",
              press: ready,
            },
          ]}
        />
        <p className="mc-guide-lead">
          {ready ? "Open Minecraft Java Edition." : "When the server is ready, open Minecraft Java Edition."}
        </p>
        <Steps>
          <li>
            <span>
              Choose <strong>Multiplayer → Add Server</strong>.
            </span>
          </li>
          <li>
            <span>
              Type or paste the address into <strong>Server Address</strong>.
            </span>
          </li>
          <li>
            <span>
              Choose <strong>Done</strong>, then <strong>Join Server</strong>.
            </span>
          </li>
        </Steps>
      </Panel>

      <Panel id="bedrock" shown={device}>
        <Slot
          ready={ready}
          values={[
            {
              label: "Server address",
              value: connection.bedrock?.address,
              copy: "Copy address",
              copied: "Address copied.",
              press: ready,
            },
            {
              label: "Port",
              value: connection.bedrock ? String(connection.bedrock.port) : undefined,
              copy: "Copy port",
              copied: "Port copied.",
              press: false,
            },
          ]}
        />
        <p className="mc-guide-lead">
          {ready
            ? "Open Minecraft on your computer, phone or tablet."
            : "When the server is ready, open Minecraft on your computer, phone or tablet."}
        </p>
        <Steps>
          <li>
            <span>
              Choose <strong>Play → Servers → Add Server</strong>.
            </span>
          </li>
          <li>
            <span>Give it a name, such as {serverName}, then enter the address and port shown here.</span>
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
          <p className="mc-unconfirmed">Joining this server from your console hasn’t been confirmed yet.</p>
          <p>
            For now, play from a computer, phone or tablet with the Java Edition or Bedrock steps. Your console may also
            need its own online membership for multiplayer.
          </p>
        </div>
      </Panel>

      <div className="mc-before">
        <p>
          <strong>Before you join:</strong> sign in to Minecraft with the account Craig added to the server, and make
          sure it’s allowed to play multiplayer. Your Lobby sign-in doesn’t add your Minecraft account.
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

type Copyable = { label: string; value: string | undefined; copy: string; copied: string; press: boolean };

// Copied says so for a few seconds; a copy that had to be done by hand
// keeps its instruction until the next copy.
const COPIED_FOR_MS = 4000;
type Outcome = { text: string; lasts: boolean } | null;

// The slot frames the connection values, the thing a Member came for. Its
// values share one feedback line just under the frame, so the address's and
// the port's never stack; the line keeps its space, so feedback never
// covers the steps below.
function Slot({ values, ready }: { values: Copyable[]; ready: boolean }) {
  const [outcome, setOutcome] = useState<Outcome>(null);
  const texts = useRef<(HTMLSpanElement | null)[]>([]);

  // New values (a new device, or a server that changed) start without old
  // feedback.
  const shown = values.map((v) => v.value ?? "").join(" ");
  useEffect(() => setOutcome(null), [shown]);

  useEffect(() => {
    if (!outcome || outcome.lasts) return;
    const t = window.setTimeout(() => setOutcome(null), COPIED_FOR_MS);
    return () => window.clearTimeout(t);
  }, [outcome]);

  // Copying never claims to reach another device. When the clipboard
  // refuses, the value is selected so it can be copied by hand.
  const copy = async (i: number) => {
    const { value, copied } = values[i];
    if (!value) return;
    setOutcome(null);
    try {
      await navigator.clipboard.writeText(value);
      setOutcome({ text: copied, lasts: false });
    } catch {
      const el = texts.current[i];
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
      <div className="mc-slot">
        {values.map((v, i) => (
          <div key={v.label} className="mc-value">
            <p className="mc-value-label">
              {v.label}
              {v.value && !ready && <span className="mc-value-when">Join after the server is ready</span>}
            </p>
            {v.value ? (
              <div className="mc-value-row">
                {/* Shown in parts that wrap after a dot or colon (a part too
                  long for the line breaks inside it); copied whole, exactly
                  as sent. The parts are only for wrapping, so assistive
                  technology reads the value as one. */}
                <span
                  ref={(el) => {
                    texts.current[i] = el;
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
                  onClick={() => void copy(i)}
                >
                  {v.copy}
                </button>
              </div>
            ) : (
              <p className="mc-value-missing">Connection details unavailable</p>
            )}
          </div>
        ))}
      </div>
      <p className="mc-copy-outcome" role="status">
        {outcome?.text}
      </p>
    </>
  );
}
