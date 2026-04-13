package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/helixml/kodit/domain/search"
)

// errEmbeddingCountMismatch indicates the API returned fewer embedding vectors
// than requested. This is retryable because transient upstream issues (e.g.
// rate-limiting behind a 200 status) can produce partial responses.
var errEmbeddingCountMismatch = errors.New("embedding response count mismatch")

// errUpstreamProviderFailure indicates the API returned HTTP 200 but the
// response body contained an error instead of embedding data. This happens
// with routing providers like OpenRouter when all upstream providers fail.
// The response has zero data, zero usage, and an empty model. This is
// retryable because the failure is transient — the same model succeeds on
// subsequent requests once OpenRouter re-routes to a healthy provider.
var errUpstreamProviderFailure = errors.New("upstream provider failure")

// OpenAIProvider implements both text generation and embedding using OpenAI API.
type OpenAIProvider struct {
	client            *openai.Client
	chatModel         string
	embeddingModel    string
	maxRetries        int
	initialDelay      time.Duration
	backoffFactor     float64
	extraParams         map[string]any
	queryInstruction    string
	documentInstruction string
	supportsText        bool
	supportsEmbedding   bool
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
	HTTPClient     *http.Client
	ExtraParams         map[string]any
	QueryInstruction    string
	DocumentInstruction string
}

// NewOpenAIProviderFromConfig creates a provider from configuration.
func NewOpenAIProviderFromConfig(cfg OpenAIConfig) *OpenAIProvider {
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
		extraParams:         cfg.ExtraParams,
		queryInstruction:    cfg.QueryInstruction,
		documentInstruction: cfg.DocumentInstruction,
		supportsText:        true,
		supportsEmbedding:   true,
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

	if len(p.extraParams) > 0 {
		if err := applyExtraParams(&openaiReq, p.extraParams); err != nil {
			return ChatCompletionResponse{}, fmt.Errorf("apply extra params: %w", err)
		}
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

// Embed generates embeddings for the given text items in a single API call.
// Items without a text payload return an error — OpenAI text embedding
// endpoints do not accept image inputs.
func (p *OpenAIProvider) Embed(ctx context.Context, items []search.EmbeddingItem) ([][]float64, error) {
	if !p.supportsEmbedding {
		return nil, ErrUnsupportedOperation
	}

	if len(items) == 0 {
		return [][]float64{}, nil
	}

	texts := make([]string, len(items))
	for i, item := range items {
		if !item.HasText() {
			return nil, fmt.Errorf("openai embedding requires text, got item %d with no text", i)
		}
		texts[i] = string(item.Text())
	}

	openaiReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(p.embeddingModel),
		Input: texts,
	}

	// Apply asymmetric retrieval instruction when configured.
	// A batch is either all queries or all documents — the two are never mixed.
	if instruction := p.instructionForBatch(items); instruction != "" {
		openaiReq.ExtraBody = map[string]any{
			"instruction": instruction,
		}
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
		return nil, p.wrapError("embedding", err)
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

// instructionForBatch returns the appropriate instruction for a batch of items.
// If the first item is a query, the query instruction is returned; otherwise the
// document instruction. Returns "" when no instruction is configured.
func (p *OpenAIProvider) instructionForBatch(items []search.EmbeddingItem) string {
	if len(items) == 0 {
		return ""
	}
	if items[0].IsQuery() {
		return p.queryInstruction
	}
	return p.documentInstruction
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

	// Upstream provider routing failures (e.g. OpenRouter "No successful
	// provider responses") are transient — the same request succeeds once
	// the routing layer picks a healthy provider.
	if errors.Is(err, errUpstreamProviderFailure) {
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

// applyExtraParams merges arbitrary key-value pairs into an
// openai.ChatCompletionRequest via JSON round-trip. This lets callers set any
// field the go-openai library supports (e.g. chat_template_kwargs,
// reasoning_effort) without the provider needing per-field wiring.
func applyExtraParams(req *openai.ChatCompletionRequest, params map[string]any) error {
	base, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	extra, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal extra params: %w", err)
	}

	merged, err := jsonMerge(base, extra)
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}

	return json.Unmarshal(merged, req)
}

// jsonMerge shallow-merges two JSON objects. Keys from b override keys in a.
func jsonMerge(a, b []byte) ([]byte, error) {
	var am, bm map[string]json.RawMessage
	if err := json.Unmarshal(a, &am); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &bm); err != nil {
		return nil, err
	}
	for k, v := range bm {
		am[k] = v
	}
	return json.Marshal(am)
}

// Ensure OpenAIProvider implements the interfaces.
var (
	_ TextGenerator   = (*OpenAIProvider)(nil)
	_ search.Embedder = (*OpenAIProvider)(nil)
)
