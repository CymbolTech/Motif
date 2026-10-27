SHELL := /bin/sh

.DEFAULT_GOAL := help

COMPOSE ?= docker compose
# Docker Compose reads COMPOSE_FILE itself and accepts a colon-separated list. A host
# behind a containerised Nginx Proxy Manager sets
# COMPOSE_FILE=docker-compose.yml:docker-compose.proxy.yml in .env once, so every
# target below (up, restart, logs, down) keeps the proxy overlay.
COMPOSE_FILE ?= docker-compose.yml
COMPOSE_CMD = $(COMPOSE)
PROXY_NETWORK ?= proxy

ifneq (,$(wildcard .env))
include .env
export
endif
export COMPOSE_FILE PROXY_NETWORK

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "%-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## Install JavaScript dependencies and create local environment file
	@test -f .env || cp .env.example .env
	@pnpm install --frozen-lockfile=false --ignore-scripts

doctor: ## Check required local tools
	@command -v go >/dev/null && go version
	@command -v node >/dev/null && node --version
	@command -v pnpm >/dev/null && pnpm --version
	@command -v docker >/dev/null && docker --version
	@docker compose version

env-check: ## Check that required environment values are present
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is not set; run make setup or source .env"; exit 1)

compose-config: ## Validate the Docker Compose configuration
	$(COMPOSE_CMD) config --quiet

infra-up: ## Start PostgreSQL and NATS
	$(COMPOSE_CMD) up -d postgres nats

infra-down: ## Stop local infrastructure without deleting volumes
	$(COMPOSE_CMD) down

dev: ## Run API, worker and MCP server from the host
	@$(MAKE) infra-up migrate-up
	@trap 'kill 0' INT TERM EXIT; go run ./apps/api & go run ./apps/worker & MCP_ADDR=127.0.0.1:$${MCP_PORT:-18082} MOTIF_API_URL=http://$${API_ADDR:-127.0.0.1:8080} go run ./apps/mcp & wait

dev-docker: ## Start the full containerized development stack
	$(COMPOSE_CMD) up --build

up: ## Build and start the complete Docker stack in the background
	$(COMPOSE_CMD) up -d --build

up-proxy: ## Build and start the stack on the shared Nginx Proxy Manager network
	@docker network inspect "$(PROXY_NETWORK)" >/dev/null 2>&1 || docker network create "$(PROXY_NETWORK)"
	@case "$(COMPOSE_FILE)" in *docker-compose.proxy.yml*) ;; *) echo "Note: set COMPOSE_FILE=docker-compose.yml:docker-compose.proxy.yml in .env so restart, logs and down keep the proxy network.";; esac
	@$(MAKE) up COMPOSE_FILE="docker-compose.yml:docker-compose.proxy.yml"

down: ## Stop the Docker stack without deleting volumes
	$(COMPOSE_CMD) down

restart: ## Rebuild and restart the Docker stack
	$(COMPOSE_CMD) up -d --build --force-recreate

compose-build: ## Build all Docker images
	$(COMPOSE_CMD) build

compose-logs: ## Follow logs from all Docker services
	$(COMPOSE_CMD) logs -f --tail=100

compose-status: ## Show Docker service status
	$(COMPOSE_CMD) ps

health: ## Check the public dashboard, landing page, API, worker and MCP server
	@curl --fail --silent --show-error "http://127.0.0.1:$${DASHBOARD_PORT:-4173}/" >/dev/null
	@curl --fail --silent --show-error "http://127.0.0.1:$${MARKETING_PORT:-4174}/" >/dev/null
	@curl --fail --silent --show-error "http://127.0.0.1:$${API_PORT:-18080}/health/ready" >/dev/null
	@curl --fail --silent --show-error "http://127.0.0.1:$${WORKER_PORT:-18081}/health/ready" >/dev/null
	@curl --fail --silent --show-error "http://127.0.0.1:$${MCP_PORT:-18082}/health/ready" >/dev/null
	@echo "MotifEngine web and service health checks passed."

public-health: ## Check the console, API, landing page and docs through their public URLs
	@test -n "$(MOTIF_CONSOLE_URL)" && test -n "$(MOTIF_MARKETING_URL)" || (echo "Set MOTIF_CONSOLE_URL and MOTIF_MARKETING_URL"; exit 1)
	@curl --fail --silent --show-error "$(MOTIF_CONSOLE_URL)/" >/dev/null
	@curl --fail --silent --show-error "$(MOTIF_CONSOLE_URL)/api/health/ready" >/dev/null
	@curl --fail --silent --show-error "$(MOTIF_MARKETING_URL)/" >/dev/null
	@curl --fail --silent --show-error "$(MOTIF_MARKETING_URL)/docs" >/dev/null
	@test "$$(curl --silent --output /dev/null --write-out '%{http_code}' -X POST "$(MOTIF_CONSOLE_URL)/mcp")" = 401 || (echo "$(MOTIF_CONSOLE_URL)/mcp should ask for an access key"; exit 1)
	@echo "Public URLs respond: $(MOTIF_CONSOLE_URL) and $(MOTIF_MARKETING_URL)."

