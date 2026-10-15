import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';
import '@fontsource-variable/anybody/standard.css';
import '@fontsource-variable/martian-mono/standard.css';
import '@fontsource-variable/host-grotesk/index.css';
import './globals.css';
import './hud.css';
import { GameRuntime } from '@/components/hud/GameRuntime';
import { HudBar } from '@/components/hud/HudBar';

export const metadata: Metadata = {
  title: 'MotifEngine — Your product already has players',
  description:
    'MotifEngine is an open-source engine for XP, achievements, missions, point currencies and rewards. Your product sends events, your team publishes rules, every reward is ledgered.',
  openGraph: {
    title: 'MotifEngine — Your product already has players',
    description: 'Events in, published rules in the middle, ledgered rewards out. An open-source engagement and loyalty engine.',
    type: 'website',
  },
};

export const viewport: Viewport = {
  themeColor: '#e3e2dc',
  width: 'device-width',
  initialScale: 1,
};

// Runs before first paint so a returning visitor never sees the boot screen
// flash, and a visitor who redeemed Night shift never sees the day theme flash.
const preflight = `try{var d=document.documentElement;if(sessionStorage.getItem('motifengine.booted')||matchMedia('(prefers-reduced-motion: reduce)').matches)d.dataset.booted='1';var r=JSON.parse(localStorage.getItem('motifengine.run.v1')||'null');if(r&&r.unlocks&&r.unlocks['night-shift']&&localStorage.getItem('motifengine.theme')!=='day')d.dataset.theme='night';}catch(e){}`;

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: preflight }} />
      </head>
      <body>
        <a className="skip" href="#main">
          Skip to content
        </a>
        <div className="guides" aria-hidden="true">
          <div className="frame">
            <div className="grid">
              {Array.from({ length: 12 }, (_, index) => (
                <span key={index} />
              ))}
            </div>
          </div>
        </div>
        <HudBar />
        {children}
        <GameRuntime />
      </body>
    </html>
  );
}
