# Kodit Go Makefile
# Build, test, lint, format, and run commands for the Kodit Go application

# Go parameters
GOCMD=go
TAGS=fts5 ORT
ORT_VERSION?=$(shell cat .ort-version)
BUILD_TAGS=$(TAGS)
EMBED_TAGS=$(TAGS) embed_model
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
VERSION?=1.0.0
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

ALL_PROFILES := $(shell docker compose -f docker-compose.dev.yaml config --profiles | xargs -I{} echo --profile {})
PROFILES := --profile kodit

ifeq ($(shell lsof -i :11434 -sTCP:LISTEN >/dev/null 2>&1 && echo yes),)
  PROFILES += --profile ollama
endif

ifeq ($(shell lsof -i :5432 -sTCP:LISTEN >/dev/null 2>&1 && echo yes),)
  PROFILES += --profile vectorchord
endif

.PHONY: dev
dev: docker-dev ## Start Docker development environment (idempotent, non-destructive)
	docker compose -f docker-compose.dev.yaml --profile kodit logs -f kodit

.PHONY: docker-dev
docker-dev: download-model download-ort 
	docker compose -f docker-compose.dev.yaml $(PROFILES) up -d --wait

.PHONY: docker-clean
docker-clean:
	docker compose -f docker-compose.dev.yaml $(ALL_PROFILES) down -v

.PHONY: build
build: download-model download-ort ## Build the application binary (with embedded model)
	CGO_ENABLED=$(CGO_ENABLED) $(GOENV) $(GOCMD) build -tags "$(EMBED_TAGS)" $(LDFLAGS) -o $(BINARY_OUTPUT) $(CMD_DIR)

.PHONY: clean
clean: docker-clean ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf lib/
	rm -rf infrastructure/provider/models/
	rm -f $(COVERAGE_FILE)

##@ Testing

.PHONY: test
test: download-model download-ort ## Run all tests (excludes smoke tests)
	$(GOENV) $(GOCMD) test -tags "$(EMBED_TAGS)" -v $$(go list ./... | grep -v /test/smoke)

.PHONY: test-cover
test-cover: download-model download-ort ## Run tests with coverage (excludes smoke tests)
	$(GOENV) $(GOCMD) test -tags "$(EMBED_TAGS)" -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic $$(go list ./... | grep -v /test/smoke)
	$(GOCMD) tool cover -func=$(COVERAGE_FILE)

.PHONY: test-e2e
test-e2e: download-model download-ort ## Run end-to-end tests only
	$(GOENV) $(GOCMD) test -tags "$(EMBED_TAGS)" -v ./test/e2e/...

.PHONY: test-smoke
test-smoke: ## Run smoke tests (resets database for idempotency)
	$(GOTEST) -v -count=1 ./test/smoke/...

##@ Code Quality

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: lint-fix
lint-fix: format ## Run golangci-lint with auto-fix
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
	$(GOCMD) vet -tags "$(BUILD_TAGS)" ./...

.PHONY: check
check: fmt vet lint test ## Run all checks (format, vet, lint, test)

##@ Models

.PHONY: download-model
download-model: ## Convert and prepare the built-in embedding model (requires uv + Python)
	$(GOCMD) run ./cmd/download-model infrastructure/provider/models/flax-sentence-embeddings_st-codesearch-distilroberta-base

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
	$(GOCMD) install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
	$(GOCMD) install github.com/dense-analysis/openapi-spec-converter/cmd/openapi-spec-converter@latest

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
	openapi-spec-converter -t 3.0 -f json -o $(SWAGGER_DIR)/openapi.json $(SWAGGER_DIR)/swagger.json
	sed -i 's|"url":"https://[^/]*/|"url":"/|' $(SWAGGER_DIR)/openapi.json
	cp $(SWAGGER_DIR)/openapi.json $(OPENAPI_EMBED)
	@echo "OpenAPI 3.0 spec generated at $(OPENAPI_EMBED)"

.PHONY: openapi-convert
openapi-convert: ## Convert existing swagger.json to OpenAPI 3.0 (skip swag generation)
	@echo "Converting existing Swagger 2.0 to OpenAPI 3.0..."
	openapi-spec-converter -t 3.0 -f json -o $(OPENAPI_EMBED) $(SWAGGER_DIR)/swagger.json
	sed -i 's|"url":"https://[^/]*/|"url":"/|' $(OPENAPI_EMBED)
	@echo "OpenAPI 3.0 spec generated at $(OPENAPI_EMBED)"

generate-go-client: openapi
	./scripts/generate-go-client.sh

generate-clients: generate-go-client

.PHONY: docs
docs: openapi generate-clients
	uv run --script tools/dump-openapi.py
	uv run --script tools/dump-config.py

##@ Release

BUMP?=patch

.PHONY: release
release: ## Create a GitHub release (BUMP=patch|minor|major, RELEASE=1 for full release)
	@if [ "$(BUMP)" != "patch" ] && [ "$(BUMP)" != "minor" ] && [ "$(BUMP)" != "major" ]; then \
		echo "ERROR: BUMP must be patch, minor, or major (got '$(BUMP)')"; \
		exit 1; \
	fi; \
	LATEST=$$(git tag --list '[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | grep -v '\-' | head -1); \
	if [ -z "$$LATEST" ]; then \
		echo "ERROR: no semver tags found"; \
		exit 1; \
	fi; \
	MAJOR=$$(echo "$$LATEST" | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | cut -d. -f2); \
	PATCH=$$(echo "$$LATEST" | cut -d. -f3); \
	case "$(BUMP)" in \
		major) MAJOR=$$((MAJOR + 1)); MINOR=0; PATCH=0 ;; \
		minor) MINOR=$$((MINOR + 1)); PATCH=0 ;; \
		patch) PATCH=$$((PATCH + 1)) ;; \
	esac; \
	BRANCH=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$(RELEASE)" = "1" ]; then \
		TAG="$$MAJOR.$$MINOR.$$PATCH"; \
		echo "Creating release $$TAG on branch $$BRANCH..."; \
		gh release create "$$TAG" --title "$$TAG" --generate-notes --target "$$BRANCH"; \
	else \
		TAG="$$MAJOR.$$MINOR.$$PATCH-rc.$(COMMIT)"; \
		echo "Creating pre-release $$TAG on branch $$BRANCH..."; \
		gh release create "$$TAG" --title "$$TAG" --generate-notes --prerelease --target "$$BRANCH"; \
	fi
