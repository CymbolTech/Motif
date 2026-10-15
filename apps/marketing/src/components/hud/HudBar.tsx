'use client';

import Link from 'next/link';
import { useState } from 'react';
import UseAnimations from 'react-useanimations';
import menu from 'react-useanimations/lib/menu';
import { levelFor, nextLevel } from '@/game/catalog';
import { nextQuest, questText } from '@/game/quests';
import { setQuestLog, useGame, TOTAL_ACHIEVEMENTS } from '@/game/store';
import { ConsoleLink } from '@/components/ui/ConsoleLink';
import { CoinGlyph } from '@/components/ui/Glyphs';

const NAV = [
  { href: '/#loop', label: 'The loop' },
  { href: '/#arcade', label: 'Arcade' },
  { href: '/#recipes', label: 'Recipes' },
  { href: '/docs', label: 'Docs' },
];

const SEGMENTS = 20;

export function XpMeter({ xp }: { xp: number }) {
  const level = levelFor(xp);
  const next = nextLevel(level);
  const span = next ? next.threshold - level.threshold : 1;
  const into = next ? xp - level.threshold : span;
  const filled = Math.min(SEGMENTS, Math.floor((into / span) * SEGMENTS));
  return (
    <span
      className="xp-meter"
      role="progressbar"
      aria-label={next ? `XP toward level ${next.level}` : 'XP, maximum level reached'}
      aria-valuemin={0}
      aria-valuemax={span}
      aria-valuenow={Math.min(into, span)}
    >
      {Array.from({ length: SEGMENTS }, (_, index) => (
        <i key={index} data-on={index < filled ? (index === filled - 1 ? 'edge' : 'on') : undefined} />
      ))}
    </span>
  );
}

export function HudBar() {
  const run = useGame((snapshot) => snapshot.run);
  const questLogOpen = useGame((snapshot) => snapshot.questLogOpen);
  const [menuOpen, setMenuOpen] = useState(false);
  const level = levelFor(run.xp);
  const unlocked = Object.keys(run.unlocked).length;

  return (
    <header className="hud" data-menu={menuOpen ? 'open' : undefined}>
      <div className="hud-inner frame">
        <Link className="wordmark" href="/" aria-label="MotifEngine home">
          Motif<span>Engine</span>
        </Link>

        <nav className="hud-nav" id="site-nav" aria-label="Primary" data-lenis-prevent>
          {NAV.map((item) => (
            <Link key={item.href} href={item.href} onClick={() => setMenuOpen(false)}>
              {item.label}
            </Link>
          ))}
        </nav>

        <button type="button" className="hud-objective" aria-controls="quest-log" aria-expanded={questLogOpen} onClick={() => setQuestLog((open) => !open)}>
          <span className="hud-objective-label mono">Objective</span>
          <span className="hud-objective-text">{questText(nextQuest(run))}</span>
        </button>

        <button
          type="button"
          className="hud-player"
          aria-expanded={questLogOpen}
          aria-controls="quest-log"
          onClick={() => setQuestLog((open) => !open)}
        >
          <span className="hud-level mono">
            <b>LV {String(level.level).padStart(2, '0')}</b>
            <span className="hud-level-name">{level.name}</span>
          </span>
          <XpMeter xp={run.xp} />
          <span className="hud-stat mono" aria-label={`${run.balances.coins} coins`}>
            <CoinGlyph size={14} />
            {String(run.balances.coins).padStart(3, '0')}
          </span>
          <span className="hud-stat hud-stat-quests mono" aria-label={`${unlocked} of ${TOTAL_ACHIEVEMENTS} achievements`}>
            {String(unlocked).padStart(2, '0')}/{TOTAL_ACHIEVEMENTS}
          </span>
        </button>

        <ConsoleLink className="btn btn-signal hud-cta">
          <span className="hud-cta-verb">Open </span>console
        </ConsoleLink>

        <UseAnimations
          animation={menu}
          size={26}
          active={menuOpen}
          render={(eventProps, animationProps) => (
            <button
              {...eventProps}
              type="button"
              className="hud-menu"
              aria-expanded={menuOpen}
              aria-controls="site-nav"
              aria-label={menuOpen ? 'Close menu' : 'Open menu'}
              onClick={(event) => {
                eventProps.onClick(event);
                setMenuOpen((open) => !open);
              }}
            >
              <span {...animationProps} />
            </button>
          )}
        />
      </div>
    </header>
  );
}
