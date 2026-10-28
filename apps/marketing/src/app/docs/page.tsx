import type { Metadata } from 'next';
import { CodeBlock } from '@/components/CodeBlock';
import { Chevron } from '@/components/Icons';
import { SiteFooter, SiteHeader } from '@/components/SiteChrome';
import { CONCEPTS, GUIDES, OPERATORS, QUICKSTART } from '@/lib/docs';
import { DASHBOARD_URL, SITE_NAME, SITE_URL } from '@/lib/site';
import './docs.css';

const DESCRIPTION =
  'Add gamification to your product with Motif: quickstart, rule language and guides for events, XP and levels, points, rewards and leaderboards.';

export const metadata: Metadata = {
  title: 'Docs: Gamification API Quickstart and Guides',
  description: DESCRIPTION,
  alternates: { canonical: '/docs' },
  openGraph: {
    type: 'website',
    siteName: SITE_NAME,
    locale: 'en_US',
    title: `Docs: Gamification API Quickstart and Guides · ${SITE_NAME}`,
    description: DESCRIPTION,
    url: '/docs',
  },
};

const INDEX = [
  { href: '#quickstart', label: 'Quickstart' },
  { href: '#concepts', label: 'Concepts' },
  { href: '#rule-language', label: 'Rule language' },
  ...GUIDES.map((guide) => ({ href: `#${guide.id}`, label: guide.title })),
];

const STRUCTURED_DATA = {
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'TechArticle',
      headline: 'Motif documentation',
      description: DESCRIPTION,
      url: `${SITE_URL}/docs`,
      inLanguage: 'en',
      about: { '@id': `${SITE_URL}/#software` },
      publisher: { '@id': `${SITE_URL}/#organization` },
    },
    {
      '@type': 'HowTo',
      name: 'Add gamification to your product with Motif',
      step: QUICKSTART.map((step, index) => ({ '@type': 'HowToStep', position: index + 1, name: step.title, text: step.body })),
    },
    {
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: SITE_NAME, item: SITE_URL },
        { '@type': 'ListItem', position: 2, name: 'Docs', item: `${SITE_URL}/docs` },
      ],
    },
  ],
};

export default function DocsPage() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(STRUCTURED_DATA) }} />
      <SiteHeader />
      <main id="main" className="docs wrap">
        <header className="docs-hero">
          <h1 className="h1">
            Motif docs
            <br />
            <span>Add gamification and loyalty to your product</span>
          </h1>
          <p className="lead">
            The path from an empty workspace to a reward in eight steps, the concepts that keep value honest, and the API shapes behind each one. REST, described by an OpenAPI 3.0.3 contract.
          </p>
        </header>

        <div className="docs-layout">
          <nav className="docs-index" aria-label="On this page">
            <p className="docs-index-title">On this page</p>
            <ul>
              {INDEX.map((item) => (
                <li key={item.href}>
                  <a href={item.href}>{item.label}</a>
                </li>
              ))}
            </ul>
          </nav>

          <div className="docs-content">
            <section id="quickstart" className="docs-section" aria-labelledby="quickstart-title" data-reveal data-earn="docs.quickstart:10">
              <h2 id="quickstart-title" className="h2">
                Quickstart
              </h2>
              <ol className="docs-steps">
                {QUICKSTART.map((step, index) => (
                  <li key={step.title}>
                    <div className="docs-step-copy">
                      <span className="docs-step-index" aria-hidden="true">
                        {String(index + 1).padStart(2, '0')}
                      </span>
                      <h3>{step.title}</h3>
                      <p>{step.body}</p>
                    </div>
                    <CodeBlock className="docs-code" code={step.code} />
                  </li>
                ))}
              </ol>
            </section>

            <section id="concepts" className="docs-section" aria-labelledby="concepts-title" data-reveal data-earn="docs.concepts:10">
              <h2 id="concepts-title" className="h2">
                Concepts
              </h2>
              <dl className="docs-concepts">
                {CONCEPTS.map(([term, meaning]) => (
                  <div key={term}>
                    <dt>{term}</dt>
                    <dd>{meaning}</dd>
                  </div>
                ))}
              </dl>
            </section>

            <section id="rule-language" className="docs-section" aria-labelledby="rule-language-title" data-reveal data-earn="docs.rule-language:10">
              <h2 id="rule-language-title" className="h2">
                Rule language
              </h2>
              <p className="docs-p">
                Conditions are data, not executable customer code. Each condition object holds exactly one operator; paths use dot notation such as <code>$.order.total</code>. Trees are bounded to eight nesting levels, 64 nodes and 32 children per array.
              </p>
              <table className="docs-table">
                <thead>
                  <tr>
                    <th scope="col">Operator</th>
                    <th scope="col">Meaning</th>
                  </tr>
                </thead>
                <tbody>
                  {OPERATORS.map(([operator, meaning]) => (
                    <tr key={operator}>
                      <td>
                        <code>{operator}</code>
                      </td>
                      <td>{meaning}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <table className="docs-table">
                <thead>
                  <tr>
                    <th scope="col">Action</th>
                    <th scope="col">Effect</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>
                      <code>award_xp</code>
                    </td>
                    <td>Adds non-spendable XP to the external user.</td>
                  </tr>
                  <tr>
                    <td>
                      <code>award_points</code>
                    </td>
                    <td>Appends an award entry to the named currency’s ledger.</td>
                  </tr>
                  <tr>
                    <td>
                      <code>log</code>
                    </td>
                    <td>Records a rule action without changing user state.</td>
                  </tr>
                </tbody>
              </table>
            </section>

            {GUIDES.map((guide) => (
              <section key={guide.id} id={guide.id} className="docs-section" aria-labelledby={`${guide.id}-title`} data-reveal data-earn={`docs.${guide.id}:10`}>
                <h2 id={`${guide.id}-title`} className="h2">
                  {guide.title}
                </h2>
                {guide.body.map((paragraph) => (
                  <p key={paragraph.slice(0, 24)} className="docs-p">
                    {paragraph}
                  </p>
                ))}
                <CodeBlock className="docs-code" code={guide.code} />
              </section>
            ))}

            <section className="docs-cta" aria-labelledby="docs-cta-title">
              <h2 id="docs-cta-title" className="h2">
                Ready to wire it in?
                <br />
                <span>Create a workspace and send your first event to sandbox.</span>
              </h2>
              <a className="pill" href={DASHBOARD_URL} data-start>
                Start building
                <Chevron />
              </a>
            </section>
          </div>
        </div>
      </main>
      <SiteFooter />
    </>
  );
}
