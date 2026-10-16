# Integrate a customer backend through REST

MotifEngine accepts events and serves user state through a versioned HTTP API. Your trusted backend calls it with an integration key scoped to one tenant and environment. Keep that key in a server-side secret store; never send it to a browser or mobile app. The [OpenAPI description](../api/openapi.yaml) lists request and response shapes. The dashboard is for configuring event contracts, currencies, and rules; it is not needed on the runtime request path.

## 1. Configure the behavior

In the tenant workspace, register the `order.completed` event and its payload fields. Create a `coins` currency if it does not exist. Create and publish a rule whose event is `order.completed` and whose ordered outcomes award XP and coins. These definitions can be created in the configuration flow before the backend starts sending events.

Use a separate integration key for each tenant/environment. The local bootstrap key is for development only. Tenant administrators can manage configuration in their authorized scope; integration keys cannot administer tenants or rules.

## 2. Send one business fact

Set `MOTIF_API_URL` to the API origin and `MOTIF_API_KEY` to a scoped integration key in your backend environment. The examples use one stable event ID for one completed order.

### curl

```sh
curl -i -X POST "$MOTIF_API_URL/v1/events" \
  -H "X-Motif-API-Key: $MOTIF_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "event_id": "order-123-completed",
    "event_type": "order.completed",
    "user_id": "user-42",
    "payload": {"order_id": "123", "total": 125, "payment_status": "paid"}
  }'
```

### TypeScript with built-in `fetch`

```ts
const apiUrl = process.env.MOTIF_API_URL ?? 'http://127.0.0.1:18080';
const apiKey = process.env.MOTIF_API_KEY;
if (!apiKey) throw new Error('MOTIF_API_KEY is required');

const response = await fetch(`${apiUrl}/v1/events`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', 'X-Motif-API-Key': apiKey },
  body: JSON.stringify({
    event_id: 'order-123-completed',
    event_type: 'order.completed',
    user_id: 'user-42',
    payload: { order_id: '123', total: 125, payment_status: 'paid' },
  }),
});
const body = await response.json();
if (response.status !== 202) throw new Error(`MotifEngine ${response.status}: ${JSON.stringify(body)}`);
console.log(body); // { event_id, external_event_id, status: 'accepted' }
```

### Go with `net/http`

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "net/http"
    "os"
    "time"
)

func main() {
    apiURL, apiKey := os.Getenv("MOTIF_API_URL"), os.Getenv("MOTIF_API_KEY")
    if apiURL == "" || apiKey == "" { panic("set MOTIF_API_URL and MOTIF_API_KEY") }
    body := []byte(`{"event_id":"order-123-completed","event_type":"order.completed","user_id":"user-42","payload":{"order_id":"123","total":125,"payment_status":"paid"}}`)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/v1/events", bytes.NewReader(body))
    if err != nil { panic(err) }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Motif-API-Key", apiKey)
    res, err := http.DefaultClient.Do(req)
    if err != nil { panic(err) }
    defer res.Body.Close()
    receipt, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
    if err != nil { panic(err) }
    if res.StatusCode != http.StatusAccepted { panic(fmt.Sprintf("MotifEngine %s: %s", res.Status, receipt)) }
    fmt.Println(string(receipt))
}
```

`202 Accepted` means the event and its outbox record are durably stored; rule outcomes are processed asynchronously. Read status with the same key:

```sh
curl "$MOTIF_API_URL/v1/events/order-123-completed" \
  -H "X-Motif-API-Key: $MOTIF_API_KEY"
```

After processing, read the customer's profile or point balance from your backend with `GET /v1/users/{externalUserId}/profile` or `GET /v1/users/{externalUserId}/points`. Return only the fields your own application needs to its end-user UI.

## 3. Handle retries and errors

| Outcome | Backend behavior |
| --- | --- |
| Timeout or transient `5xx` | Retry with backoff and the **same** `event_id` and payload. Check event status if the first response may have been lost. |
| `429` | Back off and honor `Retry-After` if present; keep the event ID stable. |
| `409 event_id_conflict` | Stop and investigate: that ID was previously accepted with different content. |
| `401` or `403` | Check the server-side credential and its tenant/environment scope; do not retry unchanged credentials indefinitely. |
| `4xx` validation error | Correct the event type, schema, or payload before sending a new request. |

An event ID identifies one business fact, such as the completion of order `123`. Do not generate a fresh ID on every HTTP retry. Redemption requests use their own stable `idempotency_key`. Never treat an accepted event as proof that every downstream action has finished; check the event status and, when needed, the resulting user state. See the [event API guide](../../Docs/api/events.md) for receipt and deduplication details.

## Contract and optional tooling

Use [OpenAPI](../api/openapi.yaml) as the machine-readable API reference and generate a client locally if your team wants one. MotifEngine does not publish an official language SDK.

## Configure from an MCP client

Coding tools that speak the Model Context Protocol can configure and inspect a workspace through the MCP endpoint on the console origin, `https://<console-domain>/mcp`. Create a workspace access token under **Access tokens** in the console, choose its permissions (`read`, `write`, `publish`) and environments, and add it as a bearer header:

```sh
claude mcp add --transport http motif https://<console-domain>/mcp \
  --header "Authorization: Bearer motif_at_…"
```

The endpoint offers tools to inspect the workspace, register event contracts, create catalog items, check and draft rules, publish with the `publish` permission, send test events and trace them, plus integration snippets and the product documentation. It never handles integration keys, point transactions or redemptions: customer services keep sending events over REST with their integration key.
