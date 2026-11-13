import type { ServerState } from "../logic";
import { lit, waiting } from "../plot";
import { at, box, H, line, origin, shape, SIZE, U, type Pt } from "./iso";
import { FINE, INK, NAVY, rays, Slab, Solid } from "./parts";
import { EXPO_OUT, SPRING, type ArtProps, type Plot } from "./stage";

// The Build Site, the Creative world's plot: a stone gateway going up on a
// drafting sheet. Two piers of coursed masonry stand across the back with a
// doorway between them, and the arch stone that closes them is the actor: one
// long stone with a peaked back and an arched soffit. It lies on its mark at
// the front while the world is stopped, hangs high over the piers while it
// starts, sits on them while it is ready, hangs low over its mark while it
// stops, and lies dropped against the piers when a start fails. The site lamp
// beside the left pier is the plot's one light.

// The piers, in grid units: W long, D deep and TOP tall, G apart.
const W = 1.3;
const D = 0.9;
const G = 2.0;
const AX = 0.9;
const AY = 0.6;
const KX = AX + W;
const BX = KX + G;
const TOP = 3.8;
const FRONT = AY + D;

const A_FACES = box(AX, AY, 0, W, D, TOP);
const B_FACES = box(BX, AY, 0, W, D, TOP);

// Masonry, not stacked cubes: three courses of unequal height, their upright
// joints broken from one course to the next.
const BEDS = [1.45, 2.7];
const bed = (x: number, z: number) => line(at(x, FRONT, z), at(x + W, FRONT, z), at(x + W, AY, z));
const joint = (x: number, u: number, z0: number, z1: number) => line(at(x + W * u, FRONT, z0), at(x + W * u, FRONT, z1));
const COURSES = [AX, BX]
  .map((x) => BEDS.map((z) => bed(x, z)).join("") + joint(x, 0.58, 0, BEDS[0]) + joint(x, 0.38, BEDS[0], BEDS[1]) + joint(x, 0.6, BEDS[1], TOP))
  .join("");

// A prism: an outline in the upright plane along x (points of x and z, going
// anticlockwise), run from y0 forward to y1. Its front is the outline itself
// and its sides are the edges that face the eye, which looks along (1, 1, 8/9).
type XZ = readonly [number, number];
type Prism = { front: string; sides: { d: string; up: boolean }[] };

function prism(outline: readonly XZ[], y0: number, y1: number): Prism {
  const sides: Prism["sides"] = [];
  outline.forEach((a, i) => {
    const b = outline[(i + 1) % outline.length];
    const [nx, nz] = [b[1] - a[1], -(b[0] - a[0])];
    const seen = nx + (nz * U) / H;
    if (seen <= 0.02 * Math.hypot(nx, nz)) return;
    sides.push({
      d: shape(at(a[0], y0, a[1]), at(b[0], y0, b[1]), at(b[0], y1, b[1]), at(a[0], y1, a[1])),
      up: nz > nx,
    });
  });
  return { front: shape(...outline.map(([x, z]) => at(x, y1, z))), sides };
}

// The arch stone, seated. It overhangs the piers at both ends and a little at
// the front and back; its soffit is cut up into the doorway and its back
// rises to a ridge, so it is nothing like a course.
const OVER_X = 0.2;
const OVER_Y = 0.1;
const LX0 = AX - OVER_X;
const LX1 = BX + W + OVER_X;
const LY0 = AY - OVER_Y;
const LY1 = FRONT + OVER_Y;
const BODY = 0.9;
const PEAK = 0.5;
const CUT = 0.5;
const MID = (LX0 + LX1) / 2;

const STONE: XZ[] = [
  [LX0, TOP],
  [KX, TOP],
  [KX + CUT, TOP + CUT],
  [BX - CUT, TOP + CUT],
  [BX, TOP],
  [LX1, TOP],
  [LX1, TOP + BODY],
  [MID, TOP + BODY + PEAK],
  [LX0, TOP + BODY],
];
const LINTEL = prism(STONE, LY0, LY1);

// It turns about the front corner of its right foot and squashes about the
// middle of its bed.
const HEEL = at(LX1, LY1, TOP);
const BED = at(MID, LY1, TOP);

// Where it lies when it has been dropped: its right foot dug into the sheet
// in front of the far pier and its left end up against the near one. This is drawn
// as its own stone, turned in the upright plane, because a turn on the page is
// not the same thing.
const TILT = -35;
const RAD = (TILT * Math.PI) / 180;
const FELL = [5.6, FRONT + 0.15] as const;
const WRECK = prism(
  STONE.map(([x, z]) => {
    const [dx, dz] = [x - LX1, z - TOP];
    return [FELL[0] + dx * Math.cos(RAD) - dz * Math.sin(RAD), dx * Math.sin(RAD) + dz * Math.cos(RAD)] as const;
  }),
  FELL[1],
  FELL[1] + (LY1 - LY0),
);
const WRECK_HEEL = at(FELL[0], FELL[1] + (LY1 - LY0), 0);