production-env-check: ## Validate required production secrets and URLs
	@test "$(MOTIF_RUNTIME_ENV)" = "production" || (echo "MOTIF_RUNTIME_ENV must be production"; exit 1)
	@test -n "$(MOTIF_CONSOLE_URL)" || (echo "MOTIF_CONSOLE_URL is required"; exit 1)
	@test -n "$(MOTIF_MARKETING_URL)" || (echo "MOTIF_MARKETING_URL is required"; exit 1)
	@for url in "$(MOTIF_CONSOLE_URL)" "$(MOTIF_MARKETING_URL)"; do case "$$url" in https://*/) echo "$$url must not end with a slash; it is baked into the web builds"; exit 1;; https://*) ;; *) echo "$$url must use https://"; exit 1;; esac; done
	@case ",$(MOTIF_CORS_ORIGINS)," in *",$(MOTIF_CONSOLE_URL),"*) ;; *) echo "MOTIF_CORS_ORIGINS should include $(MOTIF_CONSOLE_URL)"; exit 1;; esac
	@test -n "$(MOTIF_BOOTSTRAP_API_KEY)" && test "$(MOTIF_BOOTSTRAP_API_KEY)" != "dev_only_change_me" || (echo "Set a production MOTIF_BOOTSTRAP_API_KEY"; exit 1)
	@test -n "$(MOTIF_ADMIN_API_KEY)" && test "$(MOTIF_ADMIN_API_KEY)" != "admin_dev_only_change_me" || (echo "Set a production MOTIF_ADMIN_API_KEY"; exit 1)
	@test -n "$(MOTIF_ADMIN_PASSWORD)" && test "$(MOTIF_ADMIN_PASSWORD)" != "admin_dev_only_change_me" || (echo "Set a production MOTIF_ADMIN_PASSWORD"; exit 1)
	@test -n "$(POSTGRES_PASSWORD)" && test "$(POSTGRES_PASSWORD)" != "motif_local_only" || (echo "Set a production POSTGRES_PASSWORD"; exit 1)
	@test -n "$(MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY)" || (echo "MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY is required"; exit 1)
	@test -n "$(MOTIF_CORS_ORIGINS)" || (echo "MOTIF_CORS_ORIGINS is required"; exit 1)
	@echo "Production environment looks configured."

seed-reports-demo: ## Load the local Reports demo dataset into PostgreSQL
	$(COMPOSE_CMD) exec -T postgres sh -c 'psql -U "$${POSTGRES_USER:-motif}" -d "$${POSTGRES_DB:-motif}"' < scripts/seed_reports_demo.sql

dashboard: ## Start the Vite administrative dashboard
	pnpm --dir apps/dashboard dev --host 0.0.0.0

marketing: ## Start the Next.js public website
	pnpm --dir apps/marketing dev --hostname 0.0.0.0

dashboard-build: ## Build the administrative dashboard
	pnpm nx run dashboard:build

marketing-build: ## Build the public website
	pnpm nx run marketing:build

migrate-up: ## Apply database migrations
	go run ./apps/migrate up

migrate-down: ## Roll back the latest foundation migration
	go run ./apps/migrate down

sqlc-generate: ## Generate type-safe PostgreSQL query code
	docker run --rm -v "$$(pwd):/src" -w /src sqlc/sqlc:1.31.1 generate

build: ## Build all Nx projects
	pnpm nx run-many -t build

test: ## Run all tests
	pnpm nx run-many -t test

test-integration: ## Run real PostgreSQL and NATS tests on disposable containers
	./scripts/test-integration.sh

lint: ## Run Go vet through Nx
	pnpm nx run-many -t lint

ci: ## Run the full local verification suite
	go test ./...
	$(MAKE) test-integration
	go vet ./...
	go build ./apps/api ./apps/worker ./apps/migrate ./apps/mcp
	pnpm --dir apps/dashboard build
	pnpm --dir apps/marketing build

status: ## Show service status
	$(COMPOSE_CMD) ps

logs: ## Follow service logs
	$(COMPOSE_CMD) logs -f --tail=100

clean: ## Stop services and preserve persistent data
	$(COMPOSE_CMD) down

db-reset: ## Destructively remove local database data
	$(COMPOSE_CMD) down -v

backup: ## Create an encrypted-at-rest-ready PostgreSQL custom-format backup file
	./scripts/db-backup.sh

restore: ## Restore a backup after explicitly setting MOTIF_CONFIRM_RESTORE=yes
	@test -n "$(BACKUP_FILE)" || (echo "BACKUP_FILE is required"; exit 1)
	./scripts/db-restore.sh "$(BACKUP_FILE)"

production-check: ## Check production health endpoints and metrics
	./scripts/production-check.sh
