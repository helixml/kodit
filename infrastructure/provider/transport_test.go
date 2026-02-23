package provider

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
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
	transport := NewCachingTransport(dir, srv.Client().Transport)

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

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 cache file, got %d", len(entries))
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
	transport := NewCachingTransport(dir, srv.Client().Transport)

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
	transport := NewCachingTransport(dir, srv.Client().Transport)

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

	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("expected 2 cache files, got %d", len(entries))
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
	transport := NewCachingTransport(dir, srv.Client().Transport)

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
	transport := NewCachingTransport(t.TempDir(), &failingTransport{})

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/api", strings.NewReader("body"))
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	entries, _ := os.ReadDir(transport.dir)
	if len(entries) != 0 {
		t.Errorf("expected 0 cache files, got %d", len(entries))
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
	transport := NewCachingTransport(dir, srv.Client().Transport)

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

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected 0 cache files, got %d", len(entries))
	}
}

func TestCachingTransport_CorruptCacheFile(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	transport := NewCachingTransport(dir, srv.Client().Transport)

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

	// Corrupt the cache file
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 cache file, got %d", len(entries))
	}
	_ = os.WriteFile(filepath.Join(dir, entries[0].Name()), []byte("not json{{{"), 0o644)

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

// failingTransport always returns an error.
type failingTransport struct{}

func (f *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrServerClosed
}
