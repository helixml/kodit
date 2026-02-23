package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

func TestOpenAIProvider_EmbedBatchesConcurrently(t *testing.T) {
	var counter atomic.Int64
	srv := fakeEmbeddingServer(t, &counter)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	// 25 texts should produce 3 batches: 10 + 10 + 5.
	texts := make([]string, 25)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Embeddings(), 25)
	require.Equal(t, int64(3), counter.Load(), "25 texts should produce 3 batch requests")
}

func TestOpenAIProvider_EmbedBatchesPreserveOrder(t *testing.T) {
	// Use a server that returns the input index in the embedding vector
	// so we can verify ordering is preserved across concurrent batches.
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)

		var body struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		data := make([]map[string]interface{}, len(body.Input))
		for i, text := range body.Input {
			// Encode the text length as the first element so we can verify order.
			data[i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": []float64{float64(len(text)), 0.0, 0.0},
			}
		}

		resp := map[string]interface{}{
			"object": "list",
			"data":   data,
			"model":  "test",
			"usage":  map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
	})

	// Each text has a unique length so we can verify ordering.
	texts := make([]string, 35)
	for i := range texts {
		b := make([]byte, i+1)
		for j := range b {
			b[j] = 'a'
		}
		texts[i] = string(b)
	}

	req := NewEmbeddingRequest(texts)
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 35)

	for i, emb := range embeddings {
		expected := float64(i + 1) // text length = i+1
		require.Equal(t, expected, emb[0], "embedding %d has wrong order marker", i)
	}
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

	// 20 texts = 2 batches of 10. Each batch returns prompt_tokens = n*4.
	texts := make([]string, 20)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)

	// 2 batches of 10, each reporting 10*4=40 prompt tokens.
	require.Equal(t, 80, resp.Usage().PromptTokens())
	require.Equal(t, 80, resp.Usage().TotalTokens())
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

	texts := make([]string, 25)
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

func TestPartition(t *testing.T) {
	t.Run("exact_multiple", func(t *testing.T) {
		texts := []string{"a", "b", "c", "d"}
		batches := partition(texts, 2)
		require.Len(t, batches, 2)
		require.Equal(t, []string{"a", "b"}, batches[0])
		require.Equal(t, []string{"c", "d"}, batches[1])
	})

	t.Run("remainder", func(t *testing.T) {
		texts := []string{"a", "b", "c", "d", "e"}
		batches := partition(texts, 2)
		require.Len(t, batches, 3)
		require.Equal(t, []string{"a", "b"}, batches[0])
		require.Equal(t, []string{"c", "d"}, batches[1])
		require.Equal(t, []string{"e"}, batches[2])
	})

	t.Run("single_batch", func(t *testing.T) {
		texts := []string{"a", "b"}
		batches := partition(texts, 10)
		require.Len(t, batches, 1)
		require.Equal(t, []string{"a", "b"}, batches[0])
	})

	t.Run("one_per_batch", func(t *testing.T) {
		texts := []string{"a", "b", "c"}
		batches := partition(texts, 1)
		require.Len(t, batches, 3)
	})
}

func TestOpenAIProvider_EmbedBatchError(t *testing.T) {
	// Server that fails on the second request.
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := counter.Add(1)
		if n >= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			resp := map[string]interface{}{
				"error": map[string]interface{}{
					"message": "server error",
					"type":    "server_error",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		var body struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		data := make([]map[string]interface{}, len(body.Input))
		for i := range body.Input {
			data[i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": []float64{0.1, 0.2, 0.3},
			}
		}

		resp := map[string]interface{}{
			"object": "list",
			"data":   data,
			"model":  "test",
			"usage":  map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
		MaxRetries:     0,
		InitialDelay:   time.Millisecond,
	})

	// 20 texts = 2 batches. Second batch will fail.
	texts := make([]string, 20)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	_, err := p.Embed(context.Background(), req)
	require.Error(t, err)
}
