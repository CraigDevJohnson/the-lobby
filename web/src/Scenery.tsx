// The welcome's broad edge shapes: a low sun and foliage at either side, in
// the seafoam family so they frame the greeting without competing with it.

export function LeftFoliage({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 360 520" aria-hidden="true" focusable="false" preserveAspectRatio="xMinYMax meet">
      <path d="M-40 520C-60 380 30 250 150 170c-10 120-40 250-120 350z" fill="var(--leaf-deep)" />
      <path d="M150 170c-40 90-90 200-150 330" stroke="var(--leaf-vein)" strokeWidth="6" fill="none" strokeLinecap="round" />
      <path d="M-20 520c10-150 110-270 240-300-40 110-90 220-170 300z" fill="var(--leaf-mid)" />
      <path d="M220 220C150 300 100 400 70 510" stroke="var(--leaf-vein)" strokeWidth="6" fill="none" strokeLinecap="round" />
      <path d="M40 520c40-110 150-190 290-200-60 90-140 160-230 200z" fill="var(--leaf-soft)" />
      <path d="M330 320c-90 50-170 120-230 200" stroke="var(--leaf-vein-soft)" strokeWidth="5" fill="none" strokeLinecap="round" />
      <path d="M-50 330c60-30 120-20 160 20-60 30-120 40-160-20z" fill="var(--leaf-mid)" />
    </svg>
  );
}

export function RightFoliage({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 300 560" aria-hidden="true" focusable="false" preserveAspectRatio="xMaxYMax meet">
      <path d="M90 560c-30-80-20-180 30-260 40-60 120-90 200-80v340z" fill="var(--leaf-soft)" />
      <path d="M320 70c-80 40-120 140-110 250 10 90 50 170 110 240z" fill="var(--leaf-mid)" />
      <path d="M200 560c0-120 40-230 120-300v300z" fill="var(--leaf-deep)" />
      <path d="M318 270c-60 70-95 170-100 290" stroke="var(--leaf-vein)" strokeWidth="6" fill="none" strokeLinecap="round" />
      <path d="M150 560c-10-60 10-120 60-160 20 60 10 120-20 160z" fill="var(--leaf-deep)" />
    </svg>
  );
}

export function Ground({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 1440 80" aria-hidden="true" focusable="false" preserveAspectRatio="none">
      <path d="M0 40C220 8 420 4 640 22s420 34 560 20 200-30 240-36V80H0z" fill="var(--navy)" />
    </svg>
  );
}
