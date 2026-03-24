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

Every store embeds `database.Repository[D, E]` (`internal/database/repository.go`) and implements `repository.Store[T]` (`domain/repository/store.go`).

**Prefer the generic methods from `Repository` and `Store`** — do not add custom query methods to stores. The base types already provide:

| Method | Source | Purpose |
|---|---|---|
| `Find(ctx, ...Option)` | Repository | List entities matching options |
| `FindOne(ctx, ...Option)` | Repository | Single entity or `ErrNotFound` |
| `Count(ctx, ...Option)` | Repository | Count matching entities |
| `Exists(ctx, ...Option)` | Repository | Check existence |
| `DeleteBy(ctx, ...Option)` | Repository | Remove matching entities |
| `Save(ctx, entity)` | Store impl | Create or update one entity |
| `Delete(ctx, entity)` | Store impl | Remove one entity |
| `DB(ctx)` | Repository | Raw GORM session (last resort) |
| `Mapper()` | Repository | Access the entity mapper |

**Use options to express queries** — not one-off methods. Define options in `domain/<domain>/options.go` using `repository.WithCondition`:

```go
func WithSHA(sha string) Option { return WithCondition("commit_sha", sha) }

commits, err := store.Find(ctx, repository.WithRepoID(id), repository.WithSHA(sha))
one, err := store.FindOne(ctx, repository.WithID(id))
count, err := store.Count(ctx, repository.WithRepoID(id))
exists, err := store.Exists(ctx, repository.WithSHA(sha))
err := store.DeleteBy(ctx, repository.WithRepoID(id))
saved, err := store.Save(ctx, commit)
```

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

For JOINs, use `repository.WithParam` and override `Find` in the store. See `EnrichmentStore`.

**Do not:** add `Get`/`GetBy`/`FindBy`/`DeleteByX` methods, store separate `db`/`mapper` fields, write raw `WHERE` clauses for equality filters, or rewrite `Find`/`FindOne`/`Count`/`Save`/`Delete`/`Exists`/`DeleteBy` unless JOINs are needed. If a query can be expressed with options, use the generic methods.

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

## Software Engineering

Object-oriented design principles:

- **Naming**: Classes by what they are, not what they do (avoid -er suffixes). Methods are builders (noun) or manipulators (verb), rarely both. Variables should be explainable as single/plural nouns; prefer simple names over compound ones.
- **Constructors**: One primary constructor, secondaries delegate to it. Keep constructors light. Prefer `new` only in secondary constructors.
- **Methods**: Prefer fewer than five public methods per class. Avoid static methods. Avoid null arguments and return values. Prefer richer encapsulation over getters/setters.
- **Encapsulation**: Prefer four or fewer encapsulated objects per class. Favour composition over inheritance.
- **Interfaces**: Prefer interfaces. Keep them small.
- **Immutability**: Default to immutable classes. Avoid type introspection and reflection unless the language idiom demands it.
- **No globals**: Prefer classes over public constants or enums.
- **Testing**: Prefer fakes over mocks.
- **Design**: Think in objects, not algorithms. Tell objects what you want; don't ask for data.
- **Boolean parameters**: Don't use a boolean to switch between fundamentally different behaviours (split the method or use polymorphism). Booleans are fine for orthogonal modifiers like filters or formatting options.
- **Dependency injection**: Use constructor injection. When a constructor accumulates many parameters, group related ones into a parameter/options object.
- **Language idioms**: Where these principles conflict with strong language conventions (e.g. Go exported struct fields, Python dataclasses), follow the language idiom and note the deviation.
