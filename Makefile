# Kodit Go Makefile
# Build, test, lint, format, and run commands for the Kodit Go application

# Go parameters
GOCMD=go
TAGS=fts5 ORT
ORT_VERSION?=1.23.2
BUILD_TAGS=$(TAGS) embed_model
CGO_ENABLED?=1
ORT_LIB_DIR?=$(CURDIR)/lib
CGO_LDFLAGS?=-L$(ORT_LIB_DIR)
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

.PHONY: build-all
build-all: download-model ## Build for all platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

.PHONY: run
run: ## Run the HTTP server (downloads model on first use if needed)
	$(GORUN) $(CMD_DIR) serve

.PHONY: run-stdio
run-stdio: ## Run the MCP server on stdio
	$(GORUN) $(CMD_DIR) stdio

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf lib/
	rm -f $(COVERAGE_FILE)

##@ Testing

.PHONY: test
test: ## Run all tests (excludes smoke tests)
	$(GOTEST) -v $$(go list ./... | grep -v /test/smoke)

.PHONY: test-short
test-short: ## Run tests with short flag (skip long-running tests)
	$(GOTEST) -v -short ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	$(GOTEST) -v -race ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage (excludes smoke tests)
	$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic $$(go list ./... | grep -v /test/smoke)
	$(GOCMD) tool cover -func=$(COVERAGE_FILE)

.PHONY: test-cover-html
test-cover-html: test-cover ## Run tests with coverage and open HTML report
	$(GOCMD) tool cover -html=$(COVERAGE_FILE)

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests only
	$(GOTEST) -v ./test/e2e/...

.PHONY: smoke
smoke: download-model download-ort ## Run smoke tests (starts server, tests API endpoints)
	$(GOENV) $(GOCMD) test -tags "$(BUILD_TAGS)" -v -timeout 15m -count 1 ./test/smoke/...

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
	$(GOCMD) vet ./...

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

.PHONY: docs
docs: openapi ## Generate all documentation (OpenAPI 3.0)

##@ Database

.PHONY: migrate-up
migrate-up: ## Run database migrations up
	@echo "Database migrations are handled by GORM AutoMigrate"
	@echo "Run the server to apply migrations automatically"

.PHONY: migrate-down
migrate-down: ## Run database migrations down (not implemented)
	@echo "Rollback migrations are not implemented"
	@echo "GORM AutoMigrate only creates/updates, does not delete"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t kodit:$(VERSION) .

.PHONY: docker-build-multi
docker-build-multi: ## Build multi-platform Docker image
	docker buildx build --platform linux/amd64,linux/arm64 -t kodit:$(VERSION) .

.PHONY: docker-run
docker-run: ## Run Docker container
	docker run -p 8080:8080 kodit:$(VERSION)

##@ Release

.PHONY: release-dry
release-dry: ## Dry run release (show what would be built)
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo ""
	@echo "Would build:"
	@echo "  - $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"
	@echo "  - $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"
	@echo "  - $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"
	@echo "  - $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"

.PHONY: release
release: clean build-all ## Build release binaries for all platforms
	@echo "Release binaries built in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/
