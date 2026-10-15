'use client';

import { useState } from 'react';
import { emit } from '@/game/store';
import { StrikeGlyph, TickGlyph } from '@/components/ui/Glyphs';

const ORDERS = [
  { id: 'order-2001', total: 45, status: 'paid' },
  { id: 'order-2002', total: 120, status: 'paid' },
  { id: 'order-2003', total: 88, status: 'paid' },
  { id: 'order-2004', total: 260, status: 'paid' },
  { id: 'order-2005', total: 119, status: 'paid' },
  { id: 'order-2006', total: 150, status: 'refunded' },
  { id: 'order-2007', total: 199, status: 'pending' },
  { id: 'order-2008', total: 131, status: 'paid' },
  { id: 'order-2009', total: 75, status: 'refunded' },
  { id: 'order-2010', total: 121, status: 'paid' },
] as const;

const expected = (order: (typeof ORDERS)[number]) => order.total >= 120 && order.status === 'paid';

type Operator = 'gt' | 'gte';

export function RuleTrigger() {
  const [operator, setOperator] = useState<Operator>('gt');
  const [threshold, setThreshold] = useState(100);
  const [paidOnly, setPaidOnly] = useState(false);
  const [version, setVersion] = useState(0);
  const [results, setResults] = useState<boolean[] | null>(null);

  const matches = (order: (typeof ORDERS)[number]) =>
    (operator === 'gte' ? order.total >= threshold : order.total > threshold) && (!paidOnly || order.status === 'paid');

  const conditions = [`{"${operator}": ["$.total", ${threshold}]}`, ...(paidOnly ? ['{"eq": ["$.payment_status", "paid"]}'] : [])];
  const conditionText = conditions.length > 1 ? `{"all": [\n    ${conditions.join(',\n    ')}\n  ]}` : conditions[0];
  const score = results ? results.filter((awarded, index) => awarded === expected(ORDERS[index])).length : 0;

  const publish = () => {
    const next = version + 1;
    const outcome = ORDERS.map(matches);
    setVersion(next);
    setResults(outcome);
    const accuracy = outcome.filter((awarded, index) => awarded === expected(ORDERS[index])).length / ORDERS.length;
    emit({ id: `rule-trial-v${next}-${Date.now().toString(36)}`, type: 'arcade.rule_tested', payload: { accuracy } });
  };

  const edit = <T,>(setter: (value: T) => void) => (value: T) => {
    setter(value);
    setResults(null);
  };

  return (
    <div className="game game-rule">
      <div className="game-brief">
        <p className="game-brief-text">Give 50 loyalty points to paid orders of 120 or more. Nothing else.</p>
      </div>

      <div className="game-controls">
        <fieldset className="rule-field">
          <legend className="mono">If · $.total</legend>
          <div className="segmented mono" role="radiogroup" aria-label="Comparison">
            {(['gt', 'gte'] as const).map((value) => (
              <button key={value} type="button" role="radio" aria-checked={operator === value} onClick={() => edit(setOperator)(value)}>
                {value === 'gt' ? '> gt' : '≥ gte'}
              </button>
            ))}
          </div>
          <label className="slider">
            <span className="mono">Threshold</span>
            <input type="range" min={0} max={300} step={1} value={threshold} onChange={(event) => edit(setThreshold)(Number(event.target.value))} />
            <output className="slider-value">{threshold}</output>
          </label>
        </fieldset>
        <label className="check mono">
          <input type="checkbox" checked={paidOnly} onChange={(event) => edit(setPaidOnly)(event.target.checked)} />
          <span aria-hidden="true" />
          And $.payment_status = paid
        </label>
        <pre className="rule-preview">{`{
  "event_type": "order.completed",
  "conditions": ${conditionText},
  "actions": [{"type": "award_points", "currency": "loyalty", "amount": 50}]
}`}</pre>
        <button type="button" className="btn btn-signal btn-big" onClick={publish}>
          Publish v{version + 1} and replay 10 orders
        </button>
      </div>

      <div className="game-board">
        <div className="game-board-head mono">
          <span>{results ? `rule trial@v${version}` : 'order.completed stream'}</span>
          <span>{results ? `${score}/10 correct` : 'not evaluated'}</span>
        </div>
        <ol className="order-stream mono" key={version} data-evaluated={results ? '' : undefined}>
          {ORDERS.map((order, index) => {
            const awarded = results?.[index];
            const correct = results ? awarded === expected(order) : undefined;
            return (
              <li key={order.id} data-awarded={awarded || undefined} data-correct={correct === undefined ? undefined : String(correct)} style={{ animationDelay: `${index * 70}ms` }}>
                <span>{order.id}</span>
                <span>{order.total.toFixed(2)}</span>
                <span>{order.status}</span>
                <span className="order-verdict">
                  {results ? (
                    <>
                      {awarded ? '+50 loyalty' : 'skipped'}
                      {correct ? <TickGlyph size={12} /> : <StrikeGlyph size={12} />}
                    </>
                  ) : (
                    '—'
                  )}
                </span>
              </li>
            );
          })}
        </ol>
        {results && (
          <p className="game-verdict" data-win={score === ORDERS.length || undefined}>
            {score === ORDERS.length
              ? 'Exact. Four paid orders rewarded, nothing else. Published versions stay readable, so v' + version + ' is your audit trail.'
              : `${ORDERS.length - score} order${ORDERS.length - score === 1 ? '' : 's'} disagree with the brief. Adjust the rule and publish a new version.`}
          </p>
        )}
      </div>
    </div>
  );
}
