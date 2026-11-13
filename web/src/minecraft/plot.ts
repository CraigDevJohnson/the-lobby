// What a world's plot shows and when it moves (docs/design/minecraft-brief.md,
// "Pocket Worlds"). The drawing is a function of the state the card's words
// show and of nothing else, so the picture never says more than the words do:
// a plot is lit only while its world is ready, and it moves only when the
// Member presses something or the backend answers.
import type { ServerState } from "./logic";

// lit says whether a plot's one custard light is on.
export const lit = (state: ServerState) => state === "ready";

// The drawings the page has. A world the page has no drawing for gets the
// server block on a plot of its own.
export type Art = "camp" | "build" | "gate" | "block";

const DRAWN: Record<string, Art> = { survival: "camp", creative: "build", adventure: "gate" };

export const artFor = (id: string | undefined): Art => (id !== undefined && Object.hasOwn(DRAWN, id) ? DRAWN[id] : "block");

// A plot's one-shot moves. Each answers something that happened:
//   strike  Start was pressed
//   wake    the world turned ready after starting
//   light   the world turned ready by any other road, so only its light comes on
//   pack    the world began to stop
//   rest    it finished stopping
//   fizzle  the start failed
//   beat    a status answer arrived while the world is on its way somewhere
//   flare   the ready world's address was copied
export type Cue = "strike" | "wake" | "light" | "pack" | "rest" | "fizzle" | "beat" | "flare";

// cueFor is the move for a change of state, or none: a change the page has no
// road for simply cuts to the new picture.
export function cueFor(before: ServerState, after: ServerState): Cue | null {
  if (before === after) return null;
  switch (after) {
    case "starting":
      return before === "stopped" || before === "failed" ? "strike" : null;
    case "ready":
      return before === "starting" ? "wake" : "light";
    case "stopping":
      return before === "ready" ? "pack" : null;
    case "stopped":
      return before === "stopping" ? "rest" : null;
    case "failed":
      return before === "starting" ? "fizzle" : null;
    default:
      return null;
  }
}

// waiting says whether a status answer gets a beat: only while the world is
// on its way up or down, when hearing back is the news.
export const waiting = (state: ServerState) => state === "starting" || state === "stopping";

// A world without a drawing of its own is told apart by its flag: its shape,
// which back corner its pole stands on, and how many drive slots its block
// has. The world's id picks one of the twelve, so it never changes.
export type Variant = { flag: "point" | "swallow" | "square"; pole: "left" | "right"; slots: 2 | 3 };

export const VARIANTS: readonly Variant[] = (["point", "swallow", "square"] as const).flatMap((flag) =>
  (["left", "right"] as const).flatMap((pole) => ([2, 3] as const).map((slots) => ({ flag, pole, slots }))),
);

// hash is FNV-1a over the id's characters.
export function hash(id: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < id.length; i++) {
    h ^= id.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

// variants gives every undrawn world in a list its variant, by id. Each
// starts from its id's own and steps on past any an earlier world in the
// list has taken, so two worlds on one page never share a flag while there
// are flags left.
export function variants(ids: readonly (string | undefined)[]): Map<string, Variant> {
  const out = new Map<string, Variant>();
  const taken = new Set<number>();
  for (const id of ids) {
    if (id === undefined || artFor(id) !== "block" || out.has(id)) continue;
    let n = hash(id) % VARIANTS.length;
    for (let step = 0; step < VARIANTS.length && taken.has(n); step++) n = (n + 1) % VARIANTS.length;
    taken.add(n);
    out.set(id, VARIANTS[n]);
  }
  return out;
}
