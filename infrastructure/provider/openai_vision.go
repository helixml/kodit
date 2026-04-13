package provider

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/helixml/kodit/domain/search"
)

// Default instructions for asymmetric retrieval. Qwen3-VL-Embedding uses
// different instructions for queries vs documents so that each is placed
// correctly in the vector space.
const (
	defaultVisionQueryInstruction    = "Retrieve images relevant to the user's query."
	defaultVisionDocumentInstruction = "Represent the image for retrieval."
)

// OpenAIVisionProvider embeds text or image inputs via an OpenAI-compatible
// vision-language embedding API (e.g. Qwen3-VL-Embedding). It implements
// Embedder and uses the vLLM "messages" format for all inputs so that
// the model's chat template is applied consistently across modalities.
type OpenAIVisionProvider struct {
	client              *openai.Client
	embeddingModel      string
	queryInstruction    string
	documentInstruction string
	maxRetries          int
	initialDelay        time.Duration
	backoffFactor       float64
}

// NewOpenAIVisionProvider creates a provider from configuration.
func NewOpenAIVisionProvider(cfg OpenAIConfig) *OpenAIVisionProvider {
	config := openai.DefaultConfig(cfg.APIKey)

	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}

	if cfg.HTTPClient != nil {
		config.HTTPClient = cfg.HTTPClient
	} else if cfg.Timeout > 0 {
		config.HTTPClient = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	client := openai.NewClientWithConfig(config)

	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "Qwen/Qwen3-VL-Embedding-8B"
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 5
	}

	initialDelay := cfg.InitialDelay
	if initialDelay == 0 {
		initialDelay = 2 * time.Second
	}

	backoffFactor := cfg.BackoffFactor
	if backoffFactor == 0 {
		backoffFactor = 2.0
	}

	queryInstruction := cfg.QueryInstruction
	if queryInstruction == "" {
		queryInstruction = defaultVisionQueryInstruction
	}

	documentInstruction := cfg.DocumentInstruction
	if documentInstruction == "" {
		documentInstruction = defaultVisionDocumentInstruction
	}

	return &OpenAIVisionProvider{
		client:              client,
		embeddingModel:      embeddingModel,
		queryInstruction:    queryInstruction,
		documentInstruction: documentInstruction,
		maxRetries:          maxRetries,
		initialDelay:        initialDelay,
		backoffFactor:       backoffFactor,
	}
}

// Close is a no-op for the remote provider.
func (p *OpenAIVisionProvider) Close() error {
	return nil
}

// Embed sends each item to the remote API using the vLLM "messages"
// format. Both text and image items are sent as chat messages because
// Qwen3-VL-Embedding applies a chat template that must be consistent
// across modalities for cross-modal search to work. Sending text queries
// via the plain "input" field would bypass the chat template, placing
// them in a different embedding space than image embeddings.
func (p *OpenAIVisionProvider) Embed(ctx context.Context, items []search.EmbeddingItem) ([][]float64, error) {
	if len(items) == 0 {
		return [][]float64{}, nil
	}

	embeddings := make([][]float64, len(items))

	for i, item := range items {
		vec, err := p.embedItem(ctx, item)
		if err != nil {
			return nil, fmt.Errorf("embed item %d: %w", i, err)
		}
		embeddings[i] = vec
	}

	return embeddings, nil
}

// embedItem sends a single item using the vLLM "messages" format.
// A system message with the appropriate instruction is prepended based on
// whether the item is a query or a document — this is the asymmetric
// retrieval pattern used by Qwen3-VL-Embedding.
func (p *OpenAIVisionProvider) embedItem(ctx context.Context, item search.EmbeddingItem) ([]float64, error) {
	content := buildMessageContent(item)

	var messages []map[string]any

	// Asymmetric retrieval: queries and documents get different instructions.
	instruction := p.documentInstruction
	if item.IsQuery() {
		instruction = p.queryInstruction
	}
	messages = append(messages, map[string]any{
		"role":    "system",
		"content": []map[string]any{{"type": "text", "text": instruction}},
	})

	messages = append(messages, map[string]any{
		"role":    "user",
		"content": content,
	})

	openaiReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(p.embeddingModel),
		Input: "",
		ExtraBody: map[string]any{
			"messages": messages,
		},
	}

	var resp openai.EmbeddingResponse
	var err error

	err = p.withRetry(ctx, func() error {
		resp, err = p.client.CreateEmbeddings(ctx, openaiReq)
		if err != nil {
			return err
		}
		if len(resp.Data) == 0 && string(resp.Model) == "" && resp.Usage.TotalTokens == 0 {
			return fmt.Errorf(
				"%w: provider returned HTTP 200 with no embedding data, no model, and zero usage (upstream routing failure)",
				errUpstreamProviderFailure,
			)
		}
		if len(resp.Data) == 0 {
			return fmt.Errorf("%w: got 0 vectors for 1 item", errEmbeddingCountMismatch)
		}
		return nil
	})

	if err != nil {
		return nil, p.wrapError("vision_embedding", err)
	}

	vec := make([]float64, len(resp.Data[0].Embedding))
	for j, v := range resp.Data[0].Embedding {
		vec[j] = float64(v)
	}
	return vec, nil
}

// buildMessageContent returns the "content" array for a chat message. Both
// text-only and image items are formatted as content parts so that vLLM's
// chat template processes them consistently, keeping text and image
// embeddings in the same vector space for cross-modal search.
func buildMessageContent(item search.EmbeddingItem) []map[string]any {
	var parts []map[string]any

	if item.HasImage() {
		dataURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(item.Image())
		parts = append(parts, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": dataURI,
			},
		})
	}

	if item.HasText() {
		parts = append(parts, map[string]any{
			"type": "text",
			"text": string(item.Text()),
		})
	}

	return parts
}

func (p *OpenAIVisionProvider) withRetry(ctx context.Context, fn func() error) error {
	delay := p.initialDelay
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if !p.isRetryable(lastErr) {
			return lastErr
		}

		if attempt < p.maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay = time.Duration(float64(delay) * p.backoffFactor)
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (p *OpenAIVisionProvider) isRetryable(err error) bool {
	if errors.Is(err, errEmbeddingCountMismatch) {
		return true
	}
	if errors.Is(err, errUpstreamProviderFailure) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		}
	}

	var reqErr *openai.RequestError
	return errors.As(err, &reqErr)
}

func (p *OpenAIVisionProvider) wrapError(operation string, err error) error {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return NewProviderError(operation, apiErr.HTTPStatusCode, apiErr.Message, err)
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		return NewProviderError(operation, reqErr.HTTPStatusCode, reqErr.Error(), err)
	}

	return NewProviderError(operation, 0, err.Error(), err)
}

var _ search.Embedder = (*OpenAIVisionProvider)(nil)
