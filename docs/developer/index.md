---
title: "Kodit Developer Documentation"
linkTitle: Developer Docs
weight: 99
---

## Database

All database operations are handled by GORM with AutoMigrate. There are no SQL migration
files -- schema changes are applied automatically when the server starts.

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

# Run the smoke tests (requires Docker)
make smoke

# Run the server locally
make run
```

## Releasing

Performing a release is designed to be fully automated. If you spot opportunities to
improve the CI to help performing an automated release, please do so.

1. Create a new release in GitHub.
2. Set the version number. Use patch versions for bugfixes or minor small improvements.
   Use minor versions when adding significant new functionality. Use major versions for
   overhauls.
3. Generate the release notes. <- this could be improved, because we use a strict
   pr/commit naming structure.
4. Wait for all jobs to succeed, then the Docker image will be published.
