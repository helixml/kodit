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

	texts := make([]string, 10)
	for i := range texts {
		texts[i] = "text"
	}

	req := NewEmbeddingRequest(texts)
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Embeddings(), 10)
	require.Equal(t, int64(1), counter.Load(), "10 texts should be one request")
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

// emptyResponseServer returns an httptest.Server that always responds with an
// empty embedding data array (simulating OpenRouter returning 200 with no vectors).
// After failCount requests, it starts returning correct responses.
func emptyResponseServer(t *testing.T, counter *atomic.Int64, failCount int64) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := counter.Add(1)

		var body struct {
			Input interface{} `json:"input"`
			Model string      `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var texts []string
		switch v := body.Input.(type) {
		case string:
			texts = []string{v}
		case []interface{}:
			for _, item := range v {
				texts = append(texts, item.(string))
			}
		}

		// Return empty data until failCount is reached.
		var data []map[string]interface{}
		if n > failCount {
			data = make([]map[string]interface{}, len(texts))
			for i := range texts {
				data[i] = map[string]interface{}{
					"object":    "embedding",
					"index":     i,
					"embedding": []float64{0.1, 0.2, 0.3},
				}
			}
		}

		resp := map[string]interface{}{
			"object": "list",
			"data":   data,
			"model":  body.Model,
			"usage":  map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestOpenAIProvider_EmbedEmptyResponseReturnsError(t *testing.T) {
	var counter atomic.Int64
	// Always return empty — never recover.
	srv := emptyResponseServer(t, &counter, 999)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
		MaxRetries:     0,
		InitialDelay:   time.Millisecond,
	})

	req := NewEmbeddingRequest([]string{"hello", "world"})
	_, err := p.Embed(context.Background(), req)
	require.Error(t, err)
	require.ErrorIs(t, err, errEmbeddingCountMismatch)
}

func TestOpenAIProvider_EmbedEmptyResponseRetries(t *testing.T) {
	var counter atomic.Int64
	// Fail the first 2 requests, then succeed.
	srv := emptyResponseServer(t, &counter, 2)
	defer srv.Close()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL,
		EmbeddingModel: "test-model",
		MaxRetries:     3,
		InitialDelay:   time.Millisecond,
	})

	req := NewEmbeddingRequest([]string{"hello", "world"})
	resp, err := p.Embed(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Embeddings(), 2)
	require.Equal(t, int64(3), counter.Load(), "should have retried twice then succeeded")
}
