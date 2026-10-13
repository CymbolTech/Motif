import {
  IconArrowRight,
  IconArrowUpRight,
  IconBolt,
  IconBraces,
  IconChevronDown,
  IconCircleCheck,
  IconCode,
  IconCoin,
  IconDatabase,
  IconGift,
  IconLock,
  IconMenu2,
  IconX,
} from '@tabler/icons-react';
import { Link } from '@tanstack/react-router';
import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { capabilities, scenarios } from '../content';

const dashboardURL = 'http://127.0.0.1:4173';

const productTabs = [
  { label: 'Events', icon: IconBraces, description: 'A typed event enters the engine and gets a traceable receipt.', headline: 'Every signal has a record.', example: 'order.completed', state: 'Accepted', detail: 'Schema v2 · external ID order-1042', action: 'Inspect receipt', rows: [['Event type', 'order.completed'], ['Environment', 'Sandbox'], ['Processing', 'Rule evaluation queued']] },
  { label: 'Rules', icon: IconCode, description: 'Published conditions decide what happens next.', headline: 'Logic your team can explain.', example: 'second_purchase', state: 'Published', detail: 'Version 3 · order.completed', action: 'View rule version', rows: [['Condition', 'purchase_count ≥ 2'], ['Action', 'Award loyalty points'], ['Schedule', 'Always active']] },
  { label: 'Points', icon: IconCoin, description: 'A separate ledger keeps each currency accountable.', headline: 'Value with a history.', example: 'loyalty_points', state: 'Recorded', detail: 'Award · source order-1042', action: 'Inspect ledger', rows: [['Currency', 'Loyalty points'], ['Entry', 'Award from rule'], ['Retry', 'Same idempotency key']] },
  { label: 'Progression', icon: IconBolt, description: 'Missions, XP and achievements make progress visible.', headline: 'Progress that means something.', example: 'learning_path', state: 'In progress', detail: 'lesson.completed · mission step', action: 'View progression', rows: [['Signal', 'lesson.completed'], ['Effect', 'XP and mission progress'], ['Milestone', 'Achievement when complete']] },
  { label: 'Rewards', icon: IconGift, description: 'Redemption and delivery stay visible through uncertainty.', headline: 'A clear path to fulfilment.', example: 'welcome_coupon', state: 'Reserved', detail: 'Redemption · provider pending', action: 'Inspect redemption', rows: [['Reward', 'Coupon'], ['Delivery', 'Provider attempt tracked'], ['Recovery', 'Retry or reconcile']] },
] as const;

const questions = [
  { question: 'What does MotifEngine do?', answer: 'It receives events from your product, evaluates published rules, records points and progression, and manages reward redemption. The console lets operators configure and inspect that flow.' },
  { question: 'How does my application connect to it?', answer: 'Your backend sends events with a tenant and environment scoped integration credential. MotifEngine returns a receipt; your application can then read the resulting state through its APIs.' },
  { question: 'Can a tenant see another tenant’s data?', answer: 'No. Platform administrators manage tenants, while tenant administrators work only within their memberships. The server determines scope for each request.' },
  { question: 'What happens if an event or reward delivery is retried?', answer: 'Event IDs and idempotency keys make repeated requests safe. Reward attempts and ambiguous provider outcomes remain visible for retry or reconciliation.' },
  { question: 'Is MotifEngine open source?', answer: 'Yes. MotifEngine is an open-source project, with the platform and documentation developed together in this codebase.' },
  { question: 'Is pricing available?', answer: 'Public pricing has not been set. The current site focuses on the product capabilities and documentation without suggesting a plan or price that does not exist.' },
];

const systemCounts = [
  { value: productTabs.length, label: 'product views' },
  { value: capabilities.length, label: 'core capabilities' },
  { value: scenarios.length, label: 'use cases' },
] as const;

