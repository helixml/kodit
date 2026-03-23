# Design: Pipeline Preset Options

## Current Architecture

The pipeline is controlled by `task.PrescribedOperations`, a value type created at client startup:

```go
// domain/task/operation.go
type PrescribedOperations struct {
    examples    bool
    enrichments bool
}
```

The `enrichments` flag controls whether LLM-based operations are included in all workflow sequences (`ScanAndIndexCommit`, `IndexCommit`, `RescanCommit`). Setting it to `false` already produces the correct "RAG-only" operation set:

| Skipped when `enrichments=false` |
|---|
| `OperationCreateCommitDescriptionForCommit` |
| `OperationCreateArchitectureEnrichmentForCommit` |
| `OperationCreateDatabaseSchemaForCommit` |
| `OperationCreateCookbookForCommit` |
| `OperationGenerateWikiForCommit` |
| `OperationCreateSummaryEmbeddingsForCommit` (nothing to embed without enrichments) |

The problem is this is set implicitly in `kodit.go` via a boolean factory:

```go
// kodit.go line 324-325 — current code
enrichments := cfg.textProvider != nil
prescribedOps := task.NewPrescribedOperations(false, enrichments)
```

Two issues: there is no public `Option` to override this decision, and `NewPrescribedOperations(bool, bool)` does not scale — a third pipeline variant would need a third boolean parameter.

## Proposed Solution: Named Constructors + Factory Function

Replace the boolean factory with named constructors in the `task` package, and store the chosen constructor as a factory function in `clientConfig`. This eliminates any switch statement in `New()` and means adding a new pipeline preset requires only a new named constructor and a new `WithXxxPipeline()` option — no changes to `New()`.

### Step 1 — Named constructors in `domain/task/operation.go`

Replace `NewPrescribedOperations(examples, enrichments bool)` with three named constructors that encode their semantics:

```go
// DefaultPrescribedOperations returns the standard operation set.
// LLM enrichments are included only when a text provider is available.
// This preserves backward-compatible behaviour.
func DefaultPrescribedOperations(hasTextProvider bool) PrescribedOperations {
    return PrescribedOperations{enrichments: hasTextProvider}
}

// RAGOnlyPrescribedOperations returns the operation set for RAG use cases:
// snippet extraction, BM25 indexing, code embeddings, and AST-based API docs.
// All LLM enrichments are excluded.
func RAGOnlyPrescribedOperations() PrescribedOperations {
    return PrescribedOperations{enrichments: false}
}

// FullPrescribedOperations returns the complete operation set including all
// LLM enrichments. The caller must ensure a text provider is configured.
func FullPrescribedOperations() PrescribedOperations {
    return PrescribedOperations{enrichments: true}
}
```

`NewPrescribedOperations` is removed; all callers (internal and external) migrate to the named constructors.

### Step 2 — Factory function in `clientConfig` (`options.go`)

Instead of a preset enum, `clientConfig` stores the factory function directly:

```go
// prescribedOpsFactory produces the PrescribedOperations at client creation time.
// hasTextProvider is true when a text generation provider is configured.
// Defaults to task.DefaultPrescribedOperations.
type clientConfig struct {
    // ...existing fields...
    prescribedOpsFactory      func(hasTextProvider bool) task.PrescribedOperations
    requiresTextProvider      bool  // set by WithFullPipeline to trigger validation
}

func newClientConfig() *clientConfig {
    return &clientConfig{
        // ...
        prescribedOpsFactory: task.DefaultPrescribedOperations,
    }
}
```

### Step 3 — New option functions (`options.go`)

```go
// WithRAGPipeline configures the indexing pipeline for RAG use cases.
// Snippet extraction, BM25 indexing, code embeddings, and AST-based API docs
// run. All LLM enrichments (commit descriptions, architecture docs, database
// schema, cookbook, wiki) are skipped even if a text provider is configured.
func WithRAGPipeline() Option {
    return func(c *clientConfig) {
        c.prescribedOpsFactory = func(_ bool) task.PrescribedOperations {
            return task.RAGOnlyPrescribedOperations()
        }
    }
}

// WithFullPipeline runs all indexing operations including LLM enrichments.
// A text provider must be configured or New() returns an error.
func WithFullPipeline() Option {
    return func(c *clientConfig) {
        c.requiresTextProvider = true
        c.prescribedOpsFactory = func(_ bool) task.PrescribedOperations {
            return task.FullPrescribedOperations()
        }
    }
}
```

### Step 4 — Simplified `New()` (`kodit.go`)

The switch statement is gone. `New()` validates once, then calls the factory:

```go
// Validate pipeline requirements
if cfg.requiresTextProvider && cfg.textProvider == nil {
    return nil, fmt.Errorf("WithFullPipeline requires a text provider (WithOpenAI, WithAnthropic, or WithTextProvider)")
}

prescribedOps := cfg.prescribedOpsFactory(cfg.textProvider != nil)

// Only warn when no explicit pipeline was chosen and enrichments are absent
if cfg.prescribedOpsFactory == nil /* unreachable */ || prescribedOps == (task.PrescribedOperations{}) {
    // ...
}
```

For the warning, track intent separately:

```go
// In clientConfig
explicitPipeline bool  // true when any WithXxxPipeline() option was set
```

Set `explicitPipeline = true` in both `WithRAGPipeline()` and `WithFullPipeline()`. The existing warning fires only when `!explicitPipeline && textProvider == nil`.

### Handler Registration

No changes to `registerHandlers()`. LLM handlers are still registered when a text provider is set — they simply receive no tasks when the prescribed operations exclude them. `validateHandlers()` validates only prescribed operations against registered handlers and passes correctly in all cases.

## Key Design Decisions

**Named constructors, not a boolean factory.** `NewPrescribedOperations(false, true)` communicates nothing. Named constructors (`RAGOnlyPrescribedOperations`, `FullPrescribedOperations`) are self-documenting and each new preset adds a new function, not a new boolean parameter.

**Factory function in `clientConfig`, not an enum.** Storing the factory directly means `New()` never needs a switch. Adding a fourth pipeline preset — `WithExamplesOnlyPipeline()`, say — requires only a new named constructor and a new `WithXxxPipeline()` option. `New()` is unchanged.

**Named presets over fine-grained toggles.** Individual per-operation flags would require users to understand inter-operation dependencies (e.g. summary embeddings depend on enrichments existing). Named presets encode those dependencies correctly without exposing them.

**`WithFullPipeline` errors without a text provider.** Consistent with the codebase's fail-fast rule: "Error on missing configuration" (CLAUDE.md). Silently downgrading to RAG-only when the caller explicitly requested full enrichments would be surprising.

## Codebase Patterns (for future agents)

- All public `Option` functions live in `options.go` and modify `clientConfig`
- `clientConfig` is internal; only `Option` functions are public
- `PrescribedOperations` is in `domain/task/operation.go` and is passed to `service.Repository`, `service.PeriodicSync`, and `service.Worker` — it controls which operations are enqueued across the entire system
- `validateHandlers()` in `handlers.go` cross-checks `prescribedOps.All()` against `registry.Operations()` at startup
- The project uses `make check` for all linting/testing — never raw `go test`
