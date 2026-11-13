import { lit, waiting } from "../plot";
import { at, box, line, onLeft, onRight, origin, shape, U } from "./iso";
import { FINE, INK, NAVY, rays, Slab, Solid } from "./parts";
import { EXPO_OUT, SPRING, type ArtProps, type Plot } from "./stage";

// The block plot: the Launcher's server block standing on a plot of its own.
// It is what a world gets until someone draws its place, and what the one
// unnamed server of a backend that lists no worlds gets. A named world is told
// apart by its initial on the block's top and by its flag (plot.ts, Variant),
// which rides at the foot of its pole until the world is ready and at the top
// while it is. The lamp is the plot's one light: custard with rays while the
// world is ready, navy otherwise.

const BLOCK = [1.2, 1.2, 0, 3.6, 3.6, 3.7] as const;
const [BX, BY, , BW, BL, BT] = BLOCK;
const FACES = box(...BLOCK);
const L = onLeft(...BLOCK);
const R = onRight(...BLOCK);

const LAMP = L(0.5, 0.3);
const LAMP_RAYS = rays(LAMP, [-90, -148, -32, 180, 0], 8.8, 4);
const FOOT = at(BX + BW, BY + BL, 0);

const slots = (n: 2 | 3) => (n === 3 ? [0.88, 0.76, 0.64] : [0.86, 0.72]).map((v) => line(L(0.17, v), L(0.83, v))).join("");
const GRILLE = [0.3, 0.5, 0.7].map((u) => line(R(u, 0.36), R(u, 0.74))).join("");

// The pole stands just inside one of the top's side corners and the flag
// flies outward from it, clear of the initial.
const POLE_TALL = 2.5;
const DROP = (POLE_TALL - 1.05) * 9;
const pole = (side: "left" | "right") => (side === "left" ? ([BX + 0.42, BY + BL - 0.42] as const) : ([BX + BW - 0.42, BY + 0.42] as const));

function flag(shape_: "point" | "swallow" | "square", side: "left" | "right") {
  const [px, py] = pole(side);
  const [x, y] = at(px, py, BT + POLE_TALL - 0.12);
  const d = side === "left" ? -1 : 1;
  const p = (dx: number, dy: number) => [x + d * dx, y + dy] as const;
  if (shape_ === "point") return shape(p(0, 0), p(13, 4.25), p(0, 8.5));
  if (shape_ === "swallow") return shape(p(0, 0), p(13.5, 0), p(9, 4.25), p(13.5, 8.5), p(0, 8.5));
  return shape(p(0, 0), p(11.5, 0), p(11.5, 8.5), p(0, 8.5));
}

// The initial lies on the top face, reading along the block's x edge.
const TOP = at(BX + BW / 2, BY + BL / 2, BT);
const ON_TOP = `matrix(0.92 0.46 -0.92 0.46 ${TOP[0]} ${TOP[1]})`;

function Art({ state, variant, initial }: ArtProps) {
  const on = lit(state);
  const [px, py] = pole(variant?.pole ?? "left");
  return (
    <>
      <Slab top="var(--seafoam)" left="var(--leaf-soft)" right="var(--leaf-mid)" />
      <g data-part="block" style={{ transformOrigin: origin(FOOT) }}>
        <Solid faces={FACES} top="var(--seafoam-light)" left="var(--limb)" right="var(--leaf-soft)" />
        <path d={slots(variant?.slots ?? 2)} fill="none" {...INK} />
        <path d={GRILLE} fill="none" {...FINE} />
        {initial && (
          <text
            className="plot-initial"
            transform={ON_TOP}
            textAnchor="middle"
            dominantBaseline="central"
            fontSize={U * 2}
            fill={NAVY}
          >
            {initial}
          </text>
        )}
        {variant && (
          <>
            <path d={line(at(px, py, BT), at(px, py, BT + POLE_TALL))} fill="none" {...INK} />
            <path
              data-part="flag"
              className="plot-face"
              d={flag(variant.flag, variant.pole)}
              fill="var(--seafoam-light)"
              style={{ transform: on ? undefined : `translateY(${state === "stopping" ? DROP / 2 : DROP}px)` }}
              {...FINE}
            />
          </>
        )}
        {/* A ring round the dark lamp while the world is on its way up or
            down; each status answer sends it out once. */}
        <ellipse
          data-part="ping"
          cx={LAMP[0]}
          cy={LAMP[1]}
          rx="8"
          ry="8.7"
          fill="none"
          style={{ transformOrigin: origin(LAMP), opacity: waiting(state) ? 1 : 0 }}
          {...FINE}
        />
        <ellipse
          className="plot-dark"
          cx={LAMP[0]}
          cy={LAMP[1]}
          rx="5"
          ry="5.5"
          transform={`rotate(-12 ${LAMP[0]} ${LAMP[1]})`}
          fill={NAVY}
          {...FINE}
        />
        <g data-part="light" style={{ transformOrigin: origin(LAMP), opacity: on ? 1 : 0 }}>
          <ellipse
            cx={LAMP[0]}
            cy={LAMP[1]}
            rx="5"
            ry="5.5"
            transform={`rotate(-12 ${LAMP[0]} ${LAMP[1]})`}
            fill="var(--custard)"
            {...FINE}
          />
          <path data-part="rays" d={LAMP_RAYS} fill="none" style={{ transformOrigin: origin(LAMP) }} {...FINE} />
        </g>
      </g>
    </>
  );
}

