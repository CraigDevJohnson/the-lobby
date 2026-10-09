// The Lobby's small fictional host. Decorative everywhere it appears, so it
// is hidden from assistive technology; the page's words carry the meaning.

const NAVY = "var(--navy)";

export function Robot({ className, wave = false }: { className?: string; wave?: boolean }) {
  return (
    <svg className={className} viewBox="0 0 128 168" aria-hidden="true" focusable="false">
      <ellipse cx="66" cy="160" rx="44" ry="6.5" fill="var(--ground-shadow)" />
      {/* legs and feet */}
      <g stroke={NAVY} strokeWidth="4" strokeLinejoin="round">
        <rect x="47" y="118" width="12" height="22" rx="3" fill={NAVY} />
        <rect x="71" y="118" width="12" height="22" rx="3" fill={NAVY} />
        <path d="M36 152a10 10 0 0 1 10-12h14v14H38a2 2 0 0 1-2-2z" fill="var(--coral)" />
        <path d="M94 152a10 10 0 0 0-10-12H70v14h22a2 2 0 0 0 2-2z" fill="var(--coral)" />
      </g>
      {/* resting arm */}
      <g strokeLinecap="round" fill="none">
        <path d="M86 92c8 4 12 12 14 22" stroke={NAVY} strokeWidth="12" />
        <path d="M86 92c8 4 12 12 14 22" stroke="var(--limb)" strokeWidth="5.5" strokeDasharray="5 3.5" />
        <circle cx="100.5" cy="119" r="6.5" fill="var(--limb)" stroke={NAVY} strokeWidth="3.5" />
      </g>
      {/* body */}
      <rect x="40" y="80" width="50" height="42" rx="15" fill="var(--coral)" stroke={NAVY} strokeWidth="4" />
      <circle cx="65" cy="99" r="7.5" fill="var(--custard)" stroke={NAVY} strokeWidth="3.5" />
      <rect x="57" y="72" width="16" height="10" rx="2" fill={NAVY} />
      {/* waving arm */}
      <g className={wave ? "robot-wave" : undefined} style={{ transformOrigin: "44px 92px" }}>
        <g strokeLinecap="round" fill="none">
          <path d="M44 92c-10-2-17-10-20-22" stroke={NAVY} strokeWidth="12" />
          <path d="M44 92c-10-2-17-10-20-22" stroke="var(--limb)" strokeWidth="5.5" strokeDasharray="5 3.5" />
        </g>
        <path
          d="M17 64c-3-5 0-10 4-10 2 0 3 2 3 4l1-7c1-3 6-3 6 1v9c3-2 7 0 6 4-1 3-5 6-10 6-5 0-8-3-10-7z"
          fill="var(--limb)"
          stroke={NAVY}
          strokeWidth="3.5"
          strokeLinejoin="round"
        />
      </g>
      {/* head */}
      <g stroke={NAVY} strokeWidth="4">
        <path d="M65 28V14" strokeLinecap="round" />
        <circle cx="65" cy="10" r="6" fill="var(--coral)" strokeWidth="3.5" />
        <circle cx="29" cy="50" r="8.5" fill="var(--custard)" strokeWidth="3.5" />
        <circle cx="101" cy="50" r="8.5" fill="var(--custard)" strokeWidth="3.5" />
        <rect x="31" y="27" width="68" height="48" rx="19" fill="var(--coral)" />
      </g>
      <rect x="41" y="37" width="48" height="29" rx="12" fill={NAVY} />
      <path
        d="M52 54q5-7 10 0M68 54q5-7 10 0"
        fill="none"
        stroke="var(--seafoam-light)"
        strokeWidth="3.5"
        strokeLinecap="round"
      />
    </svg>
  );
}
