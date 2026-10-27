import type { CSSProperties, ReactNode } from 'react';
import type { GlyphName } from '@/lib/landing';

// Line glyphs for the building-block grid, drawn in a 96 × 72 box.
// "Lit" parts use the ink colour; everything else stays a hairline.
// Class hooks (landing.css): `d` strokes draw in when the tile is revealed,
// `f` fills settle in after them, and `g-*` parts carry each glyph's hover move.

const LIT = 'var(--ink)';
const DIM = 'var(--rule-2)';
const GROUND = 'var(--tile)';

/** Stagger index for a part's hover move. */
const k = (index: number) => ({ '--k': index }) as CSSProperties;

function Square({ x, y, s, lit, className = '' }: { x: number; y: number; s: number; lit?: boolean; className?: string }) {
  return lit ? (
    <rect className={`f ${className}`} x={x} y={y} width={s} height={s} fill={LIT} stroke="none" />
  ) : (
    <rect className={`d ${className}`} pathLength={1} x={x} y={y} width={s} height={s} />
  );
}

function hexagon(cx: number, cy: number, r: number) {
  return Array.from({ length: 6 }, (_, i) => {
    const a = (Math.PI / 3) * i - Math.PI / 2;
    return `${(cx + r * Math.cos(a)).toFixed(1)},${(cy + r * Math.sin(a)).toFixed(1)}`;
  }).join(' ');
}

