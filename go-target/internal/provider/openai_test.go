package provider

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/internal/config"
)

func TestNewOpenAIProvider_Defaults(t *testing.T) {
	p := NewOpenAIProvider("test-api-key")

	if !p.SupportsTextGeneration() {
		t.Error("SupportsTextGeneration() should be true by default")
	}
	if !p.SupportsEmbedding() {
		t.Error("SupportsEmbedding() should be true by default")
	}
	if p.chatModel != "gpt-4" {
		t.Errorf("chatModel = %v, want 'gpt-4'", p.chatModel)
	}
	if p.embeddingModel != "text-embedding-3-small" {
		t.Errorf("embeddingModel = %v, want 'text-embedding-3-small'", p.embeddingModel)
	}
}

func TestNewOpenAIProvider_WithOptions(t *testing.T) {
	p := NewOpenAIProvider("test-api-key",
		WithChatModel("gpt-3.5-turbo"),
		WithEmbeddingModel("text-embedding-ada-002"),
		WithOpenAIMaxRetries(3),
		WithOpenAIInitialDelay(1*time.Second),
		WithOpenAIBackoffFactor(1.5),
	)

	if p.chatModel != "gpt-3.5-turbo" {
		t.Errorf("chatModel = %v, want 'gpt-3.5-turbo'", p.chatModel)
	}
	if p.embeddingModel != "text-embedding-ada-002" {
		t.Errorf("embeddingModel = %v, want 'text-embedding-ada-002'", p.embeddingModel)
	}
	if p.maxRetries != 3 {
		t.Errorf("maxRetries = %v, want 3", p.maxRetries)
	}
	if p.initialDelay != 1*time.Second {
		t.Errorf("initialDelay = %v, want 1s", p.initialDelay)
	}
	if p.backoffFactor != 1.5 {
		t.Errorf("backoffFactor = %v, want 1.5", p.backoffFactor)
	}
}

func TestNewOpenAIProviderFromEndpoint(t *testing.T) {
	endpoint := config.NewEndpointWithOptions(
		config.WithAPIKey("test-key"),
		config.WithModel("gpt-4-turbo"),
		config.WithBaseURL("https://custom.openai.com/v1"),
		config.WithMaxRetries(3),
		config.WithInitialDelay(1*time.Second),
		config.WithBackoffFactor(1.5),
		config.WithTimeout(30*time.Second),
	)

	p := NewOpenAIProviderFromEndpoint(endpoint)

	if p.chatModel != "gpt-4-turbo" {
		t.Errorf("chatModel = %v, want 'gpt-4-turbo'", p.chatModel)
	}
	if p.maxRetries != 3 {
		t.Errorf("maxRetries = %v, want 3", p.maxRetries)
	}
	if !p.SupportsTextGeneration() {
		t.Error("SupportsTextGeneration() should be true")
	}
	if !p.SupportsEmbedding() {
		t.Error("SupportsEmbedding() should be true")
	}
}

