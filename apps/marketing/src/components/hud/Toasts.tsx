'use client';

import { useEffect, useRef, useState } from 'react';
import UseAnimations from 'react-useanimations';
import award from 'react-useanimations/lib/award';
import { dismissToast, useGame, TOTAL_ACHIEVEMENTS, type Toast } from '@/game/store';
import { StrikeGlyph } from '@/components/ui/Glyphs';

const LIFETIME_MS = 5200;

function ToastCard({ toast }: { toast: Toast }) {
  const [paused, setPaused] = useState(false);
  const remaining = useRef(LIFETIME_MS);
  const startedAt = useRef(0);

  useEffect(() => {
    if (paused) return;
    startedAt.current = Date.now();
    const timer = window.setTimeout(() => dismissToast(toast.id), remaining.current);
    return () => {
      window.clearTimeout(timer);
      remaining.current -= Date.now() - startedAt.current;
    };
  }, [paused, toast.id]);

  const pause = {
    onMouseEnter: () => setPaused(true),
    onMouseLeave: () => setPaused(false),
    onFocus: () => setPaused(true),
    onBlur: () => setPaused(false),
  };

  if (toast.kind === 'level') {
    return (
      <div className="toast toast-level" {...pause} data-paused={paused || undefined}>
        <span className="toast-badge display" aria-hidden="true">
          {String(toast.level.level).padStart(2, '0')}
        </span>
        <p className="toast-title display">
          <span>LV {String(toast.level.level).padStart(2, '0')}</span> {toast.level.name}
        </p>
        <p className="toast-receipt mono">Level up · threshold {toast.level.threshold} XP</p>
        <button type="button" className="toast-close" aria-label="Dismiss" onClick={() => dismissToast(toast.id)}>
          <StrikeGlyph />
        </button>
        <i className="toast-timer" style={{ animationDuration: `${LIFETIME_MS}ms` }} />
      </div>
    );
  }

  const { achievement, effects, receipt, ordinal } = toast;
  return (
    <div className="toast" {...pause} data-paused={paused || undefined}>
      <span className="toast-badge" aria-hidden="true">
        <UseAnimations animation={award} size={30} autoplay loop={false} />
      </span>
      <p className="toast-title display">{achievement.title}</p>
      <p className="toast-effects mono">
        {effects.map((effect) => (
          <span key={effect.key}>{effect.label}</span>
        ))}
      </p>
      <p className="toast-receipt mono">
        <b>
          Achievement {String(ordinal).padStart(2, '0')}/{TOTAL_ACHIEVEMENTS}
        </b>
        {effects.length < 2 && ` · ${receipt}`}
      </p>
      <button type="button" className="toast-close" aria-label="Dismiss" onClick={() => dismissToast(toast.id)}>
        <StrikeGlyph />
      </button>
      <i className="toast-timer" style={{ animationDuration: `${LIFETIME_MS}ms` }} />
    </div>
  );
}

/** One toast at a time; the rest wait their turn like a console's notification queue. */
export function Toasts() {
  const toasts = useGame((snapshot) => snapshot.toasts);
  const current = toasts[0];
  const waiting = toasts.length - 1;
  return (
    <section className="toasts" aria-live="polite" aria-label="Achievements">
      {current && <ToastCard key={current.id} toast={current} />}
      {waiting > 0 && <p className="toast-queue mono">+{waiting} more</p>}
    </section>
  );
}