function useMarketingMotion(rootRef: React.RefObject<HTMLDivElement | null>) {
  useLayoutEffect(() => {
    const root = rootRef.current;
    if (!root || typeof IntersectionObserver === 'undefined') return;

    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const revealTargets = root.querySelectorAll<HTMLElement>('[data-reveal]');
    const countTargets = root.querySelectorAll<HTMLElement>('[data-count]');
    const frames = new Set<number>();

    const isOnScreen = (element: HTMLElement) => {
      const bounds = element.getBoundingClientRect();
      return bounds.top < window.innerHeight && bounds.bottom > 0;
    };

    const revealObserver = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return;
        const element = entry.target as HTMLElement;
        element.classList.remove('reveal-pending');
      });
    }, { rootMargin: '0px 0px -18% 0px', threshold: 0.05 });

    // Re-arm only after an element is completely outside the viewport. This
    // keeps a return visit animated without dimming content while it is read.
    const rearmObserver = new IntersectionObserver((entries) => {
      if (reducedMotion) return;
      entries.forEach((entry) => {
        if (!entry.isIntersecting && !isOnScreen(entry.target as HTMLElement)) {
          (entry.target as HTMLElement).classList.add('reveal-pending');
        }
      });
    });

    const countObserver = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return;
        const element = entry.target as HTMLElement;
        countObserver.unobserve(element);

        if (!reducedMotion) {
          const value = Number(element.dataset.count);
          let start: number | null = null;
          let frame = 0;
          const tick = (time: number) => {
            frames.delete(frame);
            if (start === null) start = time;
            const progress = Math.min((time - start) / 700, 1);
            element.textContent = String(Math.round(value * progress)).padStart(2, '0');
            if (progress < 1) {
              frame = requestAnimationFrame(tick);
              frames.add(frame);
            }
          };
          frame = requestAnimationFrame(tick);
          frames.add(frame);
        }
      });
    }, { rootMargin: '0px 0px -15% 0px', threshold: 0.35 });

    revealTargets.forEach((element) => {
      if (reducedMotion) return;
      if (!isOnScreen(element)) element.classList.add('reveal-pending');
      revealObserver.observe(element);
      rearmObserver.observe(element);
    });
    countTargets.forEach((element) => {
      if (reducedMotion || isOnScreen(element)) return;
      element.textContent = '00';
      countObserver.observe(element);
    });

    return () => {
      revealObserver.disconnect();
      rearmObserver.disconnect();
      countObserver.disconnect();
      frames.forEach((frame) => cancelAnimationFrame(frame));
    };
  }, [rootRef]);
}

function AnimatedCount({ value }: { value: number }) {
  const formatted = String(value).padStart(2, '0');
  return <><span data-count={value} aria-hidden="true">{formatted}</span><span className="visually-hidden">{formatted}</span></>;
}

function Brand() {
  return <Link to="/" className="wordmark" aria-label="MotifEngine home"><span className="brand-star" aria-hidden="true">✷</span><span>Motif<span className="wordmark-suffix">Engine</span></span></Link>;
}

export function SiteHeader() {
  const [open, setOpen] = useState(false);
  return <header className="site-header"><div className="header-inner"><Brand /><nav id="mobile-navigation" className={open ? 'site-nav open' : 'site-nav'} aria-label="Primary navigation"><a href="/#capabilities" onClick={() => setOpen(false)}>Capabilities</a><a href="/#how-it-works" onClick={() => setOpen(false)}>How it works</a><a href="/#use-cases" onClick={() => setOpen(false)}>Use cases</a><Link to="/docs" onClick={() => setOpen(false)}>Docs</Link><a className="nav-console" href={dashboardURL}>Open console <IconArrowUpRight size={16} /></a></nav><button className="mobile-menu" type="button" aria-expanded={open} aria-controls="mobile-navigation" aria-label={open ? 'Close menu' : 'Open menu'} onClick={() => setOpen((value) => !value)}>{open ? <IconX size={21} /> : <IconMenu2 size={21} />}</button></div></header>;
}

function Eyebrow({ children }: { children: ReactNode }) { return <span className="section-kicker">{children}</span>; }

