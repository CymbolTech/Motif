# MotifEngine scenario cookbook

This guide explains what a product team can build with the MotifEngine runtime that is in this repository today. It is written as a set of business scenarios rather than as a list of isolated endpoints.

The examples use `acme` and `sandbox` as the tenant and environment, `user-42` as a customer identifier, and `loyalty` and `coins` as point currencies. Replace those values with the identifiers in your own scope.

## 1. The one sentence mental model

Your product sends a trusted **event**. A published **rule** can inspect that event and produce one or more effects. Event processing can also advance a matching **mission** or **achievement**. Those effects update an XP total, a spendable point **currency**, or an execution log. A separate **reward** catalog lets a user redeem an in-app or externally delivered benefit. **Goals**, **campaigns**, and **analytics** measure what happened; they do not replace rules.

```text
customer backend
      │  POST /v1/events
      ▼
tenant + environment + event contract
      ▼
durable receipt → outbox → worker
      ▼
published rule + conditions
      ├── award_xp       → progression / level
      ├── award_points   → append-only currency ledger
      ├── mission        → user mission progress
      ├── achievement    → user achievement progress
      └── log            → execution audit
      │
      ├── profile, balances, missions, achievements, streaks
      └── reward redemption → provider → fulfilled / failed / retryable
```

## 2. Concepts and when to use them

| Concept | What it represents | Use it when |
| --- | --- | --- |
| Tenant | A customer organization | You operate more than one customer or product |
| Environment | A tenant-owned boundary such as `sandbox` or `production` | You need safe testing without mixing production state |
| Integration credential | A server-side key scoped to one tenant/environment | Your backend sends events or reads a user state |
| External user ID | Your own stable customer ID | You want MotifEngine state to map back to your user |
| Event | A fact that happened in your product | A customer action should be measured or cause an effect |
| Event contract | An immutable versioned payload schema | You want malformed producer data rejected early |
| Rule | A published `when / if / then` automation | A matching event should cause an effect |
| Condition | A restricted expression evaluated against the event payload | The same event has different outcomes by amount, type, plan, or flag |
| Action | A typed outcome of a rule | You need XP, points, logging, or multiple outcomes |
| Currency | A tenant-owned spendable point balance | You need coins, loyalty points, campaign tokens, or credits |
| Ledger | The append-only history behind every balance change | You need auditability, idempotency, spending safety, or reconciliation |
| XP | Non-spendable progression experience | Users should level up without consuming the value |
| Mission | A count of matching events toward a target | Users should complete a repeatable activity |
| Achievement | A named milestone unlocked after a count of matching events | Users should earn a durable recognition state |
| Level | A cumulative XP threshold and display name | You want `Starter`, `Pro`, `Expert`, or similar progression |
| Reward | A catalog item that can be free or cost one active currency | A user should receive an unlock or external benefit |
| Redemption | One durable attempt to deliver a reward | You need reservation, provider status, retry, and recovery |
| Goal | A measurable target tied to one event type | You want to state an outcome such as “100 completed orders” |
| Campaign | A named event window, optionally linked to a goal | You want event and unique-user counts for a bounded activity |
| Metric | A reusable count or unique-user definition | You want a named business measurement |
| Funnel | Ordered event steps | You want conversion through a journey |
| Retention | A cohort event plus a return event at day intervals | You want return behavior after signup or first purchase |
| Segment | Users with at least `N` occurrences of one event | You want a behavioral audience definition |
| Experiment report | Deterministic weighted variants plus a conversion event | You want a repeatable control-group comparison |

## 3. The universal implementation recipe

Every scenario in this guide is a variation of the same eight steps.

### Step 1 — Create a tenant and environment

Tenant users create their own workspace from the dashboard signup screen. Signup provisions the tenant, its first environment, a starter plan, an integration credential, and the tenant-admin membership in one flow. The equivalent REST call is public and returns a tenant session:

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "owner@acme.example",
    "password": "a-long-password-of-at-least-12-chars",
    "tenant_id": "acme",
    "name": "Acme Rewards",
    "environment_id": "production",
    "environment_name": "Production"
  }'
```

Super Admin tenant provisioning remains an internal Ops concern. The platform-only endpoint below is retained for that service and is not linked from the tenant dashboard.

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/tenants \
  -H 'Authorization: Bearer <super_admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "acme",
    "name": "Acme Corp",
    "environment_id": "sandbox",
    "environment_name": "Sandbox"
  }'
```

The dashboard loads the tenant and environment from the signed-in user's workspace membership. There is no workspace selector in the tenant console; catalog, rule, event, analytics, and reward operations stay inside that membership scope automatically.

### Step 2 — Create a credential for the customer backend

The raw integration key is returned only from create or rotate. Store it in the customer's server-side secret manager. Never put it in a browser bundle.

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/credentials \
  -H 'Authorization: Bearer <super_admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "acme",
    "environment_id": "sandbox",
    "label": "acme-sandbox-backend"
  }'
