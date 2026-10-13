SHELL := /bin/sh

.DEFAULT_GOAL := help

ifneq (,$(wildcard .env))
include .env
export
endif

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

infra-up: ## Start PostgreSQL and NATS
	docker compose up -d postgres nats

infra-down: ## Stop local infrastructure without deleting volumes
	docker compose down

dev: ## Run API and worker from the host (in separate terminals)
	@$(MAKE) infra-up migrate-up
	@trap 'kill 0' INT TERM EXIT; go run ./apps/api & go run ./apps/worker & wait

dev-docker: ## Start the full containerized development stack
	docker compose up --build

dashboard: ## Start the Vite administrative dashboard
	pnpm --dir apps/dashboard dev --host 0.0.0.0

marketing: ## Start the TanStack Start public website
	pnpm --dir apps/marketing dev --host 0.0.0.0

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
	go build ./apps/api ./apps/worker ./apps/migrate
	pnpm --dir apps/dashboard build
	pnpm --dir apps/marketing build
	pnpm --dir packages/sdk-typescript build

status: ## Show service status
	docker compose ps

logs: ## Follow service logs
	docker compose logs -f --tail=100

clean: ## Stop services and preserve persistent data
	docker compose down

db-reset: ## Destructively remove local database data
	docker compose down -v

backup: ## Create an encrypted-at-rest-ready PostgreSQL custom-format backup file
	./scripts/db-backup.sh

restore: ## Restore a backup after explicitly setting MOTIF_CONFIRM_RESTORE=yes
	@test -n "$(BACKUP_FILE)" || (echo "BACKUP_FILE is required"; exit 1)
	./scripts/db-restore.sh "$(BACKUP_FILE)"

production-check: ## Check production health endpoints and metrics
	./scripts/production-check.sh
