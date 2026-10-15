import Link from 'next/link';
import { HiddenTarget } from '@/components/ui/HiddenTarget';
import { DASHBOARD_URL } from '@/lib/content';

export function Footer() {
  return (
    <footer className="footer">
      <div className="frame">
        <p className="footer-mark display" aria-hidden="true">
          Motif<span>Engine</span>
        </p>
        <div className="footer-row">
          <nav className="footer-links" aria-label="Footer">
            <Link href="/#loop">The loop</Link>
            <Link href="/#arcade">Arcade</Link>
            <Link href="/#engine">Engine</Link>
            <Link href="/#recipes">Recipes</Link>
            <Link href="/docs">Docs</Link>
            <a href={DASHBOARD_URL}>Console</a>
          </nav>
          <p className="footer-note mono">
            Open source · API first · Ledgered
            <HiddenTarget id="charlie" className="target-footer" />
          </p>
          <p className="footer-note mono">Achievements, XP and coins on this site are a simulation stored in your browser.</p>
        </div>
      </div>
    </footer>
  );
}
