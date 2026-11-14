import { useEffect, useLayoutEffect, useRef, type CSSProperties, type RefObject } from "react";
import type { ServerState } from "../logic";
import { mayMove } from "../motion";
import { cueFor, lit, waiting, type Cue, type Variant } from "../plot";
import { VIEW } from "./iso";
import { Barrier } from "./parts";
import type { Options, Plot, Stage } from "./stage";

// One world's plot: its drawing in the state the card's words show, and the
// one-shot moves that carry it from one picture to the next. It is decorative
// and hidden from assistive technology; the words beside it say the state.
//
// A move plays only where it can be seen and wanted: never under
// prefers-reduced-motion, in a hidden tab or off screen, where the plot simply
// shows its new picture. A newer state cuts any move still running, so the
// picture never lags behind the words.
export function PlotView({
  plot,
  art,
  state,
  beat = 0,
  flare = 0,
  variant,
  initial,
  order = 0,
}: {
  plot: Plot;
  art: string;
  state: ServerState;
  // Goes up by one for each status answer the page shows.
  beat?: number;
  // Goes up by one each time this world's address is copied.
  flare?: number;
  variant?: Variant;
  initial?: string;
  // The plot's place on the board, for the order the plots arrive in.
  order?: number;
}) {
  const root = useRef<HTMLSpanElement>(null);
  useMoves(root, plot, state, beat, flare);
  const Drawing = plot.Art;
  return (
    <span
      ref={root}
      className="mc-plot"
      data-plot={art}
      data-state={state}
      style={{ "--n": order } as CSSProperties}
      aria-hidden="true"
    >
      <svg viewBox={VIEW} focusable="false">
        <Drawing state={state} variant={variant} initial={initial} />
        {state === "maintenance" && <Barrier />}
      </svg>
    </span>
  );
}

function stageOn(root: HTMLElement, running: Set<Animation>): Stage {
  return {
    play(part, keyframes, { stagger = 0, ...options }: Options = {}) {
      root.querySelectorAll(`[data-part~="${part}"]`).forEach((el, i) => {
        const move = el.animate(keyframes, {
          fill: "backwards",
          ...options,
          delay: (Number(options.delay) || 0) + i * stagger,
        });
        running.add(move);
        // A finished move is cancelled, not just forgotten: whatever fill it
        // asked for, it leaves nothing on the part, so the settled picture
        // is the drawing of the current state and nothing else.
        move.finished.then(
          () => {
            running.delete(move);
            move.cancel();
          },
          () => running.delete(move),
        );
      });
    },
  };
}

function useMoves(root: RefObject<HTMLSpanElement | null>, plot: Plot, state: ServerState, beat: number, flare: number) {
  const running = useRef(new Set<Animation>());
  const cut = () => {
    for (const move of [...running.current]) move.cancel();
    running.current.clear();
  };
  const run = (cue: Cue) => {
    const el = root.current;
    const move = plot.moves[cue];
    if (el && move && mayMove(el)) move(stageOn(el, running.current));
  };

  // A change of state: before the browser paints the new picture, so a part
  // that has just appeared starts its move from the first frame.
  const was = useRef<ServerState | null>(null);
  useLayoutEffect(() => {
    const before = was.current;
    was.current = state;
    if (before === null || before === state) return;
    cut();
    const cue = cueFor(before, state);
    if (cue) run(cue);
  }, [state]);

  // A status answer while the world is on its way, unless a move is still
  // playing: the beat never cuts into the move that Start itself began.
  const heard = useRef(beat);
  useEffect(() => {
    if (heard.current === beat) return;
    heard.current = beat;
    if (waiting(state) && running.current.size === 0) run("beat");
  }, [beat]);

  const flared = useRef(flare);
  useEffect(() => {
    if (flared.current === flare) return;
    flared.current = flare;
    if (lit(state) && running.current.size === 0) run("flare");
  }, [flare]);

  useEffect(() => cut, []);
}
