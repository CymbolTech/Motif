'use client';

import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { ACHIEVEMENTS, type AchievementId, countEvents, earn, onAward, unlock } from '@/lib/run';

// Turns reading into events for the visitor's run, and shows achievement
// toasts. Blocks opt in with data-earn="<event id>:<xp>"; building-block
// tiles, FAQ answers and the Start building links are wired here. Like the
// motion layer it lives in the root layout, so it re-arms on every route.

const TILE_COUNT = 10;

export function Game() {
  const pathname = usePathname();
  const [toasts, setToasts] = useState<{ key: number; id: AchievementId }[]>([]);

  useEffect(
    () =>
      onAward((award) => {
        if (!award.achievement) return;
        const toast = { key: award.key, id: award.achievement };
        setToasts((current) => [...current.slice(-2), toast]);
        window.setTimeout(() => setToasts((current) => current.filter((t) => t.key !== toast.key)), 4800);
      }),
    [],
  );

  useEffect(() => {
    if (pathname.startsWith('/docs')) unlock('went-deeper');

    // Reading: a block counts once it is properly on screen.
    const seen = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const [id, xp] = (entry.target as HTMLElement).dataset.earn!.split(':');
          earn(id, Number(xp) || 10);
          if (id === 'read.close') unlock('whole-pitch');
          seen.unobserve(entry.target);
        }
      },
      { threshold: 0.35 },
    );
    document.querySelectorAll<HTMLElement>('[data-earn]').forEach((element) => seen.observe(element));

    // Exploring: hovering or focusing a building block.
    const explore = (event: Event) => {
      const tile = (event.target as Element).closest?.('.tile');
      if (!tile) return;
      const name = tile.querySelector('.tile-title')?.textContent ?? '';
      if (earn(`explore.${name.toLowerCase().replace(/\W+/g, '-')}`, 5) && countEvents('explore.') >= TILE_COUNT) unlock('explorer');
    };
    document.addEventListener('pointerover', explore);
    document.addEventListener('focusin', explore);

    // Asking: opening an FAQ answer.
    const ask = (event: Event) => {
      const details = event.target as HTMLDetailsElement;
      if (!details.open || !details.closest('.faq')) return;
      const question = details.querySelector('h3')?.textContent ?? '';
      if (earn(`ask.${question.toLowerCase().replace(/\W+/g, '-')}`, 10) && countEvents('ask.') >= 3) unlock('curious');
    };
    document.addEventListener('toggle', ask, true);

    // Starting: any link into the console.
    const start = (event: MouseEvent) => {
      const link = (event.target as Element).closest?.('a[data-start]');
      if (!link) return;
      earn('start.console', 50);
      unlock('pressed-start');
    };
    document.addEventListener('click', start);

    return () => {
      seen.disconnect();
      document.removeEventListener('pointerover', explore);
      document.removeEventListener('focusin', explore);
      document.removeEventListener('toggle', ask, true);
      document.removeEventListener('click', start);
    };
  }, [pathname]);

  return (
    <div className="toasts" aria-live="polite">
      {toasts.map((toast) => (
        <div key={toast.key} className="toast" role="status">
          <span className="toast-dot" aria-hidden="true" />
          <span>
            <small>Achievement unlocked</small>
            {ACHIEVEMENTS[toast.id].title}
          </span>
        </div>
      ))}
    </div>
  );
}
