package e2e_test

import (
	"net/http"
	"testing"
)

func TestSearch_POST_MalformedJSON_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.POSTRaw("/api/v1/search", `{invalid json}`)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}
