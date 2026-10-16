# MotifEngine overview

MotifEngine is an API-first engagement and loyalty engine. A product's backend sends trusted **events**; published **rules** turn matching events into XP, spendable points and log entries; **missions** and **achievements** track progress; a **reward** catalog lets users redeem what they earned. Every balance change is an append-only ledger entry with an idempotency key, so nothing is ever counted twice.

```text
product backend ── POST /v1/events (X-Motif-API-Key) ──▶ durable receipt (202)
                                                              │
                                                     worker evaluates rules
                                                              │
        ┌──────────────┬───────────────┬──────────────┬───────┴──────┐
     award_xp     award_points      mission      achievement        log
   (levels)     (currency ledger)  progress       progress     (audit only)
```

## Concepts

| Concept | What it is | Use it when |
| --- | --- | --- |
| Workspace | Your organization in MotifEngine | Always; an access token belongs to exactly one |
| Environment | An isolated scope such as `sandbox` or `production` | Test without touching live balances; everything below is per environment |
| Integration key | A server-side REST key for one environment (`X-Motif-API-Key`) | The product backend sends events and reads user state |
| Access token | A workspace token (`motif_at_…`) for MCP clients and scripts | Configuring and inspecting the workspace |
| External user ID | The product's own stable user ID, sent as `user_id` | Every user-level effect |
| Event | A fact that happened in the product | An action should be measured or rewarded |
| Event contract | A versioned payload schema for an event type | Reject malformed payloads before rules see them |
| Rule | A versioned `when event, if conditions, then actions` automation | A matching event should cause an effect |
| Currency | A spendable point balance, such as `loyalty` or `coins` | Users earn and spend value |
| XP and levels | Non-spendable progression and named thresholds | Users level up without spending |
| Mission | Progress toward a target count of one event type | Checklists and challenges |
| Achievement | A durable milestone after a count of one event type | Badges and recognition |
| Reward | A `digital_unlock` catalog item, free or costing one currency | Users redeem a benefit |
| Goal, campaign | Measurement of one event type, optionally in a time window | Track an initiative; they never award anything |

## Who does what

- **This MCP server** configures and inspects a workspace: event contracts, catalog, rule drafts, publishing (with permission), test events, traces and diagnostics.
- **The product backend** sends real events and reads user state over REST with the environment's integration key. MCP is never the event path.
- **The product's own UI** renders balances, levels, missions, achievements and rewards. MotifEngine ships no end-user widgets.
