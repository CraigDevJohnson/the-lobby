import type { ReactNode } from "react";
import { Robot } from "./Robot";

// The Tool pages' shared ground: stepped navy blocks instead of the welcome's wave,
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
    <div className={`tool-terrace-run ${className}`}>
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

function Terrace({ children }: { children?: ReactNode }) {
  return (
    <div className="tool-terrace" aria-hidden="true">
      <Run steps={BEFORE} before={99} after={LANDING} className="tool-terrace-before" />
      <div className="tool-landing" style={{ height: `${LANDING * UNIT}rem` }}>
        {children}
      </div>
      <Run steps={AFTER} before={LANDING} after={99} className="tool-terrace-after" />
    </div>
  );
}

export function ToolFooter({ waveKey = 0 }: { waveKey?: number }) {
  return (
    <footer className="tool-footer">
      <Terrace>
        <Robot key={waveKey} className="tool-host" wave={waveKey > 0} />
      </Terrace>
      <div className="tool-footer-bar">
        <a href="/privacy">Privacy</a>
      </div>
    </footer>
  );
}