```

### Step 3 — Register event contracts

Open **Build → Events** in the console, or use the API. When an automation form needs an event that is not in the registry, use **Create event** inside the selector; the inline drawer creates it and returns you to the same draft without a page change. The schema is a bounded JSON Schema subset: `type`, `required`, nested `properties`, `enum`, `minimum`, `maximum`, and `additionalProperties`.

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/event-types \
  -H 'Authorization: Bearer <tenant_or_super_admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "acme",
    "environment_id": "sandbox",
    "name": "order.completed",
    "schema": {
      "type": "object",
      "required": ["order_id", "total", "currency"],
      "properties": {
        "order_id": {"type": "string"},
        "total": {"type": "number", "minimum": 0},
        "currency": {"type": "string"},
        "category": {"type": "string"}
      },
      "additionalProperties": true
    }
  }'
```

Event contract versions are immutable. Registering the same name again creates the next version. New events use the current version; previously accepted events retain the version used at receipt. Event names themselves are stable identifiers; renaming a signal means registering a new event type and migrating the producer.

During this bootstrap phase, an event type with no registered contract is still accepted. Register contracts for production integrations so a malformed payload returns `422 event_schema_mismatch` instead of reaching the worker.

### Step 4 — Define the catalog

From the **Loyalty** group in the sidebar, open **Currencies**, **Levels**, **Missions**, **Achievements**, or **Rewards**. Each workspace starts with a list and an **Add** action; dependencies can also be created inline from the current form. Through the admin API, create only the resources needed by the scenario:

1. currencies such as `loyalty` or `coins` (the automation and reward forms also support creating a missing currency inline and keeping the current draft);
2. levels and XP thresholds;
3. missions and achievements;
4. digital rewards, free or paid;
5. active/inactive status for catalog resources.

The API examples later in this guide show the exact fields.

### Step 5 — Build and publish the automation

Open **Build → Automations**. Choose a registered event, add conditions if needed, and add one or more typed outcomes. The builder supports the same request shape as the API and keeps saving a draft separate from publishing.

```json
{
  "tenant_id": "acme",
  "environment_id": "sandbox",
  "name": "Completed order reward",
  "description": "Give progression and spendable value after a paid order.",
  "event_type": "order.completed",
  "conditions": {
    "eq": ["$.payment_status", "paid"]
  },
  "actions": [
    {"type": "award_xp", "amount": 25},
    {"type": "award_points", "currency": "loyalty", "amount": 10}
  ]
}
```

Create the rule with `POST /v1/admin/rules`, then publish it with `POST /v1/admin/rules/{rule_id}/publish`. A draft never executes. Editing a published rule means creating a new immutable version and publishing that version; the old version remains readable for audit.

### Step 6 — Send the fact from the customer backend

```bash
curl -X POST http://127.0.0.1:18080/v1/events \
  -H 'X-Motif-API-Key: <integration_key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "event_id": "order-1001-completed",
    "event_type": "order.completed",
    "user_id": "user-42",
    "occurred_at": "2026-10-14T10:00:00Z",
    "payload": {
      "order_id": "order-1001",
      "total": 129.90,
      "currency": "USD",
      "payment_status": "paid",
      "category": "books"
    }
  }'
```

The API returns `202 Accepted` after the receipt and outbox row are committed. It does not mean all effects are complete yet.

### Step 7 — Read the result

Check the event receipt:

```bash
curl http://127.0.0.1:18080/v1/events/order-1001-completed \
  -H 'X-Motif-API-Key: <integration_key>'
```

Then read the customer state:

```bash
curl http://127.0.0.1:18080/v1/users/user-42/profile \
  -H 'X-Motif-API-Key: <integration_key>'
```

The profile includes balances, event count, mission progress, achievement progress, and progression. Use `/points/transactions` for the ledger and `/progression` for a focused XP response.

### Step 8 — Inspect operations

The Overview workspace and `GET /v1/admin/diagnostics` show event counts, pending outbox rows, active rules, failed executions, retryable executions, and failed redemptions. Use `GET /v1/admin/events`, `GET /v1/admin/rule-executions`, `GET /v1/admin/redemptions`, the audit log, and redemption attempt history to explain a specific outcome.

## 4. Rule language: the common building block

### Supported condition operators

Conditions are data, not executable customer code. A condition object contains exactly one operator.

| Operator | Example | Meaning |
| --- | --- | --- |
| `{}` | `{}` | Always matches |
| `all` | `{"all":[{"exists":"$.user_id"},{"gte":["$.total",100]}]}` | Every child must match |
| `any` | `{"any":[{"eq":["$.kind","paid"]},{"eq":["$.kind","gift"]}]}` | At least one child must match |
| `exists` | `{"exists":"$.order.id"}` | The payload path exists |
| `eq` | `{"eq":["$.status","completed"]}` | Equal |
| `neq` | `{"neq":["$.plan","free"]}` | Not equal; a missing operand also evaluates as not equal |
| `gt` / `gte` | `{"gte":["$.score",80]}` | Numeric or string comparison |
| `lt` / `lte` | `{"lt":["$.age",18]}` | Numeric or string comparison |

Paths use dot notation such as `$.order.total`. Arrays and nested objects are not valid comparison operands. Conditions are bounded to eight nesting levels, 64 expression nodes, and 32 children per `all` or `any` array.

### Supported actions

| Action | Required fields | Effect |
| --- | --- | --- |
| `log` | optional `message` | Records a rule action without changing user state |
| `award_xp` | positive integer `amount` | Adds non-spendable XP to the external user |
| `award_points` | valid `currency`, positive integer `amount` | Appends an award entry to that currency's ledger |

