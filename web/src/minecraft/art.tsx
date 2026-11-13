import type { Description } from "./logic";

// The Launcher's block motif (DESIGN.md, "The block motif"): drawn in the
// Lobby's own palette and stroke family, never Minecraft's art or colours.
// Every piece is decorative and hidden from assistive technology; the words
// beside it carry the meaning. Each world's plot is drawn in plots/.

type Mark = Description["mark"];

// The status glyph repeats the state label in shape. Most sit on a little
// block silhouette: a tick on a filled block when ready, a resting moon when
// stopped, a bang for a problem, a question on a dashed outline when
// unconfirmed. While busy it is a bare turning arrow. Every mark is always
// drawn and the stylesheet shows the one the state calls for, so one mark
// turns into the next instead of swapping: the block's outline unwinds as the
// arrow takes over, and the tick draws itself on.
const HEX = "M16 2.75 28.25 9.25v13.5L16 29.25 3.75 22.75V9.25z";
export function StatusGlyph({ mark }: { mark: Mark }) {
  return (
    <svg className="mc-glyph" data-mark={mark} viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      <path className="mc-glyph-hex" d={HEX} pathLength="1" />
      {/* Even dashes all the way round the block. */}
      <path className="mc-glyph-ghost" d={HEX} strokeDasharray="4.4 3.1" />
      <path className="mc-glyph-tick" d="M10.4 16.4l3.9 3.9 7.4-8.2" pathLength="1" />
      <path className="mc-glyph-moon" d="M19.6 9.6a7 7 0 1 0 2.8 10.6 5.6 5.6 0 0 1-2.8-10.6z" />
      <g className="mc-glyph-busy">
        <g className="mc-glyph-turn">
          <path d="M16 4.5A11.5 11.5 0 1 1 4.5 16" />
          <path d="M1.5 19 4.5 16l3 3" />
        </g>
      </g>
      <g className="mc-glyph-bang">
        <path d="M16 9.5v7.5" />
        <circle cx="16" cy="21.9" r="1.7" />
      </g>
      <g className="mc-glyph-ask">
        <path d="M12.6 12.6a3.5 3.5 0 1 1 4.6 3.3c-.8.3-1.2 1-1.2 1.8v.8" />
        <circle cx="16" cy="22.4" r="1.6" />
      </g>
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
