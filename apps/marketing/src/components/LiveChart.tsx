'use client';

import { type MutableRefObject, useEffect, useRef } from 'react';

// The hero console's chart, driven by the same replayed events as the feed:
// each event pushes one point and the series slides left by a step under a
// dot fixed at the right edge. Everything is computed in value space first:
// a monotone cubic through the values (so the line never overshoots a point),
// a scale taken every frame from exactly what is on screen (so the dot never
// leaves the chart), then one mapping to pixels. Paths are written straight
// to the DOM each frame; React does not re-render per frame.

export type ChartApi = { push: (xp: number, points: number) => void };

const W = 440;
const H = 170;
const N = 16;
/** The newest point sits on the right edge, so the line, its fill and the baseline all end where they start on the left. */
const END = W;
const STEP = END / (N - 1);
const DURATION = 650;

const ease = (t: number) => 1 - Math.pow(1 - t, 3);

/** Fritsch–Carlson tangents (per index step): monotone data gives a monotone curve. */
function tangents(v: number[]) {
  const n = v.length;
  const d = v.slice(1).map((value, i) => value - v[i]);
  const m = v.map((_, i) => (i === 0 ? d[0] : i === n - 1 ? d[n - 2] : d[i - 1] * d[i] <= 0 ? 0 : (d[i - 1] + d[i]) / 2));
  for (let i = 0; i < n - 1; i++) {
    if (d[i] === 0) {
      m[i] = 0;
      m[i + 1] = 0;
      continue;
    }
    const a = m[i] / d[i];
    const b = m[i + 1] / d[i];
    const s = a * a + b * b;
    if (s > 9) {
      const tau = 3 / Math.sqrt(s);
      m[i] = tau * a * d[i];
      m[i + 1] = tau * b * d[i];
    }
  }
  return m;
}

/** The curve's value on segment i at parameter u (0–1). */
function valueAt(v: number[], m: number[], i: number, u: number) {
  const c1 = v[i] + m[i] / 3;
  const c2 = v[i + 1] - m[i + 1] / 3;
  const r = 1 - u;
  return r * r * r * v[i] + 3 * r * r * u * c1 + 3 * r * u * u * c2 + u * u * u * v[i + 1];
}

type Series = { line: string; area: string; endValue: number; endY: number };

/**
 * Lays one series out at slide progress t (0 = resting, 1 = one step left).
 * During a slide the array holds one extra point, still off to the right.
 */
function layout(v: number[], t: number, top: number, bottom: number): Series {
  const m = tangents(v);
  const last = v.length - 1;
  const sliding = v.length > N;
  const leftValue = sliding ? valueAt(v, m, 0, t) : v[0];
  const endValue = sliding ? valueAt(v, m, last - 1, t) : v[last];
  const visible = [leftValue, endValue, ...v.slice(1, sliding ? last : last + 1)];
  const lo = Math.min(...visible);
  const hi = Math.max(...visible);
  const y = (value: number) => bottom - ((value - lo) / Math.max(1e-6, hi - lo)) * (bottom - top);
  const shift = sliding ? t : 0;
  const x = (i: number) => (i - shift) * STEP;

  let line = `M${x(0).toFixed(2)} ${y(v[0]).toFixed(2)}`;
  for (let i = 0; i < last; i++) {
    line += ` C${(x(i) + STEP / 3).toFixed(2)} ${y(v[i] + m[i] / 3).toFixed(2)} ${(x(i + 1) - STEP / 3).toFixed(2)} ${y(v[i + 1] - m[i + 1] / 3).toFixed(2)} ${x(i + 1).toFixed(2)} ${y(v[i + 1]).toFixed(2)}`;
  }
  const area = `${line} L${x(last).toFixed(2)} ${H} L${x(0).toFixed(2)} ${H}Z`;
  return { line, area, endValue, endY: y(endValue) };
}

