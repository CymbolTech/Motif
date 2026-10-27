# Boundaries

Design within what the engine does today:

1. **Rules read only the current event.** Counts, tiers, eligibility and fraud decisions that need history are computed by the product backend and sent as trusted payload fields.
2. **Actions are `award_xp`, `award_points` and `log`.** Rules send no email, push or webhooks and run no code.
3. **Rules never redeem rewards.** The product backend redeems when a user claims.
4. **The reward type is `digital_unlock`.** Coupons, discounts or shipping are expressed in its payload and fulfilled by the delivery provider.
5. **No end-user UI.** The product renders balances, levels, missions, achievements, rewards and leaderboards from the REST API, and maps user ids to names itself.
6. **Missions and achievements follow one event type each.** Multi-step journeys use several of them or several event types.
7. **Goals and campaigns only measure.** They never publish rules or award.
8. **Experiment assignment is computed at report time.** The product serves the variant.
9. **Analytics read the event table**, which suits operational reporting, not a warehouse.
10. **Conversion rates are explicit** in each conversion request.
11. **Moderation and anti-abuse stay in the product.** Emit events only after the product has decided the action counts.
12. **Leaderboards rank earnings only**, in points of one currency or XP. Ranking by event counts, balances or teams is not supported.
13. **This MCP server configures and inspects.** It never sends production traffic, holds integration keys, moves points, redeems rewards or configures delivery providers.
