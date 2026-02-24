package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestCachingTransport_CacheMiss(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(`{"input":"hello"}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"result":"ok"}` {
		t.Errorf("unexpected body: %s", body)
	}

	if count.Load() != 1 {
		t.Errorf("expected 1 upstream call, got %d", count.Load())
	}
}

func TestCachingTransport_CacheHit(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	for i := range 3 {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(`{"input":"hello"}`))
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if string(body) != `{"result":"ok"}` {
			t.Errorf("request %d: unexpected body: %s", i, body)
		}
	}

	if count.Load() != 1 {
		t.Errorf("expected 1 upstream call, got %d", count.Load())
	}
}

func TestCachingTransport_DifferentBodies(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	bodies := []string{`{"input":"hello"}`, `{"input":"world"}`}
	for _, b := range bodies {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(b))
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()
	}

	if count.Load() != 2 {
		t.Errorf("expected 2 upstream calls, got %d", count.Load())
	}
}

func TestCachingTransport_PreservesHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// First request — populates cache
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api", strings.NewReader("body"))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	// Second request — from cache
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api", strings.NewReader("body"))
	resp, err = transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Custom") != "test-value" {
		t.Errorf("expected X-Custom test-value, got %s", resp.Header.Get("X-Custom"))
	}
}

func TestCachingTransport_InnerError(t *testing.T) {
	transport, err := NewCachingTransport(t.TempDir(), &failingTransport{})
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/api", strings.NewReader("body"))
	_, err = transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCachingTransport_NonSuccessNotCached(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	for range 2 {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api", strings.NewReader("body"))
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()
	}

	if count.Load() != 2 {
		t.Errorf("expected 2 upstream calls (no caching for 500), got %d", count.Load())
	}
}

func TestCachingTransport_CorruptCacheEntry(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// First request — populates cache
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api", strings.NewReader("body"))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	if count.Load() != 1 {
		t.Fatalf("expected 1 upstream call, got %d", count.Load())
	}

	// Corrupt the cache entry's header column to invalid JSON
	key := cacheKey(http.MethodPost, srv.URL+"/api", []byte("body"))
	transport.db.GORM().Model(&cacheEntry{}).Where("`key` = ?", key).Update("header", []byte("not json{{{"))

	// Next request should fall through to upstream
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api", strings.NewReader("body"))
	resp, err = transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("unexpected body: %s", body)
	}

	if count.Load() != 2 {
		t.Errorf("expected 2 upstream calls after corruption, got %d", count.Load())
	}
}

func TestCachingTransport_EmbeddingProvider(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)

		body, _ := io.ReadAll(r.Body)
		var req openai.EmbeddingRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request"}`))
			return
		}

		// The go-openai library sends input as a JSON array of strings.
		inputs, ok := req.Input.([]any)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":"input not array: %T"}`, req.Input)
			return
		}

		data := make([]openai.Embedding, len(inputs))
		for i := range inputs {
			data[i] = openai.Embedding{
				Index:     i,
				Embedding: []float32{0.1, 0.2, 0.3},
			}
		}

		resp := openai.EmbeddingResponse{
			Data:  data,
			Model: openai.AdaEmbeddingV2,
			Usage: openai.Usage{PromptTokens: 10, TotalTokens: 10},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport, err := NewCachingTransport(dir, srv.Client().Transport)
	if err != nil {
		t.Fatalf("unexpected error creating transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	p := NewOpenAIProviderFromConfig(OpenAIConfig{
		APIKey:         "test-key",
		BaseURL:        srv.URL + "/v1",
		EmbeddingModel: "text-embedding-3-small",
		MaxRetries:     1,
		HTTPClient: &http.Client{
			Transport: transport,
		},
	})

	texts := []string{"hello world", "foo bar"}
	ctx := t.Context()

	// First call — should hit upstream
	resp1, err := p.Embed(ctx, NewEmbeddingRequest(texts))
	if err != nil {
		t.Fatalf("first embed: %v", err)
	}
	if len(resp1.Embeddings()) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(resp1.Embeddings()))
	}
	if count.Load() != 1 {
		t.Fatalf("expected 1 upstream call after first embed, got %d", count.Load())
	}

	// Second call with identical texts — should come from cache
	resp2, err := p.Embed(ctx, NewEmbeddingRequest(texts))
	if err != nil {
		t.Fatalf("second embed: %v", err)
	}
	if len(resp2.Embeddings()) != 2 {
		t.Fatalf("expected 2 embeddings from cache, got %d", len(resp2.Embeddings()))
	}
	if count.Load() != 1 {
		t.Errorf("expected 1 upstream call (cached), got %d", count.Load())
	}

	// Third call with different texts — should hit upstream again
	_, err = p.Embed(ctx, NewEmbeddingRequest([]string{"different text"}))
	if err != nil {
		t.Fatalf("third embed: %v", err)
	}
	if count.Load() != 2 {
		t.Errorf("expected 2 upstream calls after different texts, got %d", count.Load())
	}
}

// failingTransport always returns an error.
type failingTransport struct{}

func (f *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrServerClosed
}
