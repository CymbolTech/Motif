'use client';

import { useEffect, useRef, useState } from 'react';
import UseAnimations from 'react-useanimations';
import gift from 'react-useanimations/lib/gift';
import { REWARDS, type Reward } from '@/game/catalog';
import { advanceRedemption, reserveReward, type RedemptionState } from '@/game/engine';
import { emit, getSnapshot, updateRun, useGame } from '@/game/store';
import { CoinGlyph } from '@/components/ui/Glyphs';

const STATES: { id: RedemptionState; label: string }[] = [
  { id: 'reserved', label: 'Reserved' },
  { id: 'delivering', label: 'Delivering' },
  { id: 'unknown', label: 'Outcome unknown' },
  { id: 'fulfilled', label: 'Fulfilled' },
];

const COPY: Record<RedemptionState, string> = {
  reserved: 'Coins reserved against the redemption id. Nothing is spent yet.',
  delivering: 'Calling the provider with an idempotency header.',
  unknown: 'The provider timed out. The outcome is unknown, so the coins stay reserved and the redemption stays open.',
  fulfilled: 'Provider confirmed. The reward is delivered and the reservation settles.',
};

export function Redeem() {
  const coins = useGame((snapshot) => snapshot.run.balances.coins);
  const unlocks = useGame((snapshot) => snapshot.run.unlocks);
  const redemptions = useGame((snapshot) => snapshot.run.redemptions);
  const [activeId, setActiveId] = useState<string | null>(null);
  const timers = useRef<number[]>([]);

  const active = redemptions.find((redemption) => redemption.id === activeId);

  const hydrated = useGame((snapshot) => snapshot.hydrated);

  useEffect(() => () => timers.current.forEach((timer) => window.clearTimeout(timer)), []);

  // A redemption left open by a reload is treated like a process restart: unknown until checked.
  useEffect(() => {
    if (!hydrated) return;
    const stale = getSnapshot().run.redemptions.find((redemption) => redemption.state !== 'fulfilled');
    if (!stale) return;
    if (stale.state !== 'unknown') updateRun((run) => advanceRedemption(run, stale.id, 'unknown'));
    setActiveId(stale.id);
  }, [hydrated]);

  const later = (ms: number, task: () => void) => {
    timers.current.push(window.setTimeout(task, ms));
  };

  const fulfil = (id: string, rewardId: Reward['id']) => {
    updateRun((run) => advanceRedemption(run, id, 'fulfilled'));
    emit({ id: `redeem-${id}`, type: 'reward.redeemed', payload: { reward_id: rewardId } });
  };

  const redeem = (reward: Reward) => {
    const firstEver = getSnapshot().run.redemptions.length === 0;
    let id: string | undefined;
    updateRun((run) => {
      const outcome = reserveReward(run, reward.id);
      id = outcome.redemption?.id;
      return outcome.state;
    });
    if (!id) return;
    const redemptionId = id;
    setActiveId(redemptionId);
    later(650, () => updateRun((run) => advanceRedemption(run, redemptionId, 'delivering')));
    // The first redemption meets a provider timeout so the recovery path is visible.
    if (firstEver) later(1900, () => updateRun((run) => advanceRedemption(run, redemptionId, 'unknown')));
    else later(1700, () => fulfil(redemptionId, reward.id));
  };

  const statusCheck = () => {
    if (!active) return;
    const { id, rewardId } = active;
    updateRun((run) => advanceRedemption(run, id, 'delivering'));
    later(1100, () => fulfil(id, rewardId));
  };

  const reachedIndex = active ? STATES.findIndex((state) => state.id === active.state) : -1;
  const sawUnknown = Boolean(active && active.attempts > 1);

  return (
    <div className="game game-redeem">
      <div className="game-brief">
        <p className="game-brief-text">Spend the coins you earned on this page. These rewards are delivered for real, to this page.</p>
      </div>

      <ul className="rewards">
        {REWARDS.map((reward) => {
          const owned = Boolean(unlocks[reward.id]);
          const short = reward.cost - coins;
          return (
            <li key={reward.id} data-owned={owned || undefined}>
              <div className="reward-copy">
                <p className="reward-name display">{reward.name}</p>
                <p className="reward-desc">{reward.description}</p>
              </div>
              <p className="reward-cost mono">
                <CoinGlyph size={13} /> {reward.cost}
              </p>
              {owned ? (
                <p className="reward-owned mono">Delivered</p>
              ) : (
                <UseAnimations
                  animation={gift}
                  size={20}
                  render={(eventProps, animationProps) => (
                    <button
                      {...eventProps}
                      type="button"
                      className="btn btn-signal"
                      disabled={short > 0 || Boolean(active && active.state !== 'fulfilled')}
                      onClick={(event) => {
                        eventProps.onClick(event);
                        redeem(reward);
                      }}
                    >
                      <span {...animationProps} />
                      {short > 0 ? `${short} more coins` : 'Redeem'}
                    </button>
                  )}
                />
              )}
            </li>
          );
        })}
      </ul>

      {coins < Math.min(...REWARDS.map((reward) => reward.cost)) && !active && (
        <p className="game-hint">Coins come from lesson and quest events, scroll milestones, and achievements such as Rule smith and Sharpshooter.</p>
      )}

      {active && (
        <div className="redemption" aria-live="polite">
          <p className="mono redemption-id">
            {active.id} · attempts {active.attempts}
          </p>
          <ol className="redemption-states mono">
            {STATES.filter((state) => state.id !== 'unknown' || sawUnknown || active.state === 'unknown').map((state) => {
              const index = STATES.findIndex((candidate) => candidate.id === state.id);
              return (
                <li key={state.id} data-state={active.state === state.id ? 'on' : index < reachedIndex || active.state === 'fulfilled' ? 'past' : undefined}>
                  {state.label}
                </li>
              );
            })}
          </ol>
          <p className="redemption-copy">{COPY[active.state]}</p>
          {active.state === 'unknown' && (
            <button type="button" className="btn btn-signal" onClick={statusCheck}>
              Run provider status check
            </button>
          )}
        </div>
      )}
    </div>
  );
}
