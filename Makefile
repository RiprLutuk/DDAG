# DDAG — Dynamic Database API Gateway
# Local dev targets. Services run as separate processes (one per pod in prod).

GO        ?= go
BIN       ?= ./bin
PGPORT    ?= 1921
PGUSER    ?= lutuk
PGHOST    ?= localhost
SERVICES  := admin-backend auth-service api-gateway policy-engine cache-service worker \
             connector-postgres connector-mysql connector-oracle connector-sqlserver migrate

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-22s\033[0m %s\n",$$1,$$2}'

.PHONY: tidy
tidy: ## Resolve Go modules
	$(GO) mod tidy

.PHONY: build
build: ## Build all service binaries into ./bin
	@mkdir -p $(BIN)
	@for s in $(SERVICES); do echo "build $$s"; $(GO) build -o $(BIN)/ddag-$$s ./cmd/$$s || exit 1; done

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: test
test: ## Run unit tests
	$(GO) test ./... -count=1

.PHONY: createdb
createdb: ## Create the metadata database (ddag) if missing
	@psql -h $(PGHOST) -p $(PGPORT) -U $(PGUSER) -d postgres -tc "SELECT 1 FROM pg_database WHERE datname='ddag'" | grep -q 1 || createdb -h $(PGHOST) -p $(PGPORT) -U $(PGUSER) ddag
	@echo "metadata database ready"

.PHONY: migrate
migrate: createdb ## Apply metadata migrations
	$(GO) run ./cmd/migrate

.PHONY: seed
seed: createdb ## Apply migrations + core seed + demo data (ddag_demo, demo API/client)
	$(GO) run ./cmd/migrate --demo

.PHONY: seed-core
seed-core: createdb ## Apply migrations + core seed only (roles/permissions/super-admin)
	$(GO) run ./cmd/migrate --seed

# Individual service runners (foreground).
.PHONY: run-admin run-auth run-gateway run-connector-postgres run-policy run-cache run-worker
run-admin: ## Run admin-backend (:8080)
	$(GO) run ./cmd/admin-backend
run-auth: ## Run auth-service (:8081)
	$(GO) run ./cmd/auth-service
run-gateway: ## Run api-gateway (:8082)
	$(GO) run ./cmd/api-gateway
run-policy: ## Run policy-engine (:8083)
	$(GO) run ./cmd/policy-engine
run-cache: ## Run cache-service (:8084)
	$(GO) run ./cmd/cache-service
run-worker: ## Run worker (:8085)
	$(GO) run ./cmd/worker
run-connector-postgres: ## Run connector-postgres (:8090)
	$(GO) run ./cmd/connector-postgres

.PHONY: dev
dev: build seed ## Build, seed, and start the core services in the background (see scripts/dev.sh)
	@./scripts/dev.sh start

.PHONY: dev-stop
dev-stop: ## Stop background dev services
	@./scripts/dev.sh stop

.PHONY: dev-logs
dev-logs: ## Tail background dev service logs
	@./scripts/dev.sh logs

.PHONY: dashboard
dashboard: ## Run the Nuxt dashboard dev server (:3000)
	@cd apps/dashboard && (pnpm install || npm install) && (pnpm dev || npm run dev)
