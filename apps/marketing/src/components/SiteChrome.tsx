import Link from 'next/link';
import { Arrow, Chevron, Logo } from '@/components/Icons';
import { RunPill } from '@/components/RunPill';
import { COMPANY, DASHBOARD_URL, NAV, SITE_NAME } from '@/lib/site';

export function SiteHeader() {
  return (
    <>
      <a className="skip" href="#main">
        Skip to content
      </a>
      <p className="announce">
        <Link href="/docs#leaderboards">
          <span className="announce-dim">New in Motif:</span> Leaderboards with weekly, monthly and hand-run rounds
          <Arrow />
        </Link>
      </p>
      <header className="site-header wrap">
        <Link className="brand" href="/" aria-label={`${SITE_NAME} home`}>
          <Logo className="brand-mark" />
          {SITE_NAME}
        </Link>
        <nav aria-label="Primary">
          <ul className="nav-links">
            {NAV.map((item) => (
              <li key={item.href}>
                <Link href={item.href}>{item.label}</Link>
              </li>
            ))}
          </ul>
        </nav>
        <div className="nav-actions">
          <RunPill />
          <a className="nav-signin" href={DASHBOARD_URL}>
            Sign in
          </a>
          <a className="pill" href={DASHBOARD_URL} data-start>
            Start building
            <Chevron />
          </a>
        </div>
      </header>
    </>
  );
}

const FOOTER = [
  {
    title: 'Product',
    links: [
      ['Events', '/docs#events'],
      ['Rules', '/docs#rules'],
      ['XP and levels', '/docs#progression'],
      ['Points and ledger', '/docs#points'],
      ['Rewards', '/docs#rewards'],
      ['Leaderboards', '/docs#leaderboards'],
    ],
  },
  {
    title: 'Developers',
    links: [
      ['Docs', '/docs'],
      ['Quickstart', '/docs#quickstart'],
      ['Rule language', '/docs#rule-language'],
      ['MCP server', '/docs#mcp'],
    ],
  },
  {
    title: 'Learn',
    links: [
      ['Use cases', '/#use-cases'],
      ['FAQ', '/#faq'],
      ['Concepts', '/docs#concepts'],
    ],
  },
  {
    title: 'Console',
    links: [
      ['Sign in', DASHBOARD_URL],
      ['Create a workspace', DASHBOARD_URL],
    ],
  },
] as const;

export function SiteFooter() {
  return (
    <footer className="site-footer">
      <div className="wrap">
        <div className="footer-cols">
          <div className="footer-brand">
            <Link className="brand" href="/">
              <Logo className="brand-mark" />
              {SITE_NAME}
            </Link>
            <p>Gamification and loyalty as an API.</p>
          </div>
          {FOOTER.map((column) => (
            <div key={column.title}>
              <p className="footer-title">{column.title}</p>
              <ul>
                {column.links.map(([label, href]) => (
                  <li key={label}>
                    {href.startsWith('/') ? <Link href={href}>{label}</Link> : <a href={href}>{label}</a>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="footer-base">
          <p>
            © 2026 {SITE_NAME} by {COMPANY}
          </p>
          <p>Console figures on this site are synthetic.</p>
        </div>
      </div>
    </footer>
  );
}
