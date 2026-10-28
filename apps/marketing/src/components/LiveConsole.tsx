'use client';

import { useEffect, useRef, useState } from 'react';
import { Logo } from '@/components/Icons';
import { type ChartApi, LiveChart } from '@/components/LiveChart';

// The hero's console card. It replays a short scripted stream of synthetic
// events so the page shows the engine paying out; nothing here calls the API.

type Row = { key: number; type: string; effect: string };

const SCRIPT = [
  { type: 'lesson.completed', effect: '+50 xp', xp: 50, loyalty: 0 },
  { type: 'order.paid', effect: '+200 loyalty', xp: 0, loyalty: 200 },
  { type: 'streak.day', effect: '+10 coins', xp: 10, loyalty: 0 },
  { type: 'quiz.passed', effect: '+30 xp', xp: 30, loyalty: 0 },
  { type: 'review.posted', effect: '+20 loyalty', xp: 0, loyalty: 20 },
  { type: 'lesson.completed', effect: '+50 xp', xp: 50, loyalty: 0 },
];

const ACHIEVEMENTS = ['Ten lessons unlocked', 'Seven-day streak unlocked', 'First review unlocked', 'Second purchase unlocked'];

const NEXT_LEVEL = 22500;
const INTERVAL = 1200;

const format = new Intl.NumberFormat('en-US');

/** Eases a displayed number toward its target so counters roll instead of jumping. */
function useRolling(target: number) {
  const [shown, setShown] = useState(target);
  const from = useRef(target);
  useEffect(() => {
    const start = performance.now();
    const origin = from.current;
    let frame = 0;
    const step = (now: number) => {
      const t = Math.min(1, (now - start) / 420);
      const eased = 1 - Math.pow(2, -10 * t);
      const value = Math.round(origin + (target - origin) * (t === 1 ? 1 : eased));
      from.current = value;
      setShown(value);
      if (t < 1) frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [target]);
  return shown;
}

export function LiveConsole() {
  const root = useRef<HTMLElement>(null);
  const tick = useRef(0);
  const chartApi = useRef<ChartApi | null>(null);
  const [xp, setXp] = useState(18420);
  const [loyalty, setLoyalty] = useState(2350);
  const [processed, setProcessed] = useState(48112);
  const [issued, setIssued] = useState(1204350);
  const [rows, setRows] = useState<Row[]>([
    { key: -1, type: 'lesson.completed', effect: '+50 xp' },
    { key: -2, type: 'order.paid', effect: '+200 loyalty' },
    { key: -3, type: 'streak.day', effect: '+10 coins' },
    { key: -4, type: 'quiz.passed', effect: '+30 xp' },
  ]);
  const [achievement, setAchievement] = useState({ key: 0, label: ACHIEVEMENTS[0] });

  useEffect(() => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    let visible = false;
    let timer: number | undefined;

    const fire = () => {
      const index = tick.current++;
      const event = SCRIPT[index % SCRIPT.length];
      setRows((current) => [{ key: index, type: event.type, effect: event.effect }, ...current.slice(0, 3)]);
      setXp((value) => (value + event.xp >= NEXT_LEVEL ? 18420 : value + event.xp));
      setLoyalty((value) => value + event.loyalty);
      setIssued((value) => value + event.loyalty);
      setProcessed((value) => value + 1);
      // The chart takes the same event: XP lifts the solid line, points the dashed one.
      chartApi.current?.push(event.xp || 4, event.loyalty ? event.loyalty / 8 : 2);
      if (index % 4 === 3) {
        setAchievement((current) => ({ key: current.key + 1, label: ACHIEVEMENTS[(current.key + 1) % ACHIEVEMENTS.length] }));
      }
    };
    const sync = () => {
      const run = visible && document.visibilityState === 'visible';
      if (run && timer === undefined) timer = window.setInterval(fire, INTERVAL);
      if (!run && timer !== undefined) {
        window.clearInterval(timer);
        timer = undefined;
      }
    };
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      sync();
    });
    if (root.current) observer.observe(root.current);
    document.addEventListener('visibilitychange', sync);
    return () => {
      observer.disconnect();
      document.removeEventListener('visibilitychange', sync);
      if (timer !== undefined) window.clearInterval(timer);
    };
  }, []);

  const shownXp = useRolling(xp);
  const shownLoyalty = useRolling(loyalty);
  const shownProcessed = useRolling(processed);
  const shownIssued = useRolling(issued);

  return (
    <figure ref={root} className="card console">
      <figcaption className="sr-only">An example Motif console for one user, filled with synthetic data.</figcaption>
      <div className="card-head">
        <span className="card-brand">
          <Logo className="card-logo" />
          Motif
        </span>
        <span>northwind · production</span>
      </div>
      <div className="console-grid">
        <div className="console-user">
          <div className="who">
            <span className="avatar" aria-hidden="true" />
            <span>
              <b>user-42</b>
              <small>Level 7 · Expert</small>
            </span>
          </div>
          <dl className="stat-pair">
            <div>
              <dt>XP</dt>
              <dd className="num">{format.format(shownXp)}</dd>
            </div>
            <div>
              <dt>Loyalty</dt>
              <dd className="num">{format.format(shownLoyalty)}</dd>
            </div>
          </dl>
          <div className="feed" aria-hidden="true">
            <ol key={rows[0].key} className="feed-list">
              {rows.map((row, index) => (
                <li key={row.key} className={index === 0 ? 'feed-new' : undefined}>
                  <span className="mono">{row.type}</span>
                  <span className="mono feed-effect">{row.effect}</span>
                </li>
              ))}
            </ol>
          </div>
          <p className="unlock">
            <span key={achievement.key} className="unlock-dot" aria-hidden="true" />
            <span>{achievement.label}</span>
            <span className="unlock-time">just now</span>
          </p>
        </div>
        <div className="console-chart">
          <dl className="kpis">
            <div>
              <dt>Events processed</dt>
              <dd className="num">{format.format(shownProcessed)}</dd>
            </div>
            <div className="kpi-end">
              <dt>Points issued</dt>
              <dd className="num">{format.format(shownIssued)}</dd>
            </div>
          </dl>
          <LiveChart api={chartApi} />
          <div className="legend">
            <span>
              <i aria-hidden="true" />
              XP awarded
            </span>
            <span>
              <i className="legend-signal" aria-hidden="true" />
              Points issued
            </span>
            <span className="mono legend-note">synthetic data</span>
          </div>
          <div className="level">
            <div className="level-row">
              <span>Level 7 to 8</span>
              <span className="mono">
                {format.format(shownXp)} / {format.format(NEXT_LEVEL)}
              </span>
            </div>
            <div className="bar" aria-hidden="true">
              <i style={{ transform: `scaleX(${shownXp / NEXT_LEVEL})` }} />
            </div>
          </div>
        </div>
      </div>
    </figure>
  );
}
