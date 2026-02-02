// Package provider provides unified AI provider abstractions for text generation
// and embedding generation. Providers may support one or both capabilities.
package provider

import (
	"context"
	"errors"
)

// Common errors.
var (
	// ErrUnsupportedOperation indicates the provider doesn't support the requested operation.
	ErrUnsupportedOperation = errors.New("operation not supported by this provider")

	// ErrRateLimited indicates the provider rate limited the request.
	ErrRateLimited = errors.New("rate limited")

	// ErrContextTooLong indicates the input exceeded the context window.
	ErrContextTooLong = errors.New("context too long")

	// ErrProviderError indicates a general provider error.
	ErrProviderError = errors.New("provider error")
)

// Message represents a chat message.
type Message struct {
	role    string
	content string
}

// NewMessage creates a new Message.
func NewMessage(role, content string) Message {
	return Message{role: role, content: content}
}

// Role returns the message role (e.g., "system", "user", "assistant").
func (m Message) Role() string { return m.role }

// Content returns the message content.
func (m Message) Content() string { return m.content }

// SystemMessage creates a system message.
func SystemMessage(content string) Message {
	return NewMessage("system", content)
}

// UserMessage creates a user message.
func UserMessage(content string) Message {
	return NewMessage("user", content)
}

// AssistantMessage creates an assistant message.
func AssistantMessage(content string) Message {
	return NewMessage("assistant", content)
}

// ChatCompletionRequest represents a request for text generation.
type ChatCompletionRequest struct {
	messages    []Message
	maxTokens   int
	temperature float64
}

// NewChatCompletionRequest creates a new ChatCompletionRequest.
func NewChatCompletionRequest(messages []Message) ChatCompletionRequest {
	msgs := make([]Message, len(messages))
	copy(msgs, messages)
	return ChatCompletionRequest{
		messages:    msgs,
		maxTokens:   0, // Use provider default
		temperature: 0, // Use provider default
	}
}

// WithMaxTokens returns a new request with the specified max tokens.
func (r ChatCompletionRequest) WithMaxTokens(n int) ChatCompletionRequest {
	r.maxTokens = n
	return r
}

// WithTemperature returns a new request with the specified temperature.
func (r ChatCompletionRequest) WithTemperature(t float64) ChatCompletionRequest {
	r.temperature = t
	return r
}

// Messages returns the messages.
func (r ChatCompletionRequest) Messages() []Message {
	msgs := make([]Message, len(r.messages))
	copy(msgs, r.messages)
	return msgs
}

// MaxTokens returns the max tokens setting.
func (r ChatCompletionRequest) MaxTokens() int { return r.maxTokens }

// Temperature returns the temperature setting.
func (r ChatCompletionRequest) Temperature() float64 { return r.temperature }

// ChatCompletionResponse represents a text generation response.
type ChatCompletionResponse struct {
	content      string
	finishReason string
	usage        Usage
}

// NewChatCompletionResponse creates a new ChatCompletionResponse.
func NewChatCompletionResponse(content, finishReason string, usage Usage) ChatCompletionResponse {
	return ChatCompletionResponse{
		content:      content,
		finishReason: finishReason,
		usage:        usage,
	}
}

// Content returns the generated content.
func (r ChatCompletionResponse) Content() string { return r.content }

// FinishReason returns why generation stopped.
func (r ChatCompletionResponse) FinishReason() string { return r.finishReason }

// Usage returns token usage information.
func (r ChatCompletionResponse) Usage() Usage { return r.usage }

// Usage represents token usage information.
type Usage struct {
	promptTokens     int
	completionTokens int
	totalTokens      int
}

// NewUsage creates a new Usage.
func NewUsage(prompt, completion, total int) Usage {
	return Usage{
		promptTokens:     prompt,
		completionTokens: completion,
		totalTokens:      total,
	}
}

// PromptTokens returns the number of prompt tokens.
func (u Usage) PromptTokens() int { return u.promptTokens }

