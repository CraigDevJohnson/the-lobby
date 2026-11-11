import { useEffect, useRef, type RefObject } from "react";
import { ARROWS, glide, neighbour, type Box } from "./cabinet";

// Brings The VIP Lobby's arcade cabinet to life. Picking a tile (pointer,
// keyboard focus or an arrow key) closes the selector around it, plays its
// glyph's move, runs the marquee and leans the joystick. Pressing Open flashes
// the marquee, presses the deck's button and makes the host jump; a moment
// later the tile grows into a launch screen while the browser loads the Tool.
//
// The links work exactly as they would without any of this. Event handlers
// only take notes and set timers; the drawing waits for the next animation
// frame, so a tap's navigation starts before any of it runs. Under
// prefers-reduced-motion nothing moves.

const GAP = 8; // px from a tile's edge to the outside of the selector's corners
const SPREAD = 14; // how far out the corners start as the selector appears
const SETTLE = 4; // px the selector may overshoot its tile, however far it travels
const LEAD_MS = 50; // trailing corners follow the leading ones by up to this
const PICK_CHASE_MS = 1300;
const ARRIVAL_CHASE_MS = 2400;
const GREET_AFTER_MS = 12000; // the host greets a pick again after this long without one
const STICK_HOLD_MS = 140; // a little past the stick's 130ms push transition in styles.css
const LAUNCH_AFTER_MS = 90; // lets the press read before the launch screen covers it
const LAUNCH_GIVE_UP_MS = 8000; // no navigation by then: hand the lobby back
const UNPRESS_MS = 220; // a press that never became a click lets the deck's button up
const TILE_ROUND = 14; // the tile's outer corner radius (0.875rem)
const TILE_LINE = 2; // the tile's navy outline
const EXPO_OUT = "cubic-bezier(0.16, 1, 0.3, 1)";

type Frame = { x0: number; y0: number; x1: number; y1: number };
type Cabinet = { arrive(): void; destroy(): void };
type Point = { x: number; y: number };

const boxOf = (el: Element): Box => {
  const r = el.getBoundingClientRect();
  return { left: r.left, top: r.top, width: r.width, height: r.height };
};

const centre = (b: Box): Point => ({ x: b.left + b.width / 2, y: b.top + b.height / 2 });

const cardOf = (t: EventTarget | null) => (t instanceof Element ? t.closest<HTMLElement>(".tool-card") : null);

const plain = (e: MouseEvent | KeyboardEvent) => !(e.altKey || e.ctrlKey || e.metaKey || e.shiftKey);

// The cabinet's own Web Animations, one per element, so checking whether one
// is still running never makes the browser recalculate styles.
const moving = new WeakMap<Element, Animation>();

// A one-shot move. Normally a part already moving finishes first, so sweeping
// back and forth across tiles never stutters a move; `restart` cuts in instead.
function play(el: Element | null | undefined, keyframes: Keyframe[], options: KeyframeAnimationOptions, restart = false) {
  if (!el || typeof el.animate !== "function") return;
  const running = moving.get(el);
  if (running?.playState === "running") {
    if (!restart) return;
    running.cancel();
  }
  moving.set(el, el.animate(keyframes, options));
}

