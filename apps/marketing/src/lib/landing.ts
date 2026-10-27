// Landing copy. Every capability below exists in the API contract and the
// guides under docs/guides; nothing here describes customers, pricing,
// throughput or certifications.

export type GlyphName =
  | 'events'
  | 'rules'
  | 'levels'
  | 'missions'
  | 'achievements'
  | 'currencies'
  | 'ledger'
  | 'rewards'
  | 'leaderboards'
  | 'analytics';

export const LAUNCH = [
  ['XP and levels', 'Progress'],
  ['Daily streaks and missions', 'Habit'],
  ['Achievements and badges', 'Recognition'],
  ['Loyalty points and coins', 'Loyalty'],
  ['Coupons and perks', 'Rewards'],
  ['Weekly leaderboards', 'Competition'],
] as const;

export const CAPABILITIES: { glyph: GlyphName; title: string; line: string; href: string }[] = [
  { glyph: 'events', title: 'Events', line: 'Track any user action', href: '/docs#events' },
  { glyph: 'rules', title: 'Rules', line: 'Decide who earns what', href: '/docs#rules' },
  { glyph: 'levels', title: 'XP and levels', line: 'Progress that keeps building', href: '/docs#progression' },
  { glyph: 'missions', title: 'Missions', line: 'Goals users finish', href: '/docs#progression' },
  { glyph: 'achievements', title: 'Achievements', line: 'Badges that stay earned', href: '/docs#progression' },
  { glyph: 'currencies', title: 'Currencies', line: 'Points, coins and credits', href: '/docs#points' },
  { glyph: 'ledger', title: 'Ledger', line: 'Every balance accounted for', href: '/docs#points' },
  { glyph: 'rewards', title: 'Rewards', line: 'Coupons, perks and unlocks', href: '/docs#rewards' },
  { glyph: 'leaderboards', title: 'Leaderboards', line: 'Weekly rounds, live ranks', href: '/docs#leaderboards' },
  { glyph: 'analytics', title: 'Analytics', line: 'Funnels, retention, segments', href: '/docs#concepts' },
];

export const FLOW = [
  ['Your backend', 'POST /v1/events', 'trusted fact'],
  ['Contract', 'order.paid · v2', 'validated'],
  ['Rule', 'second-purchase-bonus', 'published'],
  ['Ledger', '+200 loyalty · user-42', 'append-only'],
  ['Your app', 'GET /v1/users/user-42/profile', 'XP, balances, badges'],
] as const;

export const USE_CASES = [
  { title: 'Second-purchase bonus', body: 'For shops: loyalty points on the second order, a coupon when they redeem.' },
  { title: 'XP for every lesson', body: 'For courses: levels, daily streaks and a badge after ten lessons.' },
  { title: 'Activation missions', body: 'For SaaS: count setup steps toward a finished onboarding.' },
  { title: 'Weekly leaderboards', body: 'For communities: rounds that reset on a schedule or when you say so.' },
  { title: 'Multi-currency programs', body: 'For loyalty clubs: earn, convert, expire and redeem, all on the ledger.' },
] as const;

export const FAQ = [
  {
    q: 'What is Motif?',
    a: 'Motif is a gamification and loyalty engine delivered as an API. Your backend sends events about what users do, published rules decide what they earn, and Motif keeps their XP, levels, missions, achievements, point balances, rewards and leaderboard ranks.',
  },
  {
    q: 'What is a gamification API?',
    a: 'A gamification API gives your product game mechanics such as XP, levels, streaks, badges, points and leaderboards as a service, so you do not have to build and operate the engine, the rules and the ledger yourself. Your app keeps its own interface and reads each user’s state from the API.',
  },
  {
    q: 'What can I build with Motif?',
    a: 'Loyalty programs with points and coupons, XP and level systems, daily streaks, missions and onboarding checklists, achievements and badges, multi-currency programs with conversion and expiry, and weekly or monthly leaderboards.',
  },
  {
    q: 'How does Motif connect to my product?',
    a: 'Your backend sends events to POST /v1/events with a server-side integration key. Motif validates each event against its contract, processes it in the background and updates the user. Your app then reads the profile, balances, missions, achievements and ranks over REST.',
  },
  {
    q: 'Can I change rewards without a release?',
    a: 'Yes. Rules are data: when an event arrives, if a condition holds, then award XP or points. Your team edits and publishes them from the console, and each published version is kept for audit.',
  },
  {
    q: 'How do you keep points accurate?',
    a: 'Every balance is backed by an append-only ledger. Events and point transactions carry idempotency keys, so a retried request never pays twice, and corrections are new entries rather than edits.',
  },
  {
    q: 'Can I test before going live?',
    a: 'Each workspace has separate environments such as sandbox and production, each with its own integration key, so test events never touch real balances.',
  },
  {
    q: 'Does Motif work with AI agents?',
    a: 'Yes. Motif runs an MCP server for your workspace. With a scoped access token, clients such as Claude Code or Cursor can check rules against sample payloads, send test events, and create and rotate leaderboards.',
  },
] as const;
