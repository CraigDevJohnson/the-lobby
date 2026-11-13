import type { ServerState } from "./logic";
import { artFor, type Art, type Variant } from "./plot";
import { block } from "./plots/Block";
import { build } from "./plots/Build";
import { camp } from "./plots/Camp";
import { gate } from "./plots/Gate";
import type { Plot } from "./plots/stage";
import { PlotView } from "./plots/View";

// A world's plot, at the head of its column in the board: the place drawn
// for its id, or the server block on a plot of its own for a world the page
// has no drawing for (docs/design/minecraft-brief.md, "Pocket Worlds").

const PLOTS: Record<Art, Plot> = { camp, build, gate, block };

export function WorldPlot({
  id,
  name,
  state,
  beat,
  flare,
  variant,
  order,
}: {
  // The world's id, or none for the one server of a backend that lists no worlds.
  id: string | undefined;
  name: string;
  // The state the card's words show.
  state: ServerState;
  beat: number;
  flare: number;
  // The world's variant, when it has no drawing of its own (plot.ts).
  variant: Variant | undefined;
  order: number;
}) {
  const art = artFor(id);
  // A named world's block carries its initial; the one unnamed server's is plain.
  const initial = id !== undefined && art === "block" ? Array.from(name.trim())[0]?.toLocaleUpperCase() : undefined;
  return (
    <PlotView
      plot={PLOTS[art]}
      art={art}
      state={state}
      beat={beat}
      flare={flare}
      variant={variant}
      initial={initial}
      order={order}
    />
  );
}
