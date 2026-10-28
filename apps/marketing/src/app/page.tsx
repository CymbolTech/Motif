import type { Metadata } from 'next';
import Link from 'next/link';
import type { CSSProperties, ReactNode } from 'react';
import { Glyph } from '@/components/Glyphs';
import { Arrow, Chevron, Logo, Plus } from '@/components/Icons';
import { LiveConsole } from '@/components/LiveConsole';
import { Plate } from '@/components/Plate';
import { SiteFooter, SiteHeader } from '@/components/SiteChrome';
import { CAPABILITIES, FAQ, FLOW, LAUNCH, USE_CASES } from '@/lib/landing';
import { DASHBOARD_URL, DESCRIPTION, SITE_NAME, SITE_URL } from '@/lib/site';
import './landing.css';

export const metadata: Metadata = {
  alternates: { canonical: '/' },
};

const MAP_LEVELS = [
  { n: 5, name: 'Pro', xp: 10000, earned: true },
  { n: 6, name: 'Veteran', xp: 14000, earned: true },
  { n: 7, name: 'Expert', xp: 18000, earned: true },
  { n: 8, name: 'Master', xp: 22500, earned: false },
  { n: 9, name: 'Legend', xp: 30000, earned: false },
];

/** user-42 sits at 18,420 XP: 420 of the 4,500 between level 7 and level 8. */
const MAP_YOU = (2 + 420 / 4500) / (MAP_LEVELS.length - 1);

const POINT_STORY = [
  { verb: 'Earn', amount: '+1,500', text: 'Earned on eight orders, set by your rules.', after: 1500 },
  { verb: 'Convert', amount: '−500', text: 'Swapped for 50 coins at a 10:1 rate.', after: 1000 },
  { verb: 'Redeem', amount: '−800', text: 'Spent on a 10% coupon, delivered automatically and retried if it fails.', after: 200 },
  { verb: 'Expire', amount: '−50', text: 'Points older than a year expired. The history stays.', after: 150 },
];

const STRUCTURED_DATA = {
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'SoftwareApplication',
      '@id': `${SITE_URL}/#software`,
      name: SITE_NAME,
      url: SITE_URL,
      description: DESCRIPTION,
      applicationCategory: 'DeveloperApplication',
      applicationSubCategory: 'Gamification and loyalty API',
      operatingSystem: 'Web',
      publisher: { '@id': `${SITE_URL}/#organization` },
      featureList: [
        'Gamification API',
        'Event tracking with versioned contracts',
        'Rules engine for rewards',
        'XP and levels',
        'Daily streaks',
        'Missions',
        'Achievements and badges',
        'Loyalty points and multiple currencies',
        'Append-only points ledger',
        'Rewards and coupon redemption',
        'Leaderboards with recurring rounds',
        'Funnels, retention and segments',
        'MCP server for AI agents',
      ],
    },
    {
      '@type': 'FAQPage',
      '@id': `${SITE_URL}/#faq`,
      mainEntity: FAQ.map((item) => ({
        '@type': 'Question',
        name: item.q,
        acceptedAnswer: { '@type': 'Answer', text: item.a },
      })),
    },
  ],
};

function SectionHead({ id, index, label, title, sub, children }: { id: string; index?: string; label: string; title: string; sub: string; children?: ReactNode }) {
  return (
    <div className="head" data-reveal>
      <p className="head-label">
        {index && <span className="head-index">{index}</span>}
        {label}
      </p>
      <div>
        <h2 id={id} className="h2">
          {title}
          <br />
          <span>{sub}</span>
        </h2>
        {children && <p className="lead">{children}</p>}
      </div>
    </div>
  );
}

