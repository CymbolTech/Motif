# MotifEngine — Complete Product, Architecture & Implementation Specification

**Role:** Principal Software Architect, Senior Full-Stack Engineer, Product Engineer, and Technical Owner  
**Model:** GPT SOL 6 Extra High  
**Project:** MotifEngine  
**Execution mode:** Repository-aware, autonomous implementation with incremental verification  
**Primary objective:** Deliver a coherent, secure, maintainable, production-oriented product with working backend functionality, polished frontend applications, complete documentation, and reproducible local development.

---

# 1. Mission and Definition of Done

Build **MotifEngine**, a general-purpose Gamification, Engagement, and Loyalty Engine that digital products can integrate through its versioned REST API.

MotifEngine must enable customers to define events, configure rules, run campaigns, manage user progression, award and spend points, create achievements and missions, configure rewards, and understand how these capabilities work through a clear administrative experience.

This is a long-term SaaS product. Architectural decisions must support future growth without introducing complexity that the current requirements do not justify.

The complete system must live in **one Nx monorepo** and be operable through **Docker and Make**.

The result must include:

- A public marketing website introducing the product and all its capabilities.
- A comprehensive product documentation section with practical examples.
- A React-based administrative dashboard for platform administrators and tenant administrators.
- A Go backend API.
- A Go background worker for asynchronous processing.
- PostgreSQL persistence, schema migrations, and generated sqlc queries.
- NATS JetStream for durable asynchronous messaging.
- Tenant management, environments, API credentials, plans, entitlements, and limits.
- Event ingestion, validation, rule evaluation, and idempotent action execution.
- A point ledger, independently configurable point currencies, conversions, expiration policies, and rewards foundations.
- Automated tests, security controls, operational logging, and health checks.
- A consistent developer experience using Make targets.
- Clear setup instructions and maintainable technical documentation.

Do not consider a feature complete merely because a route, screen, API endpoint, or database table exists. Its behavior must be implemented, validated, tested where appropriate, and documented.

**Do not overengineer.** Prefer the simplest architecture that satisfies the actual domain requirements reliably.

---

# 2. Mandatory Repository Inspection

Before changing files, inspect the existing repository and understand what is already present.

1. Read all applicable `AGENTS.md` files and repository-specific instructions.
2. Inspect Nx configuration, projects, dependency graph, package manager, scripts, and existing conventions.
3. Inspect the Go workspace and modules, if present.
4. Inspect existing Dockerfiles, Compose files, Makefiles, environment files, migrations, and CI configuration.
5. Identify existing applications and reusable libraries.
6. Inspect installed skills and development tools.
7. Determine which features are implemented, partially implemented, missing, or broken.
8. Identify existing Docker images that can be reused locally.
9. Create or update a concise implementation plan.
10. Record important architectural decisions in the repository.

Do not overwrite working implementations unnecessarily. Do not introduce duplicate applications, libraries, infrastructure, or documentation when suitable equivalents already exist.

Do not assume an empty repository.

Do not stop after inspection or planning. Continue with implementation.

When uncertain, choose the simplest reasonable option consistent with this specification, document significant decisions, and continue. Ask the user only when an unresolved decision materially affects security, correctness, or product direction.

---

# 3. Product Vision and Boundaries

## 3.1 Product positioning

MotifEngine is a general-purpose platform for building gamified, engaging, and loyalty-driven experiences in digital products.

It is not restricted to e-commerce or traditional loyalty programs.

It should support:

- E-commerce and commerce platforms.
- SaaS and subscription products.
- Education and learning platforms.
- Social networks and online communities.
- Content and media applications.
- Other products with configurable user behavior and engagement requirements.

## 3.2 Integration model

MotifEngine is an **API-first backend platform**. Its versioned REST API and OpenAPI description are the public integration contract.

Customers build their own end-user interfaces. MotifEngine does not need to ship mandatory customer-facing widgets, badges, leaderboards, or UI components in its initial product scope.

The customer integrates MotifEngine into their own backend or trusted server-side application using authenticated REST requests. Runnable examples help teams use their preferred HTTP client; official language SDKs are not part of the current product commitment. An optional MCP adapter may be considered later for AI-assisted administration, but it is not the runtime integration path.

The MotifEngine dashboard is an administrative interface for configuring and operating the platform.

Never confuse:

- The customer's end user with a MotifEngine dashboard user.
- A customer's external user ID with a MotifEngine account ID.
- XP with spendable loyalty points.
- A reward definition with the points used to redeem it.
- A rule with a campaign or a business goal.
- Technical event-processing metrics with business-outcome measurements.

## 3.3 Product principles

- General-purpose domain model.
- API-first integration.
- Reliable asynchronous event processing.
- Explicit tenant and environment boundaries.
- Versioned configuration where historical reproducibility matters.
- Auditable point and reward operations.
- Clear, approachable configuration workflows.
- Predictable and documented APIs.
- Minimal unnecessary abstractions.
- Strong tests around concurrency, idempotency, security, and business invariants.

---

# 4. Mandatory Technology Stack

Use the following technologies unless the existing repository already contains a compatible, justified implementation.

## 4.1 Monorepo

- Nx as the monorepo orchestration system.
- One repository containing all first-party applications, backend code, shared libraries, configuration, infrastructure, and documentation.
- Use the existing package manager and lockfile if the repository is already configured.
- Do not create separate repositories for the marketing website, dashboard, API, or worker.

A possible structure is:

```text
motifengine/
├── apps/
│   ├── marketing/             # TanStack Start
│   ├── dashboard/             # Vite + React
│   ├── api/                   # Go HTTP API
│   └── worker/                # Go background worker
├── libs/
│   ├── frontend/
│   │   ├── shared/
│   │   ├── entities/
│   │   ├── features/
│   │   └── widgets/
│   ├── contracts/             # Shared API schemas/types as appropriate
│   └── config/                # Shared tooling configuration where useful
├── docs/
│   ├── product/
│   ├── architecture/
│   ├── api/
│   ├── guides/
│   ├── examples/
│   ├── operations/
│   └── decisions/
├── deploy/
│   └── docker/
├── tools/
├── docker-compose.yml
├── Makefile
├── nx.json
├── package.json
├── pnpm-workspace.yaml
├── go.work                    # Only if a workspace is genuinely useful
├── .env.example
└── README.md
```

This is a suggested organization, not a mandate to create every directory. Adapt it to the repository and actual dependency boundaries.

Avoid unnecessary nesting, artificial libraries, and abstractions introduced only to match this example.

## 4.2 Frontend

Marketing website:
- TanStack Start.
- React.
- TypeScript.
- Tailwind CSS.
- shadcn/ui.

Administrative dashboard:
- Vite.
- React.
- TypeScript.
- Tailwind CSS.
- shadcn/ui.
- Feature-Sliced Design (FSD).

UI foundation must use **only Tailwind CSS and shadcn/ui**.

