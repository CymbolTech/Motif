// Authored icons: one 1.5px stroke, currentColor.

type IconProps = { className?: string };

/**
 * The Motif mark ("Notch"): a disc with its top-right quarter lifted out and
 * squared into a block. Activity, turned into a unit someone has earned.
 */
export function Logo({ className }: IconProps) {
  return (
    <svg className={className} viewBox="0 0 32 32" fill="currentColor" aria-hidden="true">
      <path d="M16 16H29A13 13 0 1 1 16 3Z" />
      <rect x="19" y="3" width="10" height="10" rx="1" />
    </svg>
  );
}

export function Chevron({ className }: IconProps) {
  return (
    <svg className={className} viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M4.5 2.5 8 6l-3.5 3.5" />
    </svg>
  );
}

export function Arrow({ className }: IconProps) {
  return (
    <svg className={className} viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M1.5 6h9M7 2.5 10.5 6 7 9.5" />
    </svg>
  );
}

export function Plus({ className }: IconProps) {
  return (
    <svg className={className} viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M6 1.5v9M1.5 6h9" />
    </svg>
  );
}
