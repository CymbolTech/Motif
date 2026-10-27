import type { Metadata } from 'next';
import Link from 'next/link';
import { Chevron } from '@/components/Icons';
import { SiteFooter, SiteHeader } from '@/components/SiteChrome';

export const metadata: Metadata = {
  title: 'Page not found',
  robots: { index: false },
};

export default function NotFound() {
  return (
    <>
      <SiteHeader />
      <main id="main" className="wrap not-found">
        <p className="mono">404 · event_type not registered</p>
        <h1 className="h1">
          This page does not exist
          <br />
          <span>The engine, the docs and the console are still where you left them.</span>
        </h1>
        <div className="not-found-actions">
          <Link className="pill" href="/">
            Back to Motif
            <Chevron />
          </Link>
          <Link className="pill pill-ghost" href="/docs">
            Read the docs
          </Link>
        </div>
      </main>
      <SiteFooter />
    </>
  );
}
