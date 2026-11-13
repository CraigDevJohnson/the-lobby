import { useId } from "react";
import { lit } from "../plot";
import type { ServerState } from "../logic";
import { at, box, line, origin, shape, type Pt } from "./iso";
import { FINE, INK, NAVY, Slab } from "./parts";
import { EXPO_OUT, SPRING, type ArtProps, type Plot } from "./stage";

// The Gatehouse, the Adventure world's plot: a short square tower at the back
// of its ground with a dotted trail winding up to its door. A portcullis hangs
// in the doorway and a pennant hangs limp at the foot of a tall pole. While
// the world is ready the gate is up out of sight, the doorway is lit (the
// plot's one custard light) and the pennant flies from the top of the pole.
// Starting, the gate is half up; stopping, it is most of the way down and the
// pennant is furled but not yet lowered; failed, it is jammed askew.

// The tower: T0..T1 along x and y, TALL high. Its two near walls each end in
// a merlon M wide and MH high at either corner; the roof shows behind them.
const T0 = 0.35;
const T1 = 3.75;
const TALL = 2.9;
const M = 1.1;
const MH = 0.7;
const CREST = TALL + MH;
const MID = (T0 + T1) / 2;

const TOP = box(T0, T0, 0, T1 - T0, T1 - T0, TALL).top;

// Each wall is one face from the ground to the tops of its two merlons.
const WALL_LEFT = shape(
  at(T0, T1, CREST),
  at(T0 + M, T1, CREST),
  at(T0 + M, T1, TALL),
  at(T1 - M, T1, TALL),
  at(T1 - M, T1, CREST),
  at(T1, T1, CREST),
  at(T1, T1, 0),
  at(T0, T1, 0),
);
const WALL_RIGHT = shape(
  at(T1, T1, CREST),
  at(T1, T1 - M, CREST),
  at(T1, T1 - M, TALL),
  at(T1, T0 + M, TALL),
  at(T1, T0 + M, CREST),
  at(T1, T0, CREST),
  at(T1, T0, 0),
  at(T1, T1, 0),
);

// The doorway, on the left wall: x across the wall, z up it. Its arch is a
// half circle on the wall, so its rise is its half-width in the wall's own
// measure (a step along the wall is a little shorter in view than a step up).
const wall = (x: number, z: number): Pt => at(x, T1, z);
const DOOR_X = MID;
const DOOR_R = 1.1;
const SPRING_Z = 1.25;
const RISE = DOOR_R * (Math.hypot(8, 4) / 9);
const DOOR_TALL = SPRING_Z + RISE;
const arch = (deg: number, k = 1): Pt => {
  const r = (deg * Math.PI) / 180;
  return wall(DOOR_X + DOOR_R * k * Math.cos(r), SPRING_Z + RISE * k * Math.sin(r));
};
const DOOR = shape(
  wall(DOOR_X - DOOR_R, 0),
  ...Array.from({ length: 13 }, (_, i) => arch(180 - i * 15)),
  wall(DOOR_X + DOOR_R, 0),
);
const ARCH = wall(DOOR_X, SPRING_Z);
const DOOR_MID = wall(DOOR_X, DOOR_TALL / 2);
const RAYS = [90, 50, 130].map((deg) => line(arch(deg, 1.36), arch(deg, 1.74))).join("");

// The portcullis: three uprights and two rails, long enough above the arch
// that the doorway stays barred however far the gate has dropped.
const RAIL_Z = [0.55, 1.35];
const UPRIGHTS = [-0.66, 0, 0.66].map((u) => line(wall(DOOR_X + DOOR_R * u, 0), wall(DOOR_X + DOOR_R * u, DOOR_TALL + 1.4))).join("");
const RAILS = RAIL_Z.map((z) => line(wall(DOOR_X - DOOR_R, z), wall(DOOR_X + DOOR_R, z))).join("");
const GRID = UPRIGHTS + RAILS;

// How far up the gate hangs, in view units: half way while starting, most of
// the way down while stopping, jammed askew when the start failed, and clear
// of the doorway when ready.
const tenth = (n: number) => Math.round(n * 10) / 10;
const NOTCH = tenth(DOOR_TALL * 9 * 0.54);
const LOW = tenth(DOOR_TALL * 9 * 0.26);
const JAM = tenth(DOOR_TALL * 9 * 0.14);
const RAISED = tenth(DOOR_TALL * 9 + 1);
const TILT = 20;
const hang = (up: number, turn = 0) => `translateY(${-up}px) rotate(${turn}deg)`;

