// The plots' drawing board: the 2:1 isometric grid the server block is drawn
// on. x runs down and to the right, y down and to the left, z straight up.
// One step along x or y moves U across and half of U down; one step up is H,
// which makes a 1 by 1 by 1 box read as a cube. Every plot stands on a slab
// six steps square whose top is z = 0, so the plots share one scale.

// The view is cropped close to the slab, so a plot fills the space it is given.
export const VIEW = "8 4 112 118";
export const U = 8;
export const H = 9;
export const SIZE = 6;

// Where the slab's back corner, (0, 0, 0), sits in the view.
const OX = 64;
const OY = 60;

export type Pt = readonly [number, number];

const round = (n: number) => Math.round(n * 100) / 100;

// at is where a grid point lands in the view.
export const at = (x: number, y: number, z = 0): Pt => [
  round(OX + (x - y) * U),
  round(OY + ((x + y) * U) / 2 - z * H),
];

const join = (pts: readonly Pt[]) => pts.map(([x, y]) => `${x} ${y}`).join("L");

// shape is a closed outline through the points; line is an open one.
export const shape = (...pts: Pt[]) => `M${join(pts)}z`;
export const line = (...pts: Pt[]) => `M${join(pts)}`;

// A box's three faces in view: the top, the left face (the one looking down
// and to the left, at the box's far y) and the right face (at its far x).
export type Faces = { top: string; left: string; right: string };

// box is the faces of a box whose back-bottom corner is (x, y, z), w long
// along x, l long along y and t tall.
export function box(x: number, y: number, z: number, w: number, l: number, t: number): Faces {
  return {
    top: shape(at(x, y, z + t), at(x + w, y, z + t), at(x + w, y + l, z + t), at(x, y + l, z + t)),
    left: shape(at(x, y + l, z + t), at(x + w, y + l, z + t), at(x + w, y + l, z), at(x, y + l, z)),
    right: shape(at(x + w, y, z + t), at(x + w, y + l, z + t), at(x + w, y + l, z), at(x + w, y, z)),
  };
}

// Points on a box's faces, for what is drawn on them. u runs along the face
// from its outer edge to the box's front corner, and v runs up from its foot.
export const onLeft =
  (x: number, y: number, z: number, w: number, l: number, t: number) =>
  (u: number, v: number): Pt =>
    at(x + w * u, y + l, z + t * v);

export const onRight =
  (x: number, y: number, z: number, w: number, l: number, t: number) =>
  (u: number, v: number): Pt =>
    at(x + w, y + l * (1 - u), z + t * v);

// centre is the middle of some points, for a part that turns or grows about
// its own middle.
export function centre(...pts: Pt[]): Pt {
  const n = pts.length || 1;
  return [round(pts.reduce((s, p) => s + p[0], 0) / n), round(pts.reduce((s, p) => s + p[1], 0) / n)];
}

// origin writes a point as a CSS transform-origin in the view's own units.
export const origin = ([x, y]: Pt) => `${x}px ${y}px`;
