// The public manual. Every example is taken from docs/guides and the OpenAPI
// contract; keep them in step when the API changes.

export const QUICKSTART = [
  {
    title: 'Create a workspace',
    body: 'Sign up from the console. Signup creates your workspace, its first environment, a starter plan and an integration credential in one flow.',
    code: `Console → Create account
  workspace     Acme Rewards
  environment   Production
  → integration credential, shown once`,
  },
  {
    title: 'Keep the key server-side',
    body: 'The raw integration key is returned only when it is created or rotated. Store it in your backend’s secret manager and never ship it in a browser bundle.',
    code: `X-Motif-API-Key: <integration_key>`,
  },
  {
    title: 'Register event contracts',
    body: 'Open Build → Events, or call the API. Contracts use a bounded JSON Schema subset and every registration creates a new immutable version.',
    code: `POST /v1/admin/event-types
{
  "name": "order.completed",
  "schema": {
    "type": "object",
    "required": ["order_id", "total", "currency"],
    "properties": {
      "order_id": {"type": "string"},
      "total": {"type": "number", "minimum": 0},
      "currency": {"type": "string"}
    }
  }
}`,
  },
  {
    title: 'Define the catalog',
    body: 'From the Loyalty group, create only what the scenario needs: currencies, levels and XP thresholds, missions and achievements, free or paid rewards.',
    code: `currencies   loyalty, coins
levels       1 Starter at 0 XP · 2 Regular at 250 XP
missions     lesson.completed × 5
rewards      welcome_coupon · 100 loyalty`,
  },
  {
    title: 'Publish an automation',
    body: 'Choose an event, add conditions, add one or more typed outcomes. Saving a draft and publishing are separate actions, so a draft never executes.',
    code: `{
  "event_type": "order.completed",
  "conditions": {"eq": ["$.payment_status", "paid"]},
  "actions": [
    {"type": "award_xp", "amount": 25},
    {"type": "award_points", "currency": "loyalty", "amount": 10}
  ]
}`,
  },
  {
    title: 'Send the fact',
    body: 'Your backend sends the event with a stable event id. 202 Accepted means the receipt and outbox row are committed, not that every effect has run.',
    code: `curl -X POST $MOTIF/v1/events \\
  -H 'X-Motif-API-Key: <integration_key>' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "event_id": "order-1001-completed",
    "event_type": "order.completed",
    "user_id": "user-42",
    "payload": {"order_id": "order-1001", "total": 129.90,
                "currency": "USD", "payment_status": "paid"}
  }'`,
  },
  {
    title: 'Read the result',
    body: 'Check the receipt, then read the profile: balances, event count, mission progress, achievement progress and progression in one response.',
    code: `GET /v1/events/order-1001-completed
GET /v1/users/user-42/profile
GET /v1/users/user-42/points/transactions`,
  },
  {
    title: 'Inspect operations',
    body: 'The Overview workspace and the diagnostics endpoint show event counts, pending outbox rows, active rules, failed executions and failed redemptions.',
    code: `GET /v1/admin/diagnostics
GET /v1/admin/rule-executions
GET /v1/admin/redemptions/{id}/attempts`,
  },
] as const;

export const CONCEPTS = [
  ['Environment', 'A boundary inside your workspace, such as sandbox or production.'],
  ['Integration credential', 'A server-side key scoped to one environment.'],
  ['External user ID', 'Your own stable customer id, mapped to MotifEngine state.'],
  ['Event', 'A fact that happened in your product.'],
  ['Event contract', 'An immutable, versioned payload schema.'],
  ['Rule', 'A published when / if / then automation.'],
  ['Action', 'A typed outcome: award_xp, award_points or log.'],
  ['Currency', 'A spendable balance with its own policies.'],
  ['Ledger', 'The append-only history behind every balance change.'],
  ['XP', 'Non-spendable progression experience.'],
  ['Level', 'A cumulative XP threshold and display name.'],
  ['Mission', 'A count of matching events toward a target.'],
  ['Achievement', 'A named milestone unlocked after a count of matching events.'],
  ['Reward', 'A catalog item that is free or costs one active currency.'],
  ['Redemption', 'One durable attempt to deliver a reward.'],
  ['Goal · Campaign', 'Measurable targets and bounded event windows for reporting.'],
] as const;

export const OPERATORS = [
  ['{}', 'Always matches'],
  ['all', 'Every child must match'],
  ['any', 'At least one child must match'],
  ['exists', 'The payload path exists'],
  ['eq · neq', 'Equal · not equal'],
  ['gt · gte · lt · lte', 'Numeric or string comparison'],
] as const;