export function LiveChart({ api }: { api: MutableRefObject<ChartApi | null> }) {
  const xpLine = useRef<SVGPathElement>(null);
  const xpArea = useRef<SVGPathElement>(null);
  const ptsLine = useRef<SVGPathElement>(null);
  const dot = useRef<SVGCircleElement>(null);
  const ring = useRef<SVGCircleElement>(null);

  useEffect(() => {
    let xp = [0, 4, 10, 14, 22, 26, 38, 42, 52, 60, 68, 80, 90, 106, 114, 122];
    let pts = [0, 2, 6, 10, 12, 20, 24, 30, 38, 42, 50, 56, 60, 64, 70, 76];
    let frame = 0;

    const draw = (xpValues: number[], ptsValues: number[], t: number) => {
      // XP fills the upper band, points issued sit lower, each on its own scale.
      const xpSeries = layout(xpValues, t, 22, 158);
      const ptsSeries = layout(ptsValues, t, 76, 165);
      xpLine.current?.setAttribute('d', xpSeries.line);
      xpArea.current?.setAttribute('d', xpSeries.area);
      ptsLine.current?.setAttribute('d', ptsSeries.line);
      for (const circle of [dot.current, ring.current]) {
        circle?.setAttribute('cx', END.toFixed(2));
        circle?.setAttribute('cy', xpSeries.endY.toFixed(2));
      }
    };

    draw(xp, pts, 0);

    api.current = {
      push(xpGain, pointsGain) {
        // A push during a slide finishes the current one first, so nothing skips.
        if (frame) {
          cancelAnimationFrame(frame);
          frame = 0;
          xp = xp.length > N ? xp.slice(1) : xp;
          pts = pts.length > N ? pts.slice(1) : pts;
        }
        const nextXp = [...xp, xp[xp.length - 1] + Math.max(0, xpGain)];
        const nextPts = [...pts, pts[pts.length - 1] + Math.max(0, pointsGain)];
        xp = nextXp;
        pts = nextPts;
        const start = performance.now();

        ring.current?.animate(
          [
            { opacity: 0.9, transform: 'scale(1)' },
            { opacity: 0, transform: 'scale(3.2)' },
          ],
          { duration: 900, easing: 'cubic-bezier(0.16, 1, 0.3, 1)', delay: DURATION * 0.6 },
        );

        const step = (now: number) => {
          const t = ease(Math.min(1, (now - start) / DURATION));
          if (t < 1) {
            draw(xp, pts, t);
            frame = requestAnimationFrame(step);
          } else {
            xp = xp.slice(1);
            pts = pts.slice(1);
            frame = 0;
            draw(xp, pts, 0);
          }
        };
        frame = requestAnimationFrame(step);
      },
    };

    return () => {
      cancelAnimationFrame(frame);
      api.current = null;
    };
  }, [api]);

  return (
    <svg className="chart" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" overflow="visible" aria-hidden="true">
      <defs>
        <linearGradient id="chart-fill" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stopColor="#f3f3f1" stopOpacity=".1" />
          <stop offset="1" stopColor="#f3f3f1" stopOpacity="0" />
        </linearGradient>
        {/* Lines stop at the left edge and at the dot on the right; the point entering from the right stays hidden until it slides in. */}
        <clipPath id="chart-clip">
          <rect x="0" y="-20" width={END} height={H + 40} />
        </clipPath>
      </defs>
      <g clipPath="url(#chart-clip)">
        <path ref={xpArea} fill="url(#chart-fill)" />
        <path ref={ptsLine} fill="none" stroke="var(--signal-ink)" strokeWidth="1.3" strokeDasharray="3 4" vectorEffect="non-scaling-stroke" />
        <path ref={xpLine} fill="none" stroke="#f3f3f1" strokeWidth="1.6" vectorEffect="non-scaling-stroke" />
      </g>
      {/* The baseline gives the chart a bottom edge, so the legend sits at a fixed distance below it. */}
      <line x1="0" y1={H - 0.5} x2={W} y2={H - 0.5} stroke="var(--rule-2)" strokeWidth="1" vectorEffect="non-scaling-stroke" />
      <circle ref={ring} className="chart-ring" r="3.5" fill="none" stroke="var(--reward)" strokeWidth="1.2" vectorEffect="non-scaling-stroke" />
      <circle ref={dot} r="3.5" fill="#f3f3f1" />
    </svg>
  );
}
