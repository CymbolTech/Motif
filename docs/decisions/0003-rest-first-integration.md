# 0003 — REST-first customer integration

**Status:** Accepted · 2026-10-14

## Context

MotifEngine already exposes a versioned HTTP API and an OpenAPI description. The only language-specific client in this repository was a private TypeScript package covering a small subset of endpoints. Maintaining official clients across languages would add a second compatibility and release surface for every API change.

## Decision

- The versioned REST API is the customer integration contract. Keep `docs/api/openapi.yaml` aligned with implemented request and response behavior.
- Support direct backend HTTP integration with practical examples, clear authentication, error, idempotency, and asynchronous-processing semantics. Customers may generate clients from OpenAPI or use their own HTTP libraries.
- Do not maintain or promise official MotifEngine language SDKs at this stage. Remove the private TypeScript SDK and its build/CI wiring.
- MCP is a possible later adapter for AI-assisted configuration and inspection. It would call the same authorized API and preserve tenant/environment scope. It is not an event-ingestion or backend-to-backend integration protocol for this product.

## Consequences

API documentation and contract accuracy become more important. Contract changes need implementation review and examples should be checked against real responses. A language SDK can be reconsidered if repeated customer integration problems justify its ongoing release and compatibility cost. Any future MCP server has its own authentication, authorization, and maintenance work and should be scoped to demonstrated workflows.
