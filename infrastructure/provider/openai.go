package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// DefaultBatchSize is the default number of texts per embedding API call.
const DefaultBatchSize = 10

// errEmbeddingCountMismatch indicates the API returned fewer embedding vectors
// than requested. This is retryable because transient upstream issues (e.g.
// rate-limiting behind a 200 status) can produce partial responses.
var errEmbeddingCountMismatch = errors.New("embedding response count mismatch")

// errUpstreamProviderFailure indicates the API returned HTTP 200 but the
// response body contained an error instead of embedding data. This happens
// with routing providers like OpenRouter when all upstream providers fail.
// The response has zero data, zero usage, and an empty model — retrying
// is futile because the upstream provider is down, not transiently overloaded.
var errUpstreamProviderFailure = errors.New("upstream provider failure")

// OpenAIProvider implements both text generation and embedding using OpenAI API.
type OpenAIProvider struct {
	client            *openai.Client
	chatModel         string
	embeddingModel    string
	maxRetries        int
	initialDelay      time.Duration
	backoffFactor     float64
	supportsText      bool
	supportsEmbedding bool
}

// OpenAIOption is a functional option for OpenAIProvider.
type OpenAIOption func(*OpenAIProvider)

// WithChatModel sets the chat completion model.
func WithChatModel(model string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.chatModel = model
		p.supportsText = true
	}
}

// WithEmbeddingModel sets the embedding model.
func WithEmbeddingModel(model string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.embeddingModel = model
		p.supportsEmbedding = true
	}
}

// WithMaxRetries sets the maximum retry count.
func WithMaxRetries(n int) OpenAIOption {
	return func(p *OpenAIProvider) { p.maxRetries = n }
}

// WithInitialDelay sets the initial retry delay.
func WithInitialDelay(d time.Duration) OpenAIOption {
	return func(p *OpenAIProvider) { p.initialDelay = d }
}

