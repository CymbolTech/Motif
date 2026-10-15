'use client';

import { useEffect } from 'react';
import { emit } from '@/game/store';

const MILESTONES = [0.25, 0.5, 0.75, 1];

/** Emits page.scrolled once per depth milestone; the event id makes repeats duplicates. */
export function ScrollWatch() {
  useEffect(() => {
    const reached = new Set<number>();
    let frame = 0;
    const check = () => {
      frame = 0;
      const scrollable = document.documentElement.scrollHeight - window.innerHeight;
      if (scrollable <= 0) return;
      const depth = window.scrollY / scrollable;
      for (const milestone of MILESTONES) {
        const threshold = milestone === 1 ? 0.985 : milestone;
        if (depth >= threshold && !reached.has(milestone)) {
          reached.add(milestone);
          emit({ id: `scroll-${milestone * 100}`, type: 'page.scrolled', payload: { depth: milestone } });
        }
      }
    };
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(check);
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', onScroll);
      cancelAnimationFrame(frame);
    };
  }, []);
  return null;
}
