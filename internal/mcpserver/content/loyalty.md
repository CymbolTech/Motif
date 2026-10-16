# Points, ledger and rewards

Loyalty is a spendable currency, rules that earn it, a ledger that records it, and rewards that consume it. XP is a separate progression axis and is never money.

## Currencies

A currency has a lowercase `code` used in rules (`loyalty`, `coins`), a `name`, a `unit_scale` from 0 to 6, and an optional policy `{"expiry_days": 180}`.

## The ledger

Every award, spend, expiry and conversion is an append-only ledger entry with the balance after it, provenance and an idempotency key. Rows are never edited or deleted; corrections are new entries.

- Balances cannot go negative; spending more than the balance is a conflict.
- Spends consume the earliest-expiring points first.
- With `expiry_days`, each award gets its own expiry; the worker writes an `expiration` entry when it passes.

The product backend can also award or spend directly: `POST /v1/users/{user_id}/points/transactions` with `{currency, amount, type: "award" | "spend", idempotency_key, provenance}`. Use this for refunds and corrections: emit `order.refunded`, then spend back the points with a key such as `refund-order-1001`. There is no automatic reversal of an earlier rule.

## Conversion

`POST /v1/points/conversions` converts between two currencies atomically at a rate the caller supplies (`rate_numerator`, `rate_denominator`), rounded down, with an idempotency key. It is never a rule action.

## Rewards

The reward type is `digital_unlock`. A reward is free, or costs one active currency and a positive amount. Its `payload` goes to the delivery provider, which interprets it as a coupon, entitlement or benefit.

The product backend redeems: `POST /v1/rewards/{reward_id}/redemptions` with `{external_user_id, idempotency_key}`. The lifecycle:

```text
requested → reserved (paid rewards) → delivering → fulfilled
                                   └→ failed → retry or release
```

- A paid reward reserves points before delivery; a definitive provider failure releases them with a compensating entry.
- A provider timeout leaves the redemption `delivering` until a status check or `POST /v1/redemptions/{id}/retry` resolves it. Retry the same redemption; never create a second one.
- Rules never redeem. The product decides when a user claims a reward.

This MCP server can create rewards but cannot redeem them, change balances or configure delivery providers.
