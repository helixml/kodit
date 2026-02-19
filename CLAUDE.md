# Helix Development Rules

**Current year: 2026** - When searching for browser API support, documentation, or library versions, include "2026" in searches to get current information.

## Build Pipeline

**Architecture**: Host → Kodit

### Component Dependencies

```
Kodit (outer container)
└── Database (VectorChord or SQLite)
```

### Build Instructions

```bash
# 1. Build Kodit
make build

# 2. Lint, format, and vet Kodit
make check

# 3. Test Kodit
make test

# 4. Run the smoke tests
make smoke
```

## Code Patterns

### General

Strive to develop in an object oriented way using the following guidelines:

- Class Naming: Name according to what it is, not what it does. Don't end names with er. (1.1)
- Method naming: Methods are either a builder or a manipulator. Never both. Use a noun if it's a builder, or a verb if it's a manipulator. (2.4)
- Variable naming: If you can't explain the code using single and plural nouns, refactor. Avoid compound names where possible. (5.1)
- Constructors: Make one constructor primary, others must use this constructor. (1.2) Keep constructors free of code. (1.3) Don't use new anywhere except secondary constructors. (3.6)
- Methods: Expose fewer than five public methods. (3.1) Don't use static methods. (3.2) Never accept null arguments, encapsulate. (3.3) Don't use getters and setters. (3.5) Never return null. (4.1)
- Encapsulation: Classes should encapsulate four objects or less. (2.1) Use composition, not inheritance. (5.7)
- Decoupling: Use interfaces where possible. (2.3) Keep interfaces small (2.9)
- Globals: Don't use public constants or enums, use classes instead. (2.5)
- Immutability: Make all classes immutable. (2.6) Avoid type introspection and reflection. (3.7, 6.4)
- Testing: Don't mock, use real objects where possible. If not possible, use fakes. (2.8)
- Design: Think in objects, not algorithms. (5.10) Design methods by telling them what you want, don't ask for data. (5.3)

Stylistic requirements:

- Place all imports at the TOP of the file.

### Go

- Fail fast: `return fmt.Errorf("failed: %w", err)` — never log and continue
- **Error on missing configuration**: If something is expected to be available (project settings, MCP servers, database records), fail with an error rather than silently continuing without it. Users expect configured features to work — logging a warning and continuing leaves them wondering why things are broken.
- Use structs, not `map[string]interface{}`
- GORM AutoMigrate only — no SQL migration files
- Use gomock, not testify/mock
- **NO FALLBACKS**: Pick one approach that works and stick to it. Fallback code paths are rarely tested and add complexity. If something doesn't work, fix it properly instead of adding a fallback.
- **NO TYPE ALIASES**: Always update references when moving or renaming types.
- **NO PANICS**: Never panic. If something goes wrong, return an error. If you can't return an error, rewrite the method so that it can error. In general, write all possible code with an error return variable, even if it's not used.
- **Log errors once at the top level**: Infrastructure and domain code should return errors, not log them. Only the outermost application service (e.g., worker, API handler) should log errors. This prevents duplicate error output.

## Repositories and Database Stores

Every store **embeds** `database.Repository[D, E]` (`internal/database/repository.go`).
This provides `Find`, `FindOne`, `Count`, `Exists`, `DeleteBy`, `Mapper()`, and
`DB(ctx)`. DO NOT ADD NEW METHODS TO A DERIVATIVE REPOSITORY. Use the repository.Option
to provide custom behaviour. Push the filtering down into the options infrastructure
instead of overriding.

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

**Use `Find` with options for lookups** — not raw GORM queries or one-off `Get`/`GetBy` methods. Define typed options in `domain/<domain>/options.go` using `repository.WithCondition`:

```go
// Option:
func WithSHA(sha string) Option { return WithCondition("commit_sha", sha) }

// Usage:
commits, err := store.Find(ctx, repository.WithRepoID(id), repository.WithSHA(sha))
one, err := store.FindOne(ctx, repository.WithID(id))
```

For JOINs or non-column filters, use `repository.WithParam` to pass data, then override `Find` in the store to apply the JOIN. See `EnrichmentStore` for the pattern.

**Do not:**

- Store separate `db` or `mapper` fields — use `s.DB(ctx)` and `s.Mapper()` from the embedding
- Rewrite `Find`/`FindOne`/`Count` unless you need JOINs
- Write raw `WHERE` clauses for equality/IN filters — use `WithCondition`/`WithConditionIn`
- Add `Get(id)` or `GetByName(name)` methods — use `FindOne` with the right option

## Testing

Use the `internal/testdb` package for test databases. Do not create ad-hoc SQLite connections in tests.

```go
// Migrated in-memory database (most tests):
db := testdb.New(t)

// Plain in-memory database without migrations (custom schema):
db := testdb.NewPlain(t)

// Plain database with custom schema statements:
db := testdb.WithSchema(t, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
```

The `internal/database` package tests cannot use `testdb` (import cycle) and must create their own connections.

## Database Access

The Kodit database is PostgreSQL running in the `kodit-vectorchord` container:

```bash
# Query the database
docker exec kodit-vectorchord psql -U postgres -d postgres -c "SELECT * FROM repositories LIMIT 5;"

# Interactive psql session
docker exec -it kodit-vectorchord psql -U postgres -d postgres

**Note**: The database name is `kodit`, user is `postgres`. 
```
