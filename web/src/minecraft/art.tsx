import type { ReactNode } from "react";
import type { Description } from "./logic";

// The Launcher's block motif (DESIGN.md, "The block motif"): drawn in the
// Lobby's own palette and stroke family, never Minecraft's art or colours.
// Every piece is decorative and hidden from assistive technology; the words
// beside it carry the meaning.

type Mark = Description["mark"];

// The status glyph repeats the state label in shape. Most sit on a little
// block silhouette: a tick on a filled block when ready, a resting moon when
// stopped, a bang for a problem, a question on a dashed outline when
// unconfirmed. While busy it is a bare turning arrow.
const HEX = "M16 2.75 28.25 9.25v13.5L16 29.25 3.75 22.75V9.25z";
export function StatusGlyph({ mark }: { mark: Mark }) {
  const filled = mark === "on";
  const outline = {
    fill: filled ? "var(--navy)" : "none",
    stroke: "var(--navy)",
    strokeWidth: 2.5,
    strokeLinejoin: "round" as const,
    // Even dashes all the way round the block.
    strokeDasharray: mark === "unknown" ? "4.4 3.1" : undefined,
  };
  return (
    <svg className={`mc-glyph mc-glyph-${mark}`} viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      {mark !== "busy" && <path d={HEX} {...outline} />}
      <g fill="none" strokeLinecap="round" strokeLinejoin="round">
        {mark === "on" && <path d="M10.4 16.4l3.9 3.9 7.4-8.2" stroke="var(--seafoam-light)" strokeWidth="3.2" />}
        {mark === "off" && (
          <path
            d="M19.6 9.6a7 7 0 1 0 2.8 10.6 5.6 5.6 0 0 1-2.8-10.6z"
            fill="var(--navy)"
            stroke="var(--navy)"
            strokeWidth="1.5"
          />
        )}
        {mark === "busy" && (
          <g className="mc-glyph-turn" stroke="var(--navy)" strokeWidth="2.75">
            <path d="M16 4.5A11.5 11.5 0 1 1 4.5 16" />
            <path d="M1.5 19 4.5 16l3 3" />
          </g>
        )}
        {mark === "problem" && <path d="M16 9.5v7.5" stroke="var(--navy)" strokeWidth="2.75" />}
        {mark === "unknown" && (
          <path d="M12.6 12.6a3.5 3.5 0 1 1 4.6 3.3c-.8.3-1.2 1-1.2 1.8v.8" stroke="var(--navy)" strokeWidth="2.6" />
        )}
      </g>
      {mark === "problem" && <circle cx="16" cy="21.9" r="1.7" fill="var(--navy)" />}
      {mark === "unknown" && <circle cx="16" cy="22.4" r="1.6" fill="var(--navy)" />}
    </svg>
  );
}

// Device glyphs for the joining tabs, in the Tool glyph stroke family.
export function DeviceGlyph({ device }: { device: "java" | "bedrock" | "console" }) {
  return (
    <svg className="mc-device" viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      <g fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinejoin="round" strokeLinecap="round">
        {device === "java" && (
          <>
            <rect x="3.5" y="5.5" width="25" height="16.5" rx="3" />
            <path d="M11.5 27h9M16 22v5" />
          </>
        )}
        {device === "bedrock" && (
          <>
            <rect x="3.5" y="4" width="17" height="23.5" rx="3" />
            <rect x="16.5" y="11" width="12" height="18" rx="2.75" fill="var(--seafoam)" />
            <path d="M21 25.5h3" />
          </>
        )}
        {device === "console" && (
          <>
            <path d="M9.6 9.5h12.8a6 6 0 0 1 5.8 4.5l1.5 6.6a3.4 3.4 0 0 1-5.9 2.9l-2.6-3H10.8l-2.6 3a3.4 3.4 0 0 1-5.9-2.9L3.8 14a6 6 0 0 1 5.8-4.5z" />
            <path d="M10.5 13.2v4.6M8.2 15.5h4.6" />
            <circle cx="21.5" cy="14.2" r="1.45" fill="currentColor" stroke="none" />
            <circle cx="24.4" cy="17" r="1.45" fill="currentColor" stroke="none" />
          </>
        )}
      </g>
    </svg>
  );
}

