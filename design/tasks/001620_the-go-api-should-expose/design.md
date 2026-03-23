# Design: Pipeline Customisation via Go API

## How The Pipeline Works Today

The internal type `task.PrescribedOperations` (`domain/task/operation.go`) controls which operations are enqueued. It has two boolean flags: `examples` and `enrichments`. When `enrichments` is false, all LLM-dependent operations are excluded from every workflow method (`ScanAndIndexCommit`, `IndexCommit`, `RescanCommit`).

The problem: `PrescribedOperations` is created in `kodit.go:324-325` with `enrichments` derived solely from whether a text provider is present:

```go
enrichments := cfg.textProvider != nil
prescribedOps := task.NewPrescribedOperations(false, enrichments)
```

There is no public way to override this. A caller who configures OpenAI (for embeddings) automatically gets all LLM enrichments too.

## Solution

Add a single new option to the public API that explicitly overrides the derived `enrichments` flag.

### 1. `options.go` — new field + option function

```go
// clientConfig gets one new field:
disableEnrichments bool

// New exported option:
// WithoutEnrichments disables all LLM-based enrichment pipeline stages
// (wiki generation, commit descriptions, architecture discovery, database
// schema, and cookbook). The indexing pipeline will still produce code
// chunks, BM25 indices, and vector embeddings. Use this when running Kodit
// as a pure RAG provider.
func WithoutEnrichments() Option {
    return func(c *clientConfig) {
        c.disableEnrichments = true
    }
}
```

### 2. `kodit.go` — update derivation logic

Change line 324 from:
```go
enrichments := cfg.textProvider != nil
```
to:
```go
enrichments := cfg.textProvider != nil && !cfg.disableEnrichments
```

The log warning at line 326-328 is currently printed whenever `enrichments` is false. With this change it fires when enrichments are disabled AND the caller has NOT explicitly opted out. Update the condition:

```go
if !enrichments && !cfg.disableEnrichments {
    logger.Warn().Msg("enrichment endpoint not configured — ...")
}
```

That's the entire change. No new types, no new packages.

## Example Usage

```go
// Pure RAG: embeddings only, no LLM enrichments
client, err := kodit.New(
    kodit.WithSQLite(".kodit/data.db"),
    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
    kodit.WithoutEnrichments(),
)

// Full pipeline (unchanged default behaviour)
client, err := kodit.New(
    kodit.WithSQLite(".kodit/data.db"),
    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
)
```

## Decision Notes

- **Why a boolean flag rather than a `PipelineConfig` struct?** The concrete request is a single toggle: all LLM enrichments or none. Adding a struct for one field is premature. The `disableEnrichments` field can be joined by more pipeline flags later if per-enrichment-type control is needed.
- **Why `WithoutEnrichments()` rather than `WithEnrichments(bool)`?** The positive-logic variant has a safe default (false = don't disable) and reads clearly at the call site without the user having to pass `false`. The convention in this codebase is `WithSkipProviderValidation()` style for opt-out flags.
- **Why not expose `PrescribedOperations` directly?** It is an internal domain type. Leaking it into the public API would couple callers to the task queue abstraction.

## Codebase Patterns

- All public client configuration uses `func(*clientConfig)` options in `options.go`
- Derived/computed values live in `kodit.go` in the `New()` function, not in `clientConfig`
- The `examples` flag in `PrescribedOperations` is currently always `false` at the call site in `kodit.go:325`; it is only set to `true` in tests
