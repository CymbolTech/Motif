'use client';

import { useEffect } from 'react';
import { emit, hydrateRun, useGame, THEME_KEY } from '@/game/store';
import { BootSequence } from './BootSequence';
import { Crosshair } from './Crosshair';
import { QuestLog } from './QuestLog';
import { SmoothScroll } from './SmoothScroll';
import { Toasts } from './Toasts';

const KONAMI = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'];

/** Client-only runtime: restores the run, listens for page-wide behaviours, renders the HUD overlays. */
export function GameRuntime() {
  const hydrated = useGame((snapshot) => snapshot.hydrated);
  const nightShift = useGame((snapshot) => Boolean(snapshot.run.unlocks['night-shift']));

  useEffect(() => {
    hydrateRun();
  }, []);

  // The engine starts on the visitor's first move, so the first toast never lands on an untouched hero.
  useEffect(() => {
    if (!hydrated) return;
    const start = () => {
      events.forEach((type) => window.removeEventListener(type, start));
      emit({ id: 'cold-boot', type: 'session.started' });
    };
    const events = ['scroll', 'pointerdown', 'keydown'] as const;
    events.forEach((type) => window.addEventListener(type, start, { passive: true }));
    return () => events.forEach((type) => window.removeEventListener(type, start));
  }, [hydrated]);

  useEffect(() => {
    if (!nightShift) return;
    let preference: string | null = null;
    try {
      preference = window.localStorage.getItem(THEME_KEY);
    } catch {
      preference = null;
    }
    if (preference !== 'day') document.documentElement.dataset.theme = 'night';
  }, [nightShift]);

  useEffect(() => {
    let position = 0;
    const onKey = (event: KeyboardEvent) => {
      const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
      position = key === KONAMI[position] ? position + 1 : key === KONAMI[0] ? 1 : 0;
      if (position === KONAMI.length) {
        position = 0;
        emit({ id: 'konami', type: 'konami.entered' });
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return (
    <>
      <SmoothScroll />
      <BootSequence />
      <Crosshair />
      <QuestLog />
      <Toasts />
    </>
  );
}
