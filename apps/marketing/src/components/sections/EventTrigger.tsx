'use client';

import { useEffect, useState } from 'react';
import UseAnimations from 'react-useanimations';
import zap from 'react-useanimations/lib/zap';
import { DEMO_EVENTS, RULES } from '@/game/catalog';
import type { ProcessResult } from '@/game/engine';
import { emit, getSnapshot } from '@/game/store';

type Fired = { id: string; type: string; result: ProcessResult; serial: number };

/** The hero's working form: send an event, watch the receipt, the rule and the effects print. */
export function EventTrigger() {
  const [selected, setSelected] = useState(0);
  const [fired, setFired] = useState<Fired | null>(null);
  const definition = DEMO_EVENTS[selected];
  const preview = definition.payload(1);

  const fire = () => {
    const serial = Object.keys(getSnapshot().run.receipts).length + 1;
    const id = `${definition.type.split('.')[0]}-${1000 + serial}-${Date.now().toString(36).slice(-4)}`;
    const result = emit({ id, type: definition.type, payload: definition.payload(serial), rateLimited: true });
    setFired({ id, type: definition.type, result, serial });
  };

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (event.key.toLowerCase() !== 'f' || event.metaKey || event.ctrlKey || event.altKey) return;
      if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) return;
      if (document.documentElement.dataset.booted !== '1' && document.querySelector('.boot')) return;
      fire();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  const rule = fired ? RULES.find((candidate) => candidate.eventType === fired.type) : undefined;
  const ruleEffects = fired?.result.effects.filter((effect) => rule && effect.via === `${rule.id}@v${rule.version}`) ?? [];

  return (
    <div className="trigger">
      <span className="ticks">
        <i />
      </span>
      <div className="trigger-head mono">
        <span>Event trigger</span>
        <span>POST /v1/events</span>
      </div>

      <div className="trigger-types" role="radiogroup" aria-label="Event type">
        {DEMO_EVENTS.map((event, index) => (
          <button
            key={event.type}
            type="button"
            role="radio"
            aria-checked={selected === index}
            className="mono"
            onClick={() => setSelected(index)}
          >
            <span className="type-full">{event.type}</span>
            <span className="type-short" aria-hidden="true">
              {event.label}
            </span>
          </button>
        ))}
      </div>

      <pre className="trigger-payload" aria-label="Payload preview">
        {JSON.stringify({ event_type: definition.type, user_id: 'you', payload: preview }, null, 2)}
      </pre>

      <UseAnimations
        animation={zap}
        size={20}
        render={(eventProps, animationProps) => (
          <button
            {...eventProps}
            type="button"
            className="btn btn-signal btn-big trigger-fire"
            onClick={(event) => {
              eventProps.onClick(event);
              fire();
            }}
          >
            <span {...animationProps} />
            Fire event
            <span className="key">F</span>
          </button>
        )}
      />

      <ol className="trigger-trace mono" aria-live="polite" key={fired ? `${fired.id}-${fired.result.status}` : 'idle'}>
        {!fired && <li className="trace-idle">Waiting for a signal. Pick an event and fire it.</li>}
        {fired?.result.status === 'rate_limited' && (
          <>
            <li data-tone="warn">429 rate_limited</li>
            <li>Eight requests per ten seconds. The engine bounds producers; retry in a moment.</li>
          </>
        )}
        {fired?.result.status === 'accepted' && (
          <>
            <li>
              <b>202 accepted</b> receipt {fired.result.receipt}
            </li>
            <li>
              {rule && ruleEffects.length ? `rule ${rule.id}@v${rule.version} matched` : 'no published rule matched'}
            </li>
            {ruleEffects.length > 0 && (
              <li data-tone="effect">
                {ruleEffects.map((effect) => (
                  <span key={effect.key}>{effect.label}</span>
                ))}
              </li>
            )}
            {fired.result.achievements.map((achievement) => (
              <li key={achievement.id} data-tone="effect">
                achievement unlocked · {achievement.title}
              </li>
            ))}
          </>
        )}
      </ol>
      <p className="trigger-note mono">Simulated in your browser · same semantics as the API</p>
    </div>
  );
}
