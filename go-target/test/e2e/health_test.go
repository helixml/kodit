package e2e_test

import (
	"net/http"
	"testing"
)

func TestHealth_ReturnsHealthy(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/health")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := ts.ReadBody(resp)
	expected := `{"status":"healthy"}`
	if body != expected {
		t.Errorf("body = %q, want %q", body, expected)
	}
}