// Each glyph's own move, drawn from what the Tool does: the calendar flips to
// its next page, the block hops and turns, the F stamps like a press, and an
// admin gear turns two teeth.
const MOVES: Record<string, (svg: Element) => void> = {
  calendar(svg) {
    play(
      svg.querySelector(".glyph-rings"),
      [{ transform: "none" }, { transform: "translateY(-2px)", offset: 0.3 }, { transform: "none", offset: 0.6 }, { transform: "none" }],
      { duration: 520, easing: "cubic-bezier(0.3, 0.7, 0.4, 1)" },
    );
    // The page lifts over the binding until it is edge-on, and the next page
    // drops into place and settles.
    play(
      svg.querySelector(".glyph-page"),
      [
        { transform: "scaleY(1)", easing: "cubic-bezier(0.55, 0, 0.9, 0.45)" },
        { transform: "scaleY(0.03)", offset: 0.4, easing: "cubic-bezier(0.15, 0.6, 0.3, 1)" },
        { transform: "scaleY(1.07)", offset: 0.78, easing: "ease-in-out" },
        { transform: "scaleY(1)" },
      ],
      { duration: 520 },
    );
    svg.querySelectorAll(".glyph-dot").forEach((dot, i) =>
      play(dot, [{ transform: "scale(1)" }, { transform: "scale(1.3)", offset: 0.45 }, { transform: "scale(1)" }], {
        duration: 260,
        delay: 300 + i * 40,
        easing: "ease-out",
      }),
    );
  },
  cube(svg) {
    play(
      svg.querySelector(".glyph-body"),
      // A squat, a hop that turns the block around (it is symmetric, so the
      // mirrored block is the same block), and a squashed landing. The
      // edge-on moment falls inside the fast take-off, never at a held pose.
      [
        { transform: "translateY(0) scale(1, 1)", easing: "ease-out" },
        { transform: "translateY(0.5px) scale(1.08, 0.88)", offset: 0.12, easing: "cubic-bezier(0.3, 0, 0.2, 1)" },
        { transform: "translateY(-5px) scale(-0.6, 1.06)", offset: 0.4, easing: "cubic-bezier(0.4, 0, 0.6, 1)" },
        { transform: "translateY(-1px) scale(-1, 1)", offset: 0.6, easing: "ease-in" },
        { transform: "translateY(0) scale(-1.1, 0.88)", offset: 0.72, easing: "ease-out" },
        { transform: "translateY(0) scale(-0.97, 1.03)", offset: 0.86 },
        { transform: "translateY(0) scale(-1, 1)" },
      ],
      { duration: 720 },
    );
  },
  foundry(svg) {
    play(
      svg.querySelector(".glyph-body"),
      [
        { transform: "translateY(0) scale(1)", easing: "cubic-bezier(0.2, 0.8, 0.3, 1)" },
        { transform: "translateY(-3px) scale(1.14)", offset: 0.38, easing: "cubic-bezier(0.7, 0, 1, 0.6)" },
        { transform: "translateY(0) scale(1.12, 0.84)", offset: 0.56, easing: "cubic-bezier(0.2, 0.8, 0.3, 1)" },
        { transform: "translateY(0) scale(0.96, 1.04)", offset: 0.76 },
        { transform: "translateY(0) scale(1)" },
      ],
      { duration: 560 },
    );
  },
  gear(svg) {
    play(svg.querySelector(".glyph-gear"), [{ transform: "rotate(0deg)" }, { transform: "rotate(90deg)" }], {
      duration: 760,
      easing: "cubic-bezier(0.2, 0.9, 0.3, 1.25)",
    });
  },
};
MOVES["cube-admin"] = MOVES.gear;
MOVES["foundry-admin"] = MOVES.gear;

// The corners in the selector, with which way each one faces.
const CORNERS: [string, number, number][] = [
  ["tl", -1, -1],
  ["tr", 1, -1],
  ["bl", -1, 1],
  ["br", 1, 1],
];

