// A miniature pitch and calendar, drawn in the Lobby's block materials.
// The calendar gains a check when a Team has been chosen; this decoration
// repeats the selection without making a claim about calendar delivery.
export function PitchCalendar({ selected }: { selected: boolean }) {
  return (
    <svg
      className="soccer-art"
      viewBox="0 0 360 230"
      width="360"
      height="230"
      aria-hidden="true"
      focusable="false"
    >
      <path d="m27 169 143-72 162 81-143 44z" fill="var(--ground-shadow)" />

      <g stroke="var(--navy)" strokeWidth="3.5" strokeLinejoin="round">
        <path d="m27 125 137-69 148 74-138 69z" fill="var(--leaf-soft)" />
        <path d="m27 125 147 74v15L27 140z" fill="var(--limb)" />
        <path d="m174 199 138-69v15l-138 69z" fill="var(--leaf-mid)" />
      </g>

      <g fill="none" stroke="var(--navy)" strokeWidth="2.5" strokeLinejoin="round" strokeLinecap="round">
        <path d="m46 125 118-59 129 64-119 59zM107 94l128 64" />
        <path d="M152 117c10-5 25-5 35 0s10 12 0 17-25 5-35 0-10-12 0-17z" />
        <path d="m78 109 30 15-43 22M252 109l-32 16 43 22" />
      </g>
      <circle cx="169.5" cy="125.5" r="2.5" fill="var(--navy)" />

      <g fill="var(--seafoam-light)" stroke="var(--navy)" strokeWidth="2.5" strokeLinejoin="round">
        <path d="m40 118 25-12v18l-25 13z" />
        <path d="m40 118-8-4 25-13 8 5" />
        <path d="m40 137-8-4v-19M40 118v19" fill="none" />
        <path d="m276 117 24 12v19l-24-12z" />
        <path d="m276 117 8-4 24 12-8 4M300 148l8-4v-19" fill="none" />
      </g>

      <g transform="matrix(1 .5 0 1 235 20)" stroke="var(--navy)" strokeWidth="3.2" strokeLinejoin="round">
        <path d="m0 9 10-5h78v92l-10 5z" fill="var(--leaf-soft)" />
        <rect x="0" y="9" width="78" height="92" rx="5" fill="var(--seafoam-light)" />
        <path d="M5 9h68a5 5 0 0 1 5 5v17H0V14a5 5 0 0 1 5-5z" fill="var(--navy)" />
        <path d="M18 4v14M59 4v14" strokeLinecap="round" />
        <path d="M15 47h12m15 0h20M15 61h12m15 0h20M15 75h12" strokeWidth="2.5" strokeLinecap="round" />
        {selected ? (
          <g className="soccer-art-check">
            <circle cx="60" cy="81" r="19" fill="var(--navy)" />
            <path d="m50 81 6 6 13-14" fill="none" stroke="var(--seafoam-light)" strokeLinecap="round" />
          </g>
        ) : (
          <path d="M42 75h20" strokeWidth="2.5" strokeLinecap="round" />
        )}
      </g>
    </svg>
  );
}
