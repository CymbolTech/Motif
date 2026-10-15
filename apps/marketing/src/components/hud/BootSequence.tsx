'use client';

import { useEffect, useState } from 'react';

const LINES = [
  'Open workspace',
  'Load event contracts',
  'Compile published rules',
  'Warm point ledgers',
  'Arm achievements',
  'Connect reward providers',
  'Start worker',
  'Ready player',
];

const STEP_MS = 150;
const BOOTED_KEY = 'motifengine.booted';

/** Power-on self test. Plays once per session, skippable, absent under reduced motion. */
export function BootSequence() {
  const [step, setStep] = useState(0);
  const [phase, setPhase] = useState<'running' | 'leaving' | 'gone'>('running');

  useEffect(() => {
    if (document.documentElement.dataset.booted) {
      setPhase('gone');
      return;
    }
    const finish = () => {
      setStep(LINES.length);
      setPhase('leaving');
    };
    const timer = window.setInterval(() => {
      setStep((current) => {
        if (current + 1 >= LINES.length) {
          window.clearInterval(timer);
          window.setTimeout(finish, 260);
          return LINES.length;
        }
        return current + 1;
      });
    }, STEP_MS);
    const skip = (event: KeyboardEvent) => {
      if (event.key === 'Escape' || event.key === 'Enter' || event.key === ' ') finish();
    };
    window.addEventListener('keydown', skip);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('keydown', skip);
    };
  }, []);

  useEffect(() => {
    if (phase !== 'leaving') return;
    try {
      window.sessionStorage.setItem(BOOTED_KEY, '1');
    } catch {
      // The boot simply replays next time.
    }
    const timer = window.setTimeout(() => {
      document.documentElement.dataset.booted = '1';
      setPhase('gone');
    }, 700);
    return () => window.clearTimeout(timer);
  }, [phase]);

  if (phase === 'gone') return null;
  const percent = Math.round((step / LINES.length) * 100);

  return (
    <div className="boot" data-phase={phase} role="status" aria-label="Starting MotifEngine" onClick={() => setPhase('leaving')}>
      <div className="boot-inner frame">
        <p className="boot-title mono">MotifEngine runtime · power-on self test</p>
        <ol className="boot-lines mono">
          {LINES.map((line, index) => (
            <li key={line} data-state={index < step ? 'ok' : index === step ? 'run' : undefined}>
              <span>{String(index + 1).padStart(3, '0')}</span>
              <span>{line}</span>
              <span>{index < step ? 'OK' : index === step ? '··' : ''}</span>
            </li>
          ))}
        </ol>
        <p className="boot-percent display" aria-hidden="true">
          {String(percent).padStart(3, '0')}
          <span>%</span>
        </p>
        <button type="button" className="btn boot-skip" onClick={() => setPhase('leaving')}>
          Skip <span className="key">Esc</span>
        </button>
        <span className="boot-bar" style={{ transform: `scaleX(${step / LINES.length})` }} />
      </div>
    </div>
  );
}
