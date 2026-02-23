package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// CachingTransport is an http.RoundTripper that caches POST request/response
// pairs on disk, keyed by the SHA-256 of method + URL + request body.
// Only 2xx responses are cached. Cache read/write errors are non-fatal —
// they silently fall through to the inner transport.
type CachingTransport struct {
	inner http.RoundTripper
	dir   string
}

// NewCachingTransport creates a CachingTransport that stores cache files
// under dir. If inner is nil, http.DefaultTransport is used.
func NewCachingTransport(dir string, inner http.RoundTripper) *CachingTransport {
	if inner == nil {
		inner = http.DefaultTransport
	}
	_ = os.MkdirAll(dir, 0o755)
	return &CachingTransport{inner: inner, dir: dir}
}

type cachedResponse struct {
	StatusCode int                 `json:"status_code"`
	Header     map[string][]string `json:"header"`
	Body       string              `json:"body"`
}

// RoundTrip implements http.RoundTripper.
func (t *CachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	key := t.cacheKey(req.Method, req.URL.String(), body)
	path := filepath.Join(t.dir, key+".json")

	if resp, ok := t.readCache(path, req); ok {
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

	t.writeCache(path, resp.StatusCode, resp.Header, respBody)

	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	return resp, nil
}

func (t *CachingTransport) cacheKey(method, url string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte("\n"))
	h.Write([]byte(url))
	h.Write([]byte("\n"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func (t *CachingTransport) readCache(path string, req *http.Request) (*http.Response, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var cached cachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}

	body, err := base64.StdEncoding.DecodeString(cached.Body)
	if err != nil {
		return nil, false
	}

	resp := &http.Response{
		StatusCode: cached.StatusCode,
		Header:     cached.Header,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	return resp, true
}

func (t *CachingTransport) writeCache(path string, statusCode int, header http.Header, body []byte) {
	cached := cachedResponse{
		StatusCode: statusCode,
		Header:     header,
		Body:       base64.StdEncoding.EncodeToString(body),
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
