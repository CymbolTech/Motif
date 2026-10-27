# Events, idempotency and errors

## Sending

`POST /v1/events` with header `X-Motif-API-Key: <access key>` (the key needs the `write` scope):

```json
{
  "event_id": "order.completed:order-1001",
  "event_type": "order.completed",
  "user_id": "user-42",
  "occurred_at": "2026-10-14T10:00:00Z",
  "payload": { "order_id": "order-1001", "total": 129.9, "payment_status": "paid" }
}
```

- `event_id` identifies one business fact, at most 128 characters. Derive it from the fact (`order.completed:<order id>`), never from the clock or a random value per attempt.
- `user_id` is the product's stable user ID. Award actions fail without it.
- `occurred_at` is optional; send it when the fact happened earlier than the request, so analytics and streaks use the right date.
- `payload` is a JSON object up to 64 KiB (a plan can set less). Keep secrets and unnecessary personal data out of it.

`202 Accepted` means the event is durably stored. Processing is asynchronous: effects appear moments later. Read `GET /v1/events/{event_id}` for the receipt status, or use `get_event_trace` here for the full story.

## Idempotency

- Same `event_id` and same canonical payload: the existing receipt is returned and nothing runs again.
- Same `event_id` with a different payload: `409 event_id_conflict`. Stop and investigate; do not invent a new ID.
- Every rule action has its own idempotency key (event, rule version, action position), so worker retries never double-award.

## Errors

| Response | What to do |
| --- | --- |
| `202` | Stored. Done. |
| `5xx` or a network error | Retry with backoff and the same `event_id`. |
| `429` | Back off, honour `Retry-After`, keep the same `event_id`. |
| `409 event_id_conflict` | Stop: that ID was already used for different content. |
| `422 event_schema_mismatch` | The payload breaks the registered contract; fix the producer or register a new contract version. |
| `400 invalid_event` | Fix the request shape. |
| `401` or `403` | Wrong or revoked key, or a key for another environment. Do not retry blindly. |
| `413 plan_limit_exceeded` | The payload is over the plan's limit. |

## Contracts

Event contracts use a JSON Schema subset: `type`, `required`, `properties`, `enum`, `minimum`, `maximum`, `additionalProperties`. Registering a name again creates the next immutable version; accepted events keep the version they arrived with. An event type without a contract is still accepted, unvalidated.
