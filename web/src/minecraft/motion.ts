// When the Launcher's board moves (DESIGN.md, "Board moves"). The plots have
// their own rules in plot.ts; these are for the board around them: its rows,
// and the custard control that rises when it becomes the thing to press.
// Like the plots, the board moves only when the Member presses something or
// the backend answers, never on arrival, and nothing here loops.

// How long a row takes to settle at its new height. On the house expo
// ease-out it covers four fifths of the way in its first 80ms, so the change
// is felt at once and the rest is the landing.
export const SETTLE_MS = 360;

// A custard control rises once its row has all but settled, so its small
// move is never lost inside the row's larger one.
export const RISE_AFTER_MS = 220;
export const RISE_MS = 380;

const PX_LIST = /^\d*\.?\d+px( \d*\.?\d+px)*$/;

// tracks reads a grid's rows as the browser reports the ones in use
// ("137px 79.2px 82px 155px"), or a box's height ("64px"). Anything else is
// null: "none", "subgrid", a list with a named line.
export function tracks(value: string): number[] | null {
  const v = value.trim();
  return PX_LIST.test(v) ? v.split(" ").map(parseFloat) : null;
}

// settles says whether a change of rows is one to carry the eye through: the
// same rows before and after, with at least one more than half a pixel
// taller or shorter. A grid whose rows can't be read never moves.
export function settles(before: string, after: string): boolean {
  const from = tracks(before);
  const to = tracks(after);
  if (!from || !to || from.length !== to.length) return false;
  return from.some((h, i) => Math.abs(h - to[i]) > 0.5);
}

// mayMove says whether a move would be seen and wanted: never under
// prefers-reduced-motion, in a hidden tab or off screen, where the page
// simply shows its new picture.
export function mayMove(el: Element): boolean {
  if (typeof el.animate !== "function") return false;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return false;
  if (document.visibilityState !== "visible") return false;
  const r = el.getBoundingClientRect();
  return r.bottom > 0 && r.top < window.innerHeight;
}
