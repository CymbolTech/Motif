'use client';

import { useEffect, useState } from 'react';
import UseAnimations from 'react-useanimations';
import moon from 'react-useanimations/lib/moon';
import refresh from 'react-useanimations/lib/refresh';
import { ACHIEVEMENTS, levelFor, nextLevel } from '@/game/catalog';
import { nextQuest, questText } from '@/game/quests';
import { resetRun, setQuestLog, useGame, THEME_KEY, TOTAL_ACHIEVEMENTS } from '@/game/store';
import { CoinGlyph, StrikeGlyph, TickGlyph } from '@/components/ui/Glyphs';
import { XpMeter } from './HudBar';


type Tab = 'quests' | 'ledger' | 'log';

function time(at: number) {
  return new Date(at).toLocaleTimeString('en-GB', { hour12: false });
}

export function QuestLog() {
  const run = useGame((snapshot) => snapshot.run);
  const open = useGame((snapshot) => snapshot.questLogOpen);
  const hydrated = useGame((snapshot) => snapshot.hydrated);
  const [tab, setTab] = useState<Tab>('quests');
  const [night, setNight] = useState(false);

  useEffect(() => {
    if (!open) return;
    setNight(document.documentElement.dataset.theme === 'night');
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setQuestLog(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  const level = levelFor(run.xp);
  const next = nextLevel(level);
  const unlocked = Object.keys(run.unlocked).length;
  const objective = questText(nextQuest(run));

  const toggleNight = () => {
    const nextNight = !night;
    setNight(nextNight);
    if (nextNight) document.documentElement.dataset.theme = 'night';
    else delete document.documentElement.dataset.theme;
    try {
      window.localStorage.setItem(THEME_KEY, nextNight ? 'night' : 'day');
    } catch {
      // Preference lasts for this page view only.
    }
  };

  return (
    <>
      <button
        type="button"
        className="tracker"
        data-ready={hydrated || undefined}
        aria-expanded={open}
        aria-controls="quest-log"
        onClick={() => setQuestLog((value) => !value)}
      >
        <span className="tracker-head mono">
          <span>Quest log</span>
          <span>
            {String(unlocked).padStart(2, '0')}/{TOTAL_ACHIEVEMENTS}
          </span>
        </span>
        <span className="tracker-quest">{objective}</span>
        <span className="tracker-mobile mono">
          <b>LV {String(level.level).padStart(2, '0')}</b>
          <XpMeter xp={run.xp} />
          <span>
            <CoinGlyph size={13} /> {run.balances.coins}
          </span>
        </span>
      </button>

      <aside className="questlog" id="quest-log" data-lenis-prevent data-open={open || undefined} aria-label="Quest log" inert={!open}>
        <header className="questlog-head">
          <div>
            <p className="questlog-player display">Player · you</p>
            <p className="mono questlog-endpoint">GET /v1/users/you/profile</p>
          </div>
          <button type="button" className="questlog-close" aria-label="Close quest log" onClick={() => setQuestLog(false)}>
            <StrikeGlyph size={16} />
          </button>
        </header>

        <dl className="questlog-stats mono">
          <div>
            <dt>Level</dt>
            <dd>
              {String(level.level).padStart(2, '0')} {level.name}
            </dd>
          </div>
          <div>
            <dt>Total XP</dt>
            <dd>
              {run.xp}
              {next ? ` / ${next.threshold}` : ''}
            </dd>
          </div>
          <div>
            <dt>Coins</dt>
            <dd>{run.balances.coins}</dd>
          </div>
          <div>
            <dt>Loyalty</dt>
            <dd>{run.balances.loyalty}</dd>
          </div>
        </dl>

        <div className="questlog-tabs" role="tablist" aria-label="Quest log views">
          {(['quests', 'ledger', 'log'] as const).map((value) => (
            <button key={value} type="button" role="tab" aria-selected={tab === value} onClick={() => setTab(value)}>
              {value === 'quests' ? `Achievements ${unlocked}/${TOTAL_ACHIEVEMENTS}` : value === 'ledger' ? 'Ledger' : 'Event log'}
            </button>
          ))}
        </div>

        <div className="questlog-body" role="tabpanel">
          {tab === 'quests' && (
            <ul className="quests">
              {ACHIEVEMENTS.map((achievement) => {
                const done = Boolean(run.unlocked[achievement.id]);
                const progress = run.progress[achievement.id] ?? 0;
                const hidden = achievement.secret && !done;
                return (
                  <li key={achievement.id} data-done={done || undefined}>
                    <span className="quest-check" aria-hidden="true">
                      {done ? <TickGlyph /> : null}
                    </span>
                    <span className="quest-copy">
                      <b>{hidden ? 'Secret' : achievement.title}</b>
                      <span>{hidden ? achievement.hint : achievement.description}</span>
                    </span>
                    <span className="quest-meta mono">
                      {achievement.required > 1 && !done ? `${progress}/${achievement.required} · ` : ''}+{achievement.bonus.xp} XP
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
          {tab === 'ledger' &&
            (run.ledger.length ? (
              <ol className="ledger mono">
                {run.ledger.map((entry) => (
                  <li key={entry.id}>
                    <span>{entry.id}</span>
                    <span data-sign={entry.amount < 0 ? 'neg' : undefined}>
                      {entry.amount > 0 ? '+' : ''}
                      {entry.amount} {entry.currency}
                    </span>
                    <span>{entry.kind}</span>
                    <span className="ledger-key">{entry.key}</span>
                  </li>
                ))}
              </ol>
            ) : (
              <p className="questlog-empty">No ledger entries yet. Fire an event from the trigger at the top of the page.</p>
            ))}
          {tab === 'log' &&
            (run.log.length ? (
              <ol className="eventlog mono">
                {run.log.map((line) => (
                  <li key={line.id} data-status={line.status}>
                    <span>{time(line.at)}</span>
                    <span>{line.eventType}</span>
                    <span>{line.summary}</span>
                  </li>
                ))}
              </ol>
            ) : (
              <p className="questlog-empty">No events processed yet.</p>
            ))}
        </div>

        <footer className="questlog-foot">
          {run.unlocks['night-shift'] && (
            <UseAnimations
              animation={moon}
              size={18}
              render={(eventProps, animationProps) => (
                <button
                  {...eventProps}
                  type="button"
                  className="btn"
                  aria-pressed={night}
                  onClick={(event) => {
                    eventProps.onClick(event);
                    toggleNight();
                  }}
                >
                  <span {...animationProps} />
                  Night shift {night ? 'on' : 'off'}
                </button>
              )}
            />
          )}
          <UseAnimations
            animation={refresh}
            size={18}
            render={(eventProps, animationProps) => (
              <button
                {...eventProps}
                type="button"
                className="btn"
                onClick={(event) => {
                  eventProps.onClick(event);
                  if (window.confirm('Start a new run? Your XP, coins and achievements in this browser are cleared.')) {
                    delete document.documentElement.dataset.theme;
                    resetRun();
                  }
                }}
              >
                <span {...animationProps} />
                New run
              </button>
            )}
          />
          <p className="mono questlog-note">Simulated in this browser. Nothing is sent to an API.</p>
        </footer>
      </aside>
    </>
  );
}
