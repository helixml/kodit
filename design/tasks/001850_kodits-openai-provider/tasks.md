# Implementation Tasks

- [~] Change `DefaultEndpointMaxTokens` from `4000` to `0` in `internal/config/config.go:27`
- [~] Change enricher default `maxTokens` from `2048` to `0` in `infrastructure/enricher/enricher.go:27`
- [ ] Update `internal/config/config_test.go` — assertion at line 107 expects `DefaultEndpointMaxTokens` (now `0`)
- [ ] Run `make check` and fix any other test assertions broken by the new defaults
