package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicProvider implements text generation using Anthropic Claude API.
// Note: Anthropic does not provide embeddings, so this provider only supports text generation.
type AnthropicProvider struct {
	apiKey        string
	baseURL       string
	model         string
	maxRetries    int
	initialDelay  time.Duration
	backoffFactor float64
	httpClient    *http.Client
}

// AnthropicOption is a functional option for AnthropicProvider.
type AnthropicOption func(*AnthropicProvider)

// WithAnthropicModel sets the Claude model.
func WithAnthropicModel(model string) AnthropicOption {
	return func(p *AnthropicProvider) { p.model = model }
}

// WithAnthropicMaxRetries sets the maximum retry count.
func WithAnthropicMaxRetries(n int) AnthropicOption {
	return func(p *AnthropicProvider) { p.maxRetries = n }
}

// WithAnthropicInitialDelay sets the initial retry delay.
func WithAnthropicInitialDelay(d time.Duration) AnthropicOption {
	return func(p *AnthropicProvider) { p.initialDelay = d }
}

// WithAnthropicBackoffFactor sets the backoff multiplier.
func WithAnthropicBackoffFactor(f float64) AnthropicOption {
	return func(p *AnthropicProvider) { p.backoffFactor = f }
}

// WithAnthropicTimeout sets the HTTP timeout.
func WithAnthropicTimeout(d time.Duration) AnthropicOption {
	return func(p *AnthropicProvider) {
		p.httpClient.Timeout = d
	}
}

// WithAnthropicBaseURL sets the base URL (for testing or proxies).
func WithAnthropicBaseURL(url string) AnthropicOption {
	return func(p *AnthropicProvider) { p.baseURL = url }
}

// NewAnthropicProvider creates a new Anthropic Claude provider.
func NewAnthropicProvider(apiKey string, opts ...AnthropicOption) *AnthropicProvider {
	p := &AnthropicProvider{
		apiKey:        apiKey,
		baseURL:       "https://api.anthropic.com",
		model:         "claude-sonnet-4-20250514",
		maxRetries:    5,
		initialDelay:  2 * time.Second,
		backoffFactor: 2.0,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// AnthropicConfig holds configuration for Anthropic provider.
type AnthropicConfig struct {
	APIKey        string
	BaseURL       string
	Model         string
	Timeout       time.Duration
	MaxRetries    int
	InitialDelay  time.Duration
	BackoffFactor float64
}

// NewAnthropicProviderFromConfig creates a provider from configuration.
func NewAnthropicProviderFromConfig(cfg AnthropicConfig) *AnthropicProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	model := cfg.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
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

	return &AnthropicProvider{
		apiKey:        cfg.APIKey,
		baseURL:       baseURL,
		model:         model,
		maxRetries:    maxRetries,
		initialDelay:  initialDelay,
		backoffFactor: backoffFactor,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// SupportsTextGeneration returns true.
func (p *AnthropicProvider) SupportsTextGeneration() bool {
	return true
}

// SupportsEmbedding returns false (Anthropic doesn't support embeddings).
func (p *AnthropicProvider) SupportsEmbedding() bool {
	return false
}

// Close is a no-op for the Anthropic provider.
func (p *AnthropicProvider) Close() error {
	return nil
}

// anthropicRequest represents the Anthropic API request body.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
}

// anthropicMessage represents a message in the Anthropic API.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the Anthropic API response.
type anthropicResponse struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Role       string           `json:"role"`
	Content    []anthropicBlock `json:"content"`
	Model      string           `json:"model"`
	StopReason string           `json:"stop_reason"`
	Usage      anthropicUsage   `json:"usage"`
}

// anthropicBlock represents a content block in the response.
type anthropicBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicUsage represents token usage in the response.
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicError represents an Anthropic API error response.
type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ChatCompletion generates a chat completion using Claude.
func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	messages := req.Messages()
	if len(messages) == 0 {
		return ChatCompletionResponse{}, NewProviderError("chat_completion", 0, "no messages provided", nil)
	}

	// Extract system message if present
	var systemMessage string
	var apiMessages []anthropicMessage

	for _, m := range messages {
		if m.Role() == "system" {
			systemMessage = m.Content()
		} else {
			apiMessages = append(apiMessages, anthropicMessage{
				Role:    m.Role(),
				Content: m.Content(),
			})
		}
	}

	maxTokens := req.MaxTokens()
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: maxTokens,
		Messages:  apiMessages,
		System:    systemMessage,
	}

	var resp anthropicResponse
	var err error

	err = p.withRetry(ctx, func() error {
		resp, err = p.doRequest(ctx, apiReq)
		return err
	})

	if err != nil {
		return ChatCompletionResponse{}, err
	}

	// Extract text from content blocks
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	usage := NewUsage(
		resp.Usage.InputTokens,
		resp.Usage.OutputTokens,
		resp.Usage.InputTokens+resp.Usage.OutputTokens,
	)

	return NewChatCompletionResponse(content, resp.StopReason, usage), nil
}

// doRequest performs the HTTP request to the Anthropic API.
func (p *AnthropicProvider) doRequest(ctx context.Context, req anthropicRequest) (anthropicResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return anthropicResponse{}, NewProviderError("chat_completion", 0, "failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return anthropicResponse{}, NewProviderError("chat_completion", 0, "failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return anthropicResponse{}, NewProviderError("chat_completion", 0, "request failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return anthropicResponse{}, NewProviderError("chat_completion", resp.StatusCode, "failed to read response", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr anthropicError
		if err := json.Unmarshal(respBody, &apiErr); err == nil {
			return anthropicResponse{}, NewProviderError("chat_completion", resp.StatusCode, apiErr.Message, nil)
		}
		return anthropicResponse{}, NewProviderError("chat_completion", resp.StatusCode, string(respBody), nil)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return anthropicResponse{}, NewProviderError("chat_completion", 0, "failed to unmarshal response", err)
	}

	return apiResp, nil
}

// withRetry executes the function with exponential backoff retry.
func (p *AnthropicProvider) withRetry(ctx context.Context, fn func() error) error {
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
func (p *AnthropicProvider) isRetryable(err error) bool {
	var provErr *ProviderError
	if !extractError(err, &provErr) {
		return false
	}

	switch provErr.StatusCode() {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}

	return false
}

// extractError is a type-safe error extraction helper.
func extractError(err error, target **ProviderError) bool {
	var provErr *ProviderError
	if e, ok := err.(*ProviderError); ok {
		*target = e
		return true
	}
	unwrapped := err
	for unwrapped != nil {
		if e, ok := unwrapped.(*ProviderError); ok {
			*target = e
			return true
		}
		u, ok := unwrapped.(interface{ Unwrap() error })
		if !ok {
			break
		}
		unwrapped = u.Unwrap()
	}
	_ = provErr
	return false
}

// Ensure AnthropicProvider implements the interfaces.
var (
	_ TextOnlyProvider = (*AnthropicProvider)(nil)
	_ TextGenerator    = (*AnthropicProvider)(nil)
)