function setup(root: HTMLElement): Cabinet {
  const screen = root.querySelector<HTMLElement>(".lobby-collection");
  const marquee = root.querySelector<HTMLElement>(".lobby-marquee");
  const deck = root.querySelector<HTMLElement>(".cabinet-deck");
  if (!screen || !marquee || !deck) return { arrive() {}, destroy() {} };

  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)");
  const still = () => reduced.matches;
  const springs = CSS.supports("transition-timing-function", "linear(0, 1)");
  const cards = () => Array.from(screen.querySelectorAll<HTMLElement>(".tool-card"));
  const selector = () => screen.querySelector<HTMLElement>(".cabinet-selector");
  const overlay = () => root.querySelector<HTMLElement>(".cabinet-launch");
  const robot = () => root.querySelector(".lobby-robot");

  let current: HTMLElement | null = null; // the picked tile
  let hovered: HTMLElement | null = null;
  let focused: HTMLElement | null = null;
  let shown: Frame | null = null; // where the selector's corners are heading
  // Which hand is driving. A page scrolled by the keyboard moves tiles under a
  // resting pointer, and that must not steal the cursor from the focused tile.
  let lastInput: "pointer" | "keyboard" = "pointer";
  let pointerDownAt = -Infinity;
  let pressed = false;
  let launching: HTMLElement | null = null; // the tile being opened
  let screenUp = false; // the launch screen is showing
  let returnFocus: HTMLElement | null = null;
  let quieted: HTMLElement[] = []; // the parts made inert under the launch screen
  let wanted: HTMLElement | null | undefined; // the next frame's pick (null releases)
  let pressWanted = false;
  let frameRequest = 0;
  let marqueeTimer = 0;
  let stickTimer = 0;
  let unpressTimer = 0;
  let launchTimer = 0;
  let giveUpTimer = 0;

  // ---------- Deferring the drawing to the next frame ----------

  function flush() {
    frameRequest = 0;
    if (wanted !== undefined) {
      const target = wanted;
      wanted = undefined;
      if (target?.isConnected) select(target);
      else release();
    }
    if (pressWanted) {
      pressWanted = false;
      press();
    }
  }

  function soon() {
    if (!frameRequest) frameRequest = requestAnimationFrame(flush);
  }

  function want(card: HTMLElement | null) {
    wanted = card;
    soon();
  }

  function wantPress() {
    pressWanted = true;
    soon();
  }

  // ---------- The selector ----------

  // Measured from the cabinet's padding box, where the selector is laid out.
  function frame(card: HTMLElement, spread = 0): Frame {
    const s = screen!.getBoundingClientRect();
    const ox = s.left + screen!.clientLeft;
    const oy = s.top + screen!.clientTop;
    const r = card.getBoundingClientRect();
    const g = GAP + spread;
    return { x0: r.left - ox - g, y0: r.top - oy - g, x1: r.right - ox + g, y1: r.bottom - oy + g };
  }

  function setFrame(el: HTMLElement, f: Frame) {
    for (const [k, v] of Object.entries(f)) el.style.setProperty(`--${k}`, `${v.toFixed(2)}px`);
  }

  // The corners on the side the selector travels toward set off first and the
  // others follow, so it stretches toward the new tile and settles onto it.
  function setLead(el: HTMLElement, from: Frame | null, to: Frame) {
    const dx = from ? (to.x0 + to.x1 - from.x0 - from.x1) / 2 : 0;
    const dy = from ? (to.y0 + to.y1 - from.y0 - from.y1) / 2 : 0;
    const len = Math.hypot(dx, dy);
    for (const [name, sx, sy] of CORNERS) {
      const ahead = len ? (sx * dx + sy * dy) / (len * Math.SQRT2) : 1; // -1 trailing … 1 leading
      el.style.setProperty(`--lead-${name}`, `${Math.round(((1 - ahead) / 2) * LEAD_MS)}ms`);
    }
  }

  // Where each corner box sits now, mid-glide or not, in the padding box.
  function cornersNow(el: HTMLElement) {
    const s = screen!.getBoundingClientRect();
    const ox = s.left + screen!.clientLeft;
    const oy = s.top + screen!.clientTop;
    const at = new Map<string, Point & { size: number }>();
    for (const [name] of CORNERS) {
      const r = el.querySelector(`.cabinet-corner-${name}`)?.getBoundingClientRect();
      if (r) at.set(name, { x: r.left - ox, y: r.top - oy, size: r.width });
    }
    return at;
  }

  // Each corner's easing settles the same few pixels however far that corner
  // actually has to go, including a move that cuts into the last one.
  function setGlides(el: HTMLElement, from: Map<string, Point & { size: number }>, to: Frame) {
    if (!springs) return;
    for (const [name, sx, sy] of CORNERS) {
      const now = from.get(name);
      if (!now) continue;
      const x = sx < 0 ? to.x0 : to.x1 - now.size;
      const y = sy < 0 ? to.y0 : to.y1 - now.size;
      el.style.setProperty(`--glide-${name}`, glide(Math.hypot(x - now.x, y - now.y), SETTLE));
    }
  }

  // Measuring happens before writing, so a move costs one layout (an
  // appearance one more, to set its starting point).
  function place(card: HTMLElement, how: "move" | "appear" | "snap") {
    const el = selector();
    if (!el) return;
    if (!card.isConnected) {
      release();
      return;
    }
    const to = frame(card);
    if (how !== "move") {
      // Set the starting point without a transition, then let it go.
      el.dataset.still = "";
      setFrame(el, how === "appear" ? frame(card, SPREAD) : to);
      void el.offsetWidth;
      delete el.dataset.still;
    }
    const from = cornersNow(el);
    setLead(el, how === "move" ? shown : null, to);
    setGlides(el, from, to);
    el.dataset.on = "";
    setFrame(el, to);
    shown = to;
  }

  function release() {
    const was = current;
    current = null;
    if (!was) return;
    was.removeAttribute("data-picked");
    const el = selector();
    if (!el) return;
    if (was.isConnected) {
      const out = frame(was, SPREAD / 2);
      setGlides(el, cornersNow(el), out);
      setFrame(el, out);
    }
    delete el.dataset.on;
    shown = null;
  }

  // The collection changed (an access re-check added or removed a Tool, or
  // the check failed): forget tiles that are gone, follow the picked one to
  // where it went, and if the Tool being opened is gone, hand the lobby back.
  function relayout() {
    if (launching && !launching.isConnected) reset();
    if (hovered && !hovered.isConnected) hovered = null;
    if (focused && !focused.isConnected) focused = null;
    if (current?.isConnected) place(current, "move");
    else if (current) release();
  }

  // ---------- The marquee, joystick and host ----------

  function chase(ms: number) {
    if (still()) return;
    marquee!.dataset.marquee = "chase";
    window.clearTimeout(marqueeTimer);
    marqueeTimer = window.setTimeout(() => {
      if (marquee!.dataset.marquee === "chase") delete marquee!.dataset.marquee;
    }, ms);
  }

  // The joystick leans from `from` toward `to` (screen points), then springs
  // back upright: left and right tilt it, up and down push it away or pull it
  // back. Its properties live on the deck, so only the deck restyles.
  function nudge(from: Point, to: Point) {
    const dx = to.x - from.x;
    const dy = to.y - from.y;
    const len = Math.hypot(dx, dy);
    if (len < 1) return;
    deck!.style.setProperty("--stick-tilt", `${((dx / len) * 22).toFixed(1)}deg`);
    deck!.style.setProperty("--stick-reach", `${(1 - (dy / len) * 0.18).toFixed(3)}`);
    deck!.style.setProperty("--stick-lift", `${((dy / len) * 4).toFixed(2)}px`);
    deck!.dataset.stick = "pushed";
    window.clearTimeout(stickTimer);
    stickTimer = window.setTimeout(() => {
      delete deck!.dataset.stick;
      deck!.style.removeProperty("--stick-tilt");
      deck!.style.removeProperty("--stick-reach");
      deck!.style.removeProperty("--stick-lift");
    }, STICK_HOLD_MS);
  }

  // The host greets a pick with a hop and a wave, at most once in a while.
  let lastPick = -Infinity;
  function greet() {
    const now = performance.now();
    const due = now - lastPick > GREET_AFTER_MS;
    lastPick = now;
    if (!due) return;
    play(
      robot(),
      [
        { transform: "translateY(0)" },
        { transform: "translateY(-5px)", offset: 0.3, easing: "ease-in" },
        { transform: "translateY(0)", offset: 0.55 },
        { transform: "translateY(-1.5px)", offset: 0.72, easing: "ease-in" },
        { transform: "translateY(0)", offset: 0.86 },
        { transform: "translateY(0)" },
      ],
      { duration: 620, easing: "ease-out" },
    );
    play(
      robot()?.querySelector(".robot-arm"),
      [
        { transform: "rotate(0deg)" },
        { transform: "rotate(-14deg)", offset: 0.22 },
        { transform: "rotate(6deg)", offset: 0.44 },
        { transform: "rotate(-14deg)", offset: 0.66 },
        { transform: "rotate(0deg)" },
      ],
      { duration: 1200, delay: 120, easing: "ease-in-out" },
    );
  }

  // ---------- Picking ----------

  function select(card: HTMLElement) {
    if (card === current) return;
    if (!card.isConnected) {
      if (hovered === card) hovered = null;
      if (focused === card) focused = null;
      return;
    }
    const prev = current?.isConnected ? current : null;
    const from = centre(boxOf(prev ?? screen!));
    const to = centre(boxOf(card));
    place(card, prev ? "move" : "appear");
    current?.removeAttribute("data-picked");
    card.setAttribute("data-picked", "");
    current = card;
    if (still()) return;
    const glyph = card.querySelector(".tool-glyph");
    const move = glyph && MOVES[glyph.getAttribute("data-glyph") ?? ""];
    if (glyph && move) move(glyph);
    chase(PICK_CHASE_MS);
    // A first pick leans the joystick from the middle of the cabinet.
    nudge(from, to);
    greet();
  }

  // ---------- Pressing Open and launching ----------

  // The press itself, shown before anything covers it: the marquee flashes,
  // the deck's first button sinks and lights, and the host jumps. Two
  // keyframe names let a second press restart the flash without a layout.
  function press() {
    if (pressed || still()) return;
    pressed = true;
    window.clearTimeout(unpressTimer);
    window.clearTimeout(marqueeTimer);
    marquee!.dataset.marquee = marquee!.dataset.marquee === "flash" ? "flash-again" : "flash";
    deck!.dataset.button = "pressed";
    play(
      robot(),
      [
        { transform: "none", easing: "ease-out" },
        { transform: "translateY(1px) scale(1.06, 0.92)", offset: 0.15, easing: "cubic-bezier(0.2, 0.8, 0.3, 1)" },
        { transform: "translateY(-9px) scale(0.96, 1.05)", offset: 0.45, easing: "cubic-bezier(0.6, 0, 0.9, 0.5)" },
        { transform: "translateY(0) scale(1.05, 0.94)", offset: 0.75, easing: "ease-out" },
        { transform: "none" },
      ],
      { duration: 560 },
      true,
    );
    play(
      robot()?.querySelector(".robot-arm"),
      [{ transform: "rotate(0deg)" }, { transform: "rotate(-34deg)", offset: 0.35 }, { transform: "rotate(-26deg)", offset: 0.7 }, { transform: "rotate(0deg)" }],
      { duration: 640, easing: "ease-in-out" },
      true,
    );
  }

  // A press that never became a click (dragged away) lets the button up.
  function unpress() {
    if (launching) return;
    pressed = false;
    delete deck!.dataset.button;
  }

  function sceneFor(card: HTMLElement) {
    const scenes = Array.from(overlay()?.querySelectorAll<HTMLElement>(".cabinet-launch-scene") ?? []);
    const scene = scenes.find((s) => s.dataset.tool === card.dataset.tool);
    if (scene) for (const s of scenes) s.hidden = s !== scene;
    return scene;
  }

  // The scene's name wraps where the tile's does: it shares the tile name's
  // lettering, so the same measure in ems gives the same lines.
  function fitName(card: HTMLElement, scene: HTMLElement) {
    const h3 = card.querySelector("h3");
    const name = scene.querySelector<HTMLElement>(".cabinet-launch-name");
    if (!h3 || !name) return;
    const size = parseFloat(getComputedStyle(h3).fontSize);
    name.style.maxWidth = `${(h3.getBoundingClientRect().width / size + 0.05).toFixed(3)}em`;
  }

  // Show the opened Tool's scene, its name fitted to the tile's. The callers
  // hand it the focus, so assistive technology hears "Opening" and the Tool's
  // name rather than an empty page.
  function showScene(card: HTMLElement) {
    const scene = sceneFor(card);
    if (!scene) return null;
    fitName(card, scene);
    return scene;
  }

  // The launch screen grows out of the tile: a navy edge and a field, each
  // clipped to the tile's rounded box, open out until they pass the screen's
  // edges, so the growing shape keeps the tile's 2px outline and 14px corners
  // on every frame. The field starts in the tile's own fill and settles to
  // seafoam. The Tool's glyph and name each grow out of the tile's own and
  // stay readable throughout. The rest of the lobby goes inert underneath,
  // so neither the pointer nor the keyboard reaches what the screen hides.
  function launch() {
    const card = launching;
    const screenEl = overlay();
    const edge = screenEl?.querySelector<HTMLElement>(".cabinet-launch-edge");
    const field = screenEl?.querySelector<HTMLElement>(".cabinet-launch-field");
    const tileGlyph = card?.querySelector(".tool-glyph");
    const tileName = card?.querySelector("h3");
    if (!card?.isConnected || !screenEl || !edge || !field || !tileGlyph || !tileName) return;

    // Read the tile before anything changes.
    const tile = card.getBoundingClientRect();
    const glyphBox = boxOf(tileGlyph);
    const nameBox = boxOf(tileName);
    const nameSize = parseFloat(getComputedStyle(tileName).fontSize);
    const fill = getComputedStyle(card).backgroundColor;

    const scene = showScene(card);
    const sceneGlyph = scene?.querySelector(".tool-glyph");
    const sceneName = scene?.querySelector(".cabinet-launch-name");
    if (!scene || !sceneGlyph || !sceneName) return;
    returnFocus = document.activeElement instanceof HTMLElement && document.activeElement !== document.body ? document.activeElement : null;
    quieted = Array.from(root.children).filter((el): el is HTMLElement => el instanceof HTMLElement && el !== screenEl);
    for (const el of quieted) el.inert = true;
    screenEl.hidden = false;
    screenUp = true;
    scene.focus({ preventScroll: true });

    const o = screenEl.getBoundingClientRect();
    const clip = (inset: number, round: number) =>
      `inset(${(tile.top - o.top + inset).toFixed(1)}px ${(o.right - tile.right + inset).toFixed(1)}px ${(o.bottom - tile.bottom + inset).toFixed(1)}px ${(tile.left - o.left + inset).toFixed(1)}px round ${round}px)`;
    const past = (inset: number, round: number) => `inset(${inset}px round ${round}px)`;
    const grow = { duration: 560, easing: EXPO_OUT, fill: "both" } as const;
    const seafoam = getComputedStyle(field).backgroundColor;
    edge.animate([{ clipPath: clip(0, TILE_ROUND) }, { clipPath: past(-(TILE_LINE + 14), TILE_ROUND) }], grow);
    field.animate(
      [
        { clipPath: clip(TILE_LINE, TILE_ROUND - TILE_LINE), backgroundColor: fill },
        { clipPath: past(-14, TILE_ROUND - TILE_LINE), backgroundColor: seafoam },
      ],
      grow,
    );

    // Each part starts where its counterpart in the tile is, at its size.
    const growFrom = (part: Element, box: Box, scale: number) => {
      const a = centre(box);
      const b = centre(boxOf(part));
      part.animate([{ transform: `translate(${(a.x - b.x).toFixed(1)}px, ${(a.y - b.y).toFixed(1)}px) scale(${scale.toFixed(3)})` }, { transform: "none" }], grow);
    };
    growFrom(sceneGlyph, glyphBox, glyphBox.width / boxOf(sceneGlyph).width);
    growFrom(sceneName, nameBox, nameSize / parseFloat(getComputedStyle(sceneName).fontSize));

    window.clearTimeout(giveUpTimer);
    giveUpTimer = window.setTimeout(reset, LAUNCH_GIVE_UP_MS);
  }

  // Back to the resting cabinet: after a launch that never navigated, on
  // Escape, when the Tool being opened disappears, or when the browser
  // restores this page from its back/forward cache. Focus goes back where it
  // was, scrolled into view if the lobby moved under the screen.
  function reset() {
    for (const t of [launchTimer, giveUpTimer, unpressTimer]) window.clearTimeout(t);
    launching = null;
    screenUp = false;
    pressed = false;
    pressWanted = false;
    const screenEl = overlay();
    if (screenEl) {
      screenEl.getAnimations({ subtree: true }).forEach((a) => a.cancel());
      screenEl.hidden = true;
    }
    for (const el of quieted) el.inert = false;
    quieted = [];
    delete marquee!.dataset.marquee;
    delete deck!.dataset.button;
    if (returnFocus?.isConnected) returnFocus.focus();
    returnFocus = null;
  }

  // ---------- Listeners ----------

  function onPointerOver(e: PointerEvent) {
    if (e.pointerType === "touch") return;
    const card = cardOf(e.target);
    if (!card) return;
    hovered = card;
    if (lastInput === "pointer") want(card);
  }

  // The pointer really moved: hover leads again, starting with the tile under
  // it (a whole tile is its Open link's hit area, so moving within one fires
  // no pointerover).
  function onPointerMove(e: PointerEvent) {
    if (e.pointerType === "touch" || (!e.movementX && !e.movementY)) return;
    lastInput = "pointer";
    const card = cardOf(e.target);
    if (card && card !== current) {
      hovered = card;
      want(card);
    }
  }

  function onPointerLeave(e: PointerEvent) {
    if (e.pointerType === "touch") return;
    hovered = null;
    want(focused?.isConnected ? focused : null);
  }

  // Mouse and pen press at once; a touch waits for its tap, so starting a
  // scroll on a tile never sets anything off.
  function onPointerDown(e: PointerEvent) {
    lastInput = "pointer";
    pointerDownAt = performance.now();
    if (e.pointerType === "touch" || e.button !== 0 || !plain(e)) return;
    const link = e.target instanceof Element ? e.target.closest(".tool-card a[href]") : null;
    if (link) wantPress();
  }

  function onPointerUp() {
    if (!pressed && !pressWanted) return;
    if (launching) return;
    window.clearTimeout(unpressTimer);
    unpressTimer = window.setTimeout(unpress, UNPRESS_MS);
  }

  // Focus that no pointer press just caused (a key, assistive technology, a
  // script) puts the keyboard in charge, so a scroll it causes under a
  // resting pointer cannot take the cursor away. A tap focuses at click time,
  // hence the half-second window.
  function onFocusIn(e: FocusEvent) {
    const card = cardOf(e.target);
    if (!card) return;
    if (performance.now() - pointerDownAt > 500) lastInput = "keyboard";
    focused = card;
    want(card);
  }

  function onFocusOut(e: FocusEvent) {
    if (cardOf(e.relatedTarget)) return;
    focused = null;
    want(lastInput === "pointer" && hovered?.isConnected ? hovered : null);
  }

  // Arrow keys move between tiles like a joystick. Tab still visits every
  // tile in order; an arrow with nothing that way keeps scrolling the page.
  // Enter on a tile's Open presses it before the browser follows the link.
  function onKeyDown(e: KeyboardEvent) {
    const card = cardOf(e.target);
    if (!card || !plain(e)) return;
    if (e.key === "Enter") {
      wantPress();
      return;
    }
    const dir = ARROWS[e.key];
    if (!dir) return;
    const all = cards();
    const to = neighbour(all.map(boxOf), all.indexOf(card), dir);
    if (to < 0) return;
    e.preventDefault();
    all[to].querySelector<HTMLElement>("a[href]")?.focus();
  }

  // Any key puts the keyboard in charge; Escape also stops a page load in most
  // browsers, so it hands the lobby back.
  function onDocumentKeyDown(e: KeyboardEvent) {
    lastInput = "keyboard";
    if (e.key === "Escape" && launching) reset();
  }

  function onWheel() {
    lastInput = "pointer";
  }

  // The link navigates as usual; the launch screen only decorates the wait.
  // A click that opens a new tab or window is left alone. Opening another
  // Tool while the screen is up only changes which one it names.
  function onClick(e: MouseEvent) {
    if (e.defaultPrevented || e.button !== 0 || !plain(e)) return;
    const link = e.target instanceof Element ? e.target.closest(".tool-card a[href]") : null;
    const card = cardOf(link);
    if (!card || still()) return;
    launching = card;
    window.clearTimeout(unpressTimer);
    want(card);
    wantPress();
    if (screenUp) {
      showScene(card)?.focus({ preventScroll: true });
      window.clearTimeout(giveUpTimer);
      giveUpTimer = window.setTimeout(reset, LAUNCH_GIVE_UP_MS);
      return;
    }
    window.clearTimeout(launchTimer);
    launchTimer = window.setTimeout(launch, LAUNCH_AFTER_MS);
  }

  // The tiles rise into place as they arrive; once one lands, put the selector
  // where the tile actually ended up.
  function onAnimationEnd(e: AnimationEvent) {
    if (current && e.target === current) place(current, "move");
  }

  function onPageShow(e: PageTransitionEvent) {
    if (e.persisted) reset();
  }

  // While the cabinet runs, the picked tile alone takes the limb fill (a tile
  // merely under a resting pointer does not); without it, CSS hover does.
  root.dataset.live = "";

  const resized = new ResizeObserver(() => {
    if (current) place(current, "snap");
  });
  resized.observe(screen);
  const changed = new MutationObserver(relayout);
  changed.observe(screen, { childList: true, subtree: true });

  const on: [EventTarget, string, EventListener, AddEventListenerOptions?][] = [
    [screen, "pointerover", onPointerOver as EventListener],
    [screen, "pointermove", onPointerMove as EventListener, { passive: true }],
    [screen, "pointerleave", onPointerLeave as EventListener],
    [screen, "pointerdown", onPointerDown as EventListener],
    [window, "pointerup", onPointerUp],
    [window, "pointercancel", onPointerUp],
    [window, "wheel", onWheel, { passive: true }],
    [screen, "focusin", onFocusIn as EventListener],
    [screen, "focusout", onFocusOut as EventListener],
    [screen, "keydown", onKeyDown as EventListener],
    [document, "keydown", onDocumentKeyDown as EventListener],
    [screen, "click", onClick as EventListener],
    [screen, "animationend", onAnimationEnd as EventListener],
    [window, "pageshow", onPageShow as EventListener],
  ];
  for (const [target, type, fn, options] of on) target.addEventListener(type, fn, options);

  return {
    arrive() {
      chase(ARRIVAL_CHASE_MS);
    },
    destroy() {
      resized.disconnect();
      changed.disconnect();
      for (const [target, type, fn, options] of on) target.removeEventListener(type, fn, options);
      cancelAnimationFrame(frameRequest);
      for (const t of [marqueeTimer, stickTimer, unpressTimer, launchTimer, giveUpTimer]) window.clearTimeout(t);
      for (const el of quieted) el.inert = false;
      delete root.dataset.live;
    },
  };
}

// `ready` is true once the Member's Tools are on screen; the cabinet powers up
// with a marquee chase each time they arrive.
export function useCabinet(root: RefObject<HTMLElement | null>, ready: boolean) {
  const cabinet = useRef<Cabinet | null>(null);

  useEffect(() => {
    if (!root.current) return;
    const c = setup(root.current);
    cabinet.current = c;
    return () => {
      c.destroy();
      cabinet.current = null;
    };
  }, [root]);

  useEffect(() => {
    if (ready) cabinet.current?.arrive();
  }, [ready]);
}
