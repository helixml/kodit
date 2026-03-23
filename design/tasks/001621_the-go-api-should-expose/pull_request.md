# Add WithRAGPipeline and WithFullPipeline options

## Summary

Exposes pipeline selection via the public Go API so callers can skip LLM-based
enrichments (wiki, summaries, architecture docs, cookbook, commit descriptions)
when using Kodit as a RAG provider.

## Changes

- `domain/task/operation.go` — replace boolean factory `NewPrescribedOperations(examples, enrichments bool)` with named constructors: `DefaultPrescribedOperations`, `RAGOnlyPrescribedOperations`, `FullPrescribedOperations`
- `options.go` — add `WithRAGPipeline()` and `WithFullPipeline()` option functions; store a factory function in `clientConfig` so no switch is needed in `New()` and adding future presets touches only this file
- `kodit.go` — call `cfg.prescribedOpsFactory(cfg.textProvider != nil)` instead of hardcoded boolean logic; validate `WithFullPipeline` requires a text provider early (before DB/model setup); suppress the no-provider warning when an explicit pipeline was chosen
- `kodit_test.go` — tests for `WithFullPipeline` error path and `WithRAGPipeline` success path

## Testing

- `TestWithFullPipeline_RequiresTextProvider` — verifies early error when no text provider is configured
- `TestWithRAGPipeline_WorksWithoutTextProvider` — verifies client starts successfully without a text provider
- `TestWithRAGPipeline_WorksWithTextProvider` — verifies text provider presence does not trigger enrichments
- All domain/task operation tests pass (covering the renamed constructors)