// The block's hop, as the VIP Lobby's block glyph hops: a squat, a lift, a
// squashed landing.
const HOP: Keyframe[] = [
  { transform: "translateY(0) scale(1, 1)", easing: "ease-out" },
  { transform: "translateY(0) scale(1.06, 0.9)", offset: 0.16, easing: "cubic-bezier(0.3, 0, 0.2, 1)" },
  { transform: "translateY(-7px) scale(0.97, 1.05)", offset: 0.48, easing: "cubic-bezier(0.5, 0, 0.8, 0.5)" },
  { transform: "translateY(0) scale(1.08, 0.88)", offset: 0.72, easing: "ease-out" },
  { transform: "translateY(0) scale(0.98, 1.03)", offset: 0.88 },
  { transform: "translateY(0) scale(1, 1)" },
];

const LIGHT_ON: Keyframe[] = [
  { opacity: 0, transform: "scale(0.4)" },
  { opacity: 1, transform: "scale(0.4)", offset: 0.01 },
  { opacity: 1, transform: "scale(1)" },
];

const RAISE: Keyframe[] = [{ transform: `translateY(${DROP}px)` }, { transform: "translateY(0)" }];

export const block: Plot = {
  Art,
  moves: {
    strike({ play }) {
      play("block", HOP, { duration: 520 });
    },
    beat({ play }) {
      play("ping", [{ transform: "scale(0.7)" }, { transform: "scale(1.25)", offset: 0.5 }, { transform: "scale(1)" }], {
        duration: 420,
        easing: EXPO_OUT,
      });
    },
    wake({ play }) {
      play("block", HOP, { duration: 620 });
      play("light", LIGHT_ON, { duration: 420, delay: 440, easing: SPRING });
      play("rays", [{ transform: "scale(0.5)", opacity: 0 }, { transform: "scale(1)", opacity: 1 }], {
        duration: 320,
        delay: 620,
        easing: EXPO_OUT,
      });
      play("flag", RAISE, { duration: 520, delay: 700, easing: SPRING });
    },
    light({ play }) {
      play("light", LIGHT_ON, { duration: 420, easing: SPRING });
      play("flag", RAISE, { duration: 520, delay: 120, easing: SPRING });
    },
    pack({ play }) {
      play("light", [{ opacity: 1, transform: "scale(1)" }, { opacity: 1, transform: "scale(0.3)" }], {
        duration: 220,
        easing: "ease-in",
      });
      play("flag", [{ transform: "translateY(0)" }, { transform: `translateY(${DROP / 2}px)` }], {
        duration: 480,
        delay: 120,
        easing: EXPO_OUT,
      });
    },
    rest({ play }) {
      play("flag", [{ transform: `translateY(${DROP / 2}px)` }, { transform: `translateY(${DROP}px)` }], {
        duration: 320,
        easing: EXPO_OUT,
      });
      play(
        "block",
        [{ transform: "scale(1, 1)" }, { transform: "scale(1.03, 0.96)", offset: 0.4 }, { transform: "scale(1, 1)" }],
        { duration: 320, easing: "ease-out" },
      );
    },
    fizzle({ play }) {
      play(
        "block",
        [0, -2, 2, -1.5, 1, 0].map((x) => ({ transform: `translateX(${x}px)` })),
        { duration: 380, easing: "ease-out" },
      );
    },
    flare({ play }) {
      play("light", [{ transform: "scale(1)" }, { transform: "scale(1.28)", offset: 0.35 }, { transform: "scale(1)" }], {
        duration: 420,
        easing: "ease-out",
      });
    },
  },
};
