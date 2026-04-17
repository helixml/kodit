# Design: Default max_tokens to 0 (unset)

## Overview

Change two default values so that `max_tokens` is omitted from OpenAI API requests by default. The conditional send logic in `openai.go:209` already handles the `0` case correctly — it only sends `max_tokens` when `> 0`. No structural changes needed.

## What Changes

| Location | Field | Old Default | New Default |
|---|---|---|---|
| `infrastructure/enricher/enricher.go:27` | `maxTokens` in `NewProviderEnricher` | `2048` | `0` |
| `internal/config/config.go:27` | `DefaultEndpointMaxTokens` | `4000` | `0` |

## What Stays the Same

- `openai.go:209-211` — the `if req.MaxTokens() > 0` guard already does the right thing
- `provider.go:70` — `ChatCompletionRequest` already defaults to `0`
- `WithMaxTokens()` methods — callers can still override to any positive value
- `env.go:158` — `MAX_TOKENS` env var still works; operators who need a cap can set it explicitly (note: the struct tag default was also updated from `4000` to `0`)
- `anthropic.go:219-221` — Anthropic provider defaults `0` to `4096` independently; unaffected

## Rationale

- **Chose default-to-0 over configurable-per-model**: the user's request is clear — let vLLM decide. Operators who need a cap already have the `MAX_TOKENS` env var.
- **Two defaults, not one**: the enricher and the endpoint config have separate defaults. Both must change to fully resolve the issue.

## Risks

- **Runaway token usage**: without a cap, responses could be longer and cost more. Mitigated by the fact that operators can still set `MAX_TOKENS` explicitly.
- **Anthropic provider unaffected**: it has its own fallback to `4096` when `maxTokens == 0`, so this change doesn't alter Anthropic behaviour.

## Implementation Notes

- Three defaults needed changing, not two: the constant (`config.go`), the enricher struct (`enricher.go`), and the env struct tag (`env.go`). The struct tag default must match the constant or `TestEnvDefaults_MatchConfigDefaults` fails.
- The enricher is wired up in `kodit.go:411` without calling `WithMaxTokens()`, so it always uses its own hardcoded default — changing the config constant alone would not fix the enricher path.
- `make tools` must be run first to install `golangci-lint` into `~/go/bin`; the Makefile expects it on PATH.
