package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/search"
)

// TestOpenAIVisionProvider_EmbedImage_SendsMessages verifies that image
// embedding requests use the vLLM "messages" format (chat completion style)
// rather than stuffing content parts into the "input" field. vLLM's
// /v1/embeddings endpoint expects multimodal inputs as:
//
//	{
//	  "model": "...",
//	  "messages": [{"role": "user", "content": [{"type": "image_url", ...}]}]
//	}
//
// Sending image content parts in "input" causes vLLM to reject the request
// with a validation error because it tries to parse them as token integers.
func TestOpenAIVisionProvider_EmbedImage_SendsMessages(t *testing.T) {
	var received map[string]json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)

		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3}},
			},
			"model": "test-model",
			"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 10},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIVisionProvider(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-vision-model",
		MaxRetries:     0,
	})

	// A tiny valid JPEG (1x1 pixel).
	fakeImage := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}

	_, err := p.Embed(context.Background(), []search.EmbeddingItem{
		search.NewImageItem(fakeImage),
	})
	require.NoError(t, err)

	// The request body MUST contain "messages" for image inputs.
	require.Contains(t, received, "messages",
		"image embedding request must use 'messages' field, not 'input' with content parts")

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(received["messages"], &messages))
	require.Len(t, messages, 1, "expected one message per embedding item")

	// The message must have role "user" and content with image_url part.
	var role string
	require.NoError(t, json.Unmarshal(messages[0]["role"], &role))
	require.Equal(t, "user", role)

	var content []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(messages[0]["content"], &content))
	require.NotEmpty(t, content, "content must contain at least one part")

	var partType string
	require.NoError(t, json.Unmarshal(content[0]["type"], &partType))
	require.Equal(t, "image_url", partType, "first content part must be an image_url")
}

// TestOpenAIVisionProvider_EmbedText_SendsMessagesWithInstruction verifies
// that text-only items (queries) use the "messages" format with a system
// instruction. Qwen3-VL-Embedding uses asymmetric retrieval: queries get
// an instruction in the system message, documents/images do not.
func TestOpenAIVisionProvider_EmbedText_SendsMessagesWithInstruction(t *testing.T) {
	var received map[string]json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)

		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3}},
			},
			"model": "test-model",
			"usage": map[string]int{"prompt_tokens": 4, "total_tokens": 4},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIVisionProvider(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-vision-model",
		MaxRetries:     0,
	})

	_, err := p.Embed(context.Background(), []search.EmbeddingItem{
		search.NewTextItem("hello world"),
	})
	require.NoError(t, err)

	require.Contains(t, received, "messages",
		"text embedding request must use 'messages' field for cross-modal consistency")

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(received["messages"], &messages))
	require.Len(t, messages, 2, "text query must have system + user messages")

	// First message: system with instruction.
	var sysRole string
	require.NoError(t, json.Unmarshal(messages[0]["role"], &sysRole))
	require.Equal(t, "system", sysRole)

	var sysContent []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(messages[0]["content"], &sysContent))
	require.Len(t, sysContent, 1)

	var instructionText string
	require.NoError(t, json.Unmarshal(sysContent[0]["text"], &instructionText))
	require.Equal(t, defaultQueryInstruction, instructionText)

	// Second message: user with text content.
	var userRole string
	require.NoError(t, json.Unmarshal(messages[1]["role"], &userRole))
	require.Equal(t, "user", userRole)

	var userContent []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(messages[1]["content"], &userContent))
	require.Len(t, userContent, 1)

	var partType string
	require.NoError(t, json.Unmarshal(userContent[0]["type"], &partType))
	require.Equal(t, "text", partType)
}

// TestOpenAIVisionProvider_EmbedImage_NoInstruction verifies that image
// items (documents) do NOT get a system instruction message — only queries do.
func TestOpenAIVisionProvider_EmbedImage_NoInstruction(t *testing.T) {
	var received map[string]json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)

		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3}},
			},
			"model": "test-model",
			"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 10},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIVisionProvider(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-vision-model",
		MaxRetries:     0,
	})

	fakeImage := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}

	_, err := p.Embed(context.Background(), []search.EmbeddingItem{
		search.NewImageItem(fakeImage),
	})
	require.NoError(t, err)

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(received["messages"], &messages))
	require.Len(t, messages, 1, "image embedding must have only user message, no system instruction")

	var role string
	require.NoError(t, json.Unmarshal(messages[0]["role"], &role))
	require.Equal(t, "user", role, "the single message must be from user")
}
