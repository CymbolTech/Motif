'use client';

import { levelFor } from '@/game/catalog';
import { useGame, TOTAL_ACHIEVEMENTS } from '@/game/store';

export function RunSummary() {
  const run = useGame((snapshot) => snapshot.run);
  const level = levelFor(run.xp);
  const unlocked = Object.keys(run.unlocked).length;
  const events = Object.keys(run.receipts).length;
  return (
    <dl className="run-summary mono" aria-label="Your run so far">
      <div>
        <dt>Level</dt>
        <dd>
          {String(level.level).padStart(2, '0')} {level.name}
        </dd>
      </div>
      <div>
        <dt>XP</dt>
        <dd>{run.xp}</dd>
      </div>
      <div>
        <dt>Achievements</dt>
        <dd>
          {unlocked}/{TOTAL_ACHIEVEMENTS}
        </dd>
      </div>
      <div>
        <dt>Events processed</dt>
        <dd>{events}</dd>
      </div>
    </dl>
  );
}