export const GUIDES = [
  {
    id: 'events',
    title: 'Events and contracts',
    body: [
      'Every event carries an event id. A repeated id returns the original receipt instead of creating a second event, so producers can retry safely.',
      'Payloads are bounded and validated against the current contract version. A malformed payload returns 422 event_schema_mismatch instead of reaching the worker; accepted events keep the version they were received under.',
    ],
    code: `202 Accepted   receipt committed with an outbox row
422            event_schema_mismatch
GET /v1/events/{externalEventId}   status of one receipt`,
  },
  {
    id: 'rules',
    title: 'Rules and schedule windows',
    body: [
      'Rules begin as drafts and only execute after a version is published. Editing a published rule creates a new immutable version; the old one stays readable for audit.',
      'starts_at is inclusive, ends_at is exclusive, both are optional RFC 3339 timestamps. The worker checks the window at processing time, and every outcome uses its own ordinal in the idempotency key.',
    ],
    code: `{
  "name": "Weekend lesson bonus",
  "event_type": "lesson.completed",
  "conditions": {},
  "actions": [
    {"type": "award_xp", "amount": 25},
    {"type": "award_points", "currency": "coins", "amount": 5}
  ],
  "starts_at": "2026-10-17T00:00:00Z",
  "ends_at": "2026-10-20T00:00:00Z"
}`,
  },
  {
    id: 'progression',
    title: 'XP, levels, missions, achievements',
    body: [
      'XP is awarded by the award_xp action and accumulated per external user inside an environment. Level thresholds are cumulative and unique per environment.',
      'Missions show a live count toward a target; achievements are durable recognition after a required count. They may listen to the same event and still remain separate concepts. Streaks read active UTC days.',
    ],
    code: `GET /v1/users/user-42/progression
{
  "total_xp": 275,
  "level": 3,
  "level_name": "Regular",
  "next_level_xp": 500
}`,
  },
  {
    id: 'points',
    title: 'Points and the ledger',
    body: [
      'Each currency keeps an append-only ledger with expiring point lots and atomic spending. A correction is a new entry with its own idempotency key; the original row is never deleted.',
      'Conversions move value between two currencies at a versioned rate in one operation. Transaction history is paginated with an opaque cursor.',
    ],
    code: `POST /v1/users/{externalUserId}/points/transactions
{
  "currency": "loyalty",
  "amount": 100,
  "type": "spend",
  "idempotency_key": "refund-order-1001-loyalty-reversal"
}`,
  },
  {
    id: 'rewards',
    title: 'Rewards and recovery',
    body: [
      'A redemption reserves points, records every provider attempt and moves through explicit states. HTTP providers receive idempotency headers and may expose a status URL with a {redemption_id} placeholder.',
      'Pending outcomes stay in delivering. Only a provider-confirmed failure releases reserved points; a retry reuses the durable redemption identity.',
    ],
    code: `POST /v1/redemptions/{redemption_id}/retry
reserved → delivering → fulfilled
                      → failed (points released)`,
  },
  {
    id: 'access',
    title: 'Access and identity',
    body: [
      'Console users work inside their own workspace; the console loads the workspace and its environment from the signed-in account.',
      'Admin sessions are hashed and expire. Integration credentials are hashed, shown once, and can be rotated or revoked. Administrative actions are recorded in an append-only audit stream.',
    ],
    code: `console user     your workspace, signed in
integration key  one environment, server-side
access token     one workspace, scoped, revocable
admin session    hashed, expiring, revocable`,
  },
  {
    id: 'mcp',
    title: 'Connect an MCP client',
    body: [
      'Coding tools that speak the Model Context Protocol, such as Claude Code, Cursor and VS Code, can configure and inspect your workspace through the MCP endpoint on the console domain. Create an access token under Access tokens in the console and choose its permissions: read, write to register contracts, catalog items, rule drafts and test events, and publish to make rule versions live.',
      'The client can check a rule against a sample payload and name the exact condition that decided it, send a test event and trace what it awarded, and generate the backend code that sends each event. It never sees integration keys, moves points or redeems rewards; your backend keeps sending events over REST.',
    ],
    code: `claude mcp add --transport http motif \\
  https://<console-domain>/mcp \\
  --header "Authorization: Bearer motif_at_…"

check_rule       explain a match before saving
send_test_event  wait for processing, return the trace
publish_rule     only with the publish permission`,
  },
] as const;
