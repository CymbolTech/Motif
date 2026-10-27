# Launch checklist

Before relying on an integration in production:

- [ ] The production environment exists, and an access key for it with only the scopes the backend needs (usually `read` and `write`) is in a server-side secret store.
- [ ] Every event type the backend sends has a registered contract.
- [ ] Every event has a stable `event_id` derived from the business fact.
- [ ] `user_id` is the product's stable user ID and is present on every user-level event.
- [ ] Currencies, levels, missions, achievements and rewards are active in the production environment (each environment has its own catalog).
- [ ] Every rule was checked with `check_rule`, reviewed by a human and published deliberately.
- [ ] Schedule windows are in UTC and tested at their edges.
- [ ] A test event reached `processed` and produced exactly the expected awards (`send_test_event`, `get_event_trace`).
- [ ] The same event sent twice awarded once.
- [ ] A paid reward was tested with too few points and with enough.
- [ ] The backend retries 5xx, 429 and network errors with the same `event_id`, and stops on 409 and 422.
- [ ] Someone watches `get_diagnostics` (or the console Overview) for failed executions and redemptions.
- [ ] The product UI knows where to read profile, points, progression, missions, achievements and redemptions.