// The stone's poses, as what carries it from its seat. `by` is in view units.
const move = (dx: number, dy: number, dz: number): Pt => [
  Math.round((dx - dy) * U * 100) / 100,
  Math.round((((dx + dy) * U) / 2 - dz * H) * 100) / 100,
];
const PARK = [0.2, 2.0] as const;
const HIGH = 1.7;
const LOW_UP = 1.6;

type Pose = { by: Pt; turn?: number };
const SEATED: Pose = { by: [0, 0] };
const OVER: Pose = { by: move(0, 0, HIGH) };
const PARKED: Pose = { by: move(PARK[0], PARK[1], -TOP) };
const LOW: Pose = { by: move(PARK[0], PARK[1], LOW_UP - TOP) };
// The page turn that lays the seated stone nearest to the dropped one: the
// move ends on it, and the dropped stone takes over.
const TURN = ((Math.atan2((U / 2) * Math.cos(RAD) - H * Math.sin(RAD), U * Math.cos(RAD)) - Math.atan2(1, 2)) * 180) / Math.PI;
const DROPPED: Pose = {
  by: [Math.round((WRECK_HEEL[0] - HEEL[0]) * 100) / 100, Math.round((WRECK_HEEL[1] - HEEL[1]) * 100) / 100],
  turn: Math.round(TURN * 10) / 10,
};

const up = (pose: Pose, units: number): Pose => ({ ...pose, by: [pose.by[0], pose.by[1] - units] });
const css = ({ by, turn = 0 }: Pose) => `translate(${by[0]}px, ${by[1]}px) rotate(${turn}deg)`;

function pose(state: ServerState): Pose {
  switch (state) {
    case "starting":
      return OVER;
    case "ready":
      return SEATED;
    case "stopping":
      return LOW;
    case "failed":
      return DROPPED;
    default:
      return PARKED;
  }
}

// The drafting grid on the sheet, and the outline of where the stone parks:
// empty whenever the stone is anywhere else.
const GRID = [1, 2, 3, 4, 5].map((n) => line(at(n, 0), at(n, SIZE)) + line(at(0, n), at(SIZE, n))).join("");
const [MX0, MX1, MY0, MY1] = [LX0 + PARK[0], LX1 + PARK[0], LY0 + PARK[1], LY1 + PARK[1]];
const MARK = shape(at(MX0, MY0), at(MX1, MY0), at(MX1, MY1), at(MX0, MY1));

// Two ticks of air under the stone's feet while it hangs.
const tick = (x: number) => {
  const [px, py] = at(x, LY1, TOP);
  return `M${px} ${py + 4.5}v5`;
};
const TICKS = tick(AX + W * 0.4) + tick(BX + W * 0.6);

// The site lamp: a post beside the left pier, a round lens and a flat-topped
// shade over it.
const POST = [0.15, 4.25] as const;
const LAMP = at(POST[0], POST[1], 2.5);
const LENS = 7.4;
const LAMP_POST = line(at(POST[0], POST[1], 0), LAMP);
const SHADE = `M${LAMP[0] - 9.6} ${LAMP[1] - 3.2}L${LAMP[0] - 5.4} ${LAMP[1] - 10.4}h10.8L${LAMP[0] + 9.6} ${LAMP[1] - 3.2}z`;
const LAMP_RAYS = rays(LAMP, [18, 162, 55, 125], 11, 4.6);

// The dust of the landing, at the stone's two ends where it meets the piers.
const JOINT_L = at(LX0, LY1, TOP);
const JOINT_R = at(LX1, LY0, TOP);
const DUST_L = `M${JOINT_L[0] - 3} ${JOINT_L[1] + 1}l-3.5 1.5`;
const DUST_R = `M${JOINT_R[0] + 3} ${JOINT_R[1] + 1}l3.5 -1.5`;

function Stone({ faces }: { faces: Prism }) {
  return (
    <g {...INK}>
      {faces.sides.map((side) => (
        <path key={side.d} className="plot-face" d={side.d} fill={side.up ? "var(--seafoam-light)" : "var(--leaf-soft)"} />
      ))}
      <path className="plot-face" d={faces.front} fill="var(--limb)" />
    </g>
  );
}