Use `actions` for multiple effects. The older single `action` object remains accepted for compatibility and is normalized to a one-item action list.

Every action is idempotent using the event, immutable rule version, and action position. A broker redelivery or worker retry cannot award the same effect twice.

### Schedule windows

`starts_at` is inclusive. `ends_at` is exclusive. A missing start means immediately eligible; a missing end means open-ended. The worker checks the window at processing time, so a delayed event is evaluated against the clock when it is processed. A window controls rule execution; campaign analytics use the event's `occurred_at` when present, otherwise its receipt time.

## 5. Commerce scenarios

### 5.1 Give 10 XP and 5 coins after a completed purchase

**Need:** A paid order should advance the customer and add spendable loyalty value.

**Configure:** Register `order.completed`, create `coins`, and publish:

```json
{
  "name": "Paid order reward",
  "event_type": "order.completed",
  "conditions": {"eq":["$.payment_status","paid"]},
  "actions": [
    {"type":"award_xp","amount":10},
    {"type":"award_points","currency":"coins","amount":5}
  ]
}
```

**Send:** Include `user_id`, a unique `event_id`, and `payment_status: paid`.

**Result:** The worker creates one XP effect and one `award` ledger entry. The profile shows the new total XP and coins balance.

### 5.2 Reward the second purchase

**Need:** Give a larger bonus when a customer places their second order.

**Important design choice:** The current rule evaluator reads the event payload; it does not query event history to calculate a purchase count. Calculate the count in the customer's order service and include it as a trusted field.

```json
{
  "event_id": "order-1002-completed",
  "event_type": "order.completed",
  "user_id": "user-42",
  "payload": {
    "order_id": "order-1002",
    "payment_status": "paid",
    "customer_order_number": 2
  }
}
```

Rule:

```json
{
  "conditions": {"eq":["$.customer_order_number",2]},
  "actions": [{"type":"award_points","currency":"loyalty","amount":100}]
}
```

If you do not want to trust a derived field, keep the count in the customer backend and only emit the event after that service has made the decision.

### 5.3 Give a threshold bonus

**Need:** Orders of at least 100 USD receive an extra 25 points.

```json
{
  "event_type": "order.completed",
  "conditions": {
    "all": [
      {"eq":["$.payment_status","paid"]},
      {"gte":["$.total",100]}
    ]
  },
  "actions": [{"type":"award_points","currency":"loyalty","amount":25}]
}
```

Create a separate rule if small and large orders should receive different outcomes. Multiple published rules may match one event, so name them so the combined effect is obvious.

### 5.4 Reward a product category

**Need:** Purchases in `books` receive XP and purchases in `home` receive coins.

Use one rule per category:

```json
{
  "event_type": "order.completed",
  "conditions": {"eq":["$.category","books"]},
  "actions": [{"type":"award_xp","amount":30}]
}
```

```json
{
  "event_type": "order.completed",
  "conditions": {"eq":["$.category","home"]},
  "actions": [{"type":"award_points","currency":"coins","amount":8}]
}
```

### 5.5 Issue a coupon or free unlock

**Need:** A user exchanges points for a coupon or an in-app entitlement.

Create a `digital_unlock` reward. The payload is passed to the provider, so the customer backend or provider can interpret it as a coupon code, SKU, entitlement key, or benefit descriptor.

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/rewards \
  -H 'Authorization: Bearer <admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "acme",
    "environment_id": "sandbox",
    "name": "10 USD book coupon",
    "reward_type": "digital_unlock",
    "description": "Coupon delivered by the commerce provider",
    "cost_currency": "loyalty",
    "cost_amount": 250,
    "payload": {
      "benefit": "coupon",
      "coupon_value": 10,
      "currency": "USD",
      "campaign": "books-october"
    }
  }'
```

The customer backend redeems it:

```bash
curl -X POST http://127.0.0.1:18080/v1/rewards/<reward_id>/redemptions \
  -H 'X-Motif-API-Key: <integration_key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "external_user_id": "user-42",
    "idempotency_key": "coupon-user-42-books-october"
  }'
```

The engine reserves/spends 250 loyalty points, delivers the payload through the configured provider, and returns a redemption state. A free reward omits both `cost_currency` and `cost_amount`.

### 5.6 Handle a refund

**Need:** Reverse a reward when an order is refunded.

There is no built-in “reverse the rule that ran earlier” action. Use one of these explicit approaches:

1. Emit a trusted `order.refunded` event for audit and let the customer backend call the scoped point transaction route with `type: "spend"`.
2. For an operator correction, use `POST /v1/admin/points/transactions` with `type: "spend"` and a unique idempotency key.
3. Keep the original award and the compensating entry. Never delete the original ledger row.

Example operator correction:

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "external_user_id":"user-42",
  "currency":"loyalty",
  "amount":100,
  "type":"spend",
  "idempotency_key":"refund-order-1001-loyalty-reversal",
  "provenance":{"source":"order_refund","order_id":"order-1001"}
}
```

The same transaction shape is available to the trusted customer backend at `POST /v1/users/{externalUserId}/points/transactions`:

```bash
curl -X POST http://127.0.0.1:18080/v1/users/user-42/points/transactions \
  -H 'X-Motif-API-Key: <integration_key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "currency":"loyalty",
    "amount":100,
    "type":"spend",
    "idempotency_key":"refund-order-1001-loyalty-reversal",
    "provenance":{"source":"order_refund","order_id":"order-1001"}
  }'
```

