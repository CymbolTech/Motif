# Integration API reference

These are the routes a product backend calls with the environment's integration key in `X-Motif-API-Key`. The key selects the workspace and environment; never send it from a browser or mobile app.

## Events

| Route | Purpose |
| --- | --- |
| `POST /v1/events` | Send an event. Body: `event_id`, `event_type`, `user_id`, optional `occurred_at`, `payload`. Returns `202` with a receipt. |
| `GET /v1/events/{event_id}` | Read the receipt and processing status. |

## Users

| Route | Purpose |
| --- | --- |
| `GET /v1/users/{user_id}/profile` | Balances, event count, mission and achievement progress, progression. |
| `GET /v1/users/{user_id}/points` | Balances per currency. |
| `GET /v1/users/{user_id}/points/transactions?limit=20&cursor=` | Ledger entries, newest first, cursor-paginated. |
| `POST /v1/users/{user_id}/points/transactions` | Award or spend directly: `currency`, `amount`, `type` (`award` or `spend`), `idempotency_key`, optional `provenance`. |
| `GET /v1/users/{user_id}/missions` | Mission progress. |
| `GET /v1/users/{user_id}/achievements` | Achievement progress and unlocks. |
| `GET /v1/users/{user_id}/progression` | Total XP, current level, next threshold. |
| `GET /v1/users/{user_id}/streaks?event_type=` | Current and longest daily streak. |
| `GET /v1/users/{user_id}/leaderboards` | The user's rank and score on every active leaderboard's current round. |

## Leaderboards

| Route | Purpose |
| --- | --- |
| `GET /v1/leaderboards` | Active boards with the round each one shows. |
| `GET /v1/leaderboards/{key}?round=current&limit=10&offset=0` | Ranked entries of the current round; `round` may also be `previous` or a round number. |
| `GET /v1/leaderboards/{key}/rounds?limit=10` | Rounds, newest first, with each round's leader. |

## Points and rewards

| Route | Purpose |
| --- | --- |
| `POST /v1/points/conversions` | Convert: `external_user_id`, `source_currency`, `target_currency`, `source_amount`, `rate_numerator`, `rate_denominator`, `idempotency_key`. |
| `POST /v1/rewards/{reward_id}/redemptions` | Redeem: `external_user_id`, `idempotency_key`. |
| `GET /v1/redemptions/{redemption_id}` | Redemption status. |
| `POST /v1/redemptions/{redemption_id}/retry` | Retry a failed or interrupted redemption. |

Errors use `{"error": {"code": "...", "message": "..."}}`. Retry `5xx`, `429` and network errors with the same idempotency key or event ID; do not retry `4xx` unchanged.