function Art({ state }: ArtProps) {
  const on = lit(state);
  const failed = state === "failed";
  return (
    <>
      <Slab top="var(--card)" left="var(--limb)" right="var(--leaf-soft)">
        <path d={GRID} fill="none" stroke={NAVY} strokeWidth="1.25" opacity="0.3" />
        <path d={MARK} fill="none" {...FINE} />
      </Slab>
      <Solid faces={A_FACES} top="var(--seafoam)" left="var(--leaf-soft)" right="var(--leaf-mid)" />
      <Solid faces={B_FACES} top="var(--seafoam)" left="var(--leaf-soft)" right="var(--leaf-mid)" />
      <path d={COURSES} fill="none" {...FINE} />

      <path d={LAMP_POST} fill="none" {...INK} />
      <circle className="plot-face" cx={LAMP[0]} cy={LAMP[1]} r={LENS} fill="var(--card)" {...FINE} />
      <g data-part="light" style={{ transformOrigin: origin(LAMP), opacity: on ? 1 : 0 }}>
        <circle cx={LAMP[0]} cy={LAMP[1]} r={LENS} fill="var(--custard)" {...FINE} />
        <path data-part="rays" d={LAMP_RAYS} fill="none" style={{ transformOrigin: origin(LAMP) }} {...FINE} />
      </g>
      <path className="plot-dark" d={SHADE} fill={NAVY} {...FINE} />

      <g data-part="wreck" style={{ transformOrigin: origin(WRECK_HEEL), opacity: failed ? 1 : 0 }}>
        <Stone faces={WRECK} />
      </g>
      <g data-part="lintel" style={{ transformOrigin: origin(HEEL), transform: css(pose(state)), opacity: failed ? 0 : 1 }}>
        <g data-part="nudge" style={{ transformOrigin: origin(BED) }}>
          <Stone faces={LINTEL} />
          <path data-part="ticks" d={TICKS} fill="none" style={{ transformOrigin: origin(BED), opacity: waiting(state) ? 1 : 0 }} {...FINE} />
        </g>
      </g>
      <path data-part="dust dust-l" d={DUST_L} fill="none" style={{ transformOrigin: origin(JOINT_L), opacity: 0 }} {...FINE} />
      <path data-part="dust dust-r" d={DUST_R} fill="none" style={{ transformOrigin: origin(JOINT_R), opacity: 0 }} {...FINE} />
    </>
  );
}

// A frame of the stone's road, and of the squash it lands with.
const frame = (p: Pose, rest: Keyframe = {}): Keyframe => ({ transform: css(p), ...rest });
const squash = (x: number, y: number, rest: Keyframe = {}): Keyframe => ({ transform: `scale(${x}, ${y})`, ...rest });
const nudge = (y: number, rest: Keyframe = {}): Keyframe => ({ transform: `translateY(${y}px)`, ...rest });
const held = (opacity: number): Keyframe[] => [{ opacity }, { opacity }];

const GRAVITY = "cubic-bezier(0.55, 0, 1, 0.45)";
const LIFT = "cubic-bezier(0.3, 0, 0.2, 1)";

const LIGHT_ON: Keyframe[] = [
  { opacity: 0, transform: "scale(0.4)" },
  { opacity: 1, transform: "scale(0.4)", offset: 0.01 },
  { opacity: 1, transform: "scale(1)" },
];
const RAYS_OUT: Keyframe[] = [
  { transform: "scale(0.5)", opacity: 0 },
  { transform: "scale(1)", opacity: 1 },
];
const dust = (dx: number): Keyframe[] => [
  { opacity: 1, transform: "translate(0px, 0px)" },
  { opacity: 0, transform: `translate(${dx}px, -2px)` },
];

