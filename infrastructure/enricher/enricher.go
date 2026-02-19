// Package enricher provides AI-powered enrichment generation.
package enricher

import (
	"context"
	"fmt"
	"log/slog"

	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/infrastructure/provider"
)

// ProviderEnricher uses a TextGenerator to create enrichments.
type ProviderEnricher struct {
	generator   provider.TextGenerator
	maxTokens   int
	temperature float64
	log         *slog.Logger
}

// NewProviderEnricher creates a new ProviderEnricher.
func NewProviderEnricher(generator provider.TextGenerator, log *slog.Logger) *ProviderEnricher {
	return &ProviderEnricher{
		generator:   generator,
		maxTokens:   2048,
		temperature: 0.7,
		log:         log,
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

// Enrich processes requests sequentially and returns responses.
// Implements domainservice.Enricher interface.
func (e *ProviderEnricher) Enrich(ctx context.Context, requests []domainservice.EnrichmentRequest) ([]domainservice.EnrichmentResponse, error) {
	var filtered []domainservice.EnrichmentRequest
	for _, req := range requests {
		if req.Text() != "" {
			filtered = append(filtered, req)
		}
	}

	if len(filtered) == 0 {
		e.log.Warn("no valid requests for enrichment")
		return nil, nil
	}

	responses := make([]domainservice.EnrichmentResponse, 0, len(filtered))

	for _, req := range filtered {
		select {
		case <-ctx.Done():
			return responses, ctx.Err()
		default:
		}

		response, err := e.processRequest(ctx, req)
		if err != nil {
			return responses, fmt.Errorf("enrich request %s: %w", req.ID(), err)
		}

		responses = append(responses, response)
	}

	return responses, nil
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
