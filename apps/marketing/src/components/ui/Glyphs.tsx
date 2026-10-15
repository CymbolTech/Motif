// Authored glyphs in the same 1.5px stroke as the useanimations set.

type GlyphProps = { size?: number; className?: string };

export function CoinGlyph({ size = 16, className }: GlyphProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="5.5" />
      <path d="M12 9v6" strokeLinecap="square" />
    </svg>
  );
}

export function TargetGlyph({ size = 22, className }: GlyphProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <circle cx="12" cy="12" r="7" />
      <circle cx="12" cy="12" r="2" />
      <path d="M12 1.5v5M12 17.5v5M1.5 12h5M17.5 12h5" strokeLinecap="square" />
    </svg>
  );
}

export function StrikeGlyph({ size = 14, className }: GlyphProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <path d="M5 5l14 14M19 5L5 19" strokeLinecap="square" />
    </svg>
  );
}

export function TickGlyph({ size = 14, className }: GlyphProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <path d="M4 12.5l5 5L20 6.5" strokeLinecap="square" />
    </svg>
  );
}
