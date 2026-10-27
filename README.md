# MotifEngine

MotifEngine is an API-first gamification, engagement, and loyalty engine. The repository is being built incrementally from the product specification in [`Docs/initial-implementation.md`](Docs/initial-implementation.md).

## Current implementation

The first increment provides:

- A pnpm and Nx workspace with Go API, worker, and migration projects.
- PostgreSQL 16 and NATS JetStream local services.
- Separate API and worker processes with JSON logs and live/readiness endpoints.
- A versioned foundation migration and repeatable local Make commands.
- A checked-in Go vendor tree so container builds do not depend on downloading modules during the image build.
- A complete event vertical slice: access-key-authenticated receipt, tenant/environment scoped deduplication, transactional outbox, JetStream publication, and worker processing.
- Versioned rules with restricted conditions, typed actions, idempotent execution traces, retries, and optional UTC schedule windows.
- Independent point currencies with an append-only ledger, expiring point lots, atomic spending, and currency conversion.
- Missions, achievements, XP levels, progression profiles, digital rewards, mock-provider redemption, and admin redemption inspection.
- Operational diagnostics for event queues, rule execution failures, and redemption recovery.
- Tenant status enforcement, hashed, scoped access keys with revocation, plan entitlements, and an append-only admin audit stream.
- Session-based Super Admin and tenant-admin RBAC with expiring hashed sessions; the bootstrap admin key remains a local migration fallback.
- Immutable event schema versions with bounded JSON Schema operators and the version recorded on each accepted event.
- Mock and configurable HTTP reward providers with idempotency headers and unknown-outcome handling.
- Request IDs, configurable CORS, bounded rate limiting, structured API request logs, Prometheus metrics at `/metrics`, manual plan administration, and usage snapshots.
- A versioned REST API and [OpenAPI contract](docs/api/openapi.yaml) for server-side integrations, with a [direct integration guide](docs/guides/rest-integration.md).
- A public Next.js marketing and documentation application in `apps/marketing`: the Motif landing page, statically rendered with search metadata, structured data, a sitemap and robots rules, plus the product manual at `/docs`.
- An MCP server in `apps/mcp` that lets MCP clients configure and inspect a workspace with a revocable access key that has the `mcp` scope: event contracts, catalog, rule checks and drafts, publishing, test events with traces, and integration snippets. See [ADR 0005](docs/decisions/0005-mcp-adapter.md).

The tenant dashboard is available at `http://127.0.0.1:4173` when the Vite dev server is running. A user can create a workspace account from the sign-in screen and then manage events, automations, loyalty resources, analytics, and test events inside that membership-scoped workspace. Super Admin accounts are reserved for the future Ops service and are intentionally blocked from this dashboard.

The public marketing and documentation application is available at `http://127.0.0.1:4174` when `make marketing` is running. The landing page follows the direction recorded in [`docs/decisions/0006-public-site-direction.md`](docs/decisions/0006-public-site-direction.md) and describes only capabilities the engine has today.

## Prerequisites

Go 1.27, Node.js 20+, pnpm 12, Docker, Docker Compose, and Make.

## Local setup

```sh
make setup
make doctor
make infra-up
make migrate-up
make build
make test
```

In a second terminal, run the dashboard with:

```sh
make dashboard
```

Then open `http://127.0.0.1:4173`. The dashboard proxies `/api` to the local API on port `18080` and `/worker` to the worker health endpoint on port `18081`.
Its Docs link opens the local marketing documentation on port `4174`; set
`VITE_DOCS_URL` when hosting the documentation at another address.

To run the public site in another terminal:

```sh
make marketing
```

Open `http://127.0.0.1:4174` for the landing page or `/docs` for the product manual.

For the full containerized stack:

```sh
make up
make health
```

The Docker workflow serves the console at `http://127.0.0.1:4173` and the public site and docs at `http://127.0.0.1:4174`. The dashboard container proxies `/api` and `/worker` to the private services, and `/mcp` to the MCP server (also on `http://127.0.0.1:18082/mcp`). Create an access key with the `mcp` scope under **Workspace → Access keys** in the console to connect an MCP client. API liveness is also available at `http://127.0.0.1:18080/health/live` and worker readiness at `http://127.0.0.1:18081/health/ready`.

For a VPS deployment behind Nginx Proxy Manager, see [`deploy/README.md`](deploy/README.md).

The event API example and its delivery semantics are documented in [`Docs/api/events.md`](Docs/api/events.md).

Customer backends integrate through REST using a tenant/environment-scoped key. See the [REST integration guide](docs/guides/rest-integration.md) for direct HTTP examples, retry and idempotency rules, and the role of the OpenAPI contract. MotifEngine does not maintain an official language SDK.

The administration endpoints are documented in [`Docs/api/admin.md`](Docs/api/admin.md). Rule schedules are described in [`docs/guides/rules.md`](docs/guides/rules.md).

The scenario cookbook explains how to compose the implemented primitives for
commerce, SaaS, learning, content, community, gaming, loyalty, measurement,
and platform operations: [`docs/guides/use-cases.md`](docs/guides/use-cases.md).

The local CI-equivalent command is `make ci`; it runs Go unit tests, disposable
PostgreSQL and NATS integration tests, Go vet/build, and the dashboard and marketing
builds. Run `make test-integration` for the integration suite alone; it
starts temporary containers and does not use the development database or stream.

The integration suite verifies migration rollback and reapplication, concurrent
point spending and idempotency, conversion lot accounting and repair of legacy
conversions, converted-point redemption, definitive and unknown provider outcomes,
and the event outbox through JetStream processing, duplicate delivery, and
recovery of a pending outbox item after a worker restart. Automated browser
workflows still need dedicated tests.

`make clean` preserves database and NATS volumes. Use `make db-reset` only when local data should be removed.
