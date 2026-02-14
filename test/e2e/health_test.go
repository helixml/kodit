package e2e_test

import (
	"net/http"
	"testing"
)

func TestHealth_ReturnsOK(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/healthz")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