function bars(state: ServerState) {
  if (state === "ready") return hang(RAISED);
  if (state === "starting") return hang(NOTCH);
  if (state === "stopping") return hang(LOW);
  if (state === "failed") return hang(JAM, TILT);
  return hang(0);
}

// Where a point of the gate lands once the gate is jammed.
const jammed = ([x, y]: Pt): Pt => {
  const r = (TILT * Math.PI) / 180;
  const [dx, dy] = [x - DOOR_MID[0], y - DOOR_MID[1]];
  return [tenth(DOOR_MID[0] + dx * Math.cos(r) - dy * Math.sin(r)), tenth(DOOR_MID[1] + dx * Math.sin(r) + dy * Math.cos(r) - JAM)];
};
// The jammed gate's top rail, sticking out through the arch: shown only when
// the start failed.
const STUB = line(jammed(wall(DOOR_X - DOOR_R * 1.62, RAIL_Z[1])), jammed(wall(DOOR_X - DOOR_R * 0.8, RAIL_Z[1])));

// The arrow slit, on the right wall. It is never lit.
const SLIT = shape(at(T1, MID + 0.36, 1.05), at(T1, MID + 0.36, 2.2), at(T1, MID - 0.36, 2.2), at(T1, MID - 0.36, 1.05));

// The pole stands in the middle of the roof. The pennant flies from its top,
// and hangs narrow and limp at its foot when it is down.
const POLE_TALL = 4.55;
const POLE_FOOT = at(MID, MID, TALL);
const POLE_TOP = at(MID, MID, TALL + POLE_TALL);
const HOIST: Pt = [POLE_TOP[0], POLE_TOP[1] + 1.2];
const FLY = 20;
const DEEP = 12;
const PENNANT = shape(HOIST, [HOIST[0] + FLY, HOIST[1] + DEEP / 2], [HOIST[0], HOIST[1] + DEEP]);
// Limp, the pennant is a narrow tongue hanging down the pole.
const SLIM = 0.5;
const SHORT = 0.8;
const SAG = 52;
const fly = (drop: number, lean = 0, wide = 1, deep = 1) => `translateY(${drop}px) skewY(${lean}deg) scale(${wide}, ${deep})`;
const limp = (drop: number, lean = SAG) => fly(drop, lean, SLIM, SHORT);
const HANG = (DEEP / 2) * SHORT + FLY * SLIM * Math.tan((SAG * Math.PI) / 180);
const DROP = tenth(POLE_FOOT[1] - HOIST[1] - HANG - 2);
const FLYING = fly(0);
const FURLED_UP = limp(0);
const FURLED = limp(DROP);

function pennant(state: ServerState) {
  if (state === "ready") return FLYING;
  if (state === "stopping") return FURLED_UP;
  return FURLED;
}

// The trail: round dots on the ground, from the front corner to the door.
const TRAIL: readonly Pt[] = [at(5.35, 5.2), at(4.55, 5.5), at(3.7, 5.35), at(3.1, 4.8), at(2.3, 4.75), at(2.05, 4.15)];

