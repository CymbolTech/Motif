// The simulated workspace behind the landing page. Every definition mirrors a
// MotifEngine primitive: rules carry immutable versions and typed actions,
// achievements count matching events, levels are cumulative XP thresholds.

export type Currency = 'coins' | 'loyalty';

export type Payload = Record<string, string | number | boolean>;

export type Action =
  | { type: 'award_xp'; amount: number }
  | { type: 'award_points'; currency: Currency; amount: number };

export type Rule = {
  id: string;
  version: number;
  eventType: string;
  /** Display form of the restricted condition tree, as the API stores it. */
  condition: string;
  matches: (payload: Payload) => boolean;
  actions: Action[];
};

export type Achievement = {
  id: string;
  title: string;
  description: string;
  /** Shown instead of the description until a secret achievement unlocks. */
  hint?: string;
  secret?: boolean;
  on: string[];
  where?: (payload: Payload) => boolean;
  required: number;
  bonus: { xp: number; coins?: number };
};

export type Level = { level: number; name: string; threshold: number };

export const LEVELS: Level[] = [
  { level: 1, name: 'Starter', threshold: 0 },
  { level: 2, name: 'Player', threshold: 100 },
  { level: 3, name: 'Regular', threshold: 250 },
  { level: 4, name: 'Strategist', threshold: 500 },
  { level: 5, name: 'Operator', threshold: 850 },
  { level: 6, name: 'Architect', threshold: 1250 },
  { level: 7, name: 'Legend', threshold: 1700 },
];

export const RULES: Rule[] = [
  {
    id: 'completed_order_reward',
    version: 3,
    eventType: 'order.completed',
    condition: '{"eq":["$.payment_status","paid"]}',
    matches: (p) => p.payment_status === 'paid',
    actions: [
      { type: 'award_xp', amount: 25 },
      { type: 'award_points', currency: 'loyalty', amount: 50 },
    ],
  },
  {
    id: 'lesson_xp',
    version: 1,
    eventType: 'lesson.completed',
    condition: '{}',
    matches: () => true,
    actions: [
      { type: 'award_xp', amount: 25 },
      { type: 'award_points', currency: 'coins', amount: 5 },
    ],
  },
  {
    id: 'weekend_quest_bonus',
    version: 2,
    eventType: 'quest.completed',
    condition: '{"eq":["$.eligible",true]}',
    matches: (p) => p.eligible === true,
    actions: [
      { type: 'award_xp', amount: 50 },
      { type: 'award_points', currency: 'coins', amount: 10 },
    ],
  },
  {
    id: 'scroll_milestone',
    version: 2,
    eventType: 'page.scrolled',
    condition: '{"any":[{"eq":["$.depth",0.25]},{"eq":["$.depth",0.75]}]}',
    matches: (p) => p.depth === 0.25 || p.depth === 0.75,
    actions: [
      { type: 'award_xp', amount: 10 },
      { type: 'award_points', currency: 'coins', amount: 5 },
    ],
  },
];

const ALL_DEMO_EVENTS = ['order.completed', 'lesson.completed', 'quest.completed'];

