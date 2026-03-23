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

The `enrichments` flag already controls whether LLM-based operations are included in all workflow sequences (`ScanAndIndexCommit`, `IndexCommit`, `RescanCommit`). Setting it to `false` already produces the correct "RAG-only" operation set:

| Skipped when `enrichments=false` |
|---|
| `OperationCreateCommitDescriptionForCommit` |
| `OperationCreateArchitectureEnrichmentForCommit` |
| `OperationCreateDatabaseSchemaForCommit` |
| `OperationCreateCookbookForCommit` |
| `OperationGenerateWikiForCommit` |
| `OperationCreateSummaryEmbeddingsForCommit` (nothing to embed without enrichments) |

The problem is this flag is set implicitly in `kodit.go`:

```go
// kodit.go line 324
enrichments := cfg.textProvider != nil
prescribedOps := task.NewPrescribedOperations(false, enrichments)
```

There is no public `Option` to override this decision.

## Proposed Solution: Named Pipeline Presets

Add a `PipelinePreset` type and two named `Option` functions. This is the simplest change that satisfies the requirements and follows the existing `Option` pattern in `options.go`.

### New Types in `options.go`

```go
// PipelinePreset selects a curated set of indexing operations.
type PipelinePreset int

const (
    pipelineDefault PipelinePreset = iota // use provider presence to decide
    PipelineRAGOnly                       // skip all LLM enrichments
    PipelineFull                          // require and run all enrichments
)
```

Add `pipeline PipelinePreset` to `clientConfig`.

### New Option Functions in `options.go`

```go
// WithRAGPipeline configures the indexing pipeline for RAG use cases.
// Only snippet extraction, BM25 indexing, code embeddings, and AST-based
// API docs are run. All LLM-based enrichments (commit descriptions,
// architecture docs, database schema, cookbook, wiki) are skipped even
// if a text provider is configured.
func WithRAGPipeline() Option {
    return func(c *clientConfig) { c.pipeline = PipelineRAGOnly }
}

// WithFullPipeline runs all indexing operations including LLM-based
// enrichments. A text provider must be configured or New() returns an error.
// This is the default when a text provider is configured.
func WithFullPipeline() Option {
    return func(c *clientConfig) { c.pipeline = PipelineFull }
}
```

### Change in `kodit.go` `New()`

Replace the current one-liner with a switch on the preset:

```go
var enrichments bool
switch cfg.pipeline {
case PipelineRAGOnly:
    enrichments = false
case PipelineFull:
    if cfg.textProvider == nil {
        return nil, fmt.Errorf("WithFullPipeline requires a text provider (WithOpenAI, WithAnthropic, or WithTextProvider)")
    }
    enrichments = true
default: // pipelineDefault — preserve existing behaviour
    enrichments = cfg.textProvider != nil
}
prescribedOps := task.NewPrescribedOperations(false, enrichments)
```

### Handler Registration

No changes needed to `registerHandlers()`. LLM handlers are still registered when a text provider is set — they just receive no tasks when the prescribed operations exclude them. `validateHandlers()` checks only prescribed operations against registered handlers, so it passes correctly in all cases.

### Warning Log Adjustment

The existing warning at line 326-328:

```go
if !enrichments {
    logger.Warn().Msg("enrichment endpoint not configured — ...")
}
```

Should be updated to only warn when `pipeline == pipelineDefault && !enrichments`, so `WithRAGPipeline()` users don't see a confusing warning.

## Key Design Decisions

**Named presets over fine-grained toggles.** Individual operation toggles would require users to understand inter-operation dependencies (e.g. summary embeddings depend on enrichments existing). Named presets encode those dependencies correctly without exposing them.

**Preset overrides provider-based inference.** The preset wins unconditionally over `textProvider != nil`. This makes behavior predictable and explicit.

**`PipelineFull` errors without a text provider.** Consistent with the existing fail-fast philosophy in CLAUDE.md: "Error on missing configuration". Silently downgrading to RAG-only when `WithFullPipeline()` is requested would be surprising.

**No changes to `PrescribedOperations`.** The domain type already supports this distinction. The fix is purely in the Go API layer (`options.go` + `kodit.go`).

## Codebase Patterns (for future agents)

- All public `Option` functions live in `options.go` and modify `clientConfig`
- `clientConfig` is internal; only `Option` functions are public
- `PrescribedOperations` is in `domain/task/operation.go` and controls which operations are enqueued throughout the system (passed to `service.Repository`, `service.PeriodicSync`, `service.Worker`)
- `validateHandlers()` in `handlers.go` cross-checks `prescribedOps.All()` against `registry.Operations()` at startup
- The project uses `make check` for all linting/testing — never raw `go test`