// CompletionTokens returns the number of completion tokens.
func (u Usage) CompletionTokens() int { return u.completionTokens }

// TotalTokens returns the total number of tokens.
func (u Usage) TotalTokens() int { return u.totalTokens }

// EmbeddingRequest represents a request for embeddings.
type EmbeddingRequest struct {
	texts []string
}

// NewEmbeddingRequest creates a new EmbeddingRequest.
func NewEmbeddingRequest(texts []string) EmbeddingRequest {
	t := make([]string, len(texts))
	copy(t, texts)
	return EmbeddingRequest{texts: t}
}

// Texts returns the texts to embed.
func (r EmbeddingRequest) Texts() []string {
	t := make([]string, len(r.texts))
	copy(t, r.texts)
	return t
}

// EmbeddingResponse represents an embedding response.
type EmbeddingResponse struct {
	embeddings [][]float64
	usage      Usage
}

// NewEmbeddingResponse creates a new EmbeddingResponse.
func NewEmbeddingResponse(embeddings [][]float64, usage Usage) EmbeddingResponse {
	embs := make([][]float64, len(embeddings))
	for i, e := range embeddings {
		embs[i] = make([]float64, len(e))
		copy(embs[i], e)
	}
	return EmbeddingResponse{
		embeddings: embs,
		usage:      usage,
	}
}

// Embeddings returns the embedding vectors.
func (r EmbeddingResponse) Embeddings() [][]float64 {
	embs := make([][]float64, len(r.embeddings))
	for i, e := range r.embeddings {
		embs[i] = make([]float64, len(e))
		copy(embs[i], e)
	}
	return embs
}

// Usage returns token usage information.
func (r EmbeddingResponse) Usage() Usage { return r.usage }

// TextGenerator generates text completions.
type TextGenerator interface {
	// ChatCompletion generates a text completion for the given messages.
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)
}

// Embedder generates embeddings for text.
type Embedder interface {
	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}

// Provider combines text generation and embedding capabilities.
// A provider may implement one or both interfaces.
type Provider interface {
	// SupportsTextGeneration returns true if the provider can generate text.
	SupportsTextGeneration() bool

	// SupportsEmbedding returns true if the provider can generate embeddings.
	SupportsEmbedding() bool

	// Close releases any resources held by the provider.
	Close() error
}

// FullProvider implements both text generation and embedding.
type FullProvider interface {
	Provider
	TextGenerator
	Embedder
}

// TextOnlyProvider implements only text generation.
type TextOnlyProvider interface {
	Provider
	TextGenerator
}

// EmbeddingOnlyProvider implements only embedding.
type EmbeddingOnlyProvider interface {
	Provider
	Embedder
}

// ProviderError wraps provider errors with additional context.
type ProviderError struct {
	operation  string
	statusCode int
	message    string
	cause      error
}

// NewProviderError creates a new ProviderError.
func NewProviderError(operation string, statusCode int, message string, cause error) *ProviderError {
	return &ProviderError{
		operation:  operation,
		statusCode: statusCode,
		message:    message,
		cause:      cause,
	}
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.cause != nil {
		return e.message + ": " + e.cause.Error()
	}
	return e.message
}

// Unwrap returns the underlying cause.
func (e *ProviderError) Unwrap() error {
	return e.cause
}

// Operation returns the operation that failed.
func (e *ProviderError) Operation() string { return e.operation }

// StatusCode returns the HTTP status code if available.
func (e *ProviderError) StatusCode() int { return e.statusCode }

// Message returns the error message.
func (e *ProviderError) Message() string { return e.message }

// IsRateLimited returns true if the error is due to rate limiting.
func (e *ProviderError) IsRateLimited() bool {
	return e.statusCode == 429
}

// IsContextTooLong returns true if the error is due to context length.
func (e *ProviderError) IsContextTooLong() bool {
	return e.statusCode == 400 && e.message != ""
}