### 5.7 Measure the checkout journey

Emit `product.viewed`, `cart.created`, `checkout.started`, and `order.completed`. Then run a funnel:

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "steps":[
    {"name":"Product viewed","event_type":"product.viewed"},
    {"name":"Cart created","event_type":"cart.created"},
    {"name":"Checkout started","event_type":"checkout.started"},
    {"name":"Order completed","event_type":"order.completed"}
  ]
}
```

The report counts users that reach each ordered step and returns each step's conversion rate relative to the first step.

### 5.8 VIP or repeat-buyer tiers

Put the current tier or lifetime order count in a trusted event payload, then route the event to tier-specific rules. Use XP levels for an engine-owned progression label; use a customer-owned `vip_tier` field when the tier is calculated from billing or CRM rules outside MotifEngine.

## 6. SaaS and subscription scenarios

### 6.1 Onboarding after signup

Emit `account.created` with a stable user ID. Publish:

```json
{
  "event_type":"account.created",
  "conditions":{},
  "actions":[{"type":"award_xp","amount":50}]
}
```

Create levels such as:

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/levels \
  -H 'Authorization: Bearer <admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id":"acme",
    "environment_id":"sandbox",
    "level":1,
    "name":"Getting started",
    "xp_threshold":0
  }'
```

`GET /v1/users/{externalUserId}/progression` returns total XP, the highest active threshold reached, its name, and the next threshold if one exists.

### 6.2 Complete an onboarding checklist

Create missions for `workspace.created`, `team.member_invited`, and `integration.connected`. Each mission advances only for its own event type and caps at its target count.

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "name":"Connect an integration",
  "event_type":"integration.connected",
  "target_count":1
}
```

The profile endpoint reports `progress`, `target_count`, `completed`, and `completed_at`. A mission is progress tracking; it does not automatically award points unless you add a matching rule.

### 6.3 Recognize feature adoption

Create an achievement:

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/achievements \
  -H 'Authorization: Bearer <admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id":"acme",
    "environment_id":"sandbox",
    "name":"Automation builder",
    "description":"Run the automation builder five times",
    "event_type":"automation.run",
    "required_count":5
  }'
```

The worker increments progress once per accepted event ID and records an immutable unlock timestamp when the count is reached.

### 6.4 Subscription renewal and trial conversion

Emit `subscription.renewed` and `subscription.converted` from the billing backend. Use payload conditions for plan and amount:

```json
{
  "event_type":"subscription.renewed",
  "conditions":{"eq":["$.plan","growth"]},
  "actions":[
    {"type":"award_xp","amount":100},
    {"type":"award_points","currency":"coins","amount":20}
  ]
}
```

For a trial-to-paid flow, use an `account.trial_converted` rule and a mission that counts onboarding events. The engine records the fact and performs the configured effects; billing state remains owned by the customer's system.

### 6.5 Churn and return reporting

Use retention analytics with `account.created` as the cohort and `session.started` or `workspace.opened` as the return event. Users are included only when the event has an external user ID.

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "cohort_event_type":"account.created",
  "return_event_type":"session.started",
  "intervals":[1,7,30]
}
```

The result contains the cohort denominator and returning users/rates for each UTC day interval.

### 6.6 Tenant plans and usage limits

This is a platform-admin scenario rather than an end-user loyalty rule. Define a plan with entitlements such as `max_rules`, `max_currencies`, `max_rewards`, and `max_event_payload_bytes`; assign it to a tenant; inspect usage. Resource creation over a limit returns `409 plan_limit_exceeded`, and an oversized event can return `413 plan_limit_exceeded`. Billing is manual; MotifEngine does not call a payment processor.

## 7. Education and learning scenarios

### 7.1 Lesson completion gives XP

```json
{
  "event_type":"lesson.completed",
  "conditions":{"gte":["$.score",70]},
  "actions":[{"type":"award_xp","amount":25}]
}
```

The learning backend sends `lesson_id`, `course_id`, `score`, and an event ID that is unique for the completion. A failed quiz can be accepted as an event but will not match this rule.

### 7.2 Complete a course and unlock a benefit

Publish an `course.completed` rule that awards XP, and create a free `digital_unlock` reward that the application redeems after the event. The engine deliberately separates the event/rule effect from reward redemption so the customer can decide whether an unlock is automatic or requires a user click.

### 7.3 Quiz-score tiers

Use `all` with numeric comparisons:

```json
{
  "event_type":"quiz.completed",
  "conditions":{
    "all":[
      {"gte":["$.score",90]},
      {"eq":["$.attempt_type","final"]}
    ]
  },
  "actions":[
    {"type":"award_xp","amount":40},
    {"type":"award_points","currency":"coins","amount":10}
  ]
}
```

### 7.4 Learning streaks

Send one event per active day, such as `lesson.completed`, and read:

```bash
curl 'http://127.0.0.1:18080/v1/users/user-42/streaks?event_type=lesson.completed' \
  -H 'X-Motif-API-Key: <integration_key>'
