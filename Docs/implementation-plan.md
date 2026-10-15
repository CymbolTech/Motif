# MotifEngine implementation plan

The product specification is [initial-implementation.md](initial-implementation.md). This plan records the order of working increments. Each increment ends with executable checks and documentation of the behavior that exists.

## Increment 1: reproducible foundation

1. Establish a pnpm and Nx workspace that can run Go application targets without duplicating the Go build system.
2. Reuse the locally available PostgreSQL 16, Go 1.27, Node 22, and sqlc images. Run NATS 2.10 with JetStream. Provide Docker Compose, environment examples, and Make commands.
3. Add separate Go API and worker processes with structured logs, graceful shutdown, and liveness/readiness checks. Verify readiness changes when dependencies are unavailable.
4. Add the first versioned PostgreSQL migration and sqlc configuration. Verify migration up and down on a clean database and generated queries.
5. Document exact setup, operations, and the limitations of this increment.

**Exit check:** A new engineer can run `make setup`, `make dev-docker`, inspect both services, run `make test` and `make build`, and shut the stack down without losing database data.

This increment is implemented. The Compose stack has been exercised with a clean named volume: migration version 1 was applied, both readiness endpoints returned HTTP 200, and the worker created the `MOTIF_EVENTS` JetStream stream.

## Increment 2: secure event receipt and processing

1. Bootstrap one platform administrator from secrets, with server-side authorization.
2. Create tenants, environments, hashed integration credentials, and event type schemas through authenticated administration operations.
3. Accept bounded and validated events with tenant and environment context derived from credentials; deduplicate by external event ID and reject a changed payload for the same ID.
4. Establish durable publication through a transactional outbox and NATS JetStream, then process and acknowledge events in the worker.
5. Expose receipt and processing status. Test concurrent duplicates, crash and retry behavior, tenant isolation, and database to broker recovery.

**Exit check:** A tenant event is durably accepted, processed, and traceable without duplicate effects after retries.

The event-processing items are implemented for the bootstrap tenant: hashed integration credential lookup, bounded event validation, tenant/environment scoped deduplication, transactional outbox publication, durable JetStream processing, receipt status persistence, and status lookup. An initial platform-admin key can create and list tenants and environments, issue integration keys once, and register a required-field event schema. Session-based Super Admin identity, versioned event schemas, and failure recovery diagnostics remain in the next slice.

## Increment 3: published rules and worker audit

1. Add versioned rule definitions with draft, published, and archived lifecycle states.
2. Validate rule scope, event type, conditions, and typed actions through authenticated admin APIs.
3. Publish rules explicitly and evaluate matching events in the worker.
4. Record idempotent rule executions and expose them to the dashboard.

**Exit check:** An administrator can draft and publish a rule, send a matching event, and see one recorded execution without duplicate effects after redelivery.

This increment is implemented. The dashboard can create and publish rules; the worker evaluates published rules by tenant, environment, and event type, and stores one `recorded` execution per rule-event pair.

## Later increments

4. Versioned rule conditions and typed action delivery, followed by an auditable point ledger with independent currencies, atomic spending, and conversion. **Implemented.**
5. Engagement and progression primitives: missions, achievements, XP levels, profile reads, and idempotent `award_xp`. **Implemented.**
6. Rewards and redemption delivery with a provider boundary, retry-safe state, admin inspection, and live dashboard surfaces. **Implemented.**
7. Dashboard hardening and rule scheduling: live rule windows in the API, worker enforcement, dashboard editing/display, OpenAPI coverage, and operator guides. **Implemented in the current increment.**
8. Operational hardening now includes strict request decoding, live diagnostics, scoped list filters, redemption retry, immutable rule-version tooling, paged operator feeds, background refresh, and live catalog administration for currencies, missions, achievements, levels, and rewards.

9. Production reward delivery now supports scope-specific HTTP provider configuration,
encrypted provider credentials, provider status polling, bounded reconciliation
backoff, and idempotent point release after a provider-confirmed failure.

The dashboard scope layer now loads tenant/environment pairs from the authenticated membership API and selects the first authorized workspace for the session. It carries that membership scope through diagnostics, event feeds, rule and event-type registries, and catalog administration without asking the user to choose a tenant. Workspace Settings exposes the current boundary and API/worker readiness without rendering credentials. Tenant Admin scope listing is filtered by memberships server-side, and platform-only controls are hidden in the Tenant Admin workspace.

## Platform foundation completion

The platform foundation now includes migration 17 with plans, tenant plan assignments,
credential metadata, and an append-only audit log. Integration credentials are hashed
at rest, can be created, rotated, and revoked, and are rejected when their tenant is
suspended. A central entitlement store merges plan limits with tenant overrides and
enforces payload, rule, currency, and reward limits at the API boundary. The API adds
request IDs, bounded per-client rate limiting, configurable CORS, structured request
logs, and conservative HTTP timeouts. Customer backends use the versioned REST
API directly for event receipt, event status, profiles, points, and redemptions.
The OpenAPI contract and direct HTTP examples document that integration path.

The five platform completion stages are now implemented: session-based admin
identity and RBAC, a configurable HTTP reward provider adapter, immutable event
schema versions with bounded JSON Schema operators, Prometheus metrics and CI
verification, and manual plan administration with usage snapshots. The public
marketing and documentation app is implemented in `apps/marketing` using
TanStack Start, and its route content is limited to capabilities that exist in
the engine today.

## Final product increment

10. Role-aware tenant dashboard and a public marketing/documentation application. Tenant Admins receive membership-scoped tenant/environment pairs and tenant configuration workflows; Super Admin controls stay out of this dashboard for the future Ops service. The public app runs independently on port 4174 and links back to the console and repository guides.

## Phase 5: measurement foundation (P2)

The first measurement increment is implemented. Tenant-scoped metric definitions
support event counts and unique-user counts, while admin reports expose campaign
analytics, ordered event funnels, cohort retention intervals, on-demand segments,
and deterministic experiment/control-group reports directly from the
canonical event history. The dashboard exposes reports from the Analyze → Reports
workspace; Settings is reserved for platform and runtime operations. Persistent
experiment lifecycle management and warehouse-backed
reporting remain later Phase 5 work.

## Decisions for this increment

- One Go module contains the API, worker, migration command, and shared internal packages. They remain separate runtime processes.
- PostgreSQL 16 is reused because an exact compatible local image exists. No extension is needed for the foundation.
- NATS 2.10 is the initial durable broker. The local Docker Hub request was denied, so the identical version was retrieved from an available mirror and tagged locally as `nats:2.10-alpine`.
- The first migration contains only tables needed by implemented behavior. Domain tables arrive with their behavior in later increments.
- Make is the documented interface; Nx orchestrates project tasks, while Go remains responsible for compiling and testing Go code.

The dashboard is implemented as live Overview, Build (Events and Automations), Loyalty (Currencies, Levels, Missions, Achievements, and Rewards), Engagement (Goals and Campaigns), Analyze (Reports), and Settings workspaces. Each resource workspace opens on a list with a focused Add flow, and missing event or currency dependencies can be created inline without leaving the current draft. It covers readiness, membership-scoped workspace health, a real test-event loop, automation publishing and schedules, worker matches, point balances and ledger entries, XP progression, achievements, rewards, campaign analytics, and redemption status. All displayed domain data comes from the local API.
