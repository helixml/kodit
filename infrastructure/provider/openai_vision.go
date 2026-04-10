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

// OpenAIVisionProvider embeds text or image inputs via an OpenAI-compatible
// vision-language embedding API (e.g. Qwen3-VL-Embedding). It implements
// Embedder: each input in a batch is inspected by its magic bytes and sent
// to the API as either a plain text string or a base64-encoded image content
// part. Both produce vectors in the same embedding space.
type OpenAIVisionProvider struct {
	client         *openai.Client
	embeddingModel string
	maxRetries     int
	initialDelay   time.Duration
	backoffFactor  float64
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

	return &OpenAIVisionProvider{
		client:         client,
		embeddingModel: embeddingModel,
		maxRetries:     maxRetries,
		initialDelay:   initialDelay,
		backoffFactor:  backoffFactor,
	}
}

// Close is a no-op for the remote provider.
func (p *OpenAIVisionProvider) Close() error {
	return nil
}

// Embed sends each item to the remote API, using its text field, image
// field, or both. Items carrying only text are sent as plain strings;
// items carrying only an image are sent as base64 image content parts;
// items carrying both are sent as a combined content-parts list, which
// multimodal-capable models may use to jointly embed text and image.
func (p *OpenAIVisionProvider) Embed(ctx context.Context, items []search.EmbeddingItem) ([][]float64, error) {
	if len(items) == 0 {
		return [][]float64{}, nil
	}

	apiInputs := make([]any, len(items))
	for i, item := range items {
		input, err := buildVisionAPIInput(item)
		if err != nil {
			return nil, fmt.Errorf("build vision input %d: %w", i, err)
		}
		apiInputs[i] = input
	}

	openaiReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(p.embeddingModel),
		Input: apiInputs,
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
		if len(resp.Data) != len(items) {
			return fmt.Errorf("%w: got %d vectors for %d items", errEmbeddingCountMismatch, len(resp.Data), len(items))
		}
		return nil
	})

	if err != nil {
		return nil, p.wrapError("vision_embedding", err)
	}

	embeddings := make([][]float64, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = make([]float64, len(data.Embedding))
		for j, v := range data.Embedding {
			embeddings[i][j] = float64(v)
		}
	}

	return embeddings, nil
}

// buildVisionAPIInput converts an EmbeddingItem into the value that the
// OpenAI-compatible vision embedding API expects as a single element of
// its `input` array. Text-only items are sent as plain strings; image
// items are sent as a list of content parts (one image part, plus an
// optional text part if the item is multimodal).
func buildVisionAPIInput(item search.EmbeddingItem) (any, error) {
	if !item.HasText() && !item.HasImage() {
		return nil, fmt.Errorf("item has neither text nor image")
	}

	if item.HasImage() {
		dataURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(item.Image())
		parts := []any{
			map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": dataURI,
				},
			},
		}
		if item.HasText() {
			parts = append(parts, map[string]any{
				"type": "text",
				"text": string(item.Text()),
			})
		}
		return parts, nil
	}

	return string(item.Text()), nil
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
