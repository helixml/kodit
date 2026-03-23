# Implementation Tasks

- [~] Add named constructors `DefaultPrescribedOperations`, `RAGOnlyPrescribedOperations`, and `FullPrescribedOperations` to `domain/task/operation.go`
- [~] Remove `NewPrescribedOperations(examples, enrichments bool)` and migrate all call sites to the named constructors
- [ ] Add `prescribedOpsFactory func(hasTextProvider bool) task.PrescribedOperations`, `requiresTextProvider bool`, and `explicitPipeline bool` fields to `clientConfig` in `options.go`; set `prescribedOpsFactory` default to `task.DefaultPrescribedOperations` in `newClientConfig()`
- [ ] Implement `WithRAGPipeline() Option` in `options.go`
- [ ] Implement `WithFullPipeline() Option` in `options.go`
- [ ] Replace the `enrichments` boolean derivation and `task.NewPrescribedOperations` call in `kodit.go` `New()` with a validation check (`requiresTextProvider`) and a single `cfg.prescribedOpsFactory(cfg.textProvider != nil)` call
- [ ] Update the warning log so it only fires when `!explicitPipeline && cfg.textProvider == nil`
- [ ] Add unit tests: `WithRAGPipeline()` excludes all LLM operations from prescribed ops
- [ ] Add unit tests: `WithFullPipeline()` errors when no text provider is configured
- [ ] Add unit tests: default behaviour is unchanged (backward compatibility)
- [ ] Update the package-level doc comment in `kodit.go` with a `WithRAGPipeline` usage example
