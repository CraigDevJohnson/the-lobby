import type { CSSProperties, ReactNode } from "react";
import { MARQUEE, type Bulb } from "./cabinet";
import { ToolGlyph } from "./glyphs";
import type { ToolEntry } from "./tools";

// The VIP Lobby's arcade cabinet, the drawn parts: the marquee's bulbs around
// the title, the selector's corners, the control deck and the launch screen.
// All of it is decorative and hidden from assistive technology; the title,
// Tool names and Open links carry the meaning. useCabinet.ts moves it.

function Bulbs({ bulbs, side }: { bulbs: Bulb[]; side: string }) {
  return (
    <span className={`marquee-bulbs marquee-${side}`}>
      {bulbs.map((b) => (
        <span
          key={b.ring}
          className={b.lit ? "marquee-bulb is-lit" : "marquee-bulb"}
          style={{ "--i": b.ring } as CSSProperties}
        />
      ))}
    </span>
  );
}

// Custard bulbs (drawn light) ringing the title like an arcade marquee.
export function Marquee({ children }: { children: ReactNode }) {
  return (
    <div className="lobby-marquee" style={{ "--bulbs": MARQUEE.count } as CSSProperties}>
      <span className="marquee-lights" aria-hidden="true">
        <Bulbs bulbs={MARQUEE.top} side="top" />
        <Bulbs bulbs={MARQUEE.right} side="right" />
        <Bulbs bulbs={MARQUEE.bottom} side="bottom" />
        <Bulbs bulbs={MARQUEE.left} side="left" />
      </span>
      {children}
    </div>
  );
}

// Each corner on a 36px box: a 3.5px round-capped stroke whose outer edge
// keeps a 22px radius, concentric with the tile's 14px corner 8px inside it.
const CORNERS: [string, string][] = [
  ["tl", "M1.75 34.25V22A20.25 20.25 0 0 1 22 1.75h12.25"],
  ["tr", "M1.75 1.75H14a20.25 20.25 0 0 1 20.25 20.25v12.25"],
  ["bl", "M1.75 1.75V14a20.25 20.25 0 0 0 20.25 20.25h12.25"],
  ["br", "M34.25 1.75V14A20.25 20.25 0 0 1 14 34.25H1.75"],
];

// Four navy corners that close around the picked tile, like a game menu's cursor.
export function Selector() {
  return (
    <span className="cabinet-selector" aria-hidden="true">
      {CORNERS.map(([corner, d]) => (
        <svg key={corner} className={`cabinet-corner cabinet-corner-${corner}`} viewBox="0 0 36 36" focusable="false">
          <path d={d} />
        </svg>
      ))}
    </span>
  );
}

function DeckButton({ x, first = false }: { x: number; first?: boolean }) {
  return (
    <g className={first ? "deck-button deck-button-first" : "deck-button"}>
      <ellipse cx={x} cy="49" rx="17" ry="6.5" className="deck-housing" />
      {/* Domed like the cap, so a pressed cap sinks into a dark rim. */}
      <path d={`M${x - 12.5} 40a12.5 5 0 0 1 25 0v9h-25z`} className="deck-collar" />
      <ellipse cx={x} cy="40" rx="12.5" ry="5" className="deck-cap" />
    </g>
  );
}

// The ledge under the screen: a ball-top joystick that leans toward each move
// and two buttons, the first pressed by Open. Drawn in the Tool glyph stroke.
export function ControlDeck() {
  return (
    <div className="cabinet-deck" aria-hidden="true">
      <svg className="deck-art" viewBox="0 0 400 64" focusable="false">
        <ellipse cx="104" cy="52" rx="22" ry="6.5" className="deck-washer" />
        <g className="deck-stick">
          <path d="M104 51V26" className="deck-shaft" />
          <g className="deck-knob">
            <circle cx="104" cy="18.5" r="12" className="deck-ball" />
            <path d="M97 14.5a8 8 0 0 1 6.5-5" className="deck-shine" />
          </g>
        </g>
        <DeckButton x={262} first />
        <DeckButton x={308} />
      </svg>
    </div>
  );
}

// The screen a Tool's tile grows into while its page loads: a navy edge and
// the seafoam field its page opens on, both opened out from the tile's
// rounded box, with the Tool's glyph and name growing out of the tile's own,
// then the house loading dots if it takes a moment. One scene per visible
// Tool, shown on Open. The shown scene takes the focus and reads "Opening"
// and the Tool's name; the drawing around it is hidden from assistive
// technology.
export function LaunchScreen({ tools }: { tools: ToolEntry[] }) {
  return (
    <div className="cabinet-launch" hidden>
      <div className="cabinet-launch-edge" aria-hidden="true" />
      <div className="cabinet-launch-field" aria-hidden="true" />
      {tools.map((t) => (
        <div key={t.id} className="cabinet-launch-scene" data-tool={t.id} tabIndex={-1} hidden>
          <ToolGlyph glyph={t.glyph} />
          <p className="cabinet-launch-name">
            <span className="visually-hidden">Opening </span>
            {t.name}
          </p>
          <span className="lobby-status-dots" aria-hidden="true">
            <span />
            <span />
            <span />
          </span>
        </div>
      ))}
    </div>
  );
}
