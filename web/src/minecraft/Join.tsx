import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { DeviceGlyph } from "./art";
import { javaAddress, segments, type World } from "./logic";

// The joining help (docs/design/minecraft-brief.md): an explicit, reversible
// device choice, each world's connection values for it, and the steps in
// Minecraft. The Launcher shows the choice across the top of its board, each
// world's values in that world's column, and the steps under the board.
// Choosing a device never touches a server.

export type Device = "java" | "bedrock" | "console";

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

// useDevice holds the device choice, which three places follow: the tabs,
// each world's values and the steps. The remembered choice is read after the
// first paint, so the prerendered page and the browser agree.
export function useDevice() {
  const [device, setDevice] = useState<Device>("java");
  useEffect(() => {
    const d = remembered();
    if (d) setDevice(d);
  }, []);
  const choose = useCallback((d: Device, focus = false) => {
    setDevice(d);
    try {
      window.localStorage.setItem(REMEMBERED, d);
    } catch {
      // Not remembered; the choice still holds on this page.
    }
    if (focus) requestAnimationFrame(() => document.getElementById(`tab-${d}`)?.focus());
  }, []);
  return { device, choose };
}

type Choose = (d: Device, focus?: boolean) => void;

// The device tabs. Arrow keys move along them, as tabs do everywhere else.
export function DeviceTabs({ device, choose }: { device: Device; choose: Choose }) {
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
  return (
    <div className="mc-tabs" role="tablist" aria-label="Where you play Minecraft" onKeyDown={onKey}>
      {DEVICES.map((d) => (
        <button
          key={d.id}
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
  );
}

// The steps for the chosen device and what to check before joining. Each
// world's values are with that world, so the steps name no address.
export function JoinSteps({ worlds, device, choose }: { worlds: readonly World[]; device: Device; choose: Choose }) {
  const named = worlds.some((w) => w.id);
  const anyReady = worlds.some((w) => w.state === "ready");
  const example = (worlds.find((w) => w.state === "ready") ?? worlds[0])?.name ?? "";
  return (
    <>
      <Panel id="java" shown={device}>
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
    </>
  );
}

// One world's connection values for the chosen device, in that world's
// column beside its state and Start or Stop. Its Copy buttons share one feedback line
// under the values. A ready world's Copy address takes the custard face;
// every other Copy stays quiet.
export function WorldValues({
  world,
  device,
  named,
  onCopied,
}: {
  world: World;
  device: Device;
  named: boolean;
  // Called when the ready world's address really reached the clipboard.
  onCopied?: () => void;
}) {
  const ready = world.state === "ready";
  const name = named ? world.name : undefined;
  const values: Copyable[] =
    device === "java"
      ? [
          {
            label: name ? "Java address" : "Server address",
            value: world.connection.java ? javaAddress(world.connection.java) : undefined,
            copy: "Copy address",
            copyFor: name ? `Copy Java address for ${name}` : undefined,
            copied: name ? `${name}’s address copied.` : "Address copied.",
            press: ready,
          },
        ]
      : device === "bedrock"
        ? [
            {
              label: name ? "Bedrock address" : "Server address",
              value: world.connection.bedrock?.address,
              copy: "Copy address",
              copyFor: name ? `Copy Bedrock address for ${name}` : undefined,
              copied: name ? `${name}’s address copied.` : "Address copied.",
              press: ready,
            },
            {
              label: "Port",
              value: world.connection.bedrock ? String(world.connection.bedrock.port) : undefined,
              copy: "Copy port",
              copyFor: name ? `Copy port for ${name}` : undefined,
              copied: name ? `${name}’s port copied.` : "Port copied.",
              press: false,
            },
          ]
        : [];
  const key = `${world.id ?? ""}:${device}:${values.map((v) => v.value ?? "").join(" ")}`;
  const { outcome, copy, texts } = useCopy(key, onCopied);

  if (device === "console")
    return (
      <div className="mc-join-values">
        <p className="mc-value-missing">Joining from a console hasn’t been confirmed yet.</p>
      </div>
    );

  const has = values.some((v) => v.value);
  return (
    <div className="mc-join-values">
      {values.map((v) => {
        const where = `${world.id ?? ""}:${v.label}`;
        return (
          <div key={v.label} className="mc-value">
            <p className="mc-value-label">
              {v.label}
              {has && !ready && v === values[0] && (
                <span className="mc-value-when">Join after it’s ready</span>
              )}
            </p>
            {v.value ? (
              <div className="mc-value-row">
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
      <p className="mc-copy-outcome" role="status">
        {outcome?.text}
      </p>
    </div>
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
  copyFor?: string; // the button's name, saying which world and edition
  copied: string;
  press: boolean;
};

// Copied says so for a few seconds; a copy that had to be done by hand
// keeps its instruction until the next copy.
const COPIED_FOR_MS = 4000;
type Outcome = { text: string; lasts: boolean } | null;

// useCopy copies a value and says how it went. New values (key) start
// without old feedback. Copying never claims to reach another device; when
// the clipboard refuses, the value is selected so it can be copied by hand.
function useCopy(key: string, onCopied?: () => void) {
  const [outcome, setOutcome] = useState<Outcome>(null);
  const texts = useRef(new Map<string, HTMLSpanElement>());
  useEffect(() => setOutcome(null), [key]);
  useEffect(() => {
    if (!outcome || outcome.lasts) return;
    const t = window.setTimeout(() => setOutcome(null), COPIED_FOR_MS);
    return () => window.clearTimeout(t);
  }, [outcome]);
  const copy = async (where: string, { value, copied, press }: Copyable) => {
    if (!value) return;
    setOutcome(null);
    try {
      await navigator.clipboard.writeText(value);
      setOutcome({ text: copied, lasts: false });
      if (press) onCopied?.();
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
  return { outcome, copy, texts };
}
