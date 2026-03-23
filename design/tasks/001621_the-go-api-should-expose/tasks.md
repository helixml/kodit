# Implementation Tasks

- [ ] Add `PipelinePreset` type and constants (`pipelineDefault`, `PipelineRAGOnly`, `PipelineFull`) to `options.go`
- [ ] Add `pipeline PipelinePreset` field to `clientConfig` struct in `options.go`
- [ ] Implement `WithRAGPipeline() Option` in `options.go`
- [ ] Implement `WithFullPipeline() Option` in `options.go`
- [ ] Replace the `enrichments := cfg.textProvider != nil` line in `kodit.go` `New()` with a switch on `cfg.pipeline` (including error for `PipelineFull` without a text provider)
- [ ] Update the warning log so it only fires when the default preset is active and no text provider is configured (not when `WithRAGPipeline()` is explicitly chosen)
- [ ] Add unit tests for `WithRAGPipeline()`: verify prescribed ops exclude all LLM operations
- [ ] Add unit tests for `WithFullPipeline()`: verify it errors without a text provider
- [ ] Add unit tests for backward compatibility: verify default preset is unchanged
- [ ] Update the package-level doc comment in `kodit.go` with a `WithRAGPipeline` usage example
