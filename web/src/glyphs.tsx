import { useId } from "react";
import type { ToolEntry } from "./tools";

// One stroke family for every Tool glyph: 32px grid, 2.5px navy stroke,
// round joins. The admin glyphs are the ordinary glyph plus a small gear, so
// Minecraft Launcher and Minecraft Admin stay distinct side by side.

const Gear = () => (
  <g transform="translate(24 24)">
    <circle r="6.2" fill="currentColor" />
    {[0, 45, 90, 135, 180, 225, 270, 315].map((a) => (
      <rect key={a} x="-1.7" y="-8.4" width="3.4" height="4" rx="0.8" fill="currentColor" transform={`rotate(${a})`} />
    ))}
    <circle r="2.3" fill="var(--card)" />
  </g>
);

const Cube = ({ mask }: { mask?: string }) => (
  <g mask={mask}>
    <path d="M16 3.5 27 9.5v13L16 28.5 5 22.5v-13z" />
    <path d="M5 9.5 16 15.5 27 9.5M16 15.5v13" />
  </g>
);

const F = ({ mask }: { mask?: string }) => (
  <g mask={mask}>
    <path d="M8 28V5h15M8 15.5h11" strokeWidth="5" strokeLinecap="square" strokeLinejoin="miter" />
  </g>
);

const Calendar = () => (
  <g>
    <rect x="4.5" y="6.5" width="23" height="21" rx="4" />
    <path d="M4.5 12.5h23M10.5 3.5v6M21.5 3.5v6" />
    {[10.5, 16, 21.5].map((x) =>
      [17.5, 22.5].map((y) => <circle key={`${x}-${y}`} cx={x} cy={y} r="1.3" fill="currentColor" stroke="none" />),
    )}
  </g>
);

export function ToolGlyph({ glyph }: { glyph: ToolEntry["glyph"] }) {
  const admin = glyph === "cube-admin" || glyph === "foundry-admin";
  const maskId = "gear-cut-" + useId().replace(/[^a-zA-Z0-9_-]/g, "");
  const mask = admin ? `url(#${maskId})` : undefined;
  return (
    <svg className="tool-glyph" viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      {admin && (
        <defs>
          <mask id={maskId} maskUnits="userSpaceOnUse">
            <rect width="32" height="32" fill="white" />
            <circle cx="24" cy="24" r="10.5" fill="black" />
          </mask>
        </defs>
      )}
      <g fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinejoin="round" strokeLinecap="round">
        {glyph === "calendar" && <Calendar />}
        {(glyph === "cube" || glyph === "cube-admin") && <Cube mask={mask} />}
        {(glyph === "foundry" || glyph === "foundry-admin") && <F mask={mask} />}
      </g>
      {admin && <Gear />}
    </svg>
  );
}