// WithBackoffFactor sets the backoff multiplier.
func WithBackoffFactor(f float64) OpenAIOption {
	return func(p *OpenAIProvider) { p.backoffFactor = f }
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey string, opts ...OpenAIOption) *OpenAIProvider {
	client := openai.NewClient(apiKey)

	p := &OpenAIProvider{
		client:            client,
		chatModel:         "gpt-4",
		embeddingModel:    "text-embedding-3-small",
		maxRetries:        5,
		initialDelay:      2 * time.Second,
		backoffFactor:     2.0,
		supportsText:      true,
		supportsEmbedding: true,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// OpenAIConfig holds configuration for OpenAI provider.
type OpenAIConfig struct {
	APIKey         string
	BaseURL        string
	ChatModel      string
	EmbeddingModel string
	Timeout        time.Duration
	MaxRetries     int
	InitialDelay   time.Duration
	BackoffFactor  float64
}

// NewOpenAIProviderFromConfig creates a provider from configuration.
func NewOpenAIProviderFromConfig(cfg OpenAIConfig) *OpenAIProvider {
	config := openai.DefaultConfig(cfg.APIKey)

	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}

	if cfg.Timeout > 0 {
		config.HTTPClient = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	client := openai.NewClientWithConfig(config)

	chatModel := cfg.ChatModel
	if chatModel == "" {
		chatModel = "gpt-4"
	}

	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-3-small"
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

	return &OpenAIProvider{
		client:            client,
		chatModel:         chatModel,
		embeddingModel:    embeddingModel,
		maxRetries:        maxRetries,
		initialDelay:      initialDelay,
		backoffFactor:     backoffFactor,
		supportsText:      true,
		supportsEmbedding: true,
	}
}

// SupportsTextGeneration returns true.
func (p *OpenAIProvider) SupportsTextGeneration() bool {
	return p.supportsText
}

// SupportsEmbedding returns true.
func (p *OpenAIProvider) SupportsEmbedding() bool {
	return p.supportsEmbedding
}

// Close is a no-op for the OpenAI provider.
func (p *OpenAIProvider) Close() error {
	return nil
}

// ChatCompletion generates a chat completion.
func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	if !p.supportsText {
		return ChatCompletionResponse{}, ErrUnsupportedOperation
	}

	messages := make([]openai.ChatCompletionMessage, len(req.Messages()))
	for i, m := range req.Messages() {
		messages[i] = openai.ChatCompletionMessage{
			Role:    m.Role(),
			Content: m.Content(),
		}
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:    p.chatModel,
		Messages: messages,
	}

	if req.MaxTokens() > 0 {
		openaiReq.MaxTokens = req.MaxTokens()
	}
	if req.Temperature() > 0 {
		openaiReq.Temperature = float32(req.Temperature())
	}

	var resp openai.ChatCompletionResponse
	var err error

	err = p.withRetry(ctx, func() error {
		resp, err = p.client.CreateChatCompletion(ctx, openaiReq)
		return err
	})

	if err != nil {
		return ChatCompletionResponse{}, p.wrapError("chat_completion", err)
	}

	if len(resp.Choices) == 0 {
		return ChatCompletionResponse{}, NewProviderError(
			"chat_completion", 0, "no choices in response", nil,
		)
	}

	usage := NewUsage(
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens,
	)

	return NewChatCompletionResponse(
		resp.Choices[0].Message.Content,
		string(resp.Choices[0].FinishReason),
		usage,
	), nil
}

// Embed generates embeddings for the given texts in a single API call.
func (p *OpenAIProvider) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	if !p.supportsEmbedding {
		return EmbeddingResponse{}, ErrUnsupportedOperation
	}

	texts := req.Texts()
	if len(texts) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	openaiReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(p.embeddingModel),
		Input: texts,
	}

	var resp openai.EmbeddingResponse
	var err error

	err = p.withRetry(ctx, func() error {
		resp, err = p.client.CreateEmbeddings(ctx, openaiReq)
		if err != nil {
			return err
		}
		// Detect upstream provider failure: routing providers (e.g. OpenRouter)
		// return HTTP 200 with an error body that the go-openai library silently
		// parses as an empty response. When zero data comes back with zero usage
		// and no model, the upstream is down — not transiently overloaded.
		if len(resp.Data) == 0 && string(resp.Model) == "" && resp.Usage.TotalTokens == 0 {
			return fmt.Errorf(
				"%w: provider returned HTTP 200 with no embedding data, no model, and zero usage (upstream routing failure)",
				errUpstreamProviderFailure,
			)
		}
		if len(resp.Data) != len(texts) {
			return fmt.Errorf("%w: got %d vectors for %d texts", errEmbeddingCountMismatch, len(resp.Data), len(texts))
		}
		return nil
	})

	if err != nil {
		return EmbeddingResponse{}, p.wrapError("embedding", err)
	}

	embeddings := make([][]float64, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = make([]float64, len(data.Embedding))
		for j, v := range data.Embedding {
			embeddings[i][j] = float64(v)
		}
	}

	usage := NewUsage(resp.Usage.PromptTokens, 0, resp.Usage.TotalTokens)
	return NewEmbeddingResponse(embeddings, usage), nil
}

// withRetry executes the function with exponential backoff retry.
func (p *OpenAIProvider) withRetry(ctx context.Context, fn func() error) error {
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

// isRetryable determines if an error should be retried.
func (p *OpenAIProvider) isRetryable(err error) bool {
	// Empty or partial embedding responses are retryable — upstream providers
	// can return 200 with no data under transient load conditions.
	if errors.Is(err, errEmbeddingCountMismatch) {
		return true
	}

	// HTTP client timeouts are retryable
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
	if errors.As(err, &reqErr) {
		// Network errors are retryable
		return true
	}

	return false
}

// wrapError wraps an OpenAI error into a ProviderError.
func (p *OpenAIProvider) wrapError(operation string, err error) error {
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

// Ensure OpenAIProvider implements the interfaces.
var (
	_ FullProvider  = (*OpenAIProvider)(nil)
	_ TextGenerator = (*OpenAIProvider)(nil)
	_ Embedder      = (*OpenAIProvider)(nil)
)
