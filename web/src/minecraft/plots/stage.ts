import type { ReactNode } from "react";
import type { ServerState } from "../logic";
import type { Cue, Variant } from "../plot";

// What a plot is: a drawing of each state, and the one-shot moves between
// them. The drawing is the whole truth; a move only carries the eye from one
// picture to the next, so with every move switched off the plot is still right.

// What a drawing is given. Only the block reads the variant and the initial.
export type ArtProps = { state: ServerState; variant?: Variant; initial?: string };

export type Options = KeyframeAnimationOptions & {
  // With several parts of one name, each starts this long after the last.
  stagger?: number;
};

// The stage a move plays on: its plot's named parts (data-part in the
// drawing). A part holds its move's first frame until its delay is up, and
// rests in its drawn pose once the move ends.
export type Stage = {
  play(part: string, keyframes: Keyframe[], options?: Options): void;
};

export type Move = (stage: Stage) => void;

export type Plot = {
  Art: (props: ArtProps) => ReactNode;
  moves: Partial<Record<Cue, Move>>;
};

// The house expo ease-out, and a spring with about 13% overshoot: the
// cabinet's sampled spring where linear() is supported, a cubic-bezier
// stand-in where it is not.
export const EXPO_OUT = "cubic-bezier(0.16, 1, 0.3, 1)";

const sampled =
  typeof CSS !== "undefined" &&
  typeof CSS.supports === "function" &&
  CSS.supports("transition-timing-function", "linear(0, 1)");

export const SPRING = sampled
  ? "linear(0, 0.045, 0.159, 0.311, 0.477, 0.638, 0.782, 0.903, 0.996, 1.061, 1.102, 1.122, 1.126, 1.118, 1.102, 1.082, 1.061, 1.041, 1.023, 1.009, 0.998, 0.991, 0.986, 0.984, 0.984, 0.986, 0.988, 0.99, 1)"
  : "cubic-bezier(0.34, 1.5, 0.64, 1)";
