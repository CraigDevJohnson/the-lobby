// The VIP Lobby's arcade cabinet, the parts that are plain arithmetic: which
// tile an arrow key moves to, how the marquee's bulbs are numbered, and the
// selector's glide. No DOM here, so Node's test runner can check it
// (cabinet.test.ts).

export type Box = { left: number; top: number; width: number; height: number };
export type Direction = "left" | "right" | "up" | "down";

export const ARROWS: Readonly<Record<string, Direction>> = {
  ArrowLeft: "left",
  ArrowRight: "right",
  ArrowUp: "up",
  ArrowDown: "down",
};

// The tile an arrow key reaches from `from`, like a joystick in a game menu.
// Left and right stay in the row and stop at its ends. Up and down take the
// nearest tile that way, preferring one that shares some of the column. -1
// when nothing qualifies, so the key keeps its usual job of scrolling the page.
export function neighbour(boxes: readonly Box[], from: number, dir: Direction): number {
  const a = boxes[from];
  if (!a) return -1;
  const sideways = dir === "left" || dir === "right";
  const sign = dir === "left" || dir === "up" ? -1 : 1;
  let best = -1;
  let bestScore = Infinity;
  boxes.forEach((b, i) => {
    if (i === from) return;
    const dx = b.left + b.width / 2 - (a.left + a.width / 2);
    const dy = b.top + b.height / 2 - (a.top + a.height / 2);
    const along = sign * (sideways ? dx : dy);
    if (along <= 1) return;
    const across = Math.abs(sideways ? dy : dx);
    const shares = sideways
      ? b.top < a.top + a.height && a.top < b.top + b.height
      : b.left < a.left + a.width && a.left < b.left + b.width;
    if (sideways && !shares) return;
    const score = (shares ? 0 : 1e6) + along + across * 2;
    if (score < bestScore) {
      bestScore = score;
      best = i;
    }
  });
  return best;
}

// The marquee's bulbs, numbered clockwise from the top-left corner so a chase
// runs around the sign. Rows and sides are listed in the order they appear in
// the markup (left to right, top to bottom); `ring` is the clockwise number.
// Every other bulb is lit at rest.
export type Bulb = { ring: number; lit: boolean };
export type Marquee = { top: Bulb[]; right: Bulb[]; bottom: Bulb[]; left: Bulb[]; count: number };

export function marquee(perRow: number, perSide: number): Marquee {
  const count = 2 * perRow + 2 * perSide;
  const bulb = (ring: number): Bulb => ({ ring, lit: ring % 2 === 0 });
  const range = (n: number) => Array.from({ length: n }, (_, i) => i);
  return {
    top: range(perRow).map((i) => bulb(i)),
    right: range(perSide).map((i) => bulb(perRow + i)),
    // The bottom row and left side run against markup order.
    bottom: range(perRow).map((i) => bulb(perRow + perSide + (perRow - 1 - i))),
    left: range(perSide).map((i) => bulb(2 * perRow + perSide + (perSide - 1 - i))),
    count,
  };
}

// 16 bulbs a row and 2 a side: 36 in all. The chase in styles.css runs in
// CHASE_PHASES phases (its duration is three steps and its keyframes switch at
// 33.333%), and the count must divide by it for the pattern to run seamlessly
// round the corner where it wraps. Change both together.
export const MARQUEE = marquee(16, 2);
export const CHASE_PHASES = 3;

// The selector's glide as a CSS linear() easing: a sampled spring that
// overshoots its target by `settle` pixels at most (and never more than 8% of
// the move), peaking halfway through and settling by the end. A long jump
// across the cabinet settles as gently as a step to the next tile, so the
// corners never swing past a section rule or out of the cabinet.
export function glide(travel: number, settle = 4, samples = 24): string {
  const overshoot = Math.min(0.08, Math.max(0.002, settle / Math.max(travel, 1)));
  const ln = Math.log(overshoot);
  const damping = -ln / Math.sqrt(Math.PI ** 2 + ln ** 2);
  const wd = Math.PI / 0.5; // first peak halfway through
  const w0 = wd / Math.sqrt(1 - damping ** 2);
  const at = (t: number) => 1 - Math.exp(-damping * w0 * t) * (Math.cos(wd * t) + ((damping * w0) / wd) * Math.sin(wd * t));
  const points = Array.from({ length: samples + 1 }, (_, i) => (i === samples ? 1 : Number(at(i / samples).toFixed(4))));
  return `linear(${points.join(", ")})`;
}