const GLYPHS: Record<GlyphName, () => ReactNode> = {
  events: () =>
    [30, 22, 36, 18, 28, 24, 34].map((length, i) => {
      const y = 10 + i * 8.5;
      return (
        <g key={i}>
          <line className="d g-bar" style={k(i)} pathLength={1} x1="22" y1={y} x2={22 + length} y2={y} stroke={i === 0 ? LIT : undefined} />
          <line className="d" pathLength={1} x1="70" y1={y} x2="74" y2={y} />
        </g>
      );
    }),
  rules: () => (
    <>
      <Square x={14} y={32} s={8} lit />
      <line className="d" pathLength={1} x1="22" y1="36" x2="40" y2="36" />
      <line className="d" pathLength={1} x1="40" y1="16" x2="40" y2="56" />
      {[16, 36, 56].map((y, i) => (
        <g key={y}>
          <line className="d" pathLength={1} x1="40" y1={y} x2={i === 1 ? 56 : 72} y2={y} />
          {i === 1 && <line className="d g-switch" pathLength={1} x1="58" y1={y} x2="66" y2={y - 5} />}
          <circle className="d" pathLength={1} cx="52" cy={y} r="2" fill={GROUND} />
          <Square x={74} y={y - 4} s={8} lit={i !== 1} className={i === 1 ? 'g-out' : ''} />
        </g>
      ))}
    </>
  ),
  levels: () => (
    <>
      <path className="d" pathLength={1} d="M14 60 h10 v-8 h10 v-8 h10 v-8 h10 v-8 h10 v-8 h10 v-8 h8" />
      <g className="g-climb">
        <circle className="f" cx="82" cy="12" r="2.4" fill={LIT} stroke="none" />
        <line className="d" pathLength={1} x1="82" y1="6" x2="82" y2="2" />
        <line className="d" pathLength={1} x1="78" y1="4" x2="86" y2="4" />
      </g>
    </>
  ),
  missions: () => (
    <>
      {[0, 1, 2, 3, 4].map((i) => (
        <Square key={i} x={14 + i * 14} y={26} s={11} lit={i < 3} className={i === 3 ? 'g-next' : ''} />
      ))}
      <line className="d" pathLength={1} x1="14" y1="48" x2="83" y2="48" stroke={DIM} />
      <line className="d g-progress" pathLength={1} x1="14" y1="48" x2="53" y2="48" stroke={LIT} />
    </>
  ),
  achievements: () => (
    <>
      <polygon className="d" pathLength={1} points={hexagon(48, 30, 20)} />
      <polygon className="d" pathLength={1} points={hexagon(48, 30, 13)} stroke={DIM} />
      <path className="d g-check" pathLength={1} d="M42 30l4.5 4.5 8-9" stroke={LIT} />
      <line className="d" pathLength={1} x1="40" y1="50" x2="36" y2="66" />
      <line className="d" pathLength={1} x1="56" y1="50" x2="60" y2="66" />
    </>
  ),
  currencies: () => (
    <>
      <g className="g-coin-a">
        <circle className="d" pathLength={1} cx="30" cy="36" r="14" />
        <circle className="d" pathLength={1} cx="30" cy="36" r="9" stroke={DIM} />
      </g>
      <g className="g-coin-b">
        <circle className="f" cx="68" cy="36" r="14" fill={LIT} stroke="none" />
        <circle className="d" pathLength={1} cx="68" cy="36" r="9" stroke={GROUND} />
      </g>
      <path className="d g-swap-r" pathLength={1} d="M46 31h6l-2-2" />
      <path className="d g-swap-l" pathLength={1} d="M52 41h-6l2 2" />
    </>
  ),
  ledger: () => (
    <>
      <path className="d" pathLength={1} d="M28 4h32l8 8v58H28z" />
      <path className="d" pathLength={1} d="M60 4v8h8" />
      {['+', '+', '-', '+', '-', '+'].map((sign, i) => {
        const y = 18 + i * 8;
        return (
          <g key={i}>
            <line className="d" pathLength={1} x1="33" y1={y} x2="37" y2={y} />
            {sign === '+' && <line className="d" pathLength={1} x1="35" y1={y - 2} x2="35" y2={y + 2} />}
            <line className="d g-row" style={k(i)} pathLength={1} x1="41" y1={y} x2={52 + ((i * 7) % 10)} y2={y} />
          </g>
        );
      })}
    </>
  ),
  rewards: () => (
    <>
      <circle className="d" pathLength={1} cx="14" cy="36" r="3" />
      <line className="d" pathLength={1} x1="17" y1="36" x2="32" y2="36" />
      <circle className="d" pathLength={1} cx="35" cy="36" r="3" />
      <line className="g-flow" x1="38" y1="36" x2="52" y2="36" strokeDasharray="2 3" />
      <rect className="d" pathLength={1} x="56" y="24" width="26" height="22" />
      <g className="g-lid">
        <rect className="f" x="56" y="24" width="26" height="7" fill={LIT} stroke="none" />
        <path className="d" pathLength={1} d="M69 24c-3-7-10-6-8-1M69 24c3-7 10-6 8-1" />
      </g>
      <line className="d" pathLength={1} x1="69" y1="31" x2="69" y2="46" />
    </>
  ),
  leaderboards: () => (
    <>
      {[44, 36, 30, 22, 16, 10].map((height, i) =>
        i === 0 ? (
          <rect key={i} className="f g-rank" style={k(i)} x={16} y={62 - height} width="7" height={height} fill={LIT} stroke="none" />
        ) : (
          <rect key={i} className="d g-rank" style={k(i)} pathLength={1} x={16 + i * 11} y={62 - height} width="7" height={height} />
        ),
      )}
      <line className="d" pathLength={1} x1="12" y1="62" x2="84" y2="62" stroke={DIM} />
      <circle className="d g-crown" pathLength={1} cx="19.5" cy="10" r="2.4" />
    </>
  ),
  analytics: () => (
    <>
      {Array.from({ length: 9 }, (_, i) => {
        const y = 6 + i * 7.5;
        return (
          <path
            key={i}
            className="d g-stream"
            style={k(Math.abs(i - 4))}
            pathLength={1}
            d={`M12 ${y} C40 ${y} 44 36 66 36`}
            stroke={i === 4 ? LIT : undefined}
            strokeOpacity={i === 4 ? 1 : 0.75}
          />
        );
      })}
      <circle className="d" pathLength={1} cx="69" cy="36" r="3" />
      <line className="d" pathLength={1} x1="72" y1="36" x2="84" y2="36" />
      <circle className="f g-pulse" cx="85" cy="36" r="1.4" fill={LIT} stroke="none" />
    </>
  ),
};

export function Glyph({ name }: { name: GlyphName }) {
  return (
    <svg className={`glyph glyph-${name}`} viewBox="0 0 96 72" fill="none" stroke="var(--glyph)" strokeWidth="1" strokeLinecap="square" aria-hidden="true">
      {GLYPHS[name]()}
    </svg>
  );
}
