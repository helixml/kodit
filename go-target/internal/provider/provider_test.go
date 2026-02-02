package provider

import (
	"errors"
	"testing"
)

func TestMessage(t *testing.T) {
	msg := NewMessage("user", "Hello, world!")

	if msg.Role() != "user" {
		t.Errorf("Role() = %v, want 'user'", msg.Role())
	}
	if msg.Content() != "Hello, world!" {
		t.Errorf("Content() = %v, want 'Hello, world!'", msg.Content())
	}
}

func TestMessageHelpers(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		role     string
		content  string
	}{
		{"system", SystemMessage("You are helpful"), "system", "You are helpful"},
		{"user", UserMessage("Hello"), "user", "Hello"},
		{"assistant", AssistantMessage("Hi there"), "assistant", "Hi there"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msg.Role() != tt.role {
				t.Errorf("Role() = %v, want %v", tt.msg.Role(), tt.role)
			}
			if tt.msg.Content() != tt.content {
				t.Errorf("Content() = %v, want %v", tt.msg.Content(), tt.content)
			}
		})
	}
}

func TestChatCompletionRequest(t *testing.T) {
	messages := []Message{
		SystemMessage("Be helpful"),
		UserMessage("Hello"),
	}
	req := NewChatCompletionRequest(messages)

	if len(req.Messages()) != 2 {
		t.Errorf("Messages() length = %v, want 2", len(req.Messages()))
	}
	if req.MaxTokens() != 0 {
		t.Errorf("MaxTokens() = %v, want 0 (default)", req.MaxTokens())
	}
	if req.Temperature() != 0 {
		t.Errorf("Temperature() = %v, want 0 (default)", req.Temperature())
	}

	// Test with options
	req = req.WithMaxTokens(100).WithTemperature(0.7)
	if req.MaxTokens() != 100 {
		t.Errorf("MaxTokens() = %v, want 100", req.MaxTokens())
	}
	if req.Temperature() != 0.7 {
		t.Errorf("Temperature() = %v, want 0.7", req.Temperature())
	}
}

func TestChatCompletionRequest_MessagesAreCopied(t *testing.T) {
	messages := []Message{UserMessage("Hello")}
	req := NewChatCompletionRequest(messages)

	// Modify original
	messages[0] = UserMessage("Modified")

	if req.Messages()[0].Content() == "Modified" {
		t.Error("Messages should be copied, not referenced")
	}

	// Modify returned slice
	returned := req.Messages()
	returned[0] = UserMessage("Also Modified")

	if req.Messages()[0].Content() == "Also Modified" {
		t.Error("Messages() should return a copy")
	}
}

func TestChatCompletionResponse(t *testing.T) {
	usage := NewUsage(10, 20, 30)
	resp := NewChatCompletionResponse("Generated text", "stop", usage)

	if resp.Content() != "Generated text" {
		t.Errorf("Content() = %v, want 'Generated text'", resp.Content())
	}
	if resp.FinishReason() != "stop" {
		t.Errorf("FinishReason() = %v, want 'stop'", resp.FinishReason())
	}
	if resp.Usage().TotalTokens() != 30 {
		t.Errorf("Usage().TotalTokens() = %v, want 30", resp.Usage().TotalTokens())
	}
}

func TestUsage(t *testing.T) {
	usage := NewUsage(100, 50, 150)

	if usage.PromptTokens() != 100 {
		t.Errorf("PromptTokens() = %v, want 100", usage.PromptTokens())
	}
	if usage.CompletionTokens() != 50 {
		t.Errorf("CompletionTokens() = %v, want 50", usage.CompletionTokens())
	}
	if usage.TotalTokens() != 150 {
		t.Errorf("TotalTokens() = %v, want 150", usage.TotalTokens())
	}
}

func TestEmbeddingRequest(t *testing.T) {
	texts := []string{"Hello", "World"}
	req := NewEmbeddingRequest(texts)

	if len(req.Texts()) != 2 {
		t.Errorf("Texts() length = %v, want 2", len(req.Texts()))
	}

	// Verify texts are copied
	texts[0] = "Modified"
	if req.Texts()[0] == "Modified" {
		t.Error("Texts should be copied, not referenced")
	}

	// Verify returned slice is a copy
	returned := req.Texts()
	returned[0] = "Also Modified"
	if req.Texts()[0] == "Also Modified" {
		t.Error("Texts() should return a copy")
	}
}

func TestEmbeddingResponse(t *testing.T) {
	embeddings := [][]float64{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	usage := NewUsage(10, 0, 10)
	resp := NewEmbeddingResponse(embeddings, usage)

	if len(resp.Embeddings()) != 2 {
		t.Errorf("Embeddings() length = %v, want 2", len(resp.Embeddings()))
	}
	if resp.Embeddings()[0][0] != 0.1 {
		t.Errorf("Embeddings()[0][0] = %v, want 0.1", resp.Embeddings()[0][0])
	}

	// Verify embeddings are copied
	embeddings[0][0] = 999.0
	if resp.Embeddings()[0][0] == 999.0 {
		t.Error("Embeddings should be copied, not referenced")
	}

	// Verify returned embeddings are copies
	returned := resp.Embeddings()
	returned[0][0] = 888.0
	if resp.Embeddings()[0][0] == 888.0 {
		t.Error("Embeddings() should return copies")
	}
}

func TestProviderError(t *testing.T) {
	cause := errors.New("connection failed")
	err := NewProviderError("chat_completion", 500, "provider error", cause)

	if err.Operation() != "chat_completion" {
		t.Errorf("Operation() = %v, want 'chat_completion'", err.Operation())
	}
	if err.StatusCode() != 500 {
		t.Errorf("StatusCode() = %v, want 500", err.StatusCode())
	}
	if err.Message() != "provider error" {
		t.Errorf("Message() = %v, want 'provider error'", err.Message())
	}
	if !errors.Is(err, cause) {
		t.Error("Unwrap should return the cause")
	}

	expected := "provider error: connection failed"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestProviderError_NoCause(t *testing.T) {
	err := NewProviderError("embedding", 429, "rate limited", nil)

	if err.Error() != "rate limited" {
		t.Errorf("Error() = %v, want 'rate limited'", err.Error())
	}
}

func TestProviderError_IsRateLimited(t *testing.T) {
	err := NewProviderError("chat_completion", 429, "too many requests", nil)
	if !err.IsRateLimited() {
		t.Error("IsRateLimited() should be true for 429 status")
	}

	err = NewProviderError("chat_completion", 500, "server error", nil)
	if err.IsRateLimited() {
		t.Error("IsRateLimited() should be false for non-429 status")
	}
}

func TestProviderError_IsContextTooLong(t *testing.T) {
	err := NewProviderError("chat_completion", 400, "context length exceeded", nil)
	if !err.IsContextTooLong() {
		t.Error("IsContextTooLong() should be true for 400 with message")
	}

	err = NewProviderError("chat_completion", 400, "", nil)
	if err.IsContextTooLong() {
		t.Error("IsContextTooLong() should be false for 400 without message")
	}
}

func TestSentinelErrors(t *testing.T) {
	if ErrUnsupportedOperation == nil {
		t.Error("ErrUnsupportedOperation should not be nil")
	}
	if ErrRateLimited == nil {
		t.Error("ErrRateLimited should not be nil")
	}
	if ErrContextTooLong == nil {
		t.Error("ErrContextTooLong should not be nil")
	}
	if ErrProviderError == nil {
		t.Error("ErrProviderError should not be nil")
	}
}