function ProductPreview({ active }: { active: number }) {
  const tab = productTabs[active];
  const Icon = tab.icon;
  return <div className="showcase" id="product-preview"><div className="showcase-glow" /><div className="preview-frame" role="tabpanel" id="product-preview-panel" aria-labelledby={`preview-tab-${active}`}><div className="preview-rail"><div className="preview-brand"><span className="brand-star">✷</span><span>MotifEngine</span></div><div className="preview-tenant"><span className="preview-tenant-mark">M</span><span><strong>Example workspace</strong><small>Sandbox environment</small></span><IconChevronDown size={13} /></div><span className="preview-group-label">WORKSPACE</span><div className="preview-rail-links">{productTabs.map((item, index) => { const ItemIcon = item.icon; return <div key={item.label} className={active === index ? 'preview-rail-link selected' : 'preview-rail-link'}><ItemIcon size={16} />{item.label}</div>; })}</div><div className="preview-rail-bottom"><IconLock size={14} /> Illustrative product view</div></div><div className="preview-workspace"><div className="preview-toolbar"><span>Workspace <span className="preview-slash">/</span> {tab.label}</span><span className="preview-env"><i /> Sandbox</span></div><div className="preview-content"><div className="preview-page-head"><div><span className="preview-overline">{tab.label.toUpperCase()} / OVERVIEW</span><h3>{tab.headline}</h3><p>{tab.description}</p></div><span className="preview-head-icon"><Icon size={22} /></span></div><div className="preview-cards"><div className="preview-card preview-card-main"><span className="preview-card-label">EXAMPLE RECORD</span><strong>{tab.example}</strong><small>{tab.detail}</small></div><div className="preview-card preview-card-status"><span className="preview-card-label">CURRENT STATE</span><strong><i />{tab.state}</strong><small>Traceable in the console</small></div></div><div className="preview-detail"><div className="preview-detail-head"><span>Record details</span><span className="preview-detail-action">{tab.action} <IconArrowUpRight size={13} /></span></div>{tab.rows.map(([label, value]) => <div className="preview-detail-row" key={label}><span>{label}</span><strong>{value}</strong></div>)}</div><div className="preview-timeline"><span className="preview-timeline-dot" /><span>Sample flow</span><span>Event received</span><IconArrowRight size={13} /><span>Rule evaluated</span><IconArrowRight size={13} /><span>Effect recorded</span></div></div></div></div><div className="showcase-caption">Illustrative interface · Example data, not live activity</div></div>;
}

function TabShowcase() {
  const [active, setActive] = useState(0);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);
  function handleTabKey(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next = event.key === 'ArrowRight' ? (index + 1) % productTabs.length : event.key === 'ArrowLeft' ? (index - 1 + productTabs.length) % productTabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? productTabs.length - 1 : -1;
    if (next < 0) return;
    event.preventDefault();
    setActive(next);
    tabRefs.current[next]?.focus();
  }
  return <div className="product-showcase"><div className="product-tabs" role="tablist" aria-label="Explore MotifEngine capabilities">{productTabs.map((tab, index) => { const Icon = tab.icon; return <button key={tab.label} ref={(element) => { tabRefs.current[index] = element; }} className={active === index ? 'product-tab active' : 'product-tab'} type="button" role="tab" id={`preview-tab-${index}`} aria-label={tab.label} aria-selected={active === index} aria-controls="product-preview-panel" tabIndex={active === index ? 0 : -1} onClick={() => setActive(index)} onKeyDown={(event) => handleTabKey(event, index)}><Icon size={19} stroke={1.65} /><span>{tab.label}</span></button>; })}</div><ProductPreview active={active} /></div>;
}

export function SiteFooter() {
  return <footer className="site-footer"><div className="footer-top"><div className="footer-brand"><Brand /><p>Open-source engagement infrastructure for products that care what happens next.</p></div><div className="footer-links"><div><span>Explore</span><a href="/#capabilities">Capabilities</a><a href="/#how-it-works">How it works</a><a href="/#use-cases">Use cases</a></div><div><span>Resources</span><Link to="/docs">Documentation</Link><a href={dashboardURL}>Local console</a><a href="/#faq">FAQs</a></div></div></div><div className="footer-bottom"><span>© 2026 MotifEngine</span><span>Open source · API first · Tenant aware</span></div></footer>;
}