export default function HomePage() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(STRUCTURED_DATA) }} />
      <SiteHeader />
      <main id="main">
        <section className="hero wrap" aria-labelledby="hero-title">
          <h1 id="hero-title" className="h1">
            Meet <Logo className="h1-logo" /> {SITE_NAME}
            <br />
            <span>Gamification and loyalty as an API.</span>
          </h1>
          <div className="ctas">
            <a className="pill" href={DASHBOARD_URL} data-start>
              Start building
              <Chevron />
            </a>
            <Link className="pill pill-ghost" href="#how-it-works">
              See how it works
            </Link>
          </div>

          <div className="stage">
            <Plate relief="hero" priority className="plate-hero">
              <LiveConsole />
            </Plate>
            <div className="stage-aside">
              <p className="aside-title">
                XP, missions, points and rewards.
                <span>Your app reports what users do. Motif decides what they earn, keeps every balance honest, and hands back their level, badges and rewards.</span>
              </p>
              <p className="aside-label">What you can launch</p>
              <ul className="launch">
                {LAUNCH.map(([item, tag]) => (
                  <li key={item}>
                    {item}
                    <span>{tag}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>

        <section id="building-blocks" className="blocks wrap" aria-labelledby="blocks-title" data-earn="read.blocks:15">
          <div className="head" data-reveal>
            <h2 id="blocks-title" className="h2">
              Pick your power-ups
              <br />
              <span>ten building blocks for any gamification program, one API</span>
            </h2>
            <p className="lead lead-flush">
              Pick the blocks your product needs today and add the rest later. They share one event stream and one ledger, so a finished lesson can grant XP, tick a mission and unlock a badge at once.
            </p>
          </div>
          <ul className="tiles">
            {CAPABILITIES.map((item, index) => (
              <li key={item.title} data-reveal style={{ '--i': index } as CSSProperties}>
                <Link className="tile" href={item.href}>
                  <span className="tile-index">{String(index + 1).padStart(2, '0')}</span>
                  <Glyph name={item.glyph} />
                  <span className="tile-copy">
                    <h3 className="tile-title">{item.title}</h3>
                    <span className="tile-line">{item.line}</span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>

        <section id="how-it-works" className="wrap" aria-labelledby="fits-title" data-earn="read.how:15">
          <div className="fits" data-reveal>
          <div>
            <h2 id="fits-title" className="h2">
              Three calls to level one
              <br />
              <span>your backend talks to Motif, your app shows the score</span>
            </h2>
            <p className="lead">Send what happened, let your rules decide, read what each user earned. Motif runs the workers, the ledger and the retries.</p>
            <Link className="text-link" href="/docs#quickstart">
              <span className="text-link-icon">
                <Arrow />
              </span>
              Read the quickstart
            </Link>
          </div>
          <ol className="flow">
            {FLOW.map(([where, what, note]) => (
              <li key={where}>
                <span className="flow-where">{where}</span>
                <code>{what}</code>
                <span className="flow-note">{note}</span>
              </li>
            ))}
          </ol>
          </div>
        </section>

        <section className="feature wrap" aria-labelledby="track-title" data-earn="read.track:20">
          <SectionHead id="track-title" index="01" label="Event tracking" title="Every move counts" sub="one event call, from any backend">
            Send Motif the moments that matter: a paid order, a finished lesson, a daily check-in. Each event is checked against its schema, stored once and processed in the background, so a retried request never pays out twice.
          </SectionHead>
          <Plate relief="track">
            <figure className="card code-card">
              <figcaption className="card-head">
                <span className="card-brand">POST /v1/events</span>
                <span>sandbox</span>
              </figcaption>
              <pre>
                <code>
                  {'{\n  '}
                  <span className="k">&quot;event_id&quot;</span>: <span className="v">&quot;order-8812-paid&quot;</span>
                  {',\n  '}
                  <span className="k">&quot;event_type&quot;</span>: <span className="v">&quot;order.paid&quot;</span>
                  {',\n  '}
                  <span className="k">&quot;user_id&quot;</span>: <span className="v">&quot;user-42&quot;</span>
                  {',\n  '}
                  <span className="k">&quot;payload&quot;</span>
                  {': {\n    '}
                  <span className="k">&quot;total&quot;</span>: <span className="v">64.00</span>
                  {',\n    '}
                  <span className="k">&quot;order_count&quot;</span>: <span className="v">2</span>
                  {'\n  }\n}'}
                </code>
              </pre>
              <div className="card-foot">
                <span className="mono">202 Accepted · receipt committed</span>
                <span>queued, then processed</span>
              </div>
            </figure>
          </Plate>
        </section>

        <section className="feature wrap" aria-labelledby="decide-title" data-earn="read.rules:20">
          <SectionHead id="decide-title" index="02" label="Rules engine" title="Change the rules of the game, not your code" sub="rules your product team can read, test in sandbox and publish">
            Write a rule the way you would say it: when an order is paid, if it is the second one, give 200 points and 50 XP. Publish a new version whenever the program changes. Every decision is logged.
          </SectionHead>
          <Plate relief="decide">
            <figure className="card duo">
              <figcaption className="sr-only">An example published rule and its execution log, with synthetic data.</figcaption>
              <div>
                <div className="duo-head">
                  <b>Second purchase bonus</b>
                  <span className="tag tag-on">Published · v3</span>
                </div>
                <dl className="clauses">
                  <div>
                    <dt>When</dt>
                    <dd className="mono">order.paid</dd>
                  </div>
                  <div>
                    <dt>If</dt>
                    <dd className="mono">$.order_count == 2</dd>
                  </div>
                  <div>
                    <dt>Then</dt>
                    <dd className="mono">award_points loyalty 200</dd>
                  </div>
                  <div>
                    <dt>
                      <span className="sr-only">And</span>
                    </dt>
                    <dd className="mono">award_xp 50</dd>
                  </div>
                </dl>
              </div>
              <div>
                <div className="duo-head duo-head-quiet">
                  <span>Execution log</span>
                  <span>today</span>
                </div>
                <ul className="log">
                  {[
                    ['user-42', 'matched', '14:02'],
                    ['user-17', 'skipped', '13:58'],
                    ['user-93', 'matched', '13:41'],
                    ['user-08', 'skipped', '13:40'],
                  ].map(([user, result, time]) => (
                    <li key={user} className={result === 'skipped' ? 'log-skip' : undefined}>
                      <i aria-hidden="true" />
                      <span className="mono">
                        {user} · {result}
                      </span>
                      <span className="log-time">{time}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </figure>
          </Plate>
        </section>

        <section className="feature wrap" aria-labelledby="progress-title" data-earn="read.progress:20">
          <SectionHead id="progress-title" index="03" label="Progression" title="Give them a reason to come back tomorrow" sub="XP, levels, streaks, missions and achievements">
            Levels show how far someone has come. Missions give them something to finish this week. Achievements mark the milestones. XP is kept apart from spendable points, so levelling up never costs a reward.
          </SectionHead>
          <figure className="map" data-reveal style={{ '--to': `${MAP_YOU * 100}%` } as CSSProperties}>
            <figcaption className="sr-only">An example level map for one user at 18,420 XP between level 7 and level 8, with synthetic data.</figcaption>
            <div className="map-scroll">
              <div className="map-inner">
                <div className="map-track" aria-hidden="true">
                  {MAP_LEVELS.map((step, i) => (
                    <i key={step.n} className={`map-node ${step.earned ? 'is-passed' : ''}`} style={{ left: `${(i / (MAP_LEVELS.length - 1)) * 100}%` }} />
                  ))}
                  <span className="map-fill">
                    <span className="map-marker">
                      <span className="map-avatar" />
                      <span className="map-you mono">user-42 · 18,420 XP</span>
                    </span>
                  </span>
                </div>
                <ol className="map-row map-levels">
                  {MAP_LEVELS.map((step) => (
                    <li key={step.n} className={step.earned ? 'is-passed' : undefined}>
                      <b>
                        Level {step.n} · {step.name}
                      </b>
                      <span className="mono">{step.xp.toLocaleString('en-US')} XP</span>
                    </li>
                  ))}
                </ol>
              </div>
            </div>
          </figure>
        </section>

        <section className="feature wrap" aria-labelledby="reward-title" data-earn="read.rewards:20">
          <SectionHead id="reward-title" index="04" label="Loyalty and rewards" title="Points worth playing for" sub="loyalty points people earn, swap and spend">
            Give customers points for what they do, let them trade points for coins or coupons, and expire old points on schedule. Motif keeps every balance correct along the way.
          </SectionHead>
          <figure className="points" data-reveal>
            <figcaption className="points-head">
              <span>One customer’s loyalty points, start to finish</span>
              <span className="points-now">
                Balance now <b className="num">150</b>
              </span>
            </figcaption>
            <ol className="points-steps">
              {POINT_STORY.map((step, index) => (
                <li key={step.verb} className={`is-${step.verb.toLowerCase()}`} style={{ '--i': index } as CSSProperties}>
                  <span className="points-verb">
                    <span className="points-index num">{index + 1}</span>
                    {step.verb}
                  </span>
                  <b className="points-amount num">{step.amount}</b>
                  <p>{step.text}</p>
                  <span className="points-after">
                    <span className="points-meter" aria-hidden="true">
                      <i style={{ transform: `scaleX(${step.after / 1500})` }} />
                    </span>
                    <span>
                      Balance <span className="num">{step.after.toLocaleString('en-US')}</span>
                    </span>
                  </span>
                </li>
              ))}
            </ol>
            <p className="points-note">Every change is saved as its own line and never edited, so the balance always adds up.</p>
          </figure>
        </section>

        <section className="feature wrap" aria-labelledby="agents-title" data-earn="read.agents:20">
          <SectionHead id="agents-title" index="05" label="AI agents" title="Let your agent run the game" sub="a Motif MCP server for your workspace">
            Connect Claude Code, Cursor or any MCP client with a scoped access token. Check a rule against a sample payload, send a test event and see what it awarded, or start next week’s leaderboard round in plain language.
          </SectionHead>
          <Plate relief="agents">
            <figure className="card term">
              <figcaption className="card-head">
                <span className="card-brand">mcp · northwind</span>
                <span>token scoped to production</span>
              </figcaption>
              <pre>
                <code>
                  <span className="t-prompt">› </span>
                  <span className="t-ink">Weekly coins board, reset Mondays</span>
                  {'\n  '}
                  <span className="t-ok">●</span> board created{'  '}
                  <span className="t-ink">weekly-coins</span>
                  {'\n  '}
                  <span className="t-ok">●</span> round opened{'   '}
                  <span className="t-ink">Oct 26 → Nov 2</span>
                  {'\n'}
                  <span className="t-prompt">› </span>
                  <span className="t-ink">Who&apos;s leading this round?</span>
                  {'\n  1  user-93   '}
                  <span className="t-ink">4,210</span>
                  {'\n  2  user-42   '}
                  <span className="t-ink">3,880</span>
                  {'\n  3  user-17   '}
                  <span className="t-ink">2,904</span>
                </code>
              </pre>
            </figure>
          </Plate>
        </section>

        <section id="use-cases" className="feature wrap" aria-labelledby="uses-title" data-earn="read.uses:15">
          <div className="head" data-reveal>
            <p className="head-label">
              <span className="head-index">06</span>Use cases
            </p>
            <h2 id="uses-title" className="h2">
              Gamification for products people return to
              <br />
              <span>shops, courses, apps, communities and loyalty clubs</span>
            </h2>
          </div>
          <ul className="uses">
            {USE_CASES.map((item, index) => (
              <li key={item.title} data-reveal style={{ '--i': index } as CSSProperties}>
                <h3 className="use-title">{item.title}</h3>
                <p>{item.body}</p>
              </li>
            ))}
          </ul>
        </section>

        <section id="faq" className="feature wrap" aria-labelledby="faq-title" data-earn="read.faq:15">
          <div className="head" data-reveal>
            <p className="head-label">
              <span className="head-index">07</span>Questions
            </p>
            <div>
              <h2 id="faq-title" className="h2">
                Gamification, answered
                <br />
                <span>what Motif is and how it fits your product</span>
              </h2>
              <div className="faq">
                {FAQ.map((item, index) => (
                  <details key={item.q} open={index === 0} data-reveal style={{ '--i': index } as CSSProperties}>
                    <summary>
                      <h3>{item.q}</h3>
                      <Plus className="faq-icon" />
                    </summary>
                    <p>{item.a}</p>
                  </details>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="close wrap" aria-labelledby="close-title" data-earn="read.close:25">
          <h2 id="close-title" className="h1" data-reveal>
            Player one, press start
            <br />
            <span>Send your first event to sandbox and watch it pay out</span>
          </h2>
          <div className="ctas ctas-center" data-reveal style={{ '--i': 2 } as CSSProperties}>
            <a className="pill" href={DASHBOARD_URL} data-start>
              Start building
              <Chevron />
            </a>
            <Link className="pill pill-ghost" href="/docs">
              Read the docs
            </Link>
          </div>
        </section>
      </main>
      <SiteFooter />
    </>
  );
}
