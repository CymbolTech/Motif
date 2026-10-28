'use client';

import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { Arrow, Logo } from '@/components/Icons';
import { ACHIEVEMENTS, type AchievementId, getRun, getServerRun, level, onAward, subscribe } from '@/lib/run';
import { DASHBOARD_URL } from '@/lib/site';

// The visitor's level and XP in the header. Opens "Your run": progress, the
// achievements, and one line on why the page plays like this.

const format = new Intl.NumberFormat('en-US');

export function RunPill() {
  const run = useSyncExternalStore(subscribe, getRun, getServerRun);
  const [mounted, setMounted] = useState(false);
  const [open, setOpen] = useState(false);
  const [gain, setGain] = useState<{ key: number; xp: number } | null>(null);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => setMounted(true), []);

  useEffect(
    () =>
      onAward((award) => {
        if (award.xp > 0) setGain({ key: award.key, xp: award.xp });
      }),
    [],
  );

  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent | KeyboardEvent) => {
      if (event instanceof KeyboardEvent ? event.key === 'Escape' : !root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', close);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', close);
    };
  }, [open]);

  const current = level(run.xp);

  return (
    <div ref={root} className={`run ${mounted ? 'run-ready' : ''}`}>
      <button type="button" className="run-pill" aria-expanded={open} aria-controls="run-panel" onClick={() => setOpen((value) => !value)}>
        <Logo className="run-mark" />
        <span className="run-level">Lv {current.number}</span>
        <span className="run-xp num">{format.format(run.xp)} XP</span>
        <span className="run-bar" aria-hidden="true">
          <i style={{ transform: `scaleX(${current.progress})` }} />
        </span>
        {gain && (
          <span key={gain.key} className="run-gain num" aria-hidden="true">
            +{gain.xp}
          </span>
        )}
      </button>

      {open && (
        <div id="run-panel" className="run-panel" role="dialog" aria-label="Your run">
          <div className="run-panel-head">
            <span>Your run</span>
            <span className="num">
              {current.name} · {format.format(run.xp)} XP
            </span>
          </div>
          <div className="bar run-panel-bar" aria-hidden="true">
            <i style={{ transform: `scaleX(${current.progress})` }} />
          </div>
          <p className="run-next">{current.next ? `${current.next - run.xp} XP to level ${current.number + 1}` : 'Top level reached'}</p>
          <ul className="run-achievements">
            {(Object.keys(ACHIEVEMENTS) as AchievementId[]).map((id) => {
              const earned = run.unlocked.includes(id);
              return (
                <li key={id} className={earned ? 'is-earned' : undefined}>
                  <span className="run-badge" aria-hidden="true" />
                  <span>
                    {ACHIEVEMENTS[id].title}
                    <small>{earned ? 'Unlocked' : ACHIEVEMENTS[id].hint}</small>
                  </span>
                </li>
              );
            })}
          </ul>
          <p className="run-why">
            This page runs on Motif’s own mechanics: every section you read is an event, a rule awards the XP, and achievements unlock at a count.
          </p>
          <a className="run-cta" href={DASHBOARD_URL}>
            Build this into your product
            <Arrow />
          </a>
        </div>
      )}
    </div>
  );
}
