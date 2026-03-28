---
title: "Kodit Developer Documentation"
linkTitle: Developer Docs
weight: 99
---

## Architecture

Kodit follows a layered architecture:

- **`cmd/kodit/`** — CLI entry point with `serve` and `version` commands
- **`kodit.go`** / **`options.go`** — Public library API (`kodit.New()`, `kodit.Client`)
- **`application/`** — Application services and task handlers
- **`domain/`** — Domain types (enrichment, repository, search, task)
- **`infrastructure/`** — External integrations (API, database, git, providers, enrichers)
- **`internal/`** — Internal packages (config, database, MCP, logging, testdb)
- **`clients/go/`** — Generated Go HTTP client from OpenAPI spec

## Database

All database operations are handled by GORM with AutoMigrate. There are no SQL migration
files — schema changes are applied automatically when the server starts.

### Making Schema Changes

1. Update the GORM model structs in the `infrastructure/persistence/` package
2. GORM AutoMigrate will apply the changes on the next server start
3. For destructive changes (dropping columns, renaming tables), you may need to handle
   the migration manually via a one-off script

## Building

```bash
# Build the binary (downloads model + ORT library, builds with CGo)
make build

# Run tests
make test

# Run tests for a specific package
make test PKG=./internal/foo/...

# Run all checks (format, vet, lint, test)
make check

# Run the smoke tests (requires Docker)
make test-smoke
```

## Documentation Generation

```bash
# Generate OpenAPI spec, Go client, API reference, and config reference
make docs

# Generate OpenAPI spec only
make openapi

# Generate Go client only
make generate-clients
```

## Releasing

Performing a release is designed to be fully automated. If you spot opportunities to
improve the CI to help performing an automated release, please do so.

1. Create a new release in GitHub (or use `make release BUMP=patch|minor|major RELEASE=1`).
2. Set the version number. Use patch versions for bugfixes or minor small improvements.
   Use minor versions when adding significant new functionality. Use major versions for
   overhauls.
3. Generate the release notes.
4. Wait for all jobs to succeed, then the Docker image will be published.
