# Kodit Go Makefile
# Build, test, lint, format, and run commands for the Kodit Go application

# Go parameters
GOCMD=go
TAGS=fts5 ORT
ORT_VERSION?=1.24.1
BUILD_TAGS=$(TAGS) embed_model
CGO_ENABLED?=1
ORT_LIB_DIR?=$(CURDIR)/lib
CGO_LDFLAGS?=-L$(ORT_LIB_DIR)
# Suppress "ignoring duplicate libraries" warning on macOS (multiple deps request -ldl/-lm)
ifeq ($(shell uname),Darwin)
CGO_LDFLAGS+= -Wl,-no_warn_duplicate_libraries
endif
GOENV=CGO_LDFLAGS="$(CGO_LDFLAGS)" ORT_LIB_DIR="$(ORT_LIB_DIR)"
GOBUILD=CGO_ENABLED=$(CGO_ENABLED) $(GOENV) $(GOCMD) build -tags "$(BUILD_TAGS)"
GOTEST=$(GOENV) $(GOCMD) test -tags "$(BUILD_TAGS)"
GORUN=$(GOENV) $(GOCMD) run -tags "$(BUILD_TAGS)"
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOIMPORTS=goimports

# Binary name and paths
BINARY_NAME=kodit
BINARY_OUTPUT?=$(BUILD_DIR)/$(BINARY_NAME)
CMD_DIR=./cmd/kodit
BUILD_DIR=./build
COVERAGE_FILE=coverage.out

# Version info (can be overridden via environment or make args)
VERSION?=0.1.0
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Linker flags for version info
# Set STATIC=1 for static linking (alpine/musl Docker builds)
ifdef STATIC
LINK_FLAGS=-linkmode external -extldflags '-static'
endif
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME) $(LINK_FLAGS)"

# Default target
.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: build
build: download-model download-ort ## Build the application binary (with embedded model)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_OUTPUT) $(CMD_DIR)

DB_URL ?= postgresql://postgres:mysecretpassword@localhost:5432/kodit

.PHONY: run
run: docker-dev ## Run the HTTP server (downloads model on first use if needed)
	DB_URL=$(DB_URL) $(GORUN) $(CMD_DIR) serve

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf lib/
	rm -f $(COVERAGE_FILE)

##@ Testing

.PHONY: test
test: download-model download-ort ## Run all tests (excludes smoke tests)
	$(GOTEST) -v $$(go list ./... | grep -v /test/smoke)

.PHONY: test-cover
test-cover: download-model download-ort ## Run tests with coverage (excludes smoke tests)
	$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic $$(go list ./... | grep -v /test/smoke)
	$(GOCMD) tool cover -func=$(COVERAGE_FILE)

.PHONY: test-e2e
test-e2e: download-model download-ort ## Run end-to-end tests only
	$(GOTEST) -v ./test/e2e/...

##@ Code Quality

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --fix

.PHONY: fmt
fmt: ## Format Go code with gofmt
	$(GOFMT) -w .

.PHONY: imports
imports: ## Format imports with goimports
	$(GOIMPORTS) -w .

.PHONY: format
format: fmt imports ## Format code (gofmt + goimports)

.PHONY: vet
vet: ## Run go vet
	$(GOCMD) vet -tags fts5 ./...

.PHONY: check
check: fmt vet lint test ## Run all checks (format, vet, lint, test)

##@ Models

.PHONY: download-model
download-model: ## Convert and prepare the built-in embedding model (requires uv + Python)
	$(GOCMD) run ./tools/download-model

.PHONY: download-ort
download-ort: ## Download the ONNX Runtime shared library for the current platform
	ORT_VERSION=$(ORT_VERSION) $(GOCMD) run ./tools/download-ort

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	$(GOMOD) download

.PHONY: deps-tidy
deps-tidy: ## Tidy and verify dependencies
	$(GOMOD) tidy
	$(GOMOD) verify

.PHONY: deps-update
deps-update: ## Update all dependencies
	$(GOGET) -u ./...
	$(GOMOD) tidy

##@ Tools

