# Implementation Tasks

- [ ] Add `disableEnrichments bool` field to `clientConfig` in `options.go`
- [ ] Add `WithoutEnrichments() Option` function to `options.go` with godoc comment
- [ ] Update `kodit.go` line 324: change `enrichments := cfg.textProvider != nil` to `enrichments := cfg.textProvider != nil && !cfg.disableEnrichments`
- [ ] Update the enrichments warning log condition in `kodit.go` to only fire when enrichments are off AND the caller did not explicitly opt out (i.e. `!enrichments && !cfg.disableEnrichments`)
- [ ] Add a test in `kodit_test.go` (or nearest integration test) verifying that `WithoutEnrichments()` combined with a text provider results in no enrichment operations being enqueued