```

The response includes `current_streak`, `longest_streak`, and `last_active_date`. Streak calculation uses UTC calendar dates and is a read model; it does not itself award points.

### 7.5 “Complete five lessons” mission and achievement

Create a mission with `target_count: 5` for `lesson.completed` to show a live checklist, and an achievement with `required_count: 5` for durable recognition. They may listen to the same event but intentionally remain separate concepts.

## 8. Content and media scenarios

### 8.1 Reward an article or video completion

Emit `content.completed` with `content_type`, `content_id`, and `genre`. Use a rule for XP and a second rule for a genre-specific currency bonus.

```json
{
  "event_type":"content.completed",
  "conditions":{"eq":["$.content_type","video"]},
  "actions":[{"type":"award_xp","amount":15}]
}
```

### 8.2 Daily engagement campaign

Create a `daily_open` mission with a target count, a 24-hour campaign window, and an XP rule with `starts_at` and `ends_at`. Schedule windows control rule execution; campaign analytics count matching events within the window.

### 8.3 Genre or collection milestones

Include the collection in the payload and use `eq` or `any` conditions. If the user must complete a number of items, use a mission or achievement; if each completion has a different effect, use separate rules.

## 9. Community and social scenarios

### 9.1 Reward contributions

Emit `post.created`, `comment.created`, or `reaction.added` only after the customer moderation system has accepted the action. Award small XP or points with separate rules:

```json
{
  "event_type":"comment.created",
  "conditions":{"eq":["$.moderation_status","approved"]},
  "actions":[{"type":"award_xp","amount":5}]
}
```

MotifEngine does not moderate content. The customer backend remains responsible for trusted event emission, anti-abuse controls, and taking back value for a deleted or abusive contribution.

### 9.2 Contributor milestone

Create an achievement for `comment.created` with `required_count: 100`, or a mission for a time-boxed contribution challenge. A user can read progress through `/achievements` and `/missions`.

### 9.3 Multiple outcomes from one contribution

Use one multi-action rule to award both progression and a currency entry, then use a second rule for special categories such as `accepted_answer`.

## 10. Games and high-frequency engagement

### 10.1 Match or quest completion

Send `match.completed` or `quest.completed` with result fields. Conditions can gate rewards by `result`, `difficulty`, `score`, or a server-owned `eligible` flag.

### 10.2 Weekend double rewards

Create a rule with two actions and an explicit UTC window:

```json
{
  "name":"Weekend quest bonus",
  "event_type":"quest.completed",
  "conditions":{"eq":["$.eligible",true]},
  "actions":[
    {"type":"award_xp","amount":50},
    {"type":"award_points","currency":"coins","amount":20}
  ],
  "starts_at":"2026-10-17T00:00:00Z",
  "ends_at":"2026-10-20T00:00:00Z"
}
```

### 10.3 Safe redelivery for repeated events

Give every match a stable `event_id`. Sending the same ID and the same canonical payload returns the existing receipt. Sending the same ID with a different payload returns `409 event_id_conflict`. A broker redelivery also reuses the action idempotency key.

### 10.4 What not to put in a high-frequency event

Keep payloads bounded and avoid secrets or unnecessary personal data. The event payload limit is 64 KiB, and a tenant's plan may set a lower effective limit.

## 11. Other domains that use the same primitives

MotifEngine does not hard-code commerce, learning, or games. The same event/rule/state model can support other products as long as the customer backend emits trusted facts.

| Domain need | Event and configuration |
| --- | --- |
| Marketplace seller milestone | `seller.order.fulfilled` → mission for 10 fulfilled orders → achievement for 100 → `award_xp` for each approved fulfillment |
| Fitness habit | `workout.completed` → `award_xp` + daily streak read; use a mission for a monthly target |
| Fintech education | `savings.lesson.completed` → XP and achievement; keep actual money and account balances in the fintech system |
| Customer support | `ticket.resolved` with a trusted quality flag → agent XP or a recognition achievement |
| Partner certification | `course.completed` with `partner_id` → partner-specific rule and a free digital unlock |
| Internal employee engagement | `training.completed` or `idea.accepted` → a tenant-scoped currency, levels, and recognition |

The engine records the engagement state and point value. It does not become the system of record for payments, moderation, payroll, or regulated account balances.

## 12. Loyalty: the complete lifecycle

Loyalty is a composition of a spendable currency, rules that earn it, a ledger that records it, and rewards that consume it. XP is a separate progression axis and must not be treated as money or spendable points.

### 11.1 Create a currency

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/currencies \
  -H 'Authorization: Bearer <tenant_or_super_admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id":"acme",
    "environment_id":"sandbox",
    "code":"loyalty",
    "name":"Loyalty points",
    "unit_scale":0,
    "policy":{"expiry_days":180}
  }'
```

Currency codes are lowercase identifiers. `unit_scale` is an integer precision setting from 0 to 6. The current policy surface accepts `expiry_days` as a non-negative integer; the active policy is versioned when the currency is created.

### 11.2 Earn points

Use `award_points` from a published rule for event-driven earning. Each award creates:

- an append-only ledger entry;
- a new balance projection;
- a point lot with an optional expiry timestamp;
- provenance and an idempotency key.

The customer can also use the server-side admin transaction endpoint for an operator correction or a controlled integration workflow:

