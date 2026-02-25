package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/helixml/kodit/internal/database"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// cacheEntry is the GORM model for cached HTTP responses.
type cacheEntry struct {
	Key        string `gorm:"primaryKey;column:key"`
	StatusCode int    `gorm:"column:status_code"`
	Header     []byte `gorm:"column:header"`
	Body       []byte `gorm:"column:body"`
}

func (cacheEntry) TableName() string { return "http_cache" }

// CachingTransport is an http.RoundTripper that caches POST request/response
// pairs in a SQLite database, keyed by the SHA-256 of method + URL + request body.
// Only 2xx responses are cached. Cache read/write errors are non-fatal —
// they silently fall through to the inner transport.
type CachingTransport struct {
	inner http.RoundTripper
	db    database.Database
}

// NewCachingTransport creates a CachingTransport that stores cached responses
// in a SQLite database under dir/http_cache.db.
// If inner is nil, http.DefaultTransport is used.
func NewCachingTransport(dir string, inner http.RoundTripper) (*CachingTransport, error) {
	if inner == nil {
		inner = http.DefaultTransport
	}

	db, err := database.NewDatabaseWithConfig(
		context.Background(),
		"sqlite:///"+dir+"/http_cache.db",
		&gorm.Config{Logger: logger.Discard},
	)
	if err != nil {
		return nil, err
	}

	if err := db.GORM().AutoMigrate(&cacheEntry{}); err != nil {
		_ = db.Close()
		return nil, err
	}

	// The default SQLite pool is a single connection, which serializes all
	// access. WAL mode supports concurrent readers, so we open multiple
	// connections to let reads proceed in parallel. Writes still serialize
	// at the SQLite level; busy_timeout handles contention.
	if err := db.ConfigurePool(4, 4, 0); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &CachingTransport{inner: inner, db: db}, nil
}

// Close closes the underlying SQLite database.
func (t *CachingTransport) Close() error {
	return t.db.Close()
}

// RoundTrip implements http.RoundTripper.
func (t *CachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	key := cacheKey(req.Method, req.URL.String(), body)

	if resp, ok := t.readCache(key, req); ok {
		return resp, nil
	}

	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()

	t.writeCache(key, resp.StatusCode, resp.Header, respBody)

	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	return resp, nil
}

func cacheKey(method, url string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte("\n"))
	h.Write([]byte(url))
	h.Write([]byte("\n"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func (t *CachingTransport) readCache(key string, req *http.Request) (*http.Response, bool) {
	var entry cacheEntry
	result := t.db.GORM().Where("`key` = ?", key).First(&entry)
	if result.Error != nil {
		return nil, false
	}

	var header http.Header
	if err := json.Unmarshal(entry.Header, &header); err != nil {
		return nil, false
	}

	resp := &http.Response{
		StatusCode: entry.StatusCode,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(entry.Body)),
		Request:    req,
	}
	return resp, true
}

func (t *CachingTransport) writeCache(key string, statusCode int, header http.Header, body []byte) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return
	}

	entry := cacheEntry{
		Key:        key,
		StatusCode: statusCode,
		Header:     headerJSON,
		Body:       body,
	}

	// Use Save to upsert — if the key already exists, update it.
	_ = t.db.GORM().Save(&entry).Error
}
