import { lit } from "../plot";
import { at, line, origin, shape, type Pt } from "./iso";
import { FINE, INK, NAVY, rays, Slab } from "./parts";
import { EXPO_OUT, SPRING, type ArtProps, type Plot } from "./stage";

// The Camp, the Survival world's plot: an A-frame tent, a pine and a fire on
// wild ground. The tent is the plot's main shape and the fire is its one
// light: pale sparks over cold logs while the world starts, a custard flame
// with rays and an open tent door while it is ready, one curl of smoke while
// it stops, and the logs knocked apart with the sparks fallen beside them
// after a start fails.

// The pine: three square pyramids stacked on a navy trunk, each showing its
// lit left face and its shaded right face.
const [PX, PY] = [0.7, 4.5];
const PINE_FOOT = at(PX, PY, 0);
const TRUNK_TOP = 2.6;
const TIERS = [
  { z: 2.6, tip: 4.7, r: 0.9 },
  { z: 3.9, tip: 5.9, r: 0.72 },
  { z: 5.1, tip: 6.9, r: 0.52 },
].map(({ z, tip, r }) => {
  const apex = at(PX, PY, tip);
  const front = at(PX + r, PY + r, z);
  return {
    left: shape(apex, at(PX - r, PY + r, z), front),
    right: shape(apex, front, at(PX + r, PY - r, z)),
  };
});
const TRUNK = line(PINE_FOOT, at(PX, PY, TRUNK_TOP));

// The tent: its ridge runs along y, so the pale front stands on the left
// face and one long roof slope falls away to the right.
const [TX, TY, TW, TL, TH] = [2.1, 0.5, 3.8, 1.9, 3.7];
// A point on the front: u across its foot, v up to the ridge.
const F = (u: number, v: number) => at(TX + TW * u, TY + TL, TH * v);
const FRONT = shape(F(0, 0), F(1, 0), F(0.5, 1));
const ROOF = shape(
  F(0.5, 1),
  at(TX + TW / 2, TY, TH),
  at(TX + TW, TY, 0),
  F(1, 0),
);
// The middle of its footprint, so a squash stays planted on both sides.
const TENT_FOOT = at(TX + TW / 2, TY + TL / 2, 0);
const SEAM = line(F(0.5, 1), F(0.5, 0));
// The ridge pole pokes out over the front.
const POLE = line(F(0.5, 1), at(TX + TW / 2, TY + TL + 0.45, TH));
const DOOR_FOOT = F(0.5, 0);
const DOOR_TOP = F(0.5, 0.76);
const DOOR = shape(DOOR_TOP, F(0.22, 0), F(0.64, 0));
const FOLD = shape(DOOR_TOP, F(0.64, 0), F(0.9, 0));

// The fire, at the front between the pine and the tent and clear of both.
const [GX, GY] = [3.3, 4.8];
const FIRE = at(GX, GY, 0);
const [FX, FY] = FIRE;
const LOG = 0.6;
const LOG_AT = at(GX, GY, 0.15);
const LOG_A = line(at(GX - LOG, GY, 0.15), at(GX + LOG, GY, 0.15));
const LOG_B = line(at(GX, GY - LOG, 0.15), at(GX, GY + LOG, 0.15));
// Where a failed start leaves them: one rolled off toward the tent, the
// other tipped over.
const APART = {
  a: "translate(13px, 3px) rotate(-22deg)",
  b: "translate(-5px, -1px) rotate(-32deg)",
};
// While the barrier stands across the front, the logs wait beside it.
const ASIDE = "translate(24px, 2px)";
const CROSSED = "translate(0px, 0px) rotate(0deg)";