Do not introduce MUI, Ant Design, Mantine, Chakra UI, Bootstrap, or another competing component/styling framework.

You may customize shadcn/ui extensively or implement new reusable components when necessary.

Specialized supporting libraries for routing, forms, validation, data fetching, charts, tables, dates, and icons are allowed when they solve a real problem. Avoid redundant dependencies.

## 4.3 Backend

- Go.
- PostgreSQL.
- sqlc for type-safe database access.
- Versioned SQL migrations using an established migration tool such as golang-migrate.
- NATS JetStream for durable asynchronous messaging.
- REST APIs.
- OpenAPI for API contracts.
- Separate API and worker processes.
- Structured logging.
- Explicit configuration through environment variables.

Use a **Modular Monolith**, not a collection of microservices.

The API and worker may be separate executables or deployment processes while sharing the same domain modules and business logic.

Do not use an ORM as a replacement for sqlc.

## 4.4 Infrastructure

- Docker and Docker Compose.
- Make as the primary developer-facing command interface.
- PostgreSQL as the business database.
- NATS JetStream as the durable messaging system.
- Health checks and graceful shutdown.
- Reproducible database migrations.
- Environment-driven configuration.

The entire supported local development workflow must be accessible through documented Make targets.

---

# 5. Docker Image Reuse and Local Development

Docker usage is mandatory, but unnecessary downloads must be avoided.

## 5.1 Inspect before pulling

Before running `docker pull`, or triggering a Compose operation that downloads images, inspect the local Docker environment.

For every required third-party image:

1. Identify the exact repository and tag required by the configuration.
2. Check whether that exact image reference is already available locally.
3. Inspect available local images and compatible existing service configurations.
4. Reuse an existing image when its repository, tag, architecture, and compatibility meet the requirement.
5. Pull an image only when the required image is genuinely unavailable locally or a justified compatibility/security requirement requires a different image.
6. Never pull a newer major version merely because it exists.
7. Do not run broad image cleanup, prune commands, or remove unrelated local images.

For example, if the existing environment already has a compatible PostgreSQL 16 image, use it rather than pulling PostgreSQL 17 or 18.

Do not assume an image with a similar name or a different major version is compatible. Inspect it first.

Useful inspection commands include:

```bash
docker image ls
docker image inspect postgres:16-alpine
docker compose config
```

The commands above are examples. Adapt the checks to the actual environment and configured image references.

If the exact image is absent, pull the configured, compatible image.

If an existing local image has the correct tag but cannot satisfy the required architecture or configuration, explain the issue and take the smallest necessary corrective action.

## 5.2 Docker Compose

Provide a working Compose configuration for local development.

At minimum, the local infrastructure should include:

- PostgreSQL.
- NATS with JetStream enabled.
- MotifEngine API.
- MotifEngine worker.
- Marketing and dashboard applications when the selected development workflow requires them to run in containers.

It is acceptable to use local host processes for frontend hot reloading during development, provided the entire workflow is available through Make and the documented Docker workflow works as intended.

Provide production-oriented Dockerfiles for deployable applications. Use multi-stage builds where appropriate, minimal runtime images, non-root users when feasible, and sensible `.dockerignore` files.

Do not bundle build tooling, compilers, source files, or unnecessary utilities into production images.

Configure persistent volumes for stateful infrastructure. Do not persist application build artifacts or logs unnecessarily.

Use health checks, dependency readiness checks, restart policies appropriate to the environment, and graceful shutdown.

Do not assume that `depends_on` alone guarantees application-level readiness.

Never hard-code production credentials or commit real secrets.

## 5.3 Make is the supported interface

Provide a root `Makefile` with a useful `help` target.

The developer should not need to remember a long sequence of Docker, Go, pnpm, migration, or Nx commands.

Provide appropriate targets such as:

```text
make help
make setup
make doctor
make env-check
make dev
make dev-docker
make infra-up
make infra-down
make build
make lint
make typecheck
make test
make test-unit
make test-integration
make test-e2e
make migrate-up
make migrate-down
make migrate-status
make sqlc-generate
make openapi-generate
make docs-check
make logs
make status
make clean
```

These names are examples. Implement targets only when they have a real, well-defined behavior.

`make dev` should start the recommended local development workflow. `make dev-docker` should provide a documented containerized development alternative.

`make setup` must be safe to run repeatedly. It should verify prerequisites, install dependencies using the existing lockfile, prepare local configuration where appropriate, and avoid overwriting user data.

`make doctor` should report missing prerequisites, configuration problems, unavailable services, and incompatible tool versions.

`make clean` must not delete databases, user files, unrelated Docker images, or persistent data unless a separate explicitly destructive target is invoked.

If a destructive reset is useful, make it opt-in and name it clearly, for example `make db-reset`.

Use Nx targets for frontend and monorepo orchestration where appropriate. Make should provide a stable, memorable entry point rather than duplicating every build system's internal functionality.

Ensure that commands propagate failures correctly. A successful Make command must not conceal a failed build, migration, test, or service startup.

Document every important target in the root README.

---

# 6. Nx Workspace Architecture

All first-party code belongs to the same Nx workspace.

The repository must have clear boundaries between applications and reusable modules.

Use Nx project configuration, project tags, and dependency constraints where they materially improve maintainability.

For example, enforce rules that prevent:

- Shared utilities from importing application-specific code.
- Domain modules from importing presentation components.
- Lower-level libraries from depending on higher-level features.
- Cross-workspace imports that bypass public entry points.
- Circular dependencies between frontend libraries.

Do not create a library for every trivial function. Extract a library when it represents a meaningful reusable domain, architectural boundary, or shared contract.

Use Nx caching and affected-project workflows when supported by the actual workspace.

The workspace must remain understandable to an engineer who did not create it.

---

# 7. Frontend Architecture: Feature-Sliced Design

The dashboard must follow **Feature-Sliced Design (FSD)** consistently.

Do not merely create folders named `entities`, `features`, and `widgets` while allowing unrestricted imports.

## 7.1 FSD layers

Use the following layers where appropriate:

1. `app` — application initialization, providers, global configuration, routing, and styles.
2. `pages` — route-level composition.
3. `widgets` — substantial page sections composed from lower-level modules.
4. `features` — user actions and business interactions.
5. `entities` — domain entities and their presentation, models, and API integration where appropriate.
6. `shared` — generic UI, utilities, configuration, API client infrastructure, and non-domain-specific helpers.

Follow the FSD dependency direction:

```text
app
 ↓
pages
 ↓
widgets
 ↓
features
 ↓
entities
 ↓
shared
```

An upper layer may import lower layers. Lower layers must not import upper layers.

Same-layer cross-slice imports should be avoided. When a genuine dependency exists, expose an intentional public API and follow FSD rules rather than importing arbitrary internal files.

`shared` must not depend on product-specific entities, features, or pages.

Do not create empty slices merely to make the structure look compliant.

