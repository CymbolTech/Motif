# 0005 — MCP adapter for workspace configuration and inspection

**Status:** Accepted · 2026-10-16

## Context

Teams integrate MotifEngine from coding tools that speak the Model Context Protocol (Claude Code, Cursor, VS Code). Without access to the workspace, those tools can only read static documentation: they cannot see registered contracts, check a rule against a real payload, send a test event or explain why a rule did not award. ADR 0003 kept MCP as a possible later adapter, limited to configuration and inspection, behind the same authorization and workspace/environment boundaries as the API.

The admin API had no credential suited to such a client. Sessions expire after twelve hours, and the bootstrap admin key acts across every workspace.

## Decision

- Add `apps/mcp`, a Go service built on the official MCP Go SDK. It serves stateless Streamable HTTP at `/mcp` on the console origin and can run over stdio for local use.
- The service is a thin client of the admin API. Every tool call forwards the caller's token, so the API authorizes each call exactly as it would a console request. The service holds no credentials and no database access.
- Add workspace access tokens (`motif_at_…`). They are created and revoked by a signed-in administrator in the console, stored as SHA-256 digests, confined to one workspace and optionally to listed environments, and carry `read`, `write` and `publish` scopes. Publishing a rule needs `publish`, so letting a tool make rule changes live is a separate decision from letting it draft them.
- Access tokens never reach integration credentials, reward providers, point transactions, conversions, redemptions, token management or platform administration, whatever their scopes.
- Token-authenticated writes are audited as `access_token` with the token id, and with the client label `mcp` when the request comes from this service.
- Add two read-only API routes that the tools need and the console can reuse: `POST /v1/admin/rules/check` validates a rule and explains its evaluation against a sample payload without saving, and `GET /v1/admin/events/{event_id}/trace` explains what one event did.
- Test events sent through MCP use the existing console test-event route. Customer backends keep sending real events over REST with their integration key.

## Consequences

The API now has a third admin credential type, and every admin route is subject to scope and environment checks when a token calls it. A revoked token stops working on the next API call; the MCP service caches token lookups for up to a minute, which affects only which tools are listed. The rate limiter still keys clients by connecting address, so all MCP traffic shares one bucket until limits are keyed by credential. OAuth for web-based MCP clients, analytics tools and replaying draft rules over stored events are left for later.
