import type { Mark } from "./logic";

const NAVY = "var(--navy)";

// The mark beside Foundry's state, saying it again in shape so the state
// never rests on colour (DESIGN.md). Drawn in the Tool glyph stroke family:
// 32px grid, 2.5px navy strokes, round joins, on a plain round outline.
// Responding is a filled disc with a pulse, the one mark that is solid;
// stopped carries a stop square; a problem a bang; an unconfirmed state a
// question on a dashed outline. While Foundry starts or shuts down the mark
// is a bare turning arrow, stopped under reduced motion. A stale state keeps
// its own mark, drawn open and still on the dashed outline.
export function StatusMark({ mark, stale = false }: { mark: Mark; stale?: boolean }) {
  const filled = mark === "on" && !stale;
  const dashed = stale || mark === "unknown";
  const turning = mark === "busy" && !stale;
  return (
    <svg className={`fd-mark fd-mark-${mark}`} viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      {!turning && (
        <circle
          cx="16"
          cy="16"
          r="12.75"
          fill={filled ? NAVY : "none"}
          stroke={NAVY}
          strokeWidth="2.5"
          // Ten even dashes all the way round.
          pathLength={dashed ? 80 : undefined}
          strokeDasharray={dashed ? "4.6 3.4" : undefined}
        />
      )}
      <g fill="none" strokeLinecap="round" strokeLinejoin="round">
        {mark === "on" && (
          <path
            d="M7.5 16.5h4l2.25-5 4.5 10 2.25-5h4"
            stroke={filled ? "var(--seafoam-light)" : NAVY}
            strokeWidth="2.6"
          />
        )}
        {turning && (
          <g className="fd-mark-turn" stroke={NAVY} strokeWidth="2.75">
            <path d="M16 4.5A11.5 11.5 0 1 1 4.5 16" />
            <path d="M1.5 19 4.5 16l3 3" />
          </g>
        )}
        {mark === "busy" && stale && (
          <g stroke={NAVY} strokeWidth="2.5">
            <path d="M16 9.5A6.5 6.5 0 1 1 9.5 16" />
            <path d="M7.5 18 9.5 16l2 2" />
          </g>
        )}
        {mark === "problem" && <path d="M16 9.5v7.5" stroke={NAVY} strokeWidth="2.75" />}
        {mark === "unknown" && (
          <path d="M12.6 12.6a3.5 3.5 0 1 1 4.6 3.3c-.8.3-1.2 1-1.2 1.8v.8" stroke={NAVY} strokeWidth="2.6" />
        )}
      </g>
      {mark === "off" && <rect x="11.25" y="11.25" width="9.5" height="9.5" rx="2" fill={NAVY} />}
      {mark === "problem" && <circle cx="16" cy="21.9" r="1.7" fill={NAVY} />}
      {mark === "unknown" && <circle cx="16" cy="22.4" r="1.6" fill={NAVY} />}
    </svg>
  );
}
