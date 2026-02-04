// Package enricher provides AI-powered enrichment generation.
package enricher

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/infrastructure/provider"
)

// Request represents an enrichment request with a custom system prompt.
type Request struct {
	id           string
	text         string
	systemPrompt string
}

// NewRequest creates a new enrichment request.
func NewRequest(id, text, systemPrompt string) Request {
	return Request{
		id:           id,
		text:         text,
		systemPrompt: systemPrompt,
	}
}

// ID returns the request identifier.
func (r Request) ID() string { return r.id }

// Text returns the text to be enriched.
func (r Request) Text() string { return r.text }

// SystemPrompt returns the custom system prompt.
func (r Request) SystemPrompt() string { return r.systemPrompt }

// Response represents an enrichment response.
type Response struct {
	id   string
	text string
}

// NewResponse creates a new enrichment response.
func NewResponse(id, text string) Response {
	return Response{id: id, text: text}
}

// ID returns the response identifier (matches the request ID).
func (r Response) ID() string { return r.id }

// Text returns the enriched text.
func (r Response) Text() string { return r.text }

// Enricher generates enrichments using an AI provider.
type Enricher interface {
	// Enrich processes requests and returns responses for each.
	Enrich(ctx context.Context, requests []Request) ([]Response, error)
}

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
func (e *ProviderEnricher) Enrich(ctx context.Context, requests []Request) ([]Response, error) {
	var filtered []Request
	for _, req := range requests {
		if req.text != "" {
			filtered = append(filtered, req)
		}
	}

	if len(filtered) == 0 {
		e.log.Warn("no valid requests for enrichment")
		return nil, nil
	}

	responses := make([]Response, 0, len(filtered))

	for _, req := range filtered {
		select {
		case <-ctx.Done():
			return responses, ctx.Err()
		default:
		}

		response, err := e.processRequest(ctx, req)
		if err != nil {
			e.log.Error("enrichment failed",
				"request_id", req.id,
				"error", err,
			)
			return responses, fmt.Errorf("enrich request %s: %w", req.id, err)
		}

		responses = append(responses, response)
	}

	return responses, nil
}

func (e *ProviderEnricher) processRequest(ctx context.Context, req Request) (Response, error) {
	messages := []provider.Message{
		provider.SystemMessage(req.systemPrompt),
		provider.UserMessage(req.text),
	}

	chatReq := provider.NewChatCompletionRequest(messages).
		WithMaxTokens(e.maxTokens).
		WithTemperature(e.temperature)

	chatResp, err := e.generator.ChatCompletion(ctx, chatReq)
	if err != nil {
		return Response{}, err
	}

	content := cleanThinkingTags(chatResp.Content())

	return NewResponse(req.id, content), nil
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

// Ensure ProviderEnricher implements Enricher.
var _ Enricher = (*ProviderEnricher)(nil)
