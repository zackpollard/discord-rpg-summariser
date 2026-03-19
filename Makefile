# Discord RPG Session Summariser — Development Makefile

DATABASE_URL       ?= postgres://rpg:rpg@localhost:5432/rpg_summariser?sslmode=disable
TEST_DATABASE_URL  ?= postgres://rpg:rpg@localhost:5432/rpg_summariser_test?sslmode=disable
BUILD_TAGS          = nolibopusfile
BINARY              = bot

.PHONY: dev dev-deps dev-stop build test test-unit test-integration lint clean help

help: ## Show this help
	@grep -E '^[a-z][a-z_-]+:.*## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Development
# ---------------------------------------------------------------------------

dev: dev-deps web/node_modules ## Start postgres, Go backend, and Svelte dev server
	@echo "Starting dev servers (Ctrl+C to stop)..."
	@trap 'kill 0' EXIT; \
	DATABASE_URL="$(DATABASE_URL)" \
		CGO_ENABLED=1 go run -tags $(BUILD_TAGS) ./cmd/bot/ & \
	cd web && npm run dev & \
	wait

dev-deps: ## Start PostgreSQL via Docker Compose
	docker compose up -d --wait
	@echo "PostgreSQL ready at localhost:5432"

dev-stop: ## Stop PostgreSQL
	docker compose down

web/node_modules: web/package.json
	cd web && npm install
	@touch $@

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

build: web/build $(BINARY) ## Build Go binary and Svelte app

$(BINARY): $(shell find cmd internal -name '*.go')
	CGO_ENABLED=1 go build -tags $(BUILD_TAGS) -o $(BINARY) ./cmd/bot/

web/build: web/node_modules $(shell find web/src -type f)
	cd web && npm run build

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests (no database required)
	CGO_ENABLED=1 go test -tags $(BUILD_TAGS) -count=1 \
		./internal/config/ \
		./internal/audio/ \
		./internal/bot/ \
		./internal/summarise/ \
		./internal/transcribe/ \
		./internal/voice/

test-integration: dev-deps ## Run integration tests (starts postgres if needed)
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
		CGO_ENABLED=1 go test -tags $(BUILD_TAGS) -count=1 -v ./internal/api/

# ---------------------------------------------------------------------------
# Quality
# ---------------------------------------------------------------------------

lint: ## Run Go vet and Svelte check
	CGO_ENABLED=1 go vet -tags $(BUILD_TAGS) ./...
	@if [ -d web/node_modules ]; then cd web && npx svelte-check --tsconfig tsconfig.json; fi

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf web/build web/.svelte-kit
