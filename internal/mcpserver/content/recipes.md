# Recipes

## Need to composition

| The product owner says | Compose |
| --- | --- |
| Points after an order | `order.completed` → rule → `award_points` |
| XP that can't be spent | `award_xp` + levels |
| XP and points together | One rule, two actions |
| Only paid orders over 100 | `all` of `eq $.payment_status paid` and `gte $.total 100` |
| Reward the second purchase | The backend sends a trusted `customer_order_number`; rule on `eq 2` |
| A checklist | Missions, one per event type |
| A permanent badge | Achievement |
| Points expire after 180 days | Currency policy `expiry_days: 180` |
| Spend points on a coupon | Paid `digital_unlock` reward + delivery provider |
| A free in-app unlock | Free `digital_unlock` reward |
| Weekend double points | A second rule with `starts_at`/`ends_at` in UTC |
| Undo points on refund | `order.refunded` + a backend `spend` with an idempotency key |
| Separate test from live | Separate environments and integration keys |
| Weekly top customers | Leaderboard ranking points earned, recurring weekly or manual rounds |
| A contest that ends when we say | Manual leaderboard; `start_leaderboard_round` rotates it |

## Patterns

**Paid order reward.** Register `order.completed` with `order_id`, `total`, `payment_status`. Create currency `coins`. Rule: `{"eq":["$.payment_status","paid"]}` → `award_xp 10`, `award_points coins 5`.

**Threshold bonus.** A second rule: `{"all":[{"eq":["$.payment_status","paid"]},{"gte":["$.total",100]}]}` → `award_points loyalty 25`. Both rules run on a large paid order.

**Onboarding.** `account.created` → `award_xp 50`; levels at 0, 100, 500 XP; missions for `workspace.created`, `team.member_invited`, `integration.connected`, each with target 1.

**Learning.** `lesson.completed` with `score` → rule `{"gte":["$.score",70]}` → `award_xp 25`; a mission with target 5 and an achievement with required count 5 on the same event; streaks for daily practice.

**Community.** Emit `comment.created` only after moderation approves it; `{"eq":["$.moderation_status","approved"]}` → `award_xp 5`; an achievement at 100 comments.

**Games.** `quest.completed` with a server-owned `eligible` flag; a weekend rule with a UTC window and two actions; a stable `event_id` per match so redelivery never pays twice.