// The server block: an original isometric block in the Lobby's
// own palette (no Minecraft art or colours), with two drive slots and a lamp
// on its front face. The lamp is lit custard, with rays, while the server is
// ready; dark navy otherwise.
export function ServerBlock({ lit }: { lit: boolean }) {
  // Front face: origin (18,34), across (42,21), down (0,52).
  const at = (u: number, v: number) => [18 + 42 * u, 34 + 21 * u + 52 * v] as const;
  const line = (u1: number, u2: number, v: number) => {
    const [x1, y1] = at(u1, v);
    const [x2, y2] = at(u2, v);
    return `M${x1} ${y1}L${x2} ${y2}`;
  };
  const [lx, ly] = at(0.5, 0.68);
  const rays = [-90, -145, -35, 180, 0].map((deg) => {
    const r = (deg * Math.PI) / 180;
    return `M${lx + Math.cos(r) * 11.5} ${ly + Math.sin(r) * 11.5}l${Math.cos(r) * 5} ${Math.sin(r) * 5}`;
  });
  return (
    <svg
      className={lit ? "mc-block mc-block-lit" : "mc-block"}
      viewBox="0 0 120 124"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M60 70 108 94 60 118 12 94z" fill="var(--ground-shadow)" />
      <g stroke="var(--navy)" strokeWidth="3.5" strokeLinejoin="round">
        <path d="M60 13 102 34 60 55 18 34z" fill="var(--seafoam-light)" />
        <path d="M18 34 60 55v52L18 86z" fill="var(--limb)" />
        <path d="M60 55 102 34v52l-42 21z" fill="var(--leaf-soft)" />
      </g>
      <g fill="none" stroke="var(--navy)" strokeWidth="3.5" strokeLinecap="round">
        <path d={line(0.16, 0.84, 0.12)} />
        <path d={line(0.16, 0.84, 0.25)} />
      </g>
      {/* The right face's grille. */}
      <g fill="none" stroke="var(--navy)" strokeWidth="3" strokeLinecap="round">
        <path d="M71 62v24M81 57v24M91 52v24" />
      </g>
      <ellipse
        cx={lx}
        cy={ly}
        rx="7"
        ry="7.7"
        transform={`rotate(-12 ${lx} ${ly})`}
        fill={lit ? "var(--custard)" : "var(--navy)"}
        stroke="var(--navy)"
        strokeWidth="3"
      />
      {lit && (
        <path
          className="mc-block-rays"
          d={rays.join("")}
          fill="none"
          stroke="var(--navy)"
          strokeWidth="2.75"
          strokeLinecap="round"
        />
      )}
    </svg>
  );
}

// The Launcher's ground: stepped navy blocks instead of the welcome's wave,
// in whole block heights. The landing under the left column, the
// highest step, is where the host stands. Only a step's exposed top corners
// are rounded, so the steps read as one terrace, not as separate bars.
type Step = { h: number; w: number };
const UNIT = 1.5;
const LANDING = 3;
const BEFORE: Step[] = [
  { h: 1, w: 1 },
  { h: 2, w: 1.2 },
];
const AFTER: Step[] = [
  { h: 2, w: 3 },
  { h: 1, w: 6 },
  { h: 2, w: 2.5 },
  { h: 1, w: 4 },
];

function corners(prev: number, h: number, next: number) {
  const r = "0.4rem";
  return `${prev < h ? r : "0"} ${next < h ? r : "0"} 0 0`;
}

function Run({ steps, before, after, className }: { steps: Step[]; before: number; after: number; className: string }) {
  return (
    <div className={`mc-terrace-run ${className}`}>
      {steps.map((s, i) => (
        <span
          key={i}
          style={{
            height: `${s.h * UNIT}rem`,
            flexGrow: s.w,
            borderRadius: corners(
              i === 0 ? before : steps[i - 1].h,
              s.h,
              i === steps.length - 1 ? after : steps[i + 1].h,
            ),
          }}
        />
      ))}
    </div>
  );
}

export function Terrace({ children }: { children?: ReactNode }) {
  return (
    <div className="mc-terrace" aria-hidden="true">
      <Run steps={BEFORE} before={99} after={LANDING} className="mc-terrace-before" />
      <div className="mc-landing" style={{ height: `${LANDING * UNIT}rem` }}>
        {children}
      </div>
      <Run steps={AFTER} before={LANDING} after={99} className="mc-terrace-after" />
    </div>
  );
}
