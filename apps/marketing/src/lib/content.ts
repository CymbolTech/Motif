// Product copy for the public site. Every capability, endpoint and recipe
// below exists in the repository's API contract and guides; nothing here
// describes customers, pricing, throughput or certifications.

export const DASHBOARD_URL = process.env.NEXT_PUBLIC_DASHBOARD_URL || 'http://127.0.0.1:4173';

export const LOOP_STEPS = [
  {
    id: 'event',
    word: 'Event',
    title: 'Your backend sends a fact.',
    body: 'A trusted event arrives with a stable event id. It is validated against an immutable contract version, deduplicated, written with an outbox row, and answered with a 202 receipt before any effect runs.',
    endpoint: 'POST /v1/events',
    specimen: `{
  "event_id": "order-1001-completed",
  "event_type": "order.completed",
  "user_id": "user-42",
  "payload": {
    "total": 129.90,
    "payment_status": "paid"
  }
}`,
  },
  {
    id: 'rule',
    word: 'Rule',
    title: 'A published rule decides.',
    body: 'Rules are data, not customer code: when an event type arrives, if a restricted condition holds, then typed actions run. Drafts never execute. Publishing freezes a version, and optional UTC windows bound a campaign.',
    endpoint: 'POST /v1/admin/rules',
    specimen: `{
  "event_type": "order.completed",
  "conditions": {"eq": ["$.payment_status", "paid"]},
  "actions": [
    {"type": "award_xp", "amount": 25},
    {"type": "award_points",
     "currency": "loyalty", "amount": 50}
  ]
}`,
  },
  {
    id: 'progression',
    word: 'Progress',
    title: 'Progress is not money.',
    body: 'XP accumulates per external user and resolves to the level whose threshold it has reached. Missions count matching events toward a target; achievements unlock after a required count. None of it is spendable.',
    endpoint: 'GET /v1/users/{id}/progression',
    specimen: `{
  "total_xp": 275,
  "level": 3,
  "level_name": "Regular",
  "next_level_xp": 500
}`,
  },
  {
    id: 'points',
    word: 'Points',
    title: 'Every balance is a ledger.',
    body: 'Each currency keeps its own append-only history. Awards, spends, expiring lots and conversions are entries with idempotency keys, so a retry can never pay twice and a correction never deletes the original row.',
    endpoint: 'GET /v1/users/{id}/points/transactions',
    specimen: `{
  "currency": "loyalty",
  "amount": 50,
  "type": "award",
  "idempotency_key":
    "order-1001-completed:completed_order_reward@v3:1"
}`,
  },
  {
    id: 'reward',
    word: 'Reward',
    title: 'Redemption you can recover.',
    body: 'A reward is free or costs one active currency. Each redemption reserves points, records every provider attempt, and stays in delivering when the outcome is unknown, until a status check or retry settles it.',
    endpoint: 'POST /v1/rewards/{id}/redemptions',
    specimen: `reserved → delivering
delivering → fulfilled

timeout → outcome unknown
  points stay reserved
status check → fulfilled`,
  },
] as const;

export const CAPABILITIES = [
  {
    name: 'Events and contracts',
    endpoint: 'POST /v1/events',
    body: 'Receive each fact once. Bounded payloads are validated against immutable JSON Schema versions, deduplicated by event id, and committed with an outbox row before the worker sees them.',
  },
  {
    name: 'Rules and actions',
    endpoint: 'POST /v1/admin/rules',
    body: 'When, if, then, stored as data. Conditions use all, any, exists, eq, neq, gt, gte, lt and lte; actions award XP, award points or log. Every published version stays readable for audit.',
  },
  {
    name: 'Campaigns and schedules',
    endpoint: 'starts_at · ends_at',
    body: 'Copy a UTC window into a rule version to run a weekend bonus without a deploy. Campaigns and goals measure a bounded activity by event and unique-user counts.',
  },
  {
    name: 'XP, levels, missions, achievements',
    endpoint: 'GET /v1/users/{id}/profile',
    body: 'Cumulative XP thresholds name levels. Missions track a target count, achievements unlock after a required count, and streaks read active UTC days for any event type.',
  },
  {
    name: 'Point currencies and ledger',
    endpoint: 'POST /v1/points/conversions',
    body: 'Run loyalty points, coins and credits side by side. Each has an append-only ledger with expiring lots, atomic spending and conversion at a versioned rate.',
  },
  {
    name: 'Rewards and providers',
    endpoint: 'POST /v1/rewards/{id}/redemptions',
    body: 'Free or paid rewards, reservations, idempotency headers sent to HTTP providers, attempt history, and reconciliation for outcomes that are still unknown.',
  },
  {
    name: 'Environments and credentials',
    endpoint: 'POST /v1/admin/credentials',
    body: 'The server scopes every request to one environment, such as sandbox or production, so tests never touch live balances. Integration keys are hashed, shown once, and can be rotated or revoked.',
  },
  {
    name: 'REST API and OpenAPI',
    endpoint: 'openapi 3.0.3',
    body: 'A versioned REST contract with an OpenAPI description. Any HTTP client works from your backend; there is no SDK to adopt and no widget you must ship.',
  },
  {
    name: 'Analytics and measurement',
    endpoint: 'GET /v1/admin/analytics',
    body: 'Goals, campaigns, metrics, funnels, retention cohorts, behavioural segments and experiment reports measure outcomes. They describe what happened; rules decide what happens.',
  },
  {
    name: 'Security and operations',
    endpoint: 'GET /v1/admin/diagnostics',
    body: 'Expiring hashed sessions, role-based console access, an append-only audit stream, rate limits, request ids, Prometheus metrics and a diagnostics snapshot of the pipeline.',
  },
] as const;