const FLAME_BASE: Pt = [FX, FY - 2];
const FLAME = (() => {
  const [x, y] = FLAME_BASE;
  const tip = `${x + 1.75} ${y - 22.5}`;
  return (
    `M${tip}C${x + 2.25} ${y - 15.62} ${x + 8.75} ${y - 13.75} ${x + 8.75} ${y - 6.75}` +
    `C${x + 8.75} ${y - 1.0} ${x + 4.75} ${y + 2.0} ${x} ${y + 2.0}` +
    `C${x - 4.75} ${y + 2.0} ${x - 8.75} ${y - 1.0} ${x - 8.75} ${y - 6.75}` +
    `C${x - 8.75} ${y - 12.5} ${x - 2.0} ${y - 14.38} ${tip}z`
  );
})();
const GLOW: Pt = [FX + 1.8, FY - 12];
// A narrow fan in the clear air over the flame.
const RAYS = [
  rays(GLOW, [-120], 13, 4.6),
  rays(GLOW, [-90], 17.5, 4.6),
  rays(GLOW, [-60], 13, 4.6),
];

// A spark is a four-pointed star, pale so it shows on the ground and
// against the sky alike.
const star = ([cx, cy]: Pt, r: number) => {
  const k = r * 0.38;
  return `M${cx} ${cy - r}L${cx + k} ${cy - k}L${cx + r} ${cy}L${cx + k} ${cy + k}L${cx} ${cy + r}L${cx - k} ${cy + k}L${cx - r} ${cy}L${cx - k} ${cy - k}z`;
};
const HEARTH: Pt = [FX, FY - 5];
const FLOAT = "translate(0px, 0px) scale(1, 1)";
// Each spark rises over the logs while the world starts; `fell` is where it
// lies on the ground after a failed start (the high one burns out instead).
const SPARKS = [
  {
    name: "spark-a",
    c: [FX + 1, FY - 18] as Pt,
    r: 7.5,
    fell: [FX + 2, FY + 6.5] as Pt,
  },
  { name: "spark-b", c: [FX + 7, FY - 39] as Pt, r: 6, fell: null },
  {
    name: "spark-c",
    c: [FX - 8, FY - 9] as Pt,
    r: 6,
    fell: [FX - 13, FY + 3] as Pt,
  },
].map((s) => ({
  ...s,
  d: star(s.c, s.r),
  pull: [(HEARTH[0] - s.c[0]) * 0.75, (HEARTH[1] - s.c[1]) * 0.75] as const,
  down: s.fell
    ? `translate(${s.fell[0] - s.c[0]}px, ${s.fell[1] - s.c[1]}px) scale(0.9, 0.55)`
    : "translate(4px, 14px) scale(0.3, 0.3)",
}));

const SMOKE_FOOT: Pt = [FX, FY - 7];
const SMOKE = `M${FX} ${FY - 7}c9 -5.5 -8 -11 0 -18.5c5 -4.5 9 -9 4 -12`;

