'use client';

import { useState } from 'react';
import { CONVERSION } from '@/game/catalog';
import { convertPoints, type LedgerEntry } from '@/game/engine';
import { emit, getSnapshot, updateRun, useGame } from '@/game/store';
import { CoinGlyph } from '@/components/ui/Glyphs';

const AMOUNTS = [10, 50, 100];

export function Exchange() {
  const balances = useGame((snapshot) => snapshot.run.balances);
  const [amount, setAmount] = useState(50);
  const [rows, setRows] = useState<LedgerEntry[] | null>(null);
  const [serial, setSerial] = useState(0);

  const all = Math.floor(balances.loyalty / CONVERSION.fromAmount) * CONVERSION.fromAmount;
  const spend = amount === -1 ? all : amount;
  const enough = spend >= CONVERSION.fromAmount && balances.loyalty >= spend;

  const convert = () => {
    let conversionId: string | undefined;
    updateRun((run) => {
      const outcome = convertPoints(run, spend);
      conversionId = outcome.conversionId;
      return outcome.state;
    });
    if (!conversionId) return;
    const id = conversionId;
    setRows(getSnapshot().run.ledger.filter((entry) => entry.key.startsWith(id)));
    emit({ id: `conversion-${id}`, type: 'points.converted', payload: { amount: spend } });
  };

  const earn = () => {
    const next = serial + 1;
    setSerial(next);
    emit({ id: `order-exchange-${next}-${Date.now().toString(36)}`, type: 'order.completed', payload: { order_id: `order-x${next}`, total: 80, payment_status: 'paid' }, rateLimited: true });
  };

  return (
    <div className="game game-exchange">
      <div className="game-brief">
        <p className="game-brief-text">Turn loyalty points into coins at a published rate. Both ledger rows are written together, or neither is.</p>
      </div>

      <div className="exchange-board">
        <div className="exchange-balance">
          <span className="mono">Loyalty</span>
          <span className="exchange-figure">{balances.loyalty}</span>
        </div>
        <div className="exchange-rate mono">
          <span>
            {CONVERSION.id}@v{CONVERSION.version}
          </span>
          <span className="exchange-arrow" aria-hidden="true" />
          <span>
            {CONVERSION.fromAmount} → {CONVERSION.toAmount}
          </span>
        </div>
        <div className="exchange-balance">
          <span className="mono">
            <CoinGlyph size={12} /> Coins
          </span>
          <span className="exchange-figure">{balances.coins}</span>
        </div>
      </div>

      <div className="exchange-controls">
        <div className="segmented mono" role="radiogroup" aria-label="Loyalty to convert">
          {[...AMOUNTS, -1].map((value) => (
            <button key={value} type="button" role="radio" aria-checked={amount === value} onClick={() => setAmount(value)}>
              {value === -1 ? `All (${all})` : value}
            </button>
          ))}
        </div>
        <button type="button" className="btn btn-signal btn-big" onClick={convert} disabled={!enough}>
          Convert {spend} loyalty → {Math.floor(spend / CONVERSION.fromAmount) * CONVERSION.toAmount} coins
        </button>
        {!enough && (
          <button type="button" className="btn btn-big" onClick={earn}>
            Earn 50 loyalty: send order.completed
          </button>
        )}
      </div>

      {rows && (
        <ol className="exchange-rows mono" aria-live="polite">
          {rows.map((row) => (
            <li key={row.id}>
              <span>{row.id}</span>
              <span>
                {row.amount > 0 ? '+' : ''}
                {row.amount} {row.currency}
              </span>
              <span>{row.kind}</span>
              <span className="ledger-key">{row.key}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
