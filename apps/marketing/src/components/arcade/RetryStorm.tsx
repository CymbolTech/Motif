'use client';

import { useState } from 'react';
import UseAnimations from 'react-useanimations';
import repeat from 'react-useanimations/lib/repeat';
import { emit, useGame } from '@/game/store';

const EVENT_ID = 'order-1042-completed';
const TARGET = 10;

type Attempt = { serial: number; status: string; receipt?: string };

function Counter({ label, value, tone }: { label: string; value: number; tone?: 'bad' }) {
  return (
    <div className="counter" data-tone={tone}>
      <span className="counter-value">{String(value).padStart(3, '0')}</span>
      <span className="counter-label mono">{label}</span>
    </div>
  );
}

export function RetryStorm() {
  const ledger = useGame((snapshot) => snapshot.run.ledger);
  const receipt = useGame((snapshot) => snapshot.run.receipts[EVENT_ID]);
  const [requests, setRequests] = useState(0);
  const [naive, setNaive] = useState(false);
  const [attempts, setAttempts] = useState<Attempt[]>([]);
  const [shake, setShake] = useState(0);

  const entries = ledger.filter((entry) => entry.key.startsWith(`${EVENT_ID}:`) && entry.currency === 'loyalty');
  const awarded = entries.reduce((sum, entry) => sum + entry.amount, 0);

  const send = () => {
    const serial = requests + 1;
    const result = emit({ id: EVENT_ID, type: 'order.completed', payload: { order_id: 'order-1042', total: 64, payment_status: 'paid' } });
    setRequests(serial);
    setShake((value) => value + 1);
    setAttempts((list) => [{ serial, status: result.status, receipt: result.receipt }, ...list].slice(0, 6));
    if (serial === TARGET) emit({ id: 'storm-survived', type: 'arcade.storm_survived', payload: { requests: serial } });
  };

  return (
    <div className="game game-storm">
      <div className="game-brief">
        <p className="game-brief-text">Your backend never heard back, so it retries {EVENT_ID}. Again. Hammer the button and watch what the ledger does.</p>
      </div>

      <div className="storm-counters" key={shake} data-shake={shake > 1 || undefined}>
        <Counter label="Requests sent" value={requests} />
        <Counter label="Receipts" value={receipt ? 1 : 0} />
        <Counter label="Ledger entries" value={entries.length} />
        <Counter label="Loyalty awarded" value={awarded} />
      </div>

      <div className="storm-actions">
        <UseAnimations
          animation={repeat}
          size={20}
          render={(eventProps, animationProps) => (
            <button
              {...eventProps}
              type="button"
              className="btn btn-signal btn-big"
              onClick={(event) => {
                eventProps.onClick(event);
                send();
              }}
            >
              <span {...animationProps} />
              Retry the same event
            </button>
          )}
        />
        <label className="check mono">
          <input type="checkbox" checked={naive} onChange={(event) => setNaive(event.target.checked)} />
          <span aria-hidden="true" />
          Compare with a script that has no idempotency key
        </label>
        <p className="mono storm-progress">
          {requests >= TARGET ? 'Ten retries, one payout. Survived.' : `${Math.max(0, TARGET - requests)} retries to survive the storm`}
        </p>
      </div>

      {naive && (
        <div className="storm-naive">
          <p className="mono">Without a key · every retry pays</p>
          <div className="storm-counters">
            <Counter label="Ledger entries" value={requests} tone="bad" />
            <Counter label="Loyalty awarded" value={requests * 50} tone="bad" />
          </div>
        </div>
      )}

      <ol className="storm-log mono" aria-live="polite">
        {attempts.map((attempt) => (
          <li key={attempt.serial} data-status={attempt.status}>
            <span>#{String(attempt.serial).padStart(2, '0')}</span>
            <span>{attempt.status === 'duplicate' ? 'duplicate · no new effect' : attempt.status === 'accepted' ? '202 accepted · rule ran once' : attempt.status}</span>
            <span>{attempt.receipt}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}
