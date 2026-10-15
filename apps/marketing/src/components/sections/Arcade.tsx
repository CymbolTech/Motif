'use client';

import { useRef, useState, type KeyboardEvent } from 'react';
import { Exchange } from '@/components/arcade/Exchange';
import { Redeem } from '@/components/arcade/Redeem';
import { RetryStorm } from '@/components/arcade/RetryStorm';
import { RuleTrigger } from '@/components/arcade/RuleTrigger';
import { Coord } from '@/components/ui/Coord';
import { useGame } from '@/game/store';

const GAMES = [
  { id: 'rule', name: 'Rule trigger', proves: 'Restricted conditions, published versions', achievement: 'rule-smith', Component: RuleTrigger },
  { id: 'storm', name: 'Retry storm', proves: 'Idempotent events and ledger keys', achievement: 'once-means-once', Component: RetryStorm },
  { id: 'exchange', name: 'Exchange', proves: 'Independent currencies, atomic conversion', achievement: 'money-changer', Component: Exchange },
  { id: 'redeem', name: 'Redeem', proves: 'Reservations, providers, recovery', achievement: 'cashed-in', Component: Redeem },
] as const;

export function Arcade() {
  const [active, setActive] = useState(0);
  const unlocked = useGame((snapshot) => snapshot.run.unlocked);
  const tabs = useRef<Array<HTMLButtonElement | null>>([]);
  const Game = GAMES[active].Component;

  const onKey = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const delta = event.key === 'ArrowDown' || event.key === 'ArrowRight' ? 1 : event.key === 'ArrowUp' || event.key === 'ArrowLeft' ? -1 : 0;
    if (!delta) return;
    event.preventDefault();
    const next = (index + delta + GAMES.length) % GAMES.length;
    setActive(next);
    tabs.current[next]?.focus();
  };

  return (
    <section className="arcade section" id="arcade" aria-labelledby="arcade-title">
      <Coord />
      <div className="frame section-head">
        <h2 id="arcade-title" className="section-title display">
          Insert coin.
        </h2>
        <p className="section-lede">Four proofs you can play. Each one runs the engine’s semantics in your browser, and each one pays out an achievement when you clear it.</p>
      </div>

      <div className="frame cabinet">
        <div className="cabinet-menu" role="tablist" aria-label="Arcade games" aria-orientation="vertical">
          {GAMES.map((game, index) => {
            const cleared = Boolean(unlocked[game.achievement]);
            return (
              <button
                key={game.id}
                ref={(element) => {
                  tabs.current[index] = element;
                }}
                type="button"
                role="tab"
                id={`game-tab-${game.id}`}
                aria-selected={active === index}
                aria-controls="game-screen"
                tabIndex={active === index ? 0 : -1}
                className="cabinet-item"
                onClick={() => setActive(index)}
                onKeyDown={(event) => onKey(event, index)}
              >
                <span className="cabinet-item-name display">{game.name}</span>
                <span className="cabinet-item-meta mono">
                  <span>{game.proves}</span>
                  <span data-cleared={cleared || undefined}>{cleared ? 'Cleared' : 'Ready'}</span>
                </span>
              </button>
            );
          })}
        </div>

        <div className="cabinet-screen" id="game-screen" role="tabpanel" aria-labelledby={`game-tab-${GAMES[active].id}`}>
          <span className="ticks">
            <i />
          </span>
          <Game key={GAMES[active].id} />
        </div>
      </div>
    </section>
  );
}