## 7.2 Feature boundaries

Examples of potential dashboard domains:

- Tenant management.
- Environments and credentials.
- Plans and entitlements.
- Events and event schemas.
- Rules and rule versions.
- Campaigns.
- Missions.
- Achievements.
- Progression and levels.
- Point currencies and policies.
- Point accounts and ledger history.
- Point conversion.
- Rewards and redemption.
- Audit logs.
- Platform settings.

Organize these as the product grows. Avoid creating all possible files or abstractions before a feature needs them.

## 7.3 Routing and workspaces

Use one dashboard application with separate route namespaces and layouts:

- `/platform/*` — future Ops service for Super Admin operations; it is not part of the tenant dashboard.
- `/app/*` — Tenant Admin workspace.

These are distinct authorization contexts, not merely visual themes.

The backend must enforce the permissions behind every protected operation. Frontend route guards improve UX but are not security boundaries.

Tenant-specific pages must derive tenant context from a trusted authenticated session and authorized server-side context. Never trust a client-provided `tenant_id` as proof of access.

## 7.4 TypeScript and coding standards

Use strict TypeScript wherever supported by the chosen framework.

Avoid `any` unless a narrowly scoped interoperability requirement makes it unavoidable. If used, document the reason and contain it at the boundary.

Prefer explicit, meaningful types for domain concepts, API responses, state transitions, and user interactions.

Use **Airbnb's JavaScript/TypeScript style conventions as a baseline**, adapted to the actual repository, React, TypeScript, accessibility, and maintainability requirements.

Do not blindly import every Airbnb rule if it conflicts with modern React or TypeScript practices. Define and enforce a coherent ESLint and formatting configuration.

Standards should include:

- Consistent naming and import ordering.
- Predictable file and export conventions.
- Explicit component props.
- Small, cohesive components.
- Clear separation of UI and business logic.
- Stable React keys.
- Correct hook usage.
- No unnecessary effects.
- No mutation of shared state.
- No unexplained lint suppressions.
- No unused code or imports.
- No circular dependencies.
- No unbounded type assertions.
- No arbitrary cross-layer imports.
- No duplicated API contract definitions when generation or a shared contract is practical.

Avoid creating giant components, giant hooks, or files containing unrelated domain logic.

Split code based on cohesion and responsibility, not an arbitrary line-count threshold.

Do not turn every simple expression into a helper or every UI element into a separate component.

## 7.5 State and data fetching

Use server state and client state appropriately.

Prefer a consistent, established data-fetching approach instead of hand-rolled caching and request lifecycle management throughout the app.

Do not introduce a global state library unless there is a real shared-state requirement.

Treat backend responses as untrusted runtime data. Validate at relevant boundaries, particularly for user input, external API responses, and configuration payloads.

Use explicit loading, error, empty, and success states.

Do not use fake local data as though it were persisted backend data.

## 7.6 Forms and interactions

Every visible action must have a meaningful behavior.

- Submit buttons must submit.
- Delete actions must invoke the correct operation and confirm destructive changes.
- Save buttons must persist changes.
- Filters and pagination must affect the displayed data.
- Search must actually search.
- Copy actions must copy the intended value and provide feedback.
- Test or preview controls must perform the advertised operation.
- Disabled actions must have a clear reason.
- Loading and failure states must be understandable.

Do not ship placeholder buttons, fabricated metrics, random chart values, or success notifications without a successful operation.

---

# 8. UX and Visual Design

Use the **Impeccable** skill if it is available. Read its instructions before applying its workflow. Do not claim that the skill was used if it was unavailable.

UX is a core product requirement, not a final styling step.

## 8.1 Design direction

Build a polished, trustworthy B2B SaaS product.

Prioritize:

- Clear information hierarchy.
- Excellent typography and spacing.
- Consistent design tokens.
- Readable tables and forms.
- Meaningful visual feedback.
- Responsive layouts.
- Keyboard accessibility.
- Accessible contrast and focus states.
- Consistent loading and error handling.
- Simple navigation.
- Contextual explanations for domain-specific concepts.

Use Tailwind CSS and shadcn/ui as the only foundational styling and component systems.

Customize components to fit MotifEngine. Do not treat the default appearance of shadcn/ui as the finished design.

Avoid generic AI-generated dashboard aesthetics, excessive gradients, glow effects, glassmorphism, unnecessary nested cards, too many badges, decorative charts, meaningless icons, and tiny low-contrast text.

Use charts only when they communicate meaningful information.

### Landing page inspiration reference

