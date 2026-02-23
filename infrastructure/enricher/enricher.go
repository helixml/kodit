// Package enricher provides AI-powered enrichment generation.
package enricher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/infrastructure/provider"
)

// ProviderEnricher uses a TextGenerator to create enrichments.
type ProviderEnricher struct {
	generator   provider.TextGenerator
	maxTokens   int
	temperature float64
	parallelism int
}

// NewProviderEnricher creates a new ProviderEnricher.
func NewProviderEnricher(generator provider.TextGenerator) *ProviderEnricher {
	return &ProviderEnricher{
		generator:   generator,
		maxTokens:   2048,
		temperature: 0.7,
		parallelism: 1,
	}
}

// WithMaxTokens sets the maximum tokens for generation.
func (e *ProviderEnricher) WithMaxTokens(n int) *ProviderEnricher {
	e.maxTokens = n
	return e
}

// WithTemperature sets the temperature for generation.
func (e *ProviderEnricher) WithTemperature(t float64) *ProviderEnricher {
	e.temperature = t
	return e
}

// WithParallelism sets how many requests are dispatched concurrently.
// Values <= 0 are clamped to 1.
func (e *ProviderEnricher) WithParallelism(n int) *ProviderEnricher {
	if n <= 0 {
		n = 1
	}
	e.parallelism = n
	return e
}

// Enrich processes requests in parallel and returns responses.
// Implements domainservice.Enricher interface.
func (e *ProviderEnricher) Enrich(ctx context.Context, requests []domainservice.EnrichmentRequest, opts ...domainservice.EnrichOption) ([]domainservice.EnrichmentResponse, error) {
	cfg := domainservice.NewEnrichConfig(opts...)

	var filtered []int
	for i, req := range requests {
		if req.Text() != "" {
			filtered = append(filtered, i)
		}
	}

	if len(filtered) == 0 {
		return []domainservice.EnrichmentResponse{}, nil
	}

	total := len(filtered)
	responses := make([]domainservice.EnrichmentResponse, total)

	var (
		mu            sync.Mutex
		requestErrors []error
		completed     int32
	)

	sem := make(chan struct{}, e.parallelism)
	var wg sync.WaitGroup

	for slot, reqIdx := range filtered {
		if err := ctx.Err(); err != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(slot, reqIdx int) {
			defer wg.Done()
			defer func() { <-sem }()

			req := requests[reqIdx]
			resp, err := e.processRequest(ctx, req)
			if err != nil {
				mu.Lock()
				requestErrors = append(requestErrors, fmt.Errorf("enrich request %s: %w", req.ID(), err))
				mu.Unlock()
				if cfg.RequestError() != nil {
					cfg.RequestError()(req.ID(), err)
				}
				return
			}

			responses[slot] = resp

			done := int(atomic.AddInt32(&completed, 1))
			if cfg.Progress() != nil {
				cfg.Progress()(done, total)
			}
		}(slot, reqIdx)
	}

	wg.Wait()

	if len(requestErrors) > 0 {
		rate := float64(len(requestErrors)) / float64(total)
		if rate > cfg.MaxFailureRate() {
			return nil, fmt.Errorf("%d of %d enrichment requests failed: %w", len(requestErrors), total, errors.Join(requestErrors...))
		}
	}

	// Filter out zero-value responses (failed slots).
	result := make([]domainservice.EnrichmentResponse, 0, total-len(requestErrors))
	for _, resp := range responses {
		if resp.ID() != "" {
			result = append(result, resp)
		}
	}

	return result, nil
}

func (e *ProviderEnricher) processRequest(ctx context.Context, req domainservice.EnrichmentRequest) (domainservice.EnrichmentResponse, error) {
	messages := []provider.Message{
		provider.SystemMessage(req.SystemPrompt()),
		provider.UserMessage(req.Text()),
	}

	chatReq := provider.NewChatCompletionRequest(messages).
		WithMaxTokens(e.maxTokens).
		WithTemperature(e.temperature)

	chatResp, err := e.generator.ChatCompletion(ctx, chatReq)
	if err != nil {
		return domainservice.EnrichmentResponse{}, err
	}

	content := cleanThinkingTags(chatResp.Content())

	return domainservice.NewEnrichmentResponse(req.ID(), content), nil
}

// cleanThinkingTags removes any <think>...</think> tags from model output.
// Some models (like Qwen) use these for chain-of-thought reasoning.
func cleanThinkingTags(text string) string {
	// Simple approach: look for <think> and </think> tags and remove them
	result := text
	for {
		start := indexOf(result, "<think>")
		if start == -1 {
			break
		}
		end := indexOf(result, "</think>")
		if end == -1 {
			// Unclosed tag, just remove the opening tag
			result = result[:start] + result[start+7:]
			continue
		}
		// Remove the entire think block
		result = result[:start] + result[end+8:]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Ensure ProviderEnricher implements domainservice.Enricher.
var _ domainservice.Enricher = (*ProviderEnricher)(nil)
