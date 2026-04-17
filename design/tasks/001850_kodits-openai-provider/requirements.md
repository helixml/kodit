# Requirements: Default max_tokens to 0 (unset)

## Context

Kodit's OpenAI provider (`openai.go:209`) only sends `max_tokens` when the value is > 0. The enricher hardcodes `maxTokens: 2048` (`enricher.go:27`), so every request sends `max_tokens: 2048`. This caps output at 2048 tokens even though the vLLM server's `max_model_len` is 32768. When `max_tokens` is omitted, vLLM defaults to the full remaining context budget — giving thinking models room to breathe.

## User Stories

**As a Kodit operator**, I want the enricher to not send `max_tokens` by default, so that vLLM uses the full remaining context budget without manual configuration.

## Acceptance Criteria

- [ ] `ProviderEnricher` defaults `maxTokens` to `0` (was `2048`)
- [ ] `DefaultEndpointMaxTokens` defaults to `0` (was `4000`)
- [ ] The env var `MAX_TOKENS` still overrides the default when set explicitly
- [ ] When `maxTokens` is `0`, the OpenAI provider omits `max_tokens` from the request (existing behaviour at `openai.go:209` — no change needed)
- [ ] Existing tests pass; update any assertions that check the old default values