export function MarketingApp() {
  const [openFeature, setOpenFeature] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  useMarketingMotion(rootRef);
  return <div ref={rootRef} className="site marketing-site"><SiteHeader /><main><div className="page-frame"><section className="hero" aria-labelledby="hero-title"><a className="announcement" href="#capabilities">Open-source engagement engine <span>Explore <IconArrowRight size={13} /></span></a><h1 id="hero-title">Make every moment <em>count.</em></h1><p className="hero-lede">MotifEngine is open source. Receive meaningful events, turn them into clear rules, and give every point, milestone, and reward a reliable path forward.</p><div className="hero-actions"><a className="button button-outline" href={dashboardURL}>Open console <IconArrowUpRight size={16} /></a><Link className="button button-primary" to="/docs">Read the docs <IconArrowRight size={16} /></Link></div></section><TabShowcase /><div className="system-counts" aria-label="What this site covers">{systemCounts.map(({ value, label }) => <div className="system-count" key={label}><strong><AnimatedCount value={value} /></strong><span>{label}</span></div>)}</div></div><section className="capabilities section" id="capabilities"><div className="section-intro" data-reveal><Eyebrow>One connected system</Eyebrow><h2>Everything needed to turn action into <em>lasting engagement.</em></h2><p>From the first event to the final reward, each step is visible and grounded in a versioned record.</p></div><div className="capability-cards">{capabilities.map((capability, index) => { const icons = [IconBraces, IconCode, IconCoin, IconGift]; const Icon = icons[index]; return <article className="capability-card" data-reveal key={capability.eyebrow}><span className="card-index">0{index + 1} / 04</span><div className="card-symbol"><Icon size={25} stroke={1.5} /></div><h3>{capability.title}</h3><p>{capability.body}</p><span className="card-detail">{capability.detail}</span></article>; })}</div></section><section className="manifesto"><div className="manifesto-inner" data-reveal><span className="manifesto-mark">✷</span><h2>One event can start a journey.<br /><em>Every outcome should have a reason.</em></h2><p>MotifEngine connects the decisions, records, and recovery paths behind engaging product experiences.</p><Link className="button button-primary" to="/docs">See how the engine works <IconArrowRight size={16} /></Link></div></section><section className="features section" id="features"><div className="section-intro" data-reveal><Eyebrow>Built for real products</Eyebrow><h2>Powerful where it matters.<br /><em>Clear where it counts.</em></h2><p>Configuration, execution, and operations belong in one understandable flow.</p></div><div className="feature-layout" data-reveal><div className="feature-art" aria-label="Illustrative event flow"><div className="feature-art-top"><span>EVENT TRACE</span><span>EXAMPLE</span></div><div className="trace-node"><span>01</span><IconBraces size={18} /><div><strong>order.completed</strong><small>Validated against schema v2</small></div><IconCircleCheck size={17} /></div><div className="trace-connector" /><div className="trace-node"><span>02</span><IconCode size={18} /><div><strong>second_purchase</strong><small>Published rule matched</small></div><IconCircleCheck size={17} /></div><div className="trace-connector" /><div className="trace-node"><span>03</span><IconCoin size={18} /><div><strong>loyalty_points</strong><small>Ledger entry recorded once</small></div><IconCircleCheck size={17} /></div><div className="feature-art-foot">An auditable effect from one trusted signal.</div></div><div className="feature-accordion">{[
  { title: 'Events you can trace', body: 'Validate bounded payloads against versioned schemas, preserve a receipt, and follow processing from acceptance to effect.' },
  { title: 'Rules you can explain', body: 'Compose readable conditions and typed actions, then publish an immutable version that operators can inspect.' },
  { title: 'Progression with purpose', body: 'Keep XP, missions, achievements and spendable point currencies distinct while connecting them to the same event flow.' },
  { title: 'Rewards you can recover', body: 'Track redemptions, delivery attempts and ambiguous provider outcomes without hiding them behind a success message.' },
].map((feature, index) => <div className={openFeature === index ? 'feature-item expanded' : 'feature-item'} key={feature.title}><button type="button" aria-expanded={openFeature === index} aria-controls={`feature-body-${index}`} onClick={() => setOpenFeature(index)}><span>0{index + 1}</span><strong>{feature.title}</strong><IconArrowUpRight size={18} /></button><div className="feature-body" id={`feature-body-${index}`} hidden={openFeature !== index}><p>{feature.body}</p></div></div>)}</div></div></section><section className="workflow section" id="how-it-works"><div className="section-intro" data-reveal><Eyebrow>How MotifEngine works</Eyebrow><h2>A simple flow from <em>signal to outcome.</em></h2><p>Your backend sends the signal. Your team defines the behavior. The engine keeps the result inspectable.</p></div><div className="workflow-grid"><article className="workflow-step" data-reveal><div className="workflow-graphic"><IconBraces size={42} stroke={1.2} /><span className="workflow-chip">order.completed</span></div><span className="workflow-index">01 / DEFINE</span><h3>Send a trusted event</h3><p>Register a schema, then send events from your backend with a tenant scoped credential.</p></article><article className="workflow-step" data-reveal><div className="workflow-graphic"><IconCode size={42} stroke={1.2} /><span className="workflow-chip">if purchase_count ≥ 2</span></div><span className="workflow-index">02 / DECIDE</span><h3>Publish the rule</h3><p>Choose conditions and actions, publish a version, and control when it is active.</p></article><article className="workflow-step" data-reveal><div className="workflow-graphic"><IconDatabase size={42} stroke={1.2} /><span className="workflow-chip">effect recorded</span></div><span className="workflow-index">03 / INSPECT</span><h3>Follow the effect</h3><p>Read the event, execution, ledger, progression, or redemption trail in the console.</p></article></div></section><section className="use-cases section" id="use-cases"><div className="section-intro" data-reveal><Eyebrow>Built for many journeys</Eyebrow><h2>One core loop.<br /><em>Different product stories.</em></h2><p>Start with behavior your product already knows and give it a meaningful next step.</p></div><div className="scenario-grid">{scenarios.map((scenario, index) => <article className="scenario" data-reveal key={scenario.label}><span className="scenario-number">0{index + 1}</span><span className="scenario-label">{scenario.label}</span><h3>{scenario.title}</h3><p>{scenario.body}</p><IconArrowUpRight size={20} /></article>)}</div></section><section className="security section"><div className="security-copy" data-reveal><Eyebrow>Clear operational boundaries</Eyebrow><h2>One platform.<br /><em>Distinct workspaces.</em></h2><p>Platform administrators create tenants. Tenant administrators configure only the tenants and environments they belong to. Integration credentials scope events to one environment.</p><Link className="text-link" to="/docs">Read about access and identity <IconArrowUpRight size={16} /></Link></div><div className="access-panel" data-reveal><div className="access-panel-head"><IconLock size={18} /><span>ACCESS MODEL</span></div><div className="access-row"><span className="access-avatar">SA</span><div><strong>Super Admin</strong><small>Platform operations and tenant creation</small></div><span>Platform</span></div><div className="access-row"><span className="access-avatar tenant">TA</span><div><strong>Tenant Admin</strong><small>Membership scoped configuration</small></div><span>Tenant</span></div><div className="access-row"><span className="access-avatar api">API</span><div><strong>Integration credential</strong><small>Events for one tenant environment</small></div><span>Environment</span></div><div className="access-note"><IconCircleCheck size={16} /> Scope is enforced by the server.</div></div></section><section className="faq section" id="faq"><div className="section-intro" data-reveal><Eyebrow>Frequently asked questions</Eyebrow><h2>Good questions deserve <em>clear answers.</em></h2></div><div className="faq-list" data-reveal>{questions.map((item, index) => <details key={item.question} open={index === 0}><summary><span>{item.question}</span><IconChevronDown size={19} /></summary><p>{item.answer}</p></details>)}</div></section><section className="closing"><div className="closing-inner" data-reveal><Eyebrow>Build the next meaningful moment</Eyebrow><h2>Make every action<br /><em>count for something.</em></h2><p>Explore the working console or start with the product guides.</p><div className="hero-actions"><a className="button button-outline-light" href={dashboardURL}>Open the console <IconArrowUpRight size={16} /></a><Link className="button button-light" to="/docs">Browse documentation <IconArrowRight size={16} /></Link></div></div></section></main><SiteFooter /></div>;
}
