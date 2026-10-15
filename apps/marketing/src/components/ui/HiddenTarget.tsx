'use client';

import { useState } from 'react';
import { emit, useGame } from '@/game/store';
import { TargetGlyph } from './Glyphs';

/** One of three light-gun targets hidden in the page; hitting all three unlocks Sharpshooter. */
export function HiddenTarget({ id, className }: { id: string; className?: string }) {
  const hit = useGame((snapshot) => Boolean(snapshot.run.receipts[`target-${id}`]));
  const [burst, setBurst] = useState(false);
  return (
    <button
      type="button"
      className={`target ${className ?? ''}`}
      data-hit={hit || undefined}
      data-burst={burst || undefined}
      aria-label={hit ? 'Target already hit' : 'Hidden target'}
      onClick={() => {
        emit({ id: `target-${id}`, type: 'target.hit' });
        setBurst(true);
        window.setTimeout(() => setBurst(false), 700);
      }}
    >
      <TargetGlyph size={20} />
    </button>
  );
}
