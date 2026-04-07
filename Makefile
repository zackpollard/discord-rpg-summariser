# Discord RPG Session Summariser — Development Makefile

DATABASE_URL       ?= postgres://rpg:rpg@localhost:5432/rpg_summariser?sslmode=disable
TEST_DATABASE_URL  ?= postgres://rpg:rpg@localhost:5432/rpg_summariser_test?sslmode=disable
BUILD_TAGS          = nolibopusfile
BINARY              = bot

# Whisper.cpp paths
WHISPER_DIR         = _deps/whisper.cpp
WHISPER_BUILD       = $(WHISPER_DIR)/build
WHISPER_LIB         = $(WHISPER_BUILD)/src
WHISPER_INCLUDE     = $(WHISPER_DIR)/include

# CGO flags for whisper.cpp and opus
WHISPER_GGML_LIB    = $(WHISPER_DIR)/build/ggml/src
export CGO_ENABLED  = 1

# Platform-specific flags.
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
  # macOS: clang needs Homebrew libomp instead of -fopenmp; ggml backend
  # libraries (metal, blas) live in subdirectories.
  LIBOMP_PREFIX := $(shell brew --prefix libomp 2>/dev/null)
  ifneq ($(LIBOMP_PREFIX),)
    OMP_FLAGS = -L$(LIBOMP_PREFIX)/lib -lomp
  endif
  GGML_EXTRA_LIBDIRS = -L$(abspath $(WHISPER_GGML_LIB)/ggml-metal) -L$(abspath $(WHISPER_GGML_LIB)/ggml-blas)
  GGML_EXTRA_RPATHS  = -Wl,-rpath,$(abspath $(WHISPER_GGML_LIB)/ggml-metal) -Wl,-rpath,$(abspath $(WHISPER_GGML_LIB)/ggml-blas)
else
  OMP_FLAGS = -fopenmp
endif

export CGO_LDFLAGS      += -L$(abspath $(WHISPER_LIB)) -L$(abspath $(WHISPER_GGML_LIB)) $(GGML_EXTRA_LIBDIRS) -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ $(OMP_FLAGS) -Wl,-rpath,$(abspath $(WHISPER_LIB)) -Wl,-rpath,$(abspath $(WHISPER_GGML_LIB)) $(GGML_EXTRA_RPATHS)
export CGO_CFLAGS       += -I$(abspath $(WHISPER_INCLUDE)) -I$(abspath $(WHISPER_DIR)/ggml/include)
export LD_LIBRARY_PATH  := $(abspath $(WHISPER_LIB)):$(abspath $(WHISPER_GGML_LIB)):$(LD_LIBRARY_PATH)

.PHONY: dev dev-local dev-deps dev-stop build test test-unit test-integration test-web lint clean help whisper

help: ## Show this help
	@grep -E '^[a-z][a-z_-]+:.*## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Dependencies
# ---------------------------------------------------------------------------

whisper: $(WHISPER_LIB)/libwhisper.so ## Build whisper.cpp library

$(WHISPER_LIB)/libwhisper.so:
	@if [ ! -d "$(WHISPER_DIR)" ]; then \
		echo "Cloning whisper.cpp..."; \
		git clone --depth 1 --branch v1.8.4 https://github.com/ggerganov/whisper.cpp "$(WHISPER_DIR)"; \
	fi
	cmake -B "$(WHISPER_BUILD)" -S "$(WHISPER_DIR)" \
		-DCMAKE_BUILD_TYPE=Release \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		-DWHISPER_BUILD_TESTS=OFF
	cmake --build "$(WHISPER_BUILD)" --config Release -j

dev-deps: ## Start PostgreSQL via Docker Compose
	docker compose -f docker-compose.dev.yml up -d --wait postgres
	@echo "PostgreSQL ready at localhost:5432"

dev-stop: ## Stop dev services
	docker compose -f docker-compose.dev.yml down

web/node_modules: web/package.json
	cd web && npm install
	@touch $@

# ---------------------------------------------------------------------------
# Development
# ---------------------------------------------------------------------------

dev: web/node_modules ## Start postgres, Go backend (Docker), and Svelte dev server
	@echo "Starting dev servers (Ctrl+C to stop)..."
	@trap 'docker compose -f docker-compose.dev.yml down; kill 0' EXIT; \
	docker compose -f docker-compose.dev.yml up --build & \
	sleep 5 && cd web && npm run dev & \
	wait

dev-local: dev-deps whisper web/node_modules ## Start dev servers locally (requires native deps)
	@echo "Starting dev servers (Ctrl+C to stop)..."
	@trap 'kill 0' EXIT; \
	DATABASE_URL="$(DATABASE_URL)" \
		go run -tags $(BUILD_TAGS) ./cmd/bot/ & \
	cd web && npm run dev & \
	wait

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

build: whisper web/build $(BINARY) ## Build Go binary and Svelte app

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

$(BINARY): whisper $(shell find cmd internal -name '*.go')
	go build -tags $(BUILD_TAGS) -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/bot/

web/build: web/node_modules $(shell find web/src -type f)
	cd web && npm run build

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

test: test-unit test-integration test-web ## Run all tests

test-unit: whisper ## Run unit tests (no database required)
	go test -tags $(BUILD_TAGS) -count=1 \
		./internal/auth/ \
		./internal/config/ \
		./internal/audio/ \
		./internal/bot/ \
		./internal/diarize/ \
		./internal/embed/ \
		./internal/pdf/ \
		./internal/summarise/ \
		./internal/transcribe/ \
		./internal/voice/

test-integration: dev-deps whisper ## Run integration tests (starts postgres if needed)
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
		go test -tags $(BUILD_TAGS) -count=1 -v ./internal/api/ ./internal/storage/

test-web: web/node_modules ## Run web frontend tests
	cd web && npm test

# ---------------------------------------------------------------------------
# Quality
# ---------------------------------------------------------------------------

lint: whisper ## Run Go vet and Svelte check
	go vet -tags $(BUILD_TAGS) ./...
	@if [ -d web/node_modules ]; then cd web && npx svelte-check --tsconfig tsconfig.json; fi

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf web/build web/.svelte-kit