function Art({ state }: ArtProps) {
  const on = lit(state);
  const clip = useId();
  return (
    <>
      <Slab top="var(--limb)" left="var(--leaf-soft)" right="var(--leaf-mid)">
        {TRAIL.map(([cx, cy]) => (
          <ellipse key={`${cx} ${cy}`} className="plot-dark" cx={cx} cy={cy} rx="3.4" ry="1.9" fill={NAVY} />
        ))}
      </Slab>

      <path className="plot-face" d={TOP} fill="var(--seafoam-light)" {...INK} />
      <path d={line(POLE_FOOT, POLE_TOP)} fill="none" {...INK} />
      <path
        data-part="pennant"
        className="plot-face"
        d={PENNANT}
        fill="var(--leaf-mid)"
        vectorEffect="non-scaling-stroke"
        style={{ transformOrigin: origin(HOIST), transform: pennant(state), opacity: state === "unknown" ? 0 : 1 }}
        {...FINE}
      />
      <path className="plot-face" d={WALL_LEFT} fill="var(--limb)" {...INK} />
      <path className="plot-face" d={WALL_RIGHT} fill="var(--leaf-soft)" {...INK} />

      <clipPath id={clip}>
        <path d={DOOR} />
      </clipPath>
      <path className="plot-dark" d={DOOR} fill={NAVY} />
      <path className="plot-dark" d={SLIT} fill={NAVY} {...FINE} />
      <g data-part="light" style={{ transformOrigin: origin(ARCH), opacity: on ? 1 : 0 }}>
        <path data-part="glow" d={DOOR} fill="var(--custard)" />
        <path data-part="rays" d={RAYS} fill="none" style={{ transformOrigin: origin(ARCH) }} {...FINE} />
      </g>
      <g clipPath={`url(#${clip})`}>
        <g data-part="bars" fill="none" strokeLinecap="butt" style={{ transformOrigin: origin(DOOR_MID), transform: bars(state) }}>
          <path d={GRID} stroke={NAVY} strokeWidth="3.8" strokeOpacity={state === "unknown" ? 0 : 1} />
          <path d={GRID} stroke="var(--seafoam-light)" strokeWidth="2.4" />
        </g>
      </g>
      <path d={DOOR} fill="none" {...INK} />
      <g data-part="stub" fill="none" strokeLinecap="round" style={{ transformOrigin: origin(DOOR_MID), opacity: state === "failed" ? 1 : 0 }}>
        <path d={STUB} stroke={NAVY} strokeWidth="5.4" />
        <path d={STUB} stroke="var(--seafoam-light)" strokeWidth="2.2" />
      </g>
    </>
  );
}

// The gate going up a notch at a time: each clank overshoots a touch and
// settles back onto its notch.
const CLANKS = 3;
const CLANK = 180;
const HOLD = 80;
const RATCHET: Keyframe[] = [{ transform: hang(NOTCH), easing: "ease-out" }];
for (let i = 1; i <= CLANKS; i++) {
  const to = NOTCH + ((RAISED - NOTCH) * i) / CLANKS;
  const start = (i - 1) * (CLANK + HOLD);
  const total = CLANKS * (CLANK + HOLD);
  RATCHET.push(
    { transform: hang(to + 1.3), offset: (start + CLANK * 0.6) / total, easing: "ease-in-out" },
    { transform: hang(to), offset: (start + CLANK) / total, easing: "linear" },
    { transform: hang(to), offset: (start + CLANK + HOLD) / total, easing: "ease-out" },
  );
}

// The light is a lamp: it clicks on and off, with no half-lit frame between.
const ON: Keyframe[] = [{ opacity: 0 }, { opacity: 1 }];
const OFF: Keyframe[] = [{ opacity: 1 }, { opacity: 0 }];
const CLICK_ON = { duration: 60, easing: "steps(1, start)" };
const CLICK_OFF = { duration: 60, easing: "steps(1, end)" };
const RAYS_OUT: Keyframe[] = [
  { opacity: 0, transform: "scale(0.6)" },
  { opacity: 1, transform: "scale(1)" },
];

// The pennant runs up the pole, then fills out.
const HAUL = "cubic-bezier(0.3, 0, 0.2, 1)";

