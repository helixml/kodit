# Helix Development Rules

**Current year: 2026** — include "2026" in web searches for documentation and browser APIs.

## Build, Test, and Check

**IMPORTANT: Always use `make` commands. Never run `go test`, `go vet`, or `golangci-lint` directly.** The Makefile sets required build tags, CGO flags, and environment variables that raw `go` commands miss.

```bash
make build                       # Build the binary
make test                        # Run all tests
make test PKG=./internal/foo/... # Test a specific package
make check                       # Format, vet, lint, and test
make check PKG=./internal/foo/... # Check a specific package
make test-smoke                  # Run smoke tests (needs running Docker env)
```

## Go

- Fail fast: `return fmt.Errorf("failed: %w", err)` — never log and continue
- **Error on missing configuration** — fail with an error, don't log a warning and continue
- Use structs, not `map[string]interface{}`
- GORM AutoMigrate only — no SQL migration files
- Use gomock, not testify/mock
- **NO FALLBACKS** — one approach, no fallback code paths
- **NO TYPE ALIASES** — update all references when moving or renaming types
- **NO PANICS** — return errors; rewrite methods to support error returns if needed
- **Log errors once at the top level** — domain code returns errors, only handlers/workers log them

## Repositories and Database Stores

Every store embeds `database.Repository[D, E]` (`internal/database/repository.go`), which provides `Find`, `FindOne`, `Count`, `Exists`, `DeleteBy`, `Mapper()`, and `DB(ctx)`.

```go
type CommitStore struct {
    database.Repository[repository.Commit, CommitModel]
}

func NewCommitStore(db database.Database) CommitStore {
    return CommitStore{
        Repository: database.NewRepository[repository.Commit, CommitModel](db, CommitMapper{}, "commit"),
    }
}
```

**Use `Find` with options** — not raw GORM queries or one-off methods. Define options in `domain/<domain>/options.go` using `repository.WithCondition`:

```go
func WithSHA(sha string) Option { return WithCondition("commit_sha", sha) }

commits, err := store.Find(ctx, repository.WithRepoID(id), repository.WithSHA(sha))
one, err := store.FindOne(ctx, repository.WithID(id))
```

For JOINs, use `repository.WithParam` and override `Find` in the store. See `EnrichmentStore`.

**Do not:** add `Get`/`GetBy` methods, store separate `db`/`mapper` fields, write raw `WHERE` clauses for equality filters, or rewrite `Find`/`FindOne`/`Count` unless JOINs are needed.

## Testing

When the user says "tdd", follow red-green strictly:

1. **Red**: Write a failing test. Run it, confirm it fails.
2. **Green**: Minimal fix. Run test, confirm it passes.
3. Run the full test suite for regressions.

Use `internal/testdb` for test databases:

```go
db := testdb.New(t)                // Migrated in-memory database
db := testdb.NewPlain(t)           // Plain in-memory (no migrations)
db := testdb.WithSchema(t, "...")  // Plain with custom schema
```

The `internal/database` package tests cannot use `testdb` (import cycle) and must create their own connections.

## Database Access

PostgreSQL runs in the `kodit-vectorchord` container (database: `kodit`, user: `postgres`):

```bash
docker exec kodit-vectorchord psql -U postgres -d kodit -c "SELECT * FROM repositories LIMIT 5;"
```