func TestOpenAIProvider_Close(t *testing.T) {
	p := NewOpenAIProvider("test-api-key")

	err := p.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestOpenAIProvider_InterfaceCompliance(t *testing.T) {
	var _ FullProvider = (*OpenAIProvider)(nil)
	var _ TextGenerator = (*OpenAIProvider)(nil)
	var _ Embedder = (*OpenAIProvider)(nil)
}

func TestOpenAIProvider_Embed_EmptyInput(t *testing.T) {
	p := NewOpenAIProvider("test-api-key")

	req := NewEmbeddingRequest([]string{})
	resp, err := p.Embed(context.Background(), req)

	if err != nil {
		t.Errorf("Embed() with empty input should not error: %v", err)
	}
	if len(resp.Embeddings()) != 0 {
		t.Errorf("Embed() with empty input should return empty embeddings")
	}
}

func TestOpenAIProvider_UnsupportedOperations(t *testing.T) {
	// Create a provider that only supports embeddings
	p := &OpenAIProvider{
		supportsText:      false,
		supportsEmbedding: true,
	}

	_, err := p.ChatCompletion(context.Background(), NewChatCompletionRequest([]Message{}))
	if err != ErrUnsupportedOperation {
		t.Errorf("ChatCompletion() should return ErrUnsupportedOperation when not supported")
	}

	// Create a provider that only supports text
	p = &OpenAIProvider{
		supportsText:      true,
		supportsEmbedding: false,
	}

	_, err = p.Embed(context.Background(), NewEmbeddingRequest([]string{"test"}))
	if err != ErrUnsupportedOperation {
		t.Errorf("Embed() should return ErrUnsupportedOperation when not supported")
	}
}

// FakeOpenAIProvider is a test double for OpenAIProvider.
type FakeOpenAIProvider struct {
	ChatCompletionFunc func(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)
	EmbedFunc          func(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}

func (f *FakeOpenAIProvider) SupportsTextGeneration() bool { return true }
func (f *FakeOpenAIProvider) SupportsEmbedding() bool      { return true }
func (f *FakeOpenAIProvider) Close() error                 { return nil }

func (f *FakeOpenAIProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	if f.ChatCompletionFunc != nil {
		return f.ChatCompletionFunc(ctx, req)
	}
	return NewChatCompletionResponse("fake response", "stop", NewUsage(10, 5, 15)), nil
}

func (f *FakeOpenAIProvider) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	if f.EmbedFunc != nil {
		return f.EmbedFunc(ctx, req)
	}
	embeddings := make([][]float64, len(req.Texts()))
	for i := range embeddings {
		embeddings[i] = []float64{0.1, 0.2, 0.3}
	}
	return NewEmbeddingResponse(embeddings, NewUsage(10, 0, 10)), nil
}

// Ensure FakeOpenAIProvider implements the interfaces.
var (
	_ FullProvider = (*FakeOpenAIProvider)(nil)
	_ TextGenerator = (*FakeOpenAIProvider)(nil)
	_ Embedder = (*FakeOpenAIProvider)(nil)
)

func TestFakeOpenAIProvider(t *testing.T) {
	fake := &FakeOpenAIProvider{}

	// Test chat completion
	resp, err := fake.ChatCompletion(context.Background(), NewChatCompletionRequest([]Message{
		UserMessage("Hello"),
	}))
	if err != nil {
		t.Errorf("ChatCompletion() error: %v", err)
	}
	if resp.Content() != "fake response" {
		t.Errorf("Content() = %v, want 'fake response'", resp.Content())
	}

	// Test embedding
	embResp, err := fake.Embed(context.Background(), NewEmbeddingRequest([]string{"test"}))
	if err != nil {
		t.Errorf("Embed() error: %v", err)
	}
	if len(embResp.Embeddings()) != 1 {
		t.Errorf("Embeddings() length = %v, want 1", len(embResp.Embeddings()))
	}
}

func TestFakeOpenAIProvider_CustomFunctions(t *testing.T) {
	fake := &FakeOpenAIProvider{
		ChatCompletionFunc: func(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
			return NewChatCompletionResponse("custom response", "stop", NewUsage(0, 0, 0)), nil
		},
		EmbedFunc: func(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
			return NewEmbeddingResponse([][]float64{{1, 2, 3}}, NewUsage(0, 0, 0)), nil
		},
	}

	resp, _ := fake.ChatCompletion(context.Background(), NewChatCompletionRequest(nil))
	if resp.Content() != "custom response" {
		t.Errorf("Content() = %v, want 'custom response'", resp.Content())
	}

	embResp, _ := fake.Embed(context.Background(), NewEmbeddingRequest([]string{"test"}))
	if embResp.Embeddings()[0][0] != 1 {
		t.Errorf("Embeddings()[0][0] = %v, want 1", embResp.Embeddings()[0][0])
	}
}
