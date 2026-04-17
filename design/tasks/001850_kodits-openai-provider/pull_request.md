# fix: default max_tokens to 0 (unset)

## Summary
Stop sending `max_tokens: 2048` on every OpenAI request so vLLM can use its full `max_model_len` budget. The OpenAI provider already omits `max_tokens` when 0 — this change just makes 0 the default.

## Changes
- `DefaultEndpointMaxTokens`: `4000` → `0` (`internal/config/config.go`)
- Enricher default `maxTokens`: `2048` → `0` (`infrastructure/enricher/enricher.go`)
- Env struct tag default: `4000` → `0` (`internal/config/env.go`)
- Updated test assertions to match new defaults

## Testing
`make check` passes for `./internal/config/...` and `./infrastructure/enricher/...`.
