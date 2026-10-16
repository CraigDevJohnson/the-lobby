// An original tabletop in the Lobby's palette. The small edge light follows
// freshly confirmed responding status; the decorative game pieces never
// imply that a world is open. All status meaning stays in the adjacent text.
export function Tabletop({ lit }: { lit: boolean }) {
  return (
    <svg className="fd-tabletop-art" viewBox="0 0 260 172" aria-hidden="true" focusable="false">
      <path d="m25 140 108-53 107 54-108 27z" fill="var(--ground-shadow)" />

      <g stroke="var(--navy)" strokeWidth="3.5" strokeLinejoin="round">
        <path d="M43 109v37l13 7v-38M208 111v35l-13 7v-35" fill="var(--leaf-soft)" />
        <path d="m21 88 109-53 109 53-109 55z" fill="var(--limb)" />
        <path d="m21 88 109 55v14L21 102z" fill="var(--leaf-soft)" />
        <path d="m130 143 109-55v14l-109 55z" fill="var(--leaf-mid)" />

        <path d="m83 37 30-14v44L83 81z" fill="var(--seafoam-light)" />
        <path d="m113 23 32 16v44l-32-16z" fill="var(--card)" />
        <path d="m145 39 31-15v44l-31 15z" fill="var(--seafoam-light)" />
      </g>

      <g fill="none" stroke="var(--navy)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="m92 49 12-6m-12 14 12-6m-12 14 7-3" />
        <path d="m122 55 7-9 7 17m-15-8 8 4 8 4" />
        <path d="m155 49 11-5m-11 14 11-5m-11 14 7-3" />
      </g>

      <path d="m62 95 60-29 58 29-60 30z" fill="var(--seafoam-light)" stroke="var(--navy)" strokeWidth="3" strokeLinejoin="round" />
      <g fill="none" stroke="var(--navy)" strokeWidth="2.25" strokeLinecap="round" strokeLinejoin="round">
        <path d="m82 85 58 29m-38-39 58 29M82 105l60-29m-40 39 60-29" opacity="0.3" />
        <path d="m88 96 15 7 13-7-1-10 15-7" strokeDasharray="3 5" />
        <path d="m134 95 9-5 7 8-9 5z" fill="var(--limb)" />
      </g>
      <circle cx="88" cy="96" r="3.5" fill="var(--navy)" stroke="var(--navy)" strokeWidth="2" />

      <g stroke="var(--navy)" strokeWidth="3" strokeLinejoin="round">
        <path d="m188 83 16 8-16 9-16-9z" fill="var(--seafoam-light)" />
        <path d="m172 91 16 9v19l-16-8z" fill="var(--limb)" />
        <path d="m188 100 16-9v19l-16 9z" fill="var(--leaf-soft)" />
      </g>
      <g fill="var(--navy)">
        <ellipse cx="188" cy="91" rx="2.5" ry="1.6" />
        <ellipse cx="177.5" cy="101" rx="1.7" ry="2.4" transform="rotate(-25 177.5 101)" />
        <ellipse cx="182.5" cy="110" rx="1.7" ry="2.4" transform="rotate(-25 182.5 110)" />
        <ellipse cx="196" cy="101.5" rx="1.7" ry="2.4" transform="rotate(25 196 101.5)" />
        <ellipse cx="196" cy="109" rx="1.7" ry="2.4" transform="rotate(25 196 109)" />
      </g>

      <path
        d="m67 115 11 5.5v6L67 121z"
        fill={lit ? "var(--custard)" : "var(--navy)"}
        stroke="var(--navy)"
        strokeWidth="2.5"
        strokeLinejoin="round"
      />
      {lit && (
        <path d="m61 116-7-2m13 0-4-7m12 12 2-6" fill="none" stroke="var(--navy)" strokeWidth="2.5" strokeLinecap="round" />
      )}
    </svg>
  );
}