function Art({ state }: ArtProps) {
  const on = lit(state);
  const show = (yes: boolean) => (yes ? 1 : 0);
  const failed = state === "failed";
  return (
    <>
      <Slab
        top="var(--leaf-soft)"
        left="var(--leaf-mid)"
        right="var(--leaf-deep)"
      />

      <g data-part="tent" style={{ transformOrigin: origin(TENT_FOOT) }}>
        <g {...INK}>
          <path className="plot-face" d={ROOF} fill="var(--leaf-deep)" />
          <path className="plot-face" d={FRONT} fill="var(--seafoam-light)" />
        </g>
        <path d={POLE} fill="none" {...INK} />
        <path d={SEAM} fill="none" {...FINE} />
        <g
          data-part="flap"
          style={{ transformOrigin: origin(DOOR_FOOT), opacity: show(on) }}
          {...FINE}
        >
          <path className="plot-dark" d={DOOR} fill={NAVY} />
          <path className="plot-face" d={FOLD} fill="var(--seafoam)" />
        </g>
      </g>

      <g data-part="pine" style={{ transformOrigin: origin(PINE_FOOT) }}>
        <path d={TRUNK} fill="none" {...INK} strokeWidth={5} />
        {TIERS.map((tier, i) => (
          <g key={i} {...INK}>
            <path className="plot-face" d={tier.left} fill="var(--seafoam)" />
            <path
              className="plot-face"
              d={tier.right}
              fill="var(--leaf-deep)"
            />
          </g>
        ))}
      </g>

      <path
        data-part="smoke"
        d={SMOKE}
        fill="none"
        style={{
          transformOrigin: origin(SMOKE_FOOT),
          opacity: show(state === "stopping"),
        }}
        {...INK}
      />
      <path
        data-part="flame"
        d={FLAME}
        fill="var(--custard)"
        style={{ transformOrigin: origin(FLAME_BASE), opacity: show(on) }}
        {...FINE}
      />
      <g
        data-part="logs"
        style={{ transform: state === "maintenance" ? ASIDE : undefined }}
      >
        <path
          data-part="log log-b"
          d={LOG_B}
          fill="none"
          style={{
            transformOrigin: origin(LOG_AT),
            transform: failed ? APART.b : undefined,
          }}
          {...INK}
          strokeWidth={4.6}
        />
        <path
          data-part="log log-a"
          d={LOG_A}
          fill="none"
          style={{
            transformOrigin: origin(LOG_AT),
            transform: failed ? APART.a : undefined,
          }}
          {...INK}
          strokeWidth={4.6}
        />
      </g>
      {SPARKS.map((s) => (
        <path
          key={s.name}
          data-part={`spark ${s.name}`}
          className="plot-face"
          d={s.d}
          fill="var(--seafoam-light)"
          style={{
            transformOrigin: origin(s.c),
            transform: failed ? s.down : undefined,
            opacity: show(state === "starting" || (failed && s.fell !== null)),
          }}
          {...FINE}
        />
      ))}
      <g
        data-part="rays"
        style={{ transformOrigin: origin(GLOW), opacity: show(on) }}
      >
        {RAYS.map((d, i) => (
          <path
            key={i}
            data-part="ray"
            d={d}
            fill="none"
            style={{ transformOrigin: origin(GLOW) }}
            {...INK}
          />
        ))}
      </g>
    </>
  );
}

// The door opens and shuts along the tent's front, which leans across the
// view: a squeeze toward the seam, taken along the front's own foot.
const door = (open: number) =>
  `skewY(26.565deg) scaleX(${open}) skewY(-26.565deg)`;
const SHUT = 0.02;

const CATCH: Keyframe[] = [
  { transform: "scale(0)" },
  { transform: "scale(1)" },
];
const RAY_OUT: Keyframe[] = [
  { opacity: 0, transform: "scale(0.45)" },
  { opacity: 1, transform: "scale(0.45)", offset: 0.01 },
  { opacity: 1, transform: "scale(1)" },
];
const SMOKE_UP: Keyframe[] = [
  { opacity: 0, transform: "translateY(6px) scale(0.7)" },
  { opacity: 1, transform: "translateY(0) scale(1)" },
];
const squash = (by: number): Keyframe[] => [
  { transform: "scale(1, 1)" },
  { transform: `scale(${1 + by}, ${1 - by})`, offset: 0.38 },
  { transform: "scale(1, 1)" },
];
const SWAY: Keyframe[] = [0, 4.5, -2.8, 1.5, -0.6, 0].map((deg) => ({
  transform: `rotate(${deg}deg)`,
  easing: "ease-in-out",
}));

const OPEN: Keyframe[] = [
  { opacity: 1, transform: door(SHUT) },
  { opacity: 1, transform: door(1) },
];
// The logs jump and land crossed.
const KNOCK: Keyframe[] = [
  { transform: "translateY(0)", easing: "ease-out" },
  { transform: "translateY(-5px)", offset: 0.4, easing: "ease-in" },
  { transform: "translateY(0)" },
];

