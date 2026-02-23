package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeEmbeddingServer returns an httptest.Server that mimics the OpenAI
// embeddings endpoint. It returns deterministic 3-dimensional vectors and
// tracks how many requests it received via the counter.
func fakeEmbeddingServer(t *testing.T, counter *atomic.Int64) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)

		var body struct {
			Input interface{} `json:"input"`
			Model string      `json:"model"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Input can be a single string or []string.
		var texts []string
		switch v := body.Input.(type) {
		case string:
			texts = []string{v}
		case []interface{}:
			for _, item := range v {
				texts = append(texts, item.(string))
			}
		}

		data := make([]map[string]interface{}, len(texts))
		for i := range texts {
			data[i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": []float64{0.1, 0.2, 0.3},
			}
		}

		resp := map[string]interface{}{
			"object": "list",
			"data":   data,
			"model":  body.Model,
			"usage": map[string]int{
				"prompt_tokens": len(texts) * 4,
				"total_tokens":  len(texts) * 4,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestOpenAIProvider_Capacity(t *testing.T) {
	p := NewOpenAIProvider("test-key")
	require.Equal(t, embeddingBatchSize, p.Capacity())
}

func TestOpenAIProvider_EmbedEmpty(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	req := NewEmbeddingRequest([]string{})
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)
	require.Empty(t, resp.Embeddings())
	require.Equal(t, int64(0), counter.Load(), "no HTTP request for empty input")
}

func TestOpenAIProvider_EmbedSingle(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	req := NewEmbeddingRequest([]string{"hello"})
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Embeddings(), 1)
	require.Len(t, resp.Embeddings()[0], 3)
	require.InDelta(t, 0.1, resp.Embeddings()[0][0], 1e-6)
	require.Equal(t, int64(1), counter.Load(), "single text should be one request")
}

func TestOpenAIProvider_EmbedWithinBatchLimit(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	texts := make([]string, embeddingBatchSize)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Embeddings(), embeddingBatchSize)
	require.Equal(t, int64(1), counter.Load(), "texts within batch limit should be one request")
}

func TestOpenAIProvider_EmbedExceedsCapacity(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	texts := make([]string, embeddingBatchSize+1)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	_, err := p.Embed(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds capacity")
	require.Equal(t, int64(0), counter.Load(), "no HTTP request when over capacity")
}

func TestOpenAIProvider_EmbedAggregatesUsage(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	// 10 texts = 1 batch within capacity. Each text returns 4 prompt tokens.
	texts := make([]string, 10)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, 40, resp.Usage().PromptTokens())
	require.Equal(t, 40, resp.Usage().TotalTokens())
}

func TestOpenAIProvider_EmbedCancelledContext(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
		MaxRetries:     0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	texts := make([]string, 5)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	_, err := p.Embed(ctx, req)
	require.Error(t, err)
}

func TestOpenAIProvider_EmbedUnsupported(t *testing.T) {
	p := NewOpenAIProvider("test-key", WithEmbeddingModel(""))
	// Manually disable embedding support.
	p.supportsEmbedding = false

	req := NewEmbeddingRequest([]string{"hello"})
	_, err := p.Embed(context.Background(), req)
	require.ErrorIs(t, err, ErrUnsupportedOperation)
}
