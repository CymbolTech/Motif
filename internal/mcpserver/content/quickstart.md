# Quickstart: add MotifEngine to an application

Follow these steps in order. Each step names the MCP tool to use.

1. **Orient.** Call `get_workspace` to see the environment this key can use and its scopes. Prefer a sandbox or test environment while building.
2. **Find the business facts.** Read the application code for the moments that matter (order paid, lesson completed, account created). Each becomes an event type with a stable name such as `order.completed`. Put everything a rule needs to decide in the payload: rules read the current event only, never history (send `customer_order_number: 2`, not "second order").
3. **Register contracts.** Call `register_event_type` with a payload schema for each event type. Check what exists first with `list_event_types`.
4. **Create the catalog.** Use `list_catalog`, then `create_currency`, `create_level`, `create_mission`, `create_achievement`, `create_reward` or `create_leaderboard` for what the design needs. A manual leaderboard also needs `start_leaderboard_round` before anything counts.
5. **Design rules.** For each rule, call `check_rule` with realistic `sample_payload`s, including one that should not match, until the explanation is what you expect.
6. **Save drafts.** Call `save_rule_draft`. Drafts never run.
7. **Publish with the human.** Show the human the rule, then call `publish_rule` with the exact draft version. If the key lacks the `edit` scope, give the human the review link and ask them to publish in the console.
8. **Prove it.** Call `send_test_event` with a test user. It waits for processing and returns the trace: rules that ran, points and XP awarded, missions and achievements advanced. Send the same `event_id` again to prove a retry does not award twice. Use `get_user_state` to read the result.
9. **Write the integration.** Call `get_integration_snippet` for each event type and language. Put the code where the business fact becomes true on the server: after payment is captured, not on a button click. Keep the access key in a server-side secret (`MOTIF_API_KEY`), derive `event_id` from the business fact, and retry 5xx, 429 and network errors with the same `event_id`.
10. **Walk the launch checklist.** Read the `launch-checklist` topic before going live.