```bash
curl -X POST http://127.0.0.1:18080/v1/admin/points/transactions \
  -H 'Authorization: Bearer <admin_session>' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id":"acme",
    "environment_id":"sandbox",
    "external_user_id":"user-42",
    "currency":"loyalty",
    "amount":50,
    "type":"award",
    "idempotency_key":"support-case-123-loyalty-credit",
    "provenance":{"source":"support","case_id":"123"}
  }'
```

### 11.3 Spend points

Use a reward redemption for normal customer spending. For an operator adjustment, the same admin endpoint accepts `type: "spend"`. Balances cannot go negative; an insufficient balance returns a conflict.

Spends allocate the earliest-expiring available lots first. The original award rows remain intact and the spend is a separate ledger entry.

### 11.4 Expiration

When a currency has `expiry_days`, each award or conversion credit receives its own `expires_at`. The worker runs an expiration pass every minute. Expiration writes an `expiration` debit and marks the lot consumed; it does not delete the original award. The operation is idempotent per lot.

### 11.5 Convert between currencies

Conversion is an explicit atomic operation, not a rule action. Both currencies must belong to the same tenant and environment. The source debit and target credit commit in one PostgreSQL transaction, with a recorded rate and conversion ID.

```bash
curl -X POST http://127.0.0.1:18080/v1/points/conversions \
  -H 'X-Motif-API-Key: <integration_key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "external_user_id":"user-42",
    "source_currency":"loyalty",
    "target_currency":"coins",
    "source_amount":100,
    "rate_numerator":1,
    "rate_denominator":2,
    "idempotency_key":"convert-user-42-loyalty-to-coins-001"
  }'
```

The integer result is `source_amount * rate_numerator / rate_denominator`, rounded down by integer division. A repeated request with the same key and the same request returns the existing conversion; a changed request returns an idempotency conflict.

### 11.6 Read the loyalty state

```bash
curl http://127.0.0.1:18080/v1/users/user-42/points \
  -H 'X-Motif-API-Key: <integration_key>'

curl 'http://127.0.0.1:18080/v1/users/user-42/points/transactions?limit=20' \
  -H 'X-Motif-API-Key: <integration_key>'
```

Transactions are cursor-paginated and include amount, entry type, balance after, provenance, and idempotency key. The profile endpoint combines balances, missions, achievements, and progression for a single user view.

### 11.7 Configure and deliver rewards

Rewards are independent catalog objects. The initial reward type is `digital_unlock`. A reward is free when it has no cost fields; otherwise it charges one active currency and a positive integer amount.

The lifecycle is:

```text
requested → reserved (paid rewards only) → delivering → fulfilled
                                      └──→ failed → retry or release
```

The mock provider returns a deterministic `mock:<redemption_id>` reference. An HTTP provider receives the redemption ID as its idempotency key. A timeout or 5xx response is treated as unknown and leaves the redemption in `delivering` until a status endpoint or retry resolves it. A provider-confirmed failure releases the reserved points with a compensating ledger entry.

### 11.8 Provider configuration

Configure a scope-specific HTTP provider in **Settings → Providers** or with `POST /v1/admin/reward-providers`. The delivery endpoint is required for HTTP. The optional status URL may contain `{redemption_id}` and must return `pending`, `fulfilled`, or `failed`. Provider tokens are encrypted at rest and never returned from list/update responses.

### 11.9 Redemption recovery

For a failed or interrupted redemption, call:

```bash
curl -X POST http://127.0.0.1:18080/v1/redemptions/<redemption_id>/retry \
  -H 'X-Motif-API-Key: <integration_key>'
```

Operators can use **Overview → Redemptions** or `POST /v1/admin/redemptions/retry`, then inspect `GET /v1/admin/redemptions/{id}/attempts`. Never create a second redemption just because the first provider call timed out; retry the durable redemption ID.

## 13. Measurement and optimization scenarios

### 12.1 Reusable business metric

Create a metric definition with aggregation `count` or `unique_users`:

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "id":"completed-orders",
  "name":"Completed orders",
  "description":"All paid order completions",
  "event_type":"order.completed",
  "aggregation":"count"
}
```

Metrics are named definitions. The current API exposes the definitions and the campaign analytics endpoint; it does not provide a warehouse-backed time-series query language.

Archive an obsolete definition with `POST /v1/admin/metrics/{metric_id}/archive`; archiving removes it from active configuration without deleting historical events.

### 12.2 Campaign performance

Create a goal such as “100 completed orders”, then a campaign for `order.completed` with optional `starts_at` and `ends_at`. Campaign analytics reports matching event count, unique users, and the linked target count inside the campaign window.

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "id":"october-repeat-purchase",
  "goal_id":"repeat-orders",
  "name":"October repeat purchase",
  "description":"Orders received in the October campaign",
  "event_type":"order.completed",
  "starts_at":"2026-10-01T00:00:00Z",
  "ends_at":"2026-11-01T00:00:00Z"
}
```

Goals and campaigns describe and measure an activity. The rule that awards points is still a separate automation.

### 12.3 Funnel

Use two to eight ordered steps. Each user advances at most once per step and must reach the preceding step first. This makes a checkout, onboarding, lesson, or redemption funnel possible without a separate analytics warehouse.

### 12.4 Retention

Choose a cohort event (`account.created`, `order.completed`, or `lesson.completed`), a return event (`session.started`, `order.completed`, or `lesson.completed`), and one to five day intervals from 1 to 90. The default intervals are 1, 7, and 30 days.