.PHONY: tools
tools: ## Install development tools
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install golang.org/x/tools/cmd/goimports@latest
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest

##@ Documentation

# Swagger/OpenAPI paths
DOCS_DIR=./docs
SWAGGER_DIR=$(DOCS_DIR)/swagger
OPENAPI_EMBED=./infrastructure/api/openapi.json

.PHONY: swag
swag: ## Generate Swagger 2.0 spec from annotations
	swag init -g ./cmd/kodit/main.go -o $(SWAGGER_DIR) --parseInternal -d ./,./infrastructure/api/v1/dto

.PHONY: swag-fmt
swag-fmt: ## Format swag comments
	swag fmt

.PHONY: swag-check
swag-check: ## Check swagger docs are up to date (fails if stale)
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest
	swag init -g ./cmd/kodit/main.go -o $(SWAGGER_DIR) --parseInternal -d ./,./infrastructure/api/v1/dto
	git diff --exit-code $(SWAGGER_DIR)/

.PHONY: openapi
openapi: swag ## Generate Swagger 2.0 and convert to OpenAPI 3.0
	@echo "Converting Swagger 2.0 to OpenAPI 3.0..."
	npx swagger2openapi $(SWAGGER_DIR)/swagger.json -o $(SWAGGER_DIR)/openapi.json --patch
	cp $(SWAGGER_DIR)/openapi.json $(OPENAPI_EMBED)
	@echo "OpenAPI 3.0 spec generated at $(OPENAPI_EMBED)"

.PHONY: openapi-convert
openapi-convert: ## Convert existing swagger.json to OpenAPI 3.0 (skip swag generation)
	@echo "Converting existing Swagger 2.0 to OpenAPI 3.0..."
	npx swagger2openapi $(SWAGGER_DIR)/swagger.json -o $(OPENAPI_EMBED) --patch
	@echo "OpenAPI 3.0 spec generated at $(OPENAPI_EMBED)"

generate-go-client: openapi
	./scripts/generate-go-client.sh

generate-clients: generate-go-client

.PHONY: docs
docs: openapi generate-clients
	uv run --script tools/dump-openapi.py
	uv run --script tools/dump-config.py

##@ Release

.PHONY: release
release: ## Create a GitHub release (VERSION required, e.g. make release VERSION=1.0.0)
	@if [ "$(VERSION)" = "0.1.0" ]; then \
		echo "ERROR: VERSION is required. Usage: make release VERSION=1.0.0"; \
		exit 1; \
	fi
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$BRANCH" = "main" ]; then \
		TAG="v$(VERSION)"; \
		echo "Creating release $$TAG on main..."; \
		gh release create "$$TAG" --title "$$TAG" --generate-notes; \
	else \
		TAG="v$(VERSION)-rc.$(COMMIT)"; \
		echo "Creating pre-release $$TAG on branch $$BRANCH..."; \
		gh release create "$$TAG" --title "$$TAG" --generate-notes --prerelease --target "$$BRANCH"; \
	fi

##@ Docker
PROFILES :=

ifeq ($(shell lsof -i :11434 -sTCP:LISTEN >/dev/null 2>&1 && echo yes),)
  PROFILES += --profile ollama
endif

ifeq ($(shell lsof -i :5432 -sTCP:LISTEN >/dev/null 2>&1 && echo yes),)
  PROFILES += --profile vectorchord
endif

.PHONY: docker-dev
docker-dev: ## Start Docker development environment (VectorChord + Ollama)
ifeq ($(PROFILES),)
	@echo "All services already running locally, skipping docker-dev"
else
	docker compose -f docker-compose.dev.yaml $(PROFILES) up -d --wait
endif

.PHONY: docker-build
docker-build: download-model ## Build Docker image (downloads model first, then copies into image)
	docker build -t kodit:$(VERSION) .

.PHONY: docker-build-multi
docker-build-multi: download-model ## Build multi-platform Docker image
	docker buildx build --platform linux/amd64,linux/arm64 -t kodit:$(VERSION) .

.PHONY: docker-run
docker-run: ## Run Docker container
	docker run -p 8080:8080 kodit:$(VERSION)
