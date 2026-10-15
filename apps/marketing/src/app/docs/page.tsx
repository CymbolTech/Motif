import type { Metadata } from 'next';
import { Footer } from '@/components/sections/Footer';
import { DocsVisit } from '@/components/sections/DocsVisit';
import { ConsoleLink } from '@/components/ui/ConsoleLink';
import { CONCEPTS, GUIDES, OPERATORS, QUICKSTART } from '@/lib/docs';
import '../landing.css';
import './docs.css';

export const metadata: Metadata = {
  title: 'The manual — MotifEngine',
  description: 'Quickstart, concepts, rule language and guides for MotifEngine events, rules, progression, points, rewards and access.',
};

const INDEX = [
  { href: '#quickstart', label: 'Quickstart' },
  { href: '#concepts', label: 'Concepts' },
  { href: '#rule-language', label: 'Rule language' },
  ...GUIDES.map((guide) => ({ href: `#${guide.id}`, label: guide.title })),
];

export default function DocsPage() {
  return (
    <>
      <main id="main" className="docs">
        <header className="docs-hero frame">
          <h1 className="docs-title display">The manual.</h1>
          <p className="docs-lede">
            How MotifEngine behaves today: the eight-step path from an empty workspace to a reward, the concepts that keep value honest, and the API shapes behind each one.
          </p>
          <p className="docs-meta mono">[REST · OpenAPI 3.0.3 · docs/api/openapi.yaml in the repository]</p>
        </header>

        <div className="docs-layout frame">
          <nav className="docs-index mono" aria-label="On this page">
            {INDEX.map((item) => (
              <a key={item.href} href={item.href}>
                {item.label}
              </a>
            ))}
          </nav>

          <div className="docs-content">
            <section id="quickstart" className="docs-section" aria-labelledby="quickstart-title">
              <h2 id="quickstart-title" className="docs-h2 display">
                Quickstart
              </h2>
              <ol className="steps">
                {QUICKSTART.map((step, index) => (
                  <li key={step.title}>
                    <span className="step-number display" aria-hidden="true">
                      {String(index + 1).padStart(2, '0')}
                    </span>
                    <div className="step-copy">
                      <h3>{step.title}</h3>
                      <p>{step.body}</p>
                    </div>
                    <pre className="specimen">{step.code}</pre>
                  </li>
                ))}
              </ol>
            </section>

            <section id="concepts" className="docs-section" aria-labelledby="concepts-title">
              <h2 id="concepts-title" className="docs-h2 display">
                Concepts
              </h2>
              <dl className="concepts">
                {CONCEPTS.map(([term, meaning]) => (
                  <div key={term}>
                    <dt>{term}</dt>
                    <dd>{meaning}</dd>
                  </div>
                ))}
              </dl>
            </section>

            <section id="rule-language" className="docs-section" aria-labelledby="rule-language-title">
              <h2 id="rule-language-title" className="docs-h2 display">
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
              <section key={guide.id} id={guide.id} className="docs-section" aria-labelledby={`${guide.id}-title`}>
                <h2 id={`${guide.id}-title`} className="docs-h2 display">
                  {guide.title}
                </h2>
                {guide.body.map((paragraph) => (
                  <p key={paragraph.slice(0, 24)} className="docs-p">
                    {paragraph}
                  </p>
                ))}
                <pre className="specimen">{guide.code}</pre>
              </section>
            ))}

            <section className="docs-cta" aria-label="Get started">
              <p className="docs-cta-title display">Ready to wire it in?</p>
              <ConsoleLink className="btn btn-signal btn-big">Open the console</ConsoleLink>
            </section>
          </div>
        </div>
      </main>
      <Footer />
      <DocsVisit />
    </>
  );
}