export const camp: Plot = {
  Art,
  moves: {
    strike({ play }) {
      play("log", KNOCK, { duration: 220, stagger: 50 });
      play("spark-c", CATCH, { duration: 260, delay: 120, easing: SPRING });
      play(
        "spark-a",
        [
          { transform: "scale(0)" },
          { transform: "scale(1.3)", offset: 0.55 },
          { transform: "scale(1)" },
        ],
        {
          duration: 380,
          delay: 200,
          easing: "ease-out",
        },
      );
      play(
        "spark-b",
        [
          { transform: "translateY(16px) scale(0)" },
          { transform: "translateY(0) scale(1)" },
        ],
        {
          duration: 320,
          delay: 380,
          easing: SPRING,
        },
      );
      play("tent", squash(0.03), { duration: 240, easing: "ease-out" });
    },
    beat({ play }) {
      // Whichever is showing answers: the sparks on the way up, the smoke on
      // the way down.
      play(
        "spark",
        [
          { transform: "scale(1)" },
          { transform: "scale(1.4)", offset: 0.35 },
          { transform: "scale(1)" },
        ],
        {
          duration: 400,
          easing: "ease-out",
        },
      );
      play(
        "smoke",
        [
          { transform: "translateY(0)" },
          { transform: "translateY(-3px)", offset: 0.35 },
          { transform: "translateY(0)" },
        ],
        {
          duration: 400,
          easing: "ease-out",
        },
      );
    },
    wake({ play }) {
      for (const s of SPARKS) {
        play(
          s.name,
          [
            { opacity: 1, transform: "translate(0, 0) scale(1)" },
            {
              opacity: 1,
              transform: `translate(${s.pull[0]}px, ${s.pull[1]}px) scale(0.2)`,
            },
          ],
          { duration: 180, easing: "ease-in" },
        );
      }
      play("flame", CATCH, { duration: 520, delay: 170, easing: SPRING });
      play("ray", RAY_OUT, {
        duration: 260,
        delay: 600,
        stagger: 90,
        easing: EXPO_OUT,
      });
      play("flap", OPEN, { duration: 360, delay: 960, easing: EXPO_OUT });
      play("pine", SWAY, { duration: 1050, delay: 1240 });
    },
    light({ play }) {
      play("flame", CATCH, { duration: 520, easing: SPRING });
      play("ray", RAY_OUT, {
        duration: 260,
        delay: 400,
        stagger: 90,
        easing: EXPO_OUT,
      });
      play("flap", OPEN, { duration: 360, delay: 520, easing: EXPO_OUT });
    },
    pack({ play }) {
      play(
        "flame",
        [
          { opacity: 1, transform: "scale(1)" },
          { opacity: 1, transform: "scale(0)" },
        ],
        {
          duration: 220,
          easing: "ease-in",
        },
      );
      play(
        "rays",
        [
          { opacity: 1, transform: "scale(1)" },
          { opacity: 0, transform: "scale(0.7)" },
        ],
        {
          duration: 160,
          easing: "ease-in",
        },
      );
      play(
        "flap",
        [
          { opacity: 1, transform: door(1) },
          { opacity: 1, transform: door(SHUT) },
        ],
        {
          duration: 320,
          delay: 140,
          easing: EXPO_OUT,
        },
      );
      play("smoke", SMOKE_UP, { duration: 440, delay: 200, easing: EXPO_OUT });
    },
    rest({ play }) {
      play(
        "smoke",
        [
          { opacity: 1, transform: "translateY(0) scale(1)" },
          { opacity: 0, transform: "translateY(-7px) scale(1.08)" },
        ],
        {
          duration: 420,
          easing: "ease-out",
        },
      );
      play("tent", squash(0.02), {
        duration: 320,
        delay: 120,
        easing: "ease-out",
      });
    },
    fizzle({ play }) {
      // The sparks drop out of the air to where they lie, and the logs are
      // knocked from their cross to where they end up.
      SPARKS.forEach((s, i) => {
        play(
          s.name,
          [
            { opacity: 1, transform: FLOAT },
            { opacity: s.fell ? 1 : 0, transform: s.down },
          ],
          {
            duration: 340,
            delay: i * 50,
            easing: "cubic-bezier(0.5, 0, 0.9, 0.5)",
          },
        );
      });
      play("log-a", [{ transform: CROSSED }, { transform: APART.a }], {
        duration: 360,
        delay: 260,
        easing: SPRING,
      });
      play("log-b", [{ transform: CROSSED }, { transform: APART.b }], {
        duration: 300,
        delay: 300,
        easing: SPRING,
      });
    },
    flare({ play }) {
      play(
        "flame",
        [
          { transform: "scale(1)" },
          { transform: "scale(1.16)", offset: 0.35 },
          { transform: "scale(1)" },
        ],
        {
          duration: 420,
          easing: "ease-out",
        },
      );
    },
  },
};
