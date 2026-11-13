import type { ReactNode } from "react";
import { at, box, line, shape, SIZE, type Faces, type Pt } from "./iso";

// What every plot is drawn with: the Lobby's own materials, never
// Minecraft's (DESIGN.md, "Pocket Worlds"). Three classes carry what the
// stylesheet needs to know: plot-face for a coloured face and plot-dark for
// a solid navy shape, which both drain to card mint when the world's state
// is unconfirmed, and plot-shadow for the ground footprint, which goes.

export const NAVY = "var(--navy)";

// The plots' line: the server block's navy outline at the plots' scale, and
// a finer one for what is drawn on a face.
export const INK = { stroke: NAVY, strokeWidth: 3.25, strokeLinejoin: "round", strokeLinecap: "round" } as const;
export const FINE = { ...INK, strokeWidth: 2.5 } as const;

// A box's three faces, outlined.
export function Solid({ faces, top, left, right }: { faces: Faces; top: string; left: string; right: string }) {
  return (
    <g {...INK}>
      <path className="plot-face" d={faces.top} fill={top} />
      <path className="plot-face" d={faces.left} fill={left} />
      <path className="plot-face" d={faces.right} fill={right} />
    </g>
  );
}

// The ground every plot stands on: a slab six steps square, with its flat
// footprint showing below its front edges. What is drawn on the top face
// (a grid, a trail) goes in as children, between the top and anything
// standing on it.
const THICK = 0.7;
const SLAB = box(0, 0, -THICK, SIZE, SIZE, THICK);
const FOOTPRINT = shape(at(0, 0, -1.35), at(SIZE, 0, -1.35), at(SIZE, SIZE, -1.35), at(0, SIZE, -1.35));

export function Slab({ top, left, right, children }: { top: string; left: string; right: string; children?: ReactNode }) {
  return (
    <>
      <path className="plot-shadow" d={FOOTPRINT} fill="var(--ground-shadow)" />
      <Solid faces={SLAB} top={top} left={left} right={right} />
      {children}
    </>
  );
}

// rays is short strokes fanning out from a light: one per angle in degrees
// (0 points right, -90 up), from `clear` out to `clear + length`.
export function rays([cx, cy]: Pt, angles: readonly number[], clear: number, length: number): string {
  return angles
    .map((deg) => {
      const r = (deg * Math.PI) / 180;
      const [dx, dy] = [Math.cos(r), Math.sin(r)];
      const n = (v: number) => Math.round(v * 100) / 100;
      return `M${n(cx + dx * clear)} ${n(cy + dy * clear)}l${n(dx * length)} ${n(dy * length)}`;
    })
    .join("");
}

// The barrier across the front of a plot whose world is down for
// maintenance: a pale plank with navy stripes on two navy trestles. The same
// one stands on every plot, the block's included.
const PLANK = [0.9, 5.05, 0.95, 4.2, 0.3, 0.62] as const;
const PLANK_FACES = box(...PLANK);
const stripe = (u: number) => {
  const [x, y, z, w, , t] = PLANK;
  return line(at(x + w * u, y + PLANK[4], z + t), at(x + w * (u + 0.09), y + PLANK[4], z));
};
const trestle = (x: number) => {
  const y = PLANK[1] + PLANK[4];
  return line(at(x - 0.32, y, 0), at(x, y, PLANK[2] + 0.1), at(x + 0.32, y, 0));
};

export function Barrier() {
  return (
    <g data-part="barrier">
      <path d={trestle(1.55) + trestle(4.45)} fill="none" {...INK} />
      <Solid faces={PLANK_FACES} top="var(--seafoam-light)" left="var(--seafoam-light)" right="var(--limb)" />
      <path d={[0.14, 0.36, 0.58, 0.8].map(stripe).join("")} fill="none" {...INK} strokeLinecap="butt" />
    </g>
  );
}
