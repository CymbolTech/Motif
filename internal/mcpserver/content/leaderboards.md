# Leaderboards

A leaderboard ranks users by what they **earn** during a round: points in one currency, or XP. Spending, expiry, conversions and refunds of failed redemptions never change a rank. Every award counts — from rules, the integration API and console grants — and a replayed award never counts twice. Points earned while no round is running count nowhere.

## Boards and rounds

A board has a stable **key** (for example `weekly`) that the product backend reads it by, and runs as a series of **rounds**. Its schedule decides who creates them:

| Schedule | Rounds come from | Use it when |
| --- | --- | --- |
| `manual` | `start_leaderboard_round`, or the console | An operator decides when a round ends, so rounds can run 6 days one week and 8 the next |
| `recurring` | The engine, at every boundary of `every` × `interval` in `timezone` | Daily, weekly, monthly or fortnightly boards that reset on their own |

A recurring round is created the first time it is needed, so nothing has to run at midnight. The first round starts when the board is created. The key and the metric cannot change; a new schedule takes effect at the next boundary.

## Tools

| Need | Tool |
| --- | --- |
| Create a board | `create_leaderboard` with `rank` (`points` + `currency`, or `xp`) and `rounds` |
| Rotate a manual board | `start_leaderboard_round` — the running round ends at the same instant unless `end_current` is false |
| Schedule the next round | `start_leaderboard_round` with a future `starts_at` |
| Run a round 8 days instead of 7 | `update_leaderboard_round` with `ends_at` |
| Close a round now, or cancel a scheduled one | `end_leaderboard_round` — confirm first; ended rounds cannot reopen |
| See standings and history | `get_leaderboard` (`round`: `current`, `previous`, or a number) |
| One user's ranks | `get_user_state` |
| Rename, change schedule, turn off | `update_catalog_item` with kind `leaderboard` |
| Remove a board and its history | `delete_leaderboard` — confirm first; turning it off keeps history |

## Reading from the product backend

```bash
curl "$MOTIF_API_URL/v1/leaderboards/weekly?limit=10" -H "X-Motif-API-Key: $MOTIF_API_KEY"
```

The response carries the round (`name`, `starts_at`, `ends_at`, `status`), `participants`, ranked `entries` (`rank`, `user_id`, `score`, `reached_at`) and `next_round` when one is scheduled. When no round runs it returns the last round with `status: "ended"`, so a page between two rounds shows final results. Ranks are unique: higher score first, then whoever reached the score first. The product maps `user_id` to names and avatars itself.

## Example: a café

- `weekly`: manual, points in `coins`. Staff start "Week 45" each Saturday and pick its end.
- `monthly`: recurring, `every: month`, `timezone: Asia/Tehran`.
- The café's backend calls `GET /v1/leaderboards/weekly` and `GET /v1/leaderboards/monthly` for its QR page.
