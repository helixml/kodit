# Implementation Tasks

- [x] Change `DefaultEndpointMaxTokens` from `4000` to `0` in `internal/config/config.go:27`
- [x] Change enricher default `maxTokens` from `2048` to `0` in `infrastructure/enricher/enricher.go:27`
- [x] Update `internal/config/config_test.go` — assertion at line 107 expects `DefaultEndpointMaxTokens` (now `0`)
- [x] Update env struct tag default from `4000` to `0` in `internal/config/env.go:158`
- [x] Run `make check` and fix any other test assertions broken by the new defaults