export const ACHIEVEMENTS: Achievement[] = [
  { id: 'cold-boot', title: 'Cold boot', description: 'Watch the engine start.', on: ['session.started'], required: 1, bonus: { xp: 10 } },
  { id: 'first-signal', title: 'First signal', description: 'Fire an event from the trigger.', on: ALL_DEMO_EVENTS, required: 1, bonus: { xp: 50, coins: 10 } },
  { id: 'halfway', title: 'Halfway there', description: 'Scroll through half of the page.', on: ['page.scrolled'], where: (p) => p.depth === 0.5, required: 1, bonus: { xp: 50, coins: 5 } },
  { id: 'loop-walker', title: 'Loop walker', description: 'See all five moves of the loop.', on: ['loop.step_viewed'], required: 5, bonus: { xp: 100 } },
  { id: 'rule-smith', title: 'Rule smith', description: 'Publish a rule that matches the brief exactly.', on: ['arcade.rule_tested'], where: (p) => p.accuracy === 1, required: 1, bonus: { xp: 150, coins: 25 } },
  { id: 'once-means-once', title: 'Once means once', description: 'Retry the same event ten times.', on: ['arcade.storm_survived'], required: 1, bonus: { xp: 100 } },
  { id: 'money-changer', title: 'Money changer', description: 'Convert loyalty points into coins.', on: ['points.converted'], required: 1, bonus: { xp: 75 } },
  { id: 'cashed-in', title: 'Cashed in', description: 'Redeem a reward with coins.', on: ['reward.redeemed'], required: 1, bonus: { xp: 150 } },
  { id: 'world-tour', title: 'World tour', description: 'Open every industry recipe.', on: ['recipe.opened'], required: 6, bonus: { xp: 100 } },
  { id: 'curious', title: 'Curious', description: 'Open three answers.', on: ['faq.opened'], required: 3, bonus: { xp: 50 } },
  { id: 'all-the-way', title: 'All the way down', description: 'Reach the end of the page.', on: ['page.scrolled'], where: (p) => p.depth === 1, required: 1, bonus: { xp: 75, coins: 5 } },
  { id: 'manual', title: 'Read the manual', description: 'Open the docs.', on: ['docs.opened'], required: 1, bonus: { xp: 50 } },
  { id: 'sharpshooter', title: 'Sharpshooter', description: 'Hit the three hidden targets.', hint: 'Three things on this page are worth aiming at.', secret: true, on: ['target.hit'], required: 3, bonus: { xp: 120, coins: 30 } },
  { id: 'cheat-code', title: 'Cheat code', description: 'Enter the code every console remembers.', hint: 'Old consoles remember a code. Up comes first.', secret: true, on: ['konami.entered'], required: 1, bonus: { xp: 99 } },
  { id: 'press-start', title: 'Press start', description: 'Open the console.', on: ['console.opened'], required: 1, bonus: { xp: 200 } },
  { id: 'completionist', title: 'Completionist', description: 'Unlock every other achievement.', on: ['run.completed'], required: 1, bonus: { xp: 300 } },
];

export const DEMO_EVENTS = [
  {
    type: 'order.completed',
    label: 'Order',
    payload: (n: number): Payload => ({ order_id: `order-${1000 + n}`, total: 129.9, currency: 'USD', payment_status: 'paid' }),
  },
  {
    type: 'lesson.completed',
    label: 'Lesson',
    payload: (n: number): Payload => ({ lesson_id: `lesson-${n}`, course_id: 'intro', score: 92 }),
  },
  {
    type: 'quest.completed',
    label: 'Quest',
    payload: (n: number): Payload => ({ quest_id: `quest-${n}`, eligible: true }),
  },
] as const;

export type Reward = {
  id: 'night-shift' | 'chrome-core';
  name: string;
  description: string;
  cost: number;
};

export const REWARDS: Reward[] = [
  { id: 'night-shift', name: 'Night shift', description: 'The whole page switches to its charcoal theme.', cost: 40 },
  { id: 'chrome-core', name: 'Chrome core', description: 'The core’s lattice trades clay for iridescent chrome.', cost: 60 },
];

/** Versioned conversion rate, as POST /v1/points/conversions records it. */
export const CONVERSION = { id: 'loyalty_to_coins', version: 2, from: 'loyalty' as const, to: 'coins' as const, fromAmount: 10, toAmount: 1 };

/** Requests per sliding window before the simulated API answers 429. */
export const RATE_LIMIT = { requests: 8, windowMs: 10_000 };

export function levelFor(xp: number): Level {
  let current = LEVELS[0];
  for (const level of LEVELS) if (xp >= level.threshold) current = level;
  return current;
}

export function nextLevel(level: Level): Level | undefined {
  return LEVELS.find((candidate) => candidate.level === level.level + 1);
}