Use [Atomist — Premium UI Design for Modern SaaS](https://atomist-nextjs-shadcn.vercel.app/) as a visual reference for the MotifEngine landing page direction. The reference is useful for its structured SaaS narrative, editorial typography, section rhythm, feature storytelling, social proof, pricing, resources, FAQ, and closing conversion path.

This is an inspiration source and not a request to copy its content, branding, claims, assets, or implementation. The MotifEngine marketing page now adapts the underlying presentation principles to the product's actual capabilities and truthful product state. The original reference remains a design decision, not a content or implementation dependency.

## 8.2 Make complex concepts approachable

Many users configuring MotifEngine will not be familiar with event-processing systems.

Explain concepts through the product interface.

For example:

- **Event:** Something that happened in the customer's application.
- **Rule:** Conditions that determine when an action should happen.
- **Action:** The effect produced when a rule matches.
- **Campaign:** A coordinated engagement activity, often limited by time.
- **Mission:** A goal or set of steps a user can complete.
- **Achievement:** Recognition granted when defined conditions are satisfied.
- **Point currency:** A defined type of points with its own policies.
- **Reward:** A benefit that a user can receive or redeem.
- **XP:** Progression experience, not automatically spendable points.

Use concise descriptions, realistic examples, contextual help, sensible defaults, and guided configuration.

Prefer goal-oriented workflows to exposing every low-level technical option immediately.

For advanced configurations, use progressive disclosure rather than hiding important behavior or forcing beginners to understand the entire engine.

## 8.3 UX acceptance criteria

Each major feature should provide:

- A clear entry point.
- A comprehensible empty state.
- A useful primary action.
- Form validation and actionable error messages.
- Loading, success, and failure states.
- Appropriate confirmation for destructive actions.
- Accessible controls and keyboard navigation.
- Contextual help when the concept is not self-explanatory.
- Responsive behavior.

Never fabricate production activity, customer counts, rewards, revenue, or event throughput.

A demo or seeded development dataset is acceptable only when it is clearly identified as sample data.

---

# 9. Marketing Website and Product Documentation

The marketing website is part of the product, not a placeholder.

Use TanStack Start, React, TypeScript, Tailwind CSS, and shadcn/ui.

The website must introduce MotifEngine's actual capabilities directly and comprehensively.

Do not hide most of the product behind a generic hero section with vague claims.

## 9.1 Required marketing content

Create a coherent public website with pages or sections covering:

1. Home and product positioning.
2. Product capabilities.
3. Event ingestion and event schemas.
4. Rule engine and actions.
5. Campaigns and scheduling.
6. Missions, achievements, and progression.
7. Point currencies, earning, spending, expiration, and conversion.
8. Rewards, redemption, and provider integrations.
9. Multi-tenant architecture, environments, and credentials.
10. REST API integration and the OpenAPI contract.
11. Analytics and measurement, distinguishing current features from planned capabilities.
12. Security, reliability, and operational design.
13. Use cases by industry.
14. Documentation and getting started.
15. Pricing only when real pricing has been decided.
16. A clear path to explore documentation or access the product.

Use concise, specific explanations. Show how the capabilities work together, not just isolated feature names.

Include practical scenarios such as:

- Awarding points for a second purchase.
- Giving XP for completing a lesson.
- Unlocking an achievement after several qualifying events.
- Converting loyalty points into another point currency.
- Redeeming points for a coupon.
- Expiring old points without corrupting account history.

Explain the integration flow from a customer's backend to MotifEngine and back to the customer's application.

Never invent customer logos, testimonials, usage figures, performance guarantees, certifications, pricing, or existing integrations.

## 9.2 Complete documentation is mandatory

Create a dedicated documentation section explaining all implemented product features and important domain concepts.

Documentation must be useful to both product users and developers.

Include:

- Product overview and terminology.
- Quick start and local setup.
- Tenant and environment concepts.
- API credential setup and safe handling.
- Event ingestion and event schemas.
- Rule definitions, operators, versions, and action execution.
- Campaign configuration and scheduling.
- Missions, achievements, XP, levels, and progression.
- Point currencies and policy versions.
- Point earning, spending, reservation, expiration, and conversion.
- Reward catalog, redemption lifecycle, and provider behavior.
- Idempotency and retry behavior.
- Authentication and authorization.
- API reference generated or validated against OpenAPI.
- Direct HTTP integration examples for common backend languages.
- Error responses and troubleshooting.
- Deployment and operations.
- Configuration reference.
- Architecture decisions and important limitations.

Every major feature should include:

1. What it is.
2. Why it exists.
3. When to use it.
4. How to configure it.
5. A realistic example.
6. The expected result.
7. Relevant API requests and responses when applicable.
8. Important constraints, errors, and security considerations.

For example, a rule guide should show a complete event, a condition, an action, and the resulting ledger entry. A currency guide should explain earning, spending, expiration, and conversion independently.

Examples must match the actual implementation. Do not publish API fields or capabilities that do not exist.

Use the repository's existing documentation tooling if suitable. Otherwise, choose a simple documentation approach compatible with the marketing application.

Keep documentation versioned with the code. Update it whenever relevant API contracts, domain behavior, configuration, or UX flows change.

The marketing website and documentation must never claim that a planned feature is already implemented.

---

# 10. Backend Architecture

Use a Go Modular Monolith with clear domain boundaries.

The API and worker are separate runtime processes when appropriate, but share the same domain and application modules.

A possible internal structure is:

```text
apps/api/
apps/worker/
libs/go/
  platform/
  identity/
  tenancy/
  events/
  rules/
  execution/
  progression/
  points/
  rewards/
  campaigns/
  observability/
```

Adapt this to the actual Go module organization. Go package conventions and a clean dependency graph take priority over matching a frontend-style directory structure.

## 10.1 Layer responsibilities

Separate responsibilities where doing so improves clarity:

- HTTP handlers: HTTP parsing, response mapping, and transport-level concerns.
- Application services: use-case orchestration and transaction boundaries.
- Domain logic: business invariants, policy evaluation, and state transitions.
- Repositories: persistence interfaces or focused data access abstractions.
- sqlc-generated code: database queries and generated models.
- Infrastructure adapters: PostgreSQL, NATS, external reward providers, and other I/O.

Avoid unnecessary interfaces and dependency-injection frameworks.

Do not create an interface for every concrete type by default. Introduce interfaces at genuine boundaries, especially where they enable testing or interchangeable external adapters.

Keep transaction boundaries explicit.

Do not put business rules in HTTP handlers, generated SQL code, or frontend components.

## 10.2 Domain modules

The principal domain areas are:

- Platform and tenant management.
- Identity and authorization.
- Environments and API credentials.
- Plans, entitlements, and limits.
- Event ingestion and validation.
- Rule definitions and versioning.
- Event processing and action execution.
- Campaigns and scheduling.
- Missions and progression.
- Point currencies and ledger.
- Reward definitions and redemption.
- Audit logs and observability.

Do not implement every possible module at full depth before validating the core end-to-end workflow.

Use stable module boundaries that allow additional capabilities without premature microservices.

---

# 11. Domain Model and Data Integrity

Design the domain model before creating the complete schema.

Use PostgreSQL as the authoritative business database.

Use migrations for schema evolution and sqlc for typed queries.

Add appropriate indexes, unique constraints, foreign keys, check constraints, and transactional guarantees.

Avoid excessive JSON storage for fields that require strong relational constraints or frequent querying. JSON is appropriate for validated event payloads and versioned configuration documents when the shape genuinely needs flexibility.

## 11.1 Tenancy and environments

A tenant represents a customer organization.

A tenant may have multiple environments, such as sandbox and production.

Every tenant-owned record must have a clear ownership boundary. Use tenant IDs, composite constraints, and composite foreign keys where they materially prevent cross-tenant references.

Where environment-specific data is involved, include the environment in the relevant identity and uniqueness constraints.

Do not rely solely on frontend filtering for tenant isolation.

Consider PostgreSQL row-level security as defense in depth if it can be implemented and tested without disproportionate complexity. It is not a substitute for application-level authorization.

## 11.2 Platform administration

The platform should support:

- Tenant creation and suspension.
- Plan assignment.
- Feature entitlements.
- Tenant-specific overrides.
- Configurable limits.
- Environment management.
- API credential management and revocation.
- Audit history.

The initial release has exactly one Super Admin account configured through environment variables or secrets.

The tenant dashboard provides self-serve signup: a user creates a workspace account, which provisions one tenant, its first environment, a starter plan, an integration credential, and a tenant-admin membership. Super Admin signup remains private and is reserved for the future Ops service.

The data model allows multiple users and roles per tenant. Self-serve signup creates the first tenant administrator; additional membership management remains an Ops or workspace-invitation concern.

Separate platform administrator identity and authorization from tenant administrator authorization. Do not grant Super Admin access merely because a user has a tenant membership.

Use secure password hashing if passwords are implemented, secure session handling, and a documented secret rotation procedure.

Never store raw API credentials in plaintext. Show a newly generated credential only when necessary, store an appropriate cryptographic hash, and support revocation.

## 11.3 Entitlements

Use the model:

**Plan + Tenant-specific Override + Limits**

Plans describe default capabilities. Tenant-specific overrides adjust them. Limits govern quantities such as environments, API usage, or feature-specific capacity.

Keep entitlement evaluation centralized in a server-side module.

Do not scatter plan checks throughout unrelated handlers.

Manual billing is sufficient initially. The schema should not prevent future subscription and billing automation, but do not implement a full billing platform without a real requirement.

## 11.4 Events

Events represent occurrences in a customer's product.

The customer defines event types and payload schemas. MotifEngine must not hard-code `PurchaseCompleted` or any other business event as a special engine primitive.

An event should support:

- Tenant and environment context.
- External event identifier.
- Event type.
- Event timestamp.
- Validated payload.
- Receipt and processing status.
- Traceable processing attempts.

Use a unique constraint such as:

`(tenant_id, environment_id, external_event_id)`

when the API contract requires a client-supplied event identifier.

Clearly define what happens when the same ID is received with a different payload. Do not silently treat materially different events as identical without a documented policy.

Validate event types and payloads. Apply payload size limits, reasonable schema complexity limits, rate limits, and PII minimization.

## 11.5 Rules, actions, and versioning

Rules define when conditions match.

Actions define the effects produced by matching rules.

Store rule definitions separately from execution records.

Published versions must be immutable or otherwise historically reproducible. Updating a published rule should create a new version rather than rewriting the definition used by previous executions.

Represent rule definitions using a restricted, validated structure such as a JSON AST.

Do not execute arbitrary customer-provided JavaScript, SQL, shell commands, or unrestricted code.

Bound the complexity of expressions, supported operators, payload access, nesting depth, and action counts.

The engine should make execution outcomes traceable, including successful actions, skipped actions, validation failures, and retryable errors.

## 11.6 Campaigns and goals

A goal describes a desired business outcome.

A campaign coordinates an engagement activity, often within a defined time period.

A rule defines conditions and actions.

These concepts must not be conflated.

Example:

- Goal: increase repeat purchases.
- Campaign: encourage customers to return this month.
- Rule: when a qualifying second purchase occurs within the configured period, award 100 points.
- Action: append a points ledger entry.
- Measurement: compare repeat-purchase performance with a baseline or control group when measurement capabilities exist.

Do not implement sophisticated experimentation merely to support this example in the first release.

## 11.7 Progression

Model missions, achievements, levels, XP, streaks, and progress as distinct concepts where their semantics differ.

Do not collapse all progression behavior into a single generic table that cannot preserve important business invariants.

At the same time, do not create separate services or extensive generic frameworks for every progression type.

Implement the initial concepts through a coherent shared execution model.

## 11.8 Point currencies

Point currencies are independent, tenant-owned internal point systems.

Examples include:

- Loyalty points.
- Experience XP.
- Campaign tokens.

A currency can have its own policies for:

- Earning.
- Spending.
- Expiration.
- Conversion eligibility.
- Precision or unit scale.
- Account status.
- Version history.

XP is not automatically spendable. A point currency is not real fiat currency.

Model currency policy versions so policy changes do not silently rewrite historical operations.

## 11.9 Point ledger

The ledger is the source of truth for point changes.

A balance table may be maintained as a projection, but it must be rebuildable and reconcilable from ledger data.

Every ledger operation must have clear semantics, provenance, and idempotency.

Requirements:

- No negative balances by default.
- Atomic spending and reservation.
- Concurrency-safe balance checks.
- No double spending under concurrent requests.
- Compensating entries rather than deleting historical ledger entries.
- Idempotent expiration and retry behavior.
- Explicit handling of reserved versus available points if reservations are supported.
- Defined integer or fixed-point precision. Never use floating-point arithmetic for authoritative balances.

For independently expiring earned points, model point lots with individual expiry dates and allocations.

Use earliest-expiring-first allocation by default when that matches the currency policy.

An expiration operation must record the actual debit or compensating entry and must not delete the original award.

## 11.10 Point conversion

Conversion is a domain operation, not merely a generic rule action.

A conversion must explicitly identify the source and target currencies, rate or conversion policy version, and the operation being performed.

For conversions within the same PostgreSQL database, perform the source debit and target credit atomically in one database transaction.

Record both sides of the operation and link them to a unique conversion transaction.

Do not use NATS to implement atomic transfers between two internal point accounts.

Never allow implicit cross-tenant conversion.

Define rounding, precision, directionality, insufficient balance behavior, and idempotency explicitly.

## 11.11 Rewards

Reward definitions are independent of point currencies.

Reward types may include:

- Digital unlocks.
- Badges or recognition.
- In-app benefits.
- Coupons.
- Discounts.
- Cashback-like benefits.
- External rewards delivered through providers.

Do not assume every reward costs points or that every point currency can redeem every reward.

Model reward definitions and their versions, eligibility, redemption records, payment or point consumption where relevant, and delivery attempts.

A redemption lifecycle should support meaningful states such as:

`Requested → Reserved → Delivering → Fulfilled`

Failure and cancellation flows must be explicit.

When an external provider times out and the outcome is unknown, do not assume the reward was not delivered. Reconcile or retry through an idempotent provider operation where possible.

Support safe compensation or release of reserved points when a redemption definitively fails.

Initially, implement the provider abstraction and one limited implementation, such as a mock provider or one justified real integration. Do not build an extensive marketplace of integrations before there is a concrete requirement.

---

# 12. Event Processing and Messaging

Use NATS JetStream for durable asynchronous messaging.

PostgreSQL is the business database, not the primary queue.

Expected initial event throughput is approximately 10–1,000 events per second. This is a design and benchmarking target, not a performance guarantee.

## 12.1 Processing pipeline

Implement a logical pipeline:

1. Authenticate the event producer.
2. Resolve tenant and environment from trusted credentials.
3. Validate the event identifier, type, schema, payload size, and limits.
4. Record or otherwise reliably establish the event receipt and deduplication state.
5. Publish to durable messaging.
6. Process the event in a worker.
7. Evaluate the appropriate published rule versions.
8. Execute matching actions.
9. Persist domain changes and execution outcomes.
10. Acknowledge the message after the required database transaction commits.
11. Expose processing status and traceability through supported APIs and logs.

The exact database/publish coordination strategy must prevent silently losing accepted events.

Use an outbox where database state and publication must be made atomic. Do not add an outbox merely because the pattern is popular if the chosen ingestion contract can be implemented correctly and more simply.

Clearly define when an API request is considered accepted and what guarantees that response represents.

## 12.2 Delivery semantics

Assume at-least-once delivery.

Messages may be retried, delivered more than once, or processed again after a worker restart.

Therefore:

- Event receipt deduplication is necessary.
- Business-effect idempotency is necessary.
- Action execution state must be persisted.
- Previously successful effects must not be duplicated when a later action fails.
- Database transactions must protect the relevant invariants.
- Acknowledgment must not occur before the required durable work is committed.
- Poison messages and repeated failures must have a bounded, observable recovery path.

Do not promise exactly-once delivery across PostgreSQL, NATS, and external providers.

Achieve effectively-once business outcomes through explicit idempotency, transactions, durable state, and reconciliation.

## 12.3 Retries and backpressure

Configure bounded retries, suitable retry delays, dead-letter or terminal-failure handling, and useful operational diagnostics.

Monitor message lag, oldest pending message, processing failures, retry counts, and throughput.

Avoid unlimited retries and unbounded in-memory queues.

Implement graceful shutdown and safe worker recovery.

Do not introduce Kafka, multiple brokers, or a distributed orchestration framework without demonstrated need.

---

# 13. REST API Contract

Expose a versioned REST API and maintain an OpenAPI specification.

The API should provide stable contracts, consistent error responses, validation, pagination where needed, and documented authentication requirements.

Potential endpoint families include:

```text
/v1/events
/v1/events/{eventId}

/v1/users/{externalUserId}/profile
/v1/users/{externalUserId}/points
/v1/users/{externalUserId}/points/transactions

/v1/points/conversions

/v1/users/{externalUserId}/missions
/v1/users/{externalUserId}/achievements

/v1/rewards
/v1/rewards/{rewardId}/redemptions
/v1/redemptions/{redemptionId}

/v1/admin/tenants
/v1/admin/environments
/v1/admin/credentials
/v1/admin/rules
/v1/admin/campaigns
/v1/admin/currencies
/v1/admin/rewards
```

These are illustrative route families, not a final contract. Design the actual endpoints around coherent resources and consistent authorization.

Never expose sensitive tenant administration operations through a public integration credential.

Any endpoint accepting an external user ID must verify that the credential, tenant, environment, and scope authorize the requested operation. A client-provided ID alone is not authorization.

Document event acceptance versus processing completion, error semantics, retry behavior, and idempotency requirements.

Generate or validate API documentation from the implementation rather than allowing the reference to drift.

Do not commit to official language SDKs while the REST contract covers the integration need. Prioritize a stable, implementation-validated OpenAPI description and runnable HTTP examples in common backend languages. Generated clients are a consumer choice, not a separately maintained MotifEngine product.

An MCP adapter, if justified by real AI-assisted administration use cases, must use the same API authorization and tenant/environment boundaries. It does not replace REST event ingestion or backend-to-backend integration.

---

# 14. Security Requirements

Security must be part of the initial architecture.

Implement appropriate controls for:

- Authentication and authorization.
- Platform administrator and tenant administrator separation.
- Tenant and environment isolation.
- Credential hashing and revocation.
- Secure password hashing and session handling.
- Server-side permission checks.
- Request validation.
- Payload and configuration limits.
- Rate limiting.
- CORS configuration.
- Secure cookie configuration if cookie sessions are used.
- SQL injection prevention.
- Safe error responses without sensitive details.
- Secret management.
- Audit logging for security-sensitive operations.
- Dependency and container hygiene.
- Graceful shutdown and resource limits.

Do not log raw credentials, access tokens, passwords, or unnecessary personal information.

Do not expose stack traces, SQL errors, secrets, or internal infrastructure details to public API clients.

Use least-privilege database and application credentials where practical.

If implementing an initial single Super Admin, provide a safe initialization procedure that does not create a publicly accessible default account with a hard-coded password.

Document security-sensitive assumptions and limitations.

---

# 15. Code Quality and Maintainability

Write clean, idiomatic, production-oriented Go and TypeScript.

The goal is understandable code that a future engineer can safely modify.

## 15.1 General principles

- Keep each module focused on one cohesive responsibility.
- Separate business rules from infrastructure.
- Prefer explicit control flow over unnecessary abstraction.
- Use descriptive names.
- Keep public APIs small.
- Reuse code where the abstraction is real.
- Avoid duplicated business logic.
- Validate inputs at trust boundaries.
- Handle errors explicitly.
- Preserve causal context in logs and errors.
- Avoid hidden global state.
- Avoid unnecessary singleton patterns.
- Avoid overly generic frameworks for a small number of use cases.
- Avoid premature optimization.
- Avoid unnecessary reflection and code generation.
- Do not introduce dependencies without a clear benefit.

## 15.2 Comments and documentation in code

Add meaningful comments where they improve understanding.

Comments are particularly important for:

- Non-obvious business invariants.
- Transaction boundaries.
- Idempotency decisions.
- Concurrency control.
- Ledger accounting semantics.
- Point conversion and rounding.
- Event acknowledgment and retry behavior.
- Tenant isolation.
- Security decisions.
- Versioning behavior.
- Workarounds for real external constraints.
- Complex algorithms and edge cases.

Explain **why** a decision is necessary when that reason is not apparent from the code.

Use Go doc comments for exported declarations where appropriate.

Use concise comments for exported frontend APIs and non-obvious hooks or domain utilities.

Do not add comments that merely repeat the code. Do not create large explanatory comments for trivial getters, simple JSX, or obvious assignments.

Keep comments synchronized with implementation.

Avoid boilerplate comments that make the source appear machine-generated. All code, comments, documentation, and UI copy should read like professional work maintained by an experienced engineering team.

Do not include AI-related branding, references to generated code, implementation-agent commentary, or artificial disclaimers in the product, source files, commit messages, or documentation.

Do not remove legally required notices or third-party license information.

## 15.3 File and component size

Avoid giant files and giant components, but do not split code mechanically.

Extract a module when it has a distinct responsibility, meaningful reuse, or a clear architectural boundary.

A simple feature should remain simple. A complex feature should be split into cohesive parts.

Never create layers, factories, interfaces, wrappers, or abstractions solely to make the architecture appear sophisticated.

---

# 16. Testing and Verification

Tests are required for important behavior, not merely for coverage statistics.

## 16.1 Backend tests

Prioritize tests for:

- Tenant isolation.
- Authentication and authorization.
- Event schema validation.
- Event deduplication.
- Rule evaluation.
- Rule version behavior.
- Action execution.
- Idempotent point awards.
- Ledger invariants.
- Concurrent spending.
- Concurrent redemption.
- Currency conversion atomicity.
- Expiration and lot allocation.
- Worker retries and restarts.
- Reward delivery failures.
- Transaction rollback.
- Invalid configuration handling.

Use PostgreSQL and NATS in integration tests where their actual behavior is important.

Mocks are useful for unit tests but must not replace integration tests for database constraints, locking, transactions, or JetStream semantics.

## 16.2 Frontend tests

Test critical user workflows and state transitions.

Prioritize:

- Authentication and workspace access.
- Tenant creation.
- Environment and credential management.
- Event schema creation or configuration.
- Rule configuration and validation.
- Currency creation.
- Point history and balances.
- Reward configuration and redemption status.
- Loading, empty, error, and success states.
- Responsive and accessible critical interactions.

Avoid brittle tests that assert irrelevant implementation details.

## 16.3 End-to-end workflow

The system must demonstrate a complete vertical slice:

1. Initialize or authenticate the Super Admin.
2. Create a tenant.
3. Create a sandbox environment and credentials.
4. Define two independent point currencies.
5. Configure an event type.
6. Publish a rule and an action that awards points.
7. Submit an event through the API.
8. Verify durable acceptance and worker processing.
9. Verify the resulting ledger entries and balances.
10. Submit the same event again and verify that no duplicate award occurs.
11. Convert some points between the two currencies atomically.
12. Configure a digital reward.
13. Redeem the reward through the supported redemption flow.
14. Inspect the relevant execution history and audit records.

The exact initial scope may be implemented in phases, but this workflow defines the intended architecture and eventual acceptance behavior.

## 16.4 Required verification before completion

Run the relevant commands through Make:

- Formatting.
- Linting.
- Type checking.
- Backend tests.
- Frontend tests.
- Integration tests where configured.
- Build.
- Database migration checks.
- OpenAPI generation or validation.
- Documentation checks.
- End-to-end tests for the implemented critical path.

Do not claim tests passed unless they were actually executed and passed.

If a test cannot run because of an environmental limitation, report the precise limitation and what remains unverified.

Fix failures caused by your changes rather than silently skipping them.

---

# 17. Observability and Operations

Provide useful structured logs and health endpoints.

Separate liveness from readiness where appropriate.

Expose enough information to diagnose:

- Database connectivity.
- NATS connectivity.
- Worker availability.
- Event acceptance failures.
- Processing latency and errors.
- Retry behavior.
- Queue lag.
- Dead-letter or terminal-failure volume.
- Reward delivery failures.

Use correlation identifiers or traceable event and execution IDs to follow a request through the system.

Do not log sensitive payloads indiscriminately.

Do not introduce a large observability stack by default. Start with structured logs, health checks, and actionable metrics. Add a dedicated metrics backend or tracing integration when it provides concrete value.

Document backup, restore, migration, and recovery expectations appropriate to the initial deployment model.

---

# 18. Roadmap and Implementation Order

Implement incrementally, preserving a working repository after each major phase.

## Phase 0 — Discovery and Foundation

- Inspect the existing repository.
- Confirm Nx workspace structure.
- Establish or verify Make commands.
- Inspect and reuse compatible Docker images.
- Prepare Docker Compose and environment configuration.
- Establish formatting, linting, and testing conventions.
- Document the architecture and key decisions.

**Exit criteria:** A reproducible local environment and clear development workflow.

## Phase 1 — Platform and Execution Core

- Super Admin initialization.
- Tenant creation and management.
- Environments and API credentials.
- Plan, entitlement, override, and limit foundations.
- PostgreSQL migrations and sqlc.
- Event ingestion and validation.
- Durable NATS JetStream integration.
- Worker lifecycle and retries.
- Rule definitions and immutable published versions.
- Typed action execution.
- Event and action idempotency.
- Audit and operational diagnostics.
- REST/OpenAPI foundation.
- Dashboard foundation with FSD.

**Exit criteria:** A tenant can submit an event, trigger a published rule, and produce a traceable domain effect without duplicate execution.

## Phase 2 — Point Currency and Ledger Foundation

- Point currency definitions and policy versions.
- Point accounts.
- Append-only ledger.
- Balance projection and reconciliation.
- Atomic point awards and spending.
- Idempotency keys at the business-effect level.
- Point lots and expiration.
- Conversion rates and policy versions.
- Atomic same-database conversion.
- Concurrency and recovery tests.

**Exit criteria:** Multiple independent currencies work correctly under retries, concurrent requests, and policy changes.

## Phase 3 — Engagement and Progression

- Missions.
- Achievements.
- Levels and XP.
- Progress tracking.
- Streaks where justified.
- Campaigns and scheduling.
- Relevant rule actions.
- Customer-facing read APIs for authorized integration contexts.
- Documentation and runnable examples.

**Exit criteria:** Customers can configure a meaningful engagement flow and query its resulting user state through supported APIs.

## Phase 4 — Loyalty and Rewards

- Reward catalog.
- Redemption workflow.
- Point reservation and consumption where needed.
- Digital unlocks.
- Provider adapter contract.
- One limited reward delivery implementation.
- Failure handling and compensation.
- Coupons, discounts, or additional reward types when included in the validated initial scope.

**Exit criteria:** Redemption is traceable, idempotent, and recoverable from known and ambiguous delivery failures.

## Phase 5 — Measurement and Optimization

- Business metric definitions.
- Campaign performance reporting.
- Segmentation.
- Retention and funnel analysis.
- Experiments and control groups.
- Additional integrations and optimization capabilities.

**Exit criteria:** Reporting distinguishes reliable measured results from estimates and unsupported assumptions.

Do not implement Phase 5 prematurely if the core platform is not reliable.

The marketing site and documentation should be developed alongside the product, not postponed until the end. Document planned capabilities accurately without presenting them as implemented.

---

# 19. Performance and Scalability

Design for the initial target of approximately 10–1,000 events per second, but validate the actual system through benchmarks.

Measure:

- Payload size.
- Event acceptance latency.
- Processing latency.
- Rule fan-out.
- Database write volume.
- Index overhead.
- Queue lag.
- Worker concurrency.
- Retry rates.
- Ledger contention.
- Storage and retention growth.

At 1,000 sustained events per second, the platform may process roughly 86.4 million events per day. Retention, partitioning, indexing, payload storage, and archival policies therefore require deliberate consideration.

Do not introduce complex partitioning, distributed caching, multiple databases, or a separate analytics warehouse without measured justification.

Use bounded concurrency and sensible resource limits.

Keep the system easy to operate at the expected initial scale while preserving clear paths for later optimization.

---

# 20. Explicit Anti-Overengineering Rules

These rules are mandatory.

Do not:

- Turn the Modular Monolith into microservices without evidence.
- Add Kubernetes for local development.
- Introduce Kafka alongside NATS without a demonstrated requirement.
- Use PostgreSQL as an improvised message queue.
- Build a generic workflow engine beyond the required rule/action domain.
- Execute arbitrary customer code.
- Build an unrestricted expression language.
- Implement a custom authentication framework when a secure, suitable established approach exists.
- Add Redis without a specific need.
- Add an ORM alongside sqlc.
- Create a separate Go service for each domain module.
- Build a plugin marketplace for reward providers.
- Implement every future reward provider in advance.
- Build a complete billing system while billing is managed manually.
- Build analytics infrastructure before there are meaningful metrics to collect.
- Create dozens of frontend libraries for tiny utilities.
- Introduce multiple UI component libraries.
- Build a generic design system beyond the product's actual needs.
- Add speculative abstractions for imagined future requirements.
- Create a feature-flag platform when a simpler entitlement model is sufficient.
- Duplicate business rules between frontend and backend.
- Add unnecessary database tables, interfaces, wrappers, factories, or dependency-injection layers.
- Use arbitrary event names as hard-coded domain primitives.
- Treat all point currencies as interchangeable.
- Treat a successful HTTP request as proof that an asynchronous event was processed.
- Claim exactly-once distributed delivery without the guarantees to support that claim.
- Use fake data or inert controls to disguise incomplete features.

Before introducing any significant dependency, infrastructure component, abstraction, or new architectural pattern, answer:

1. What concrete requirement does it satisfy?
2. Can the requirement be met more simply?
3. What operational or maintenance cost does it add?
4. Is it needed now or only hypothetically in the future?

Choose the simplest solution that preserves correctness, security, and extensibility.

---

# 21. Documentation and Repository Standards

Maintain a root README that explains:

- What MotifEngine is.
- High-level architecture.
- Prerequisites.
- Environment setup.
- Docker and Make usage.
- How to run migrations.
- How to run tests.
- How to access the applications.
- Where to find product and technical documentation.

Maintain a concise architecture document and record consequential decisions as ADRs when they affect future maintainability or compatibility.

Document environment variables in `.env.example` with safe example values and explanations.

Never commit real credentials.

Ensure that a new engineer can clone the repository, configure the environment, start the required services, run migrations, execute tests, and understand the basic architecture without relying on undocumented personal knowledge.

Documentation must describe the implemented system, not the aspirations of the implementation agent.

Do not include self-congratulatory statements, artificial progress logs, AI attribution, or generic generated-content disclaimers in the product or repository.

---

# 22. Completion Criteria

The project is not complete until the following conditions are satisfied or each remaining limitation is explicitly documented.

## Current implementation checkpoint (2026-10-13)

The backend engine and platform foundation are implemented through the five
platform-completion stages: session-based admin identity and RBAC, configurable
HTTP reward delivery, immutable event schema versions with bounded JSON Schema
operators, Prometheus metrics and CI checks, and manual plan administration with
usage snapshots. The dashboard consumes live APIs and exposes the selected tenant's
capacity guardrails. Its Super Admin and Tenant Admin workspaces enforce platform
versus membership-scoped controls, and the public marketing/documentation app is
implemented in `apps/marketing`.


## Repository and tooling

- [x] One Nx monorepo contains all first-party applications and shared code.
- [x] Existing repository work has been preserved where appropriate.
- [x] Docker Compose configuration is valid.
- [ ] Local Docker images were inspected before pulling missing images.
- [x] Compatible existing images are reused.
- [x] Missing required images are pulled only when necessary.
- [x] Make provides documented, working development and verification targets.
- [x] Environment configuration is documented and contains no production secrets.

## Backend and data

- [x] API and worker have clear responsibilities.
- [x] PostgreSQL migrations work from a clean database.
- [x] sqlc generation is reproducible.
- [x] NATS JetStream is configured for durable processing.
- [x] Tenant and environment isolation are enforced server-side.
- [x] Authentication and authorization are implemented.
- [x] Event ingestion and processing status are coherent.
- [x] Published rule versions are traceable.
- [x] Action execution is idempotent.
- [x] Ledger invariants are enforced transactionally.
- [x] Independent currencies and conversion semantics are correct.
- [x] Reward redemption has explicit state transitions and failure handling.

## Frontend and product

- [x] Marketing uses TanStack Start.
- [x] Dashboard uses Vite and React.
- [x] Dashboard follows FSD dependency rules.
- [ ] Airbnb-inspired, consistent linting and formatting rules are enforced.
- [ ] Tailwind CSS and shadcn/ui are the only foundational styling/component systems.
- [x] Platform and tenant workspaces have distinct layouts and authorization.
- [ ] Every implemented action works.
- [ ] Loading, empty, error, and success states exist for important workflows.
- [x] The UI is responsive and accessible.
- [x] The marketing website introduces the actual product capabilities.
- [x] Documentation includes explanations and practical examples for implemented features.

## Verification and maintainability

- [x] Formatting, linting, type checking, and builds pass.
- [x] Critical backend invariants have automated tests.
- [x] Database and messaging integration tests cover important behavior.
- [x] The critical end-to-end workflow is verified to the extent implemented.
- [x] No knowingly broken placeholder implementations are presented as finished.
- [x] Important architectural decisions and operational procedures are documented.
- [ ] No unnecessary infrastructure or abstractions have been added.

The remaining unchecked criteria are deliberate limitations of the current local
verification boundary. The repository does not retain an auditable history of
which Docker images were inspected before an earlier pull, and the test suite
does not yet include disposable-database or broker-backed integration tests.
The frontend also relies on TypeScript and Nx checks rather than a separately
installed Airbnb ESLint/formatter stack, and no shadcn component catalogue is
needed by the current custom dashboard surface. These items are not presented
as completed capabilities.

---

# 23. Required Working Method

Work in small, coherent implementation increments.

For each increment:

1. Identify the specific outcome.
2. Inspect relevant existing code and contracts.
3. Make the smallest coherent change.
4. Add or update tests.
5. Run the relevant Make targets.
6. Fix regressions before proceeding.
7. Update documentation when behavior or contracts change.
8. Keep the repository in a working, understandable state.

Prefer completing one end-to-end workflow over creating many disconnected screens and modules.

When implementing a feature, connect the frontend, API, database, processing logic, and documentation as appropriate.

Do not build an impressive-looking UI over an absent backend.

Do not create database tables that have no clear domain purpose or implementation path.

Do not silently skip verification.

If a major design decision changes, record the reasoning and consequences.

At each meaningful milestone, provide a concise report of:

- What was implemented.
- Which important files or modules changed.
- Which tests and checks actually ran.
- What remains incomplete.
- Any material technical risks or unresolved limitations.

Do not stop at reporting the plan or creating scaffolding. Continue implementation until the available execution time or an actual external blocker prevents further progress.

---

# 24. Final Instruction

Build MotifEngine as a coherent, dependable, maintainable product.

The architecture should be strong where correctness requires strength: tenant isolation, transactions, event durability, idempotency, ledger invariants, authorization, and data integrity.

The implementation should be simple where complexity adds no value: module organization, abstractions, UI composition, infrastructure, and developer workflows.

The user experience should make powerful capabilities understandable without exposing unnecessary implementation complexity.

The marketing website should explain what the product actually offers. The documentation should teach users how to configure and integrate those capabilities with concrete examples.

Docker must support reproducible execution. Existing compatible images must be reused before downloading new ones. Make must provide the primary developer workflow. The Nx monorepo must remain coherent. Frontend code must follow FSD and enforceable coding standards. Backend code must remain idiomatic, explicit, and testable.

**Prioritize correctness over cleverness, maintainability over abstraction, user experience over visual decoration, and working end-to-end behavior over superficial feature count.**

Begin by inspecting the repository and its existing environment, then proceed with the implementation plan and execution.