export const build: Plot = {
  Art,
  moves: {
    // Off its mark, straight up past the piers' tops, then back over them,
    // with the overshoot only on the arrival. The ticks come with the hang.
    strike({ play }) {
      const T = 560;
      play(
        "lintel",
        [
          frame(PARKED, { easing: LIFT }),
          frame({ by: [PARKED.by[0] * 0.8, OVER.by[1] + 6] }, { offset: 0.5, easing: SPRING }),
          frame(OVER),
        ],
        { duration: T },
      );
      play("ticks", [{ opacity: 0 }, { opacity: 0, offset: 0.75 }, { opacity: 1 }], { duration: T });
    },
    beat({ play }) {
      play("nudge", [nudge(0, { easing: "ease-in-out" }), nudge(3, { offset: 0.4, easing: EXPO_OUT }), nudge(0)], {
        duration: 400,
      });
    },
    // The one big moment: the stone dips, draws up, hangs, drops onto the
    // piers, lands and settles; then the lamp comes on, throws its rays and
    // steadies.
    wake({ play }) {
      const LAND = 1000;
      play(
        "lintel",
        [
          frame(OVER, { easing: "ease-in-out" }),
          frame(up(OVER, -2), { offset: 0.13, easing: EXPO_OUT }),
          frame(up(OVER, 5), { offset: 0.56 }),
          frame(up(OVER, 5), { offset: 0.76, easing: GRAVITY }),
          frame(SEATED),
        ],
        { duration: LAND },
      );
      play("ticks", [{ opacity: 1 }, { opacity: 1, offset: 0.76 }, { opacity: 0, offset: 0.86 }, { opacity: 0 }], { duration: LAND });
      play("nudge", [squash(1, 1), squash(1.05, 0.84, { offset: 0.01, easing: SPRING }), squash(1, 1)], {
        duration: 320,
        delay: LAND - 4,
        fill: "none",
      });
      play("dust-l", dust(-4), { duration: 280, delay: LAND, easing: "ease-out", fill: "none" });
      play("dust-r", dust(4), { duration: 280, delay: LAND, easing: "ease-out", fill: "none" });
      play("light", LIGHT_ON, { duration: 420, delay: LAND + 120, easing: SPRING });
      play("rays", RAYS_OUT, { duration: 420, delay: LAND + 340, easing: EXPO_OUT });
      play("light", [{ transform: "scale(1)" }, { transform: "scale(1.08)", offset: 0.4 }, { transform: "scale(1)" }], {
        duration: 300,
        delay: LAND + 700,
        easing: "ease-in-out",
        fill: "none",
      });
    },
    light({ play }) {
      play("light", LIGHT_ON, { duration: 420, easing: SPRING });
      play("rays", RAYS_OUT, { duration: 320, delay: 180, easing: EXPO_OUT });
    },
    // Lamp out, rays first; then the stone up off the piers and forward to
    // hang over its mark.
    pack({ play }) {
      const OUT = 200;
      const T = 560;
      // The rays are gone by 120ms and stay gone until the lamp is out.
      play(
        "rays",
        [
          { opacity: 1, transform: "scale(1)", easing: "ease-in" },
          { opacity: 0, transform: "scale(0.6)", offset: 120 / OUT },
          { opacity: 0, transform: "scale(0.6)" },
        ],
        { duration: OUT },
      );
      play("light", [{ opacity: 1, transform: "scale(1)" }, { opacity: 0, transform: "scale(0.5)" }], {
        duration: OUT,
        easing: "ease-in",
      });
      play(
        "lintel",
        [frame(SEATED, { easing: LIFT }), frame({ by: [LOW.by[0] * 0.2, -1.1 * H] }, { offset: 0.34, easing: EXPO_OUT }), frame(LOW)],
        { duration: T, delay: OUT },
      );
      play("ticks", held(0), { duration: OUT + T * 0.8 });
    },
    rest({ play }) {
      play("lintel", [frame(LOW, { easing: GRAVITY }), frame(PARKED)], { duration: 220 });
      play("ticks", [{ opacity: 1 }, { opacity: 0 }], { duration: 120 });
      play("nudge", [squash(1, 1), squash(1.03, 0.9, { offset: 0.01, easing: "ease-out" }), squash(1, 1)], {
        duration: 240,
        delay: 216,
        fill: "none",
      });
    },
    // The right end goes first: the stone tips, slides forward off the piers
    // and comes down heel first. The dropped stone takes over as it lands,
    // and rocks once.
    fizzle({ play }) {
      const T = 440;
      play(
        "lintel",
        [
          frame(OVER, { opacity: 1, easing: "ease-in" }),
          frame({ by: [DROPPED.by[0] * 0.15, OVER.by[1] + 2], turn: 8 }, { opacity: 1, offset: 0.3, easing: GRAVITY }),
          frame(DROPPED, { opacity: 1 }),
        ],
        { duration: T },
      );
      play("wreck", held(0), { duration: T });
      play(
        "wreck",
        [
          { transform: "rotate(-4deg)", easing: "ease-in" },
          { transform: "rotate(1.5deg)", offset: 0.55, easing: "ease-out" },
          { transform: "rotate(0deg)" },
        ],
        { duration: 260, delay: T },
      );
    },
    flare({ play }) {
      play("light", [{ transform: "scale(1)" }, { transform: "scale(1.25)", offset: 0.35 }, { transform: "scale(1)" }], {
        duration: 420,
        easing: "ease-out",
      });
      play("nudge", [nudge(0, { easing: EXPO_OUT }), nudge(-2, { offset: 0.4, easing: GRAVITY }), nudge(0)], { duration: 360 });
    },
  },
};
