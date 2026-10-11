import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { javaAddress, type Connection, type ServerState } from "./logic";

// The joining help (docs/design/minecraft-brief.md): an explicit, reversible
// device choice, the connection values for it and the steps in Minecraft.
// Choosing a device never touches the server.

type Device = "java" | "bedrock" | "console";

const DEVICES: readonly { id: Device; label: string }[] = [
  { id: "java", label: "Java Edition" },
  { id: "bedrock", label: "Bedrock" },
  { id: "console", label: "Console help" },
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
            aria-selected={device === d.id}
            aria-controls={`panel-${d.id}`}
            tabIndex={device === d.id ? 0 : -1}
            onClick={() => choose(d.id)}
          >
            {d.label}
          </button>
        ))}
      </div>

      <Panel id="java" shown={device}>
        <Value
          label="Server address"
          value={connection.java ? javaAddress(connection.java) : undefined}
          copy="Copy address"
          copied="Address copied."
          ready={ready}
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
        <div className="mc-values">
          <Value
            label="Server address"
            value={connection.bedrock?.address}
            copy="Copy address"
            copied="Address copied."
            ready={ready}
          />
          <Value
            label="Port"
            value={connection.bedrock ? String(connection.bedrock.port) : undefined}
            copy="Copy port"
            copied="Port copied."
            ready={ready}
            short
          />
        </div>
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

      <Panel id="console" shown={device}>
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

function Panel({ id, shown, children }: { id: Device; shown: Device; children: ReactNode }) {
  return (
    <div id={`panel-${id}`} className="mc-panel" role="tabpanel" aria-labelledby={`tab-${id}`} hidden={id !== shown}>
      {children}
    </div>
  );
}

function Steps({ children }: { children: ReactNode }) {
  return <ol className="mc-steps">{children}</ol>;
}

type Outcome = "copied" | "manual" | null;

// One connection value with its own Copy control. Copy feedback is spoken
// without moving focus; when the clipboard refuses, the value is selected so
// it can be copied by hand. Copying never claims to reach another device.
function Value({
  label,
  value,
  copy,
  copied,
  ready,
  short = false,
}: {
  label: string;
  value: string | undefined;
  copy: string;
  copied: string;
  ready: boolean;
  short?: boolean;
}) {
  const [outcome, setOutcome] = useState<Outcome>(null);
  const text = useRef<HTMLSpanElement>(null);

  // A new value (or a new device) starts without old feedback.
  useEffect(() => setOutcome(null), [value]);

  const onCopy = async () => {
    if (!value) return;
    setOutcome(null);
    try {
      await navigator.clipboard.writeText(value);
      setOutcome("copied");
    } catch {
      const el = text.current;
      const sel = window.getSelection();
      if (el && sel) {
        const range = document.createRange();
        range.selectNodeContents(el);
        sel.removeAllRanges();
        sel.addRange(range);
      }
      setOutcome("manual");
    }
  };

  return (
    <div className={short ? "mc-value mc-value-short" : "mc-value"}>
      <p className="mc-value-label">
        {label}
        {value && !ready && <span className="mc-value-when">Join after the server is ready</span>}
      </p>
      {value ? (
        <div className="mc-value-row">
          <span ref={text} className="mc-value-text">
            {value}
          </span>
          <button type="button" className="quiet-button mc-copy" onClick={onCopy}>
            {copy}
          </button>
        </div>
      ) : (
        <p className="mc-value-missing">Connection details unavailable</p>
      )}
      <p className="mc-copy-outcome" role="status">
        {outcome === "copied" && copied}
        {outcome === "manual" && "Copying didn’t work here. It’s selected above: copy it yourself."}
      </p>
    </div>
  );
}