export const gate: Plot = {
  Art,
  moves: {
    // Stopped to starting: the gate jerks half way up and the limp pennant
    // shivers at the foot of its pole.
    strike({ play }) {
      play(
        "bars",
        [0, NOTCH + 2.4, NOTCH - 0.8, NOTCH + 0.8, NOTCH].map((up) => ({ transform: hang(up) })),
        { duration: 380, easing: "ease-out" },
      );
      play(
        "pennant",
        [SAG, SAG - 16, SAG + 7, SAG].map((lean) => ({ transform: limp(DROP, lean) })),
        { duration: 340, delay: 90, easing: "ease-out" },
      );
    },
    beat({ play }) {
      play(
        "bars",
        [
          { transform: "translateY(0px)", easing: "ease-in" },
          { transform: "translateY(3.4px)", offset: 0.4, easing: "ease-out" },
          { transform: "translateY(-1.2px)", offset: 0.72, easing: "ease-in-out" },
          { transform: "translateY(0px)" },
        ],
        { duration: 400, composite: "add" },
      );
    },
    wake({ play }) {
      // The light behind the bars, the gate up and out of sight in three
      // clanks, then the pennant: up the pole, out, and three swings that
      // die away.
      play("glow", ON, CLICK_ON);
      play("bars", RATCHET, { duration: CLANKS * (CLANK + HOLD), delay: 340 });
      play("rays", RAYS_OUT, { duration: 300, delay: 1120, easing: EXPO_OUT });
      play(
        "pennant",
        [
          { transform: FURLED, easing: HAUL },
          { transform: FURLED_UP, offset: 0.36, easing: SPRING },
          { transform: FLYING, offset: 0.56, easing: "ease-in-out" },
          { transform: fly(0, -11), offset: 0.65, easing: "ease-in-out" },
          { transform: fly(0, 7), offset: 0.76, easing: "ease-in-out" },
          { transform: fly(0, -3.5), offset: 0.88, easing: "ease-in-out" },
          { transform: FLYING },
        ],
        { duration: 1400, delay: 1180 },
      );
    },
    // Ready by any other road: the lamp, the gate straight up, the rays, and
    // the pennant hauled to the top.
    light({ play }) {
      play("glow", ON, CLICK_ON);
      play("bars", [{ transform: hang(0) }, { transform: hang(RAISED) }], { duration: 420, delay: 100, easing: EXPO_OUT });
      play("rays", RAYS_OUT, { duration: 280, delay: 420, easing: EXPO_OUT });
      play(
        "pennant",
        [
          { transform: FURLED, easing: HAUL },
          { transform: FURLED_UP, offset: 0.6, easing: SPRING },
          { transform: FLYING },
        ],
        { duration: 760, delay: 280 },
      );
    },
    // Ready to stopping: the lamp goes out, the gate drops most of the way
    // and catches, and the pennant falls limp at the top of its pole.
    pack({ play }) {
      play("light", OFF, CLICK_OFF);
      play(
        "bars",
        [
          { transform: hang(RAISED), easing: "cubic-bezier(0.55, 0, 1, 0.45)" },
          { transform: hang(LOW - 1.6), offset: 0.58, easing: "ease-out" },
          { transform: hang(LOW + 2.2), offset: 0.78, easing: "ease-in" },
          { transform: hang(LOW) },
        ],
        { duration: 420, delay: 150 },
      );
      play(
        "pennant",
        [
          { transform: FLYING, easing: "ease-in" },
          { transform: limp(0, SAG + 9), offset: 0.6, easing: "ease-out" },
          { transform: FURLED_UP },
        ],
        { duration: 460, delay: 300 },
      );
    },
    // Stopping to stopped: the gate thuds shut and the pennant comes down.
    rest({ play }) {
      play(
        "bars",
        [
          { transform: hang(LOW), easing: "cubic-bezier(0.55, 0, 1, 0.45)" },
          { transform: hang(0), offset: 0.5, easing: "ease-out" },
          { transform: hang(1.8), offset: 0.74, easing: "ease-in" },
          { transform: hang(0) },
        ],
        { duration: 340 },
      );
      play("pennant", [{ transform: FURLED_UP }, { transform: FURLED }], { duration: 520, delay: 120, easing: HAUL });
    },
    // Starting to failed: the gate lurches, drops askew and jams, and a rail
    // end bursts out through the arch.
    fizzle({ play }) {
      play(
        "bars",
        [
          { transform: hang(NOTCH), easing: "ease-out" },
          { transform: hang(NOTCH + 2, -3), offset: 0.2, easing: "cubic-bezier(0.55, 0, 1, 0.45)" },
          { transform: hang(JAM, TILT), offset: 0.5, easing: "ease-out" },
          { transform: hang(JAM + 1.2, TILT + 5), offset: 0.64 },
          { transform: hang(JAM, TILT - 4), offset: 0.8 },
          { transform: hang(JAM, TILT) },
        ],
        { duration: 440 },
      );
      play("stub", ON, { ...CLICK_ON, delay: 220 });
    },
    flare({ play }) {
      play(
        "pennant",
        [0, -12, 6, 0].map((lean) => ({ transform: `skewY(${lean}deg)` })),
        { duration: 380, easing: "ease-out", composite: "add" },
      );
      play(
        "rays",
        [{ transform: "scale(1)" }, { transform: "scale(1.4)", offset: 0.3 }, { transform: "scale(1)" }],
        { duration: 420, easing: "ease-out" },
      );
    },
  },
};
