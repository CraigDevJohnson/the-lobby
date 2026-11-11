import assert from "node:assert/strict";
import { test } from "node:test";
import { CHASE_PHASES, glide, MARQUEE, marquee, neighbour, type Box } from "./cabinet.ts";

const box = (left: number, top: number, width = 200, height = 200): Box => ({ left, top, width, height });

// Desktop, all five Tools: three under Your Tools, then two wider ones under
// Admin Tools.
const desktop = [box(0, 0), box(214, 0), box(428, 0), box(0, 300, 307), box(321, 300, 307)];

// Today's largest real collection: three Your Tools over one full-width
// Admin tile (Foundry Admin is not ready yet).
const threeOverOne = [box(0, 0), box(214, 0), box(428, 0), box(0, 300, 628)];

// A tablet's two columns: Foundry alone on the second row, then both Admin Tools.
const tablet = [box(0, 0, 300), box(314, 0, 300), box(0, 214, 300), box(0, 500, 300), box(314, 500, 300)];

test("left and right stay in the row and stop at its ends", () => {
  assert.equal(neighbour(desktop, 0, "right"), 1);
  assert.equal(neighbour(desktop, 1, "right"), 2);
  assert.equal(neighbour(desktop, 2, "left"), 1);
  assert.equal(neighbour(desktop, 3, "right"), 4);
  assert.equal(neighbour(desktop, 4, "left"), 3);
  // Nothing lies further along a row's ends, so the key scrolls instead of
  // jumping diagonally into another row.
  assert.equal(neighbour(desktop, 2, "right"), -1);
  assert.equal(neighbour(desktop, 0, "left"), -1);
  assert.equal(neighbour(desktop, 3, "left"), -1);
  assert.equal(neighbour(desktop, 4, "right"), -1);
});

test("a full-width Admin tile under three Tools moves only up", () => {
  assert.equal(neighbour(threeOverOne, 3, "left"), -1);
  assert.equal(neighbour(threeOverOne, 3, "right"), -1);
  assert.equal(neighbour(threeOverOne, 3, "down"), -1);
  // It sits under all three; the middle one is nearest.
  assert.equal(neighbour(threeOverOne, 3, "up"), 1);
  for (const i of [0, 1, 2]) assert.equal(neighbour(threeOverOne, i, "down"), 3);
});

test("a tablet's lone second-row tile stays in its row", () => {
  assert.equal(neighbour(tablet, 2, "right"), -1);
  assert.equal(neighbour(tablet, 2, "left"), -1);
  assert.equal(neighbour(tablet, 0, "down"), 2);
  assert.equal(neighbour(tablet, 2, "up"), 0);
  assert.equal(neighbour(tablet, 2, "down"), 3);
  // Nothing shares the right column on the second row, so down from the
  // right column goes on to the Admin tile in that column.
  assert.equal(neighbour(tablet, 1, "down"), 4);
});

test("down moves into Admin Tools under the same column", () => {
  assert.equal(neighbour(desktop, 0, "down"), 3);
  assert.equal(neighbour(desktop, 2, "down"), 4);
  // The middle tile sits over both Admin tiles, centred between them; the
  // first in reading order wins the tie.
  assert.equal(neighbour(desktop, 1, "down"), 3);
  // Off-centre, the nearer one wins.
  const skewed = [box(0, 0), box(300, 0), box(0, 300, 307), box(321, 300, 307)];
  assert.equal(neighbour(skewed, 1, "down"), 3);
  assert.equal(neighbour(desktop, 4, "up"), 2);
  assert.equal(neighbour(desktop, 3, "up"), 0);
  assert.equal(neighbour(desktop, 3, "down"), -1);
});

test("a phone's single column moves up and down only", () => {
  const phone = [box(0, 0, 340, 140), box(0, 154, 340, 140), box(0, 340, 340, 140)];
  assert.equal(neighbour(phone, 0, "down"), 1);
  assert.equal(neighbour(phone, 1, "down"), 2);
  assert.equal(neighbour(phone, 2, "up"), 1);
  assert.equal(neighbour(phone, 1, "left"), -1);
  assert.equal(neighbour(phone, 1, "right"), -1);
});

test("a lone tile, or an unknown one, goes nowhere", () => {
  assert.equal(neighbour([box(0, 0)], 0, "right"), -1);
  assert.equal(neighbour(desktop, 9, "down"), -1);
});

test("the marquee numbers every bulb once, clockwise", () => {
  const m = marquee(4, 2);
  // Top left to right, right side down, bottom right to left, left side up.
  assert.deepEqual(
    m.top.map((b) => b.ring),
    [0, 1, 2, 3],
  );
  assert.deepEqual(
    m.right.map((b) => b.ring),
    [4, 5],
  );
  assert.deepEqual(
    m.bottom.map((b) => b.ring),
    [9, 8, 7, 6],
  );
  assert.deepEqual(
    m.left.map((b) => b.ring),
    [11, 10],
  );
  const all = [...m.top, ...m.right, ...m.bottom, ...m.left].map((b) => b.ring).sort((x, y) => x - y);
  assert.deepEqual(
    all,
    Array.from({ length: m.count }, (_, i) => i),
  );
});

test("the lobby's marquee chases seamlessly and rests with every other bulb lit", () => {
  assert.equal(MARQUEE.count % CHASE_PHASES, 0);
  const bulbs = [...MARQUEE.top, ...MARQUEE.right, ...MARQUEE.bottom, ...MARQUEE.left];
  assert.equal(bulbs.filter((b) => b.lit).length, MARQUEE.count / 2);
  for (const b of bulbs) assert.equal(b.lit, b.ring % 2 === 0);
});

const points = (easing: string) => {
  const m = /^linear\((.*)\)$/.exec(easing);
  assert.ok(m, `not a linear() easing: ${easing}`);
  return m[1].split(",").map(Number);
};

test("the selector's glide overshoots by at most its settle, whatever the distance", () => {
  for (const travel of [14, 120, 300, 700, 1200]) {
    const p = points(glide(travel, 4));
    assert.equal(p[0], 0);
    assert.equal(p.at(-1), 1);
    const peak = Math.max(...p);
    const allowed = Math.min(0.08, 4 / travel);
    // The overshoot in pixels stays within the settle (4px), or 8% for a tiny move.
    assert.ok((peak - 1) * travel <= Math.max(4, 0.08 * travel) + 0.05, `travel ${travel}: peak ${peak}`);
    assert.ok(peak - 1 <= allowed + 0.001, `travel ${travel}: overshoot ${peak - 1} above ${allowed}`);
    // It rises without wobbling until it passes the target.
    const first = p.findIndex((v) => v >= 1);
    for (let i = 1; i <= first; i++) assert.ok(p[i] >= p[i - 1], `travel ${travel}: dips at sample ${i}`);
  }
});

test("a long glide is close to settled well before it ends", () => {
  const p = points(glide(700, 4));
  // Three quarters of the way through, within a pixel of the target.
  assert.ok(Math.abs(p[18] - 1) * 700 < 1, `off by ${(p[18] - 1) * 700}px`);
});