### 12.5 Behavior segment

Ask for users with at least `N` occurrences of an event in an optional UTC time range:

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "event_type":"lesson.completed",
  "min_events":3,
  "from":"2026-10-01T00:00:00Z",
  "to":"2026-11-01T00:00:00Z"
}
```

The report returns matching users and total matching events. It is a measurement result, not a persisted audience or an automatic rule trigger.

### 12.6 Deterministic experiment report

Provide two to four variants whose weights add to 100:

```json
{
  "tenant_id":"acme",
  "environment_id":"sandbox",
  "id":"checkout-bonus-copy",
  "conversion_event_type":"order.completed",
  "variants":[
    {"id":"control","weight":50},
    {"id":"bonus-copy","weight":50}
  ]
}
```

Assignment is deterministic from tenant, experiment ID, and external user ID. Re-running the same report keeps a user in the same variant. The report returns assigned users, converted users, and conversion rate per variant. This is a reporting primitive; it does not persist experiment lifecycle metadata or change the customer's UI.

## 14. Integration patterns

### 14.1 Direct REST integration

The customer's backend sends events with a tenant/environment-scoped integration key. For example, a completed order can be sent with the platform's versioned REST API:

```sh
curl -X POST "$MOTIF_API_URL/v1/events" \
  -H "X-Motif-API-Key: $MOTIF_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"order-123-completed","event_type":"order.completed","user_id":"user-42","payload":{"order_id":"123","total":125,"payment_status":"paid"}}'