export const RECIPES = [
  {
    id: 'commerce',
    genre: 'Commerce',
    title: 'Reward the second purchase.',
    event: 'order.completed',
    rule: `{"eq": ["$.customer_order_number", 2]}
→ award_points loyalty 100`,
    effect: '+100 loyalty, once per order',
    note: 'The rule reads the payload. Your order service counts purchases and sends the number as a trusted field.',
  },
  {
    id: 'saas',
    genre: 'SaaS',
    title: 'Level up from the first login.',
    event: 'account.created',
    rule: `{}
→ award_xp 50`,
    effect: 'Level 1 “Getting started” at 0 XP',
    note: 'Levels are cumulative thresholds. GET /progression returns the current level and the next threshold.',
  },
  {
    id: 'learning',
    genre: 'Learning',
    title: 'Five lessons, one badge.',
    event: 'lesson.completed',
    rule: `mission   target_count 5
achievement required_count 5`,
    effect: 'A live checklist and a durable achievement',
    note: 'Missions and achievements may listen to the same event and still stay separate concepts. Streaks read active UTC days.',
  },
  {
    id: 'content',
    genre: 'Content',
    title: 'Pay attention to finished videos.',
    event: 'content.completed',
    rule: `{"eq": ["$.content_type", "video"]}
→ award_xp 15`,
    effect: '+15 XP per finished video',
    note: 'Add a second rule for a genre-specific currency bonus; each rule keeps its own version.',
  },
  {
    id: 'community',
    genre: 'Community',
    title: 'Count approved contributions.',
    event: 'comment.created',
    rule: `{"eq": ["$.moderation_status", "approved"]}
→ award_xp 5`,
    effect: '+5 XP per approved comment',
    note: 'MotifEngine does not moderate content. Send the event after your moderation system accepts it.',
  },
  {
    id: 'games',
    genre: 'Games',
    title: 'Double up on weekends.',
    event: 'quest.completed',
    rule: `{"eq": ["$.eligible", true]}
→ award_xp 50, award_points coins 20
starts_at 2026-10-17  ends_at 2026-10-20`,
    effect: 'Two outcomes inside a UTC window',
    note: 'Each outcome has its own ordinal in the idempotency key, so a retry resumes without paying twice.',
  },
] as const;

export const FAQ = [
  {
    id: 'simulation',
    question: 'What is this page actually running?',
    answer: 'A simulation of MotifEngine’s event, rule, ledger and redemption semantics, running in your browser. Nothing is sent to a MotifEngine API, and your run is stored only in this browser.',
  },
  {
    id: 'code',
    question: 'Does my team need engineers to use it?',
    answer: 'Once. Your backend sends events over REST with an environment-scoped key. After that, currencies, levels, missions, achievements, rewards and automations are configured in the console.',
  },
  {
    id: 'ui',
    question: 'Does it draw badges or leaderboards in my app?',
    answer: 'No. MotifEngine is API-first: you design the player-facing interface and read profiles, balances, missions, achievements and progression from the API.',
  },
  {
    id: 'xp',
    question: 'Is XP the same as points?',
    answer: 'No. XP is progression and is never spent. Point currencies are spendable balances with their own ledgers, expiration and conversion rates.',
  },
  {
    id: 'provider',
    question: 'What happens when a reward provider times out?',
    answer: 'The redemption stays in delivering and its points stay reserved. A provider status check or a retry with the same redemption id settles it; only a confirmed failure releases the points.',
  },
  {
    id: 'license',
    question: 'Is it open source, and what does it cost?',
    answer: 'MotifEngine is open source, and you can run the full stack locally with Docker Compose. Public pricing has not been set.',
  },
] as const;