```

The `202 Accepted` receipt confirms durable acceptance, not completed rule processing. Read `GET /v1/events/{event_id}` to observe processing. Reuse the same event ID only when retrying the same business fact; a different payload for that ID returns `409 event_id_conflict`. The [REST integration guide](rest-integration.md) includes TypeScript and Go HTTP examples plus error-handling guidance.

### 14.2 HTTP client choice

The same contract works with `curl`, TypeScript `fetch`, Go `net/http`, Python, or any backend HTTP client. MotifEngine does not publish an official language SDK. Keep the integration key in a server-side secret store. Use `occurred_at` when the business event happened earlier than the API call and you need accurate analytics or streak dates.

### 14.3 Customer-facing UI

MotifEngine returns state; the customer product renders it. For a loyalty screen, call profile, points, transactions, missions, achievements, progression, and reward endpoints from the customer's backend and expose only the fields the end user should see. MotifEngine does not ship a mandatory end-user widget, leaderboard, badge UI, or notification service.

## 15. Platform administration and operations (Ops service)

### 14.1 Super Admin versus Tenant Admin

- **Super Admin:** is an internal platform role for the Ops service. It creates tenants and environments, manages credentials, plans, administrator identities, and platform-wide controls.
- **Tenant Admin:** creates or joins a workspace through the tenant dashboard and can operate scoped catalog, event, rule, reward, provider, engagement, and audit resources. The dashboard never asks this user to choose a tenant.
- **Integration credential:** is for a customer backend, not for dashboard administration.

The backend enforces this boundary. A client-provided `tenant_id` is not proof of access; the authenticated session or credential must authorize the scope.

### 14.2 Environment separation

Use `sandbox` for test events and `production` for live events. Credentials, rules, currencies, user balances, and rewards are environment-scoped. Do not reuse a sandbox key in production.

### 14.3 Rotate or revoke a credential

Create a new credential, deploy it, verify events, then revoke the old credential. The raw key is never returned from list or audit endpoints.

### 14.4 Suspend a tenant

Suspending a tenant blocks its integration routes, including events, points, conversions, and reward operations. This is useful during an incident or credential investigation.

### 14.5 Audit and diagnostics

Every important platform mutation is audit logged. Use **Settings → Audit** or `GET /v1/admin/audit-log` to answer who changed a tenant, credential, event contract, rule, catalog entry, plan, or provider. Use diagnostics for queue and failure counts; use rule-execution and redemption feeds for the individual record.

### 14.6 Health and metrics

- API liveness: `/health/live`
- API readiness: `/health/ready`
- Worker readiness: worker `/health/ready`
- Prometheus text metrics: `/metrics` when enabled
- `X-Request-ID`: correlation value in request logs

Alert on readiness failures, sustained API error counters, failed rule executions, pending outbox growth, and failed or reconciliation-pending redemptions.

## 16. Failure, retry, and idempotency behavior

### Event ingestion

- Same tenant/environment + same `event_id` + same canonical payload: returns the existing receipt.
- Same `event_id` + different payload: `409 event_id_conflict`.
- Schema mismatch: `422 event_schema_mismatch`.
- Invalid JSON or invalid event shape: `400 invalid_event`.
- Suspended or revoked scope: unauthorized or tenant-suspended response.

### Rule execution

- A rule must be published before it can run.
- A failed action is recorded with attempts and a last error.
- The worker retries the execution up to three attempts.
- Each action ordinal has its own idempotency key, so a later action can resume after an earlier action already succeeded.

### Point ledger

- Reusing an idempotency key with a different request returns an idempotency conflict.
- Spending more than the available balance returns an insufficient-balance conflict.
- Concurrent spends lock the account and cannot create a negative balance.
- Expiration and provider compensation are separate ledger entries, never destructive edits.

### Redemptions

- Reusing the same user + idempotency key returns the existing redemption if the request is identical.
- A paid reward reserves points before delivery.
- A definitive provider failure releases the reservation.
- An unknown provider outcome remains `delivering`; reconcile or retry the same redemption ID.
- A fulfilled redemption is not retried; a cancelled redemption is not retryable.

## 17. Need-to-capability map

| If the product owner says… | Use this composition |
| --- | --- |
| “Give a customer points after an order” | `order.completed` event → published rule → `award_points` |
| “Give XP but do not let users spend it” | `award_xp` + configured levels |
| “Give both XP and points” | One rule with two actions |
| “Only reward paid orders over 100” | `all` of `payment_status == paid` and `total >= 100` |
| “Reward the second purchase” | Customer backend sends a trusted `customer_order_number: 2` field, then a condition |
| “Show a checklist” | Mission + customer UI reading `/missions` |
| “Unlock a permanent milestone” | Achievement + customer UI reading `/achievements` |
| “Points expire after 180 days” | Currency policy `expiry_days: 180` |
| “Convert bonus points to coins” | Explicit atomic `/v1/points/conversions` request |
| “Let users spend points on a coupon” | Paid `digital_unlock` reward + redemption provider |
| “Give a free in-app unlock” | Free `digital_unlock` reward + redemption provider |
| “Count orders in a campaign” | Goal + campaign + campaign analytics |
| “Know where checkout users drop” | Funnel report |
| “Know if new users return” | Retention report |
| “Find power users” | Segment report with `min_events` |
| “Compare two experiences” | Deterministic experiment report + customer-owned variant delivery |
| “See why a reward failed” | Diagnostics + redemption attempts + provider status/retry |
| “Stop a tenant temporarily” | Super Admin tenant suspension |
| “Keep test data away from live data” | Separate environments and credentials |

## 18. Current boundaries and unsupported assumptions

The following statements are intentionally explicit so a scenario is not designed around a capability the current runtime does not have.

1. **Rules do not query history.** Conditions inspect the current event payload only. Counts, tiers, eligibility flags, and fraud decisions that depend on outside history must be calculated by the customer backend and sent as trusted fields, or expressed as missions/achievements/analytics where that semantic fits.
2. **Rules do not send email, push, webhooks, or arbitrary code.** The supported action types are `log`, `award_xp`, and `award_points`.
3. **Rules do not redeem rewards automatically.** A rule can award points; the customer backend or an admin workflow calls the redemption endpoint when the user should claim a reward.
4. **The initial reward type is `digital_unlock`.** Coupon, discount, cashback, or physical delivery can be represented in the payload and handled by an HTTP provider, but MotifEngine does not contain a native coupon marketplace or fulfillment system.
5. **There is no mandatory end-user frontend.** The customer renders balances, missions, achievements, levels, streaks, and rewards in its own product.
6. **Missions and achievements listen to one event type.** Multi-step journeys can be modeled as multiple missions/achievements or multiple event types in the customer backend; they are not a generic workflow graph.
7. **Goals and campaigns are measurement primitives.** They do not automatically publish rules, allocate budgets, or define audience eligibility.
8. **Experiment assignment is report-time deterministic.** The engine does not persist a campaign UI variant, holdout cookie, or experiment lifecycle; the customer must serve the chosen experience.
9. **Analytics read the canonical events table.** The reports are useful at the current deployment scale and are not a replacement for a warehouse or a long-term BI model.
10. **Conversion rates are explicit requests.** The engine does not discover exchange rates or call a pricing service; the caller supplies numerator, denominator, and idempotency key.
11. **Billing is manual.** Plans and entitlements limit resources, but no payment processor or subscription billing automation is included.
12. **Customer moderation and anti-abuse remain external.** Emit trusted events only after the customer system has made its eligibility decision.
13. **Administrative sessions expire.** The browser should use the session flow; the legacy admin API key is a controlled bootstrap/migration fallback, not a replacement for RBAC.

## 19. Practical launch checklist

Before enabling a scenario in production:

- [ ] A tenant and the intended environment exist.
- [ ] A dedicated integration credential is stored server-side.
- [ ] Event names and payload schemas are registered.
- [ ] Every producer event has a stable unique `event_id`.
- [ ] `user_id` is the customer's stable external ID and is present for user-level effects.
- [ ] Currencies, levels, missions, achievements, and rewards are active in the intended scope.
- [ ] Rules are reviewed in the Configure builder and explicitly published.
- [ ] Rule windows are expressed in UTC and tested at their boundaries.
- [ ] A test event has been sent from the console or customer backend.
- [ ] The event reaches `processed` and the expected ledger/profile effect is visible.
- [ ] The same event has been retried to verify idempotency.
- [ ] A paid reward has been tested with insufficient balance and with a successful redemption.
- [ ] Provider timeout and retry behavior has been exercised if an HTTP provider is used.
- [ ] Diagnostics, audit log, and failed execution/redemption paths are visible to operators.
- [ ] The customer-facing product knows where to read profile, points, progression, missions, achievements, and redemptions.

If a new requirement can be described as **a trusted event, a bounded condition, a typed effect, or a read/report of resulting state**, it will usually fit the current engine. If it requires history-aware rule evaluation, arbitrary side effects, or a new reward protocol, keep that decision in the customer backend or treat it as a new product capability rather than hiding it inside an existing rule.
