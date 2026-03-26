package e2e_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

func TestDuplicates_MissingRepositoryIDs_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.POST("/api/v1/search/duplicates", map[string]any{
		"data": map[string]any{
			"type":       "duplicate_search",
			"attributes": map[string]any{},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDuplicates_InvalidThreshold_Zero_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repo := ts.CreateRepository("https://github.com/test/repo.git")

	threshold := 0.0
	resp := ts.POST("/api/v1/search/duplicates", dto.DuplicateSearchRequest{
		Data: dto.DuplicateSearchData{
			Type: "duplicate_search",
			Attributes: dto.DuplicateSearchAttributes{
				RepositoryIDs: []int64{repo.ID()},
				Threshold:     &threshold,
			},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("threshold=0: status = %d, want 400", resp.StatusCode)
	}
}

func TestDuplicates_InvalidThreshold_TooLarge_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repo := ts.CreateRepository("https://github.com/test/repo.git")

	threshold := 1.5
	resp := ts.POST("/api/v1/search/duplicates", dto.DuplicateSearchRequest{
		Data: dto.DuplicateSearchData{
			Type: "duplicate_search",
			Attributes: dto.DuplicateSearchAttributes{
				RepositoryIDs: []int64{repo.ID()},
				Threshold:     &threshold,
			},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("threshold=1.5: status = %d, want 400", resp.StatusCode)
	}
}

func TestDuplicates_InvalidLimit_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repo := ts.CreateRepository("https://github.com/test/repo.git")

	limit := 0
	resp := ts.POST("/api/v1/search/duplicates", dto.DuplicateSearchRequest{
		Data: dto.DuplicateSearchData{
			Type: "duplicate_search",
			Attributes: dto.DuplicateSearchAttributes{
				RepositoryIDs: []int64{repo.ID()},
				Limit:         &limit,
			},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("limit=0: status = %d, want 400", resp.StatusCode)
	}
}

func TestDuplicates_NoEmbeddings_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)
	repo := ts.CreateRepository("https://github.com/test/repo.git")

	resp := ts.POST("/api/v1/search/duplicates", dto.DuplicateSearchRequest{
		Data: dto.DuplicateSearchData{
			Type: "duplicate_search",
			Attributes: dto.DuplicateSearchAttributes{
				RepositoryIDs: []int64{repo.ID()},
			},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result dto.DuplicatesResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("want 0 pairs, got %d", len(result.Data))
	}
}

func TestDuplicates_WithSeededEmbeddings_ReturnsPairs(t *testing.T) {
	ts := NewTestServer(t)

	// Create repository + commit + two enrichments.
	repo := ts.CreateRepository("https://github.com/test/dup.git")
	commit := ts.CreateCommit(repo, "abc123def456", "Initial commit")
	e1 := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func foo() {}", "go")
	e2 := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func foo() {}", "go")

	// Seed near-identical embeddings for both snippets (same direction vector).
	vec1 := []float64{1.0, 0.5, 0.25}
	vec2 := []float64{1.0, 0.5, 0.25}
	ts.SeedCodeEmbedding(strconv.FormatInt(e1.ID(), 10), vec1)
	ts.SeedCodeEmbedding(strconv.FormatInt(e2.ID(), 10), vec2)

	threshold := 0.90
	resp := ts.POST("/api/v1/search/duplicates", dto.DuplicateSearchRequest{
		Data: dto.DuplicateSearchData{
			Type: "duplicate_search",
			Attributes: dto.DuplicateSearchAttributes{
				RepositoryIDs: []int64{repo.ID()},
				Threshold:     &threshold,
			},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	var result dto.DuplicatesResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Fatalf("want 1 pair, got %d", len(result.Data))
	}
	if result.Data[0].Attributes.Similarity < 0.99 {
		t.Errorf("want similarity ~1.0, got %f", result.Data[0].Attributes.Similarity)
	}
}

func TestDuplicates_BelowThreshold_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	repo := ts.CreateRepository("https://github.com/test/dup2.git")
	commit := ts.CreateCommit(repo, "deadbeef1234", "Initial commit")
	e1 := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func a() {}", "go")
	e2 := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func b() {}", "go")

	// Orthogonal vectors: cosine similarity = 0
	ts.SeedCodeEmbedding(strconv.FormatInt(e1.ID(), 10), []float64{1, 0, 0})
	ts.SeedCodeEmbedding(strconv.FormatInt(e2.ID(), 10), []float64{0, 1, 0})

	threshold := 0.90
	resp := ts.POST("/api/v1/search/duplicates", dto.DuplicateSearchRequest{
		Data: dto.DuplicateSearchData{
			Type: "duplicate_search",
			Attributes: dto.DuplicateSearchAttributes{
				RepositoryIDs: []int64{repo.ID()},
				Threshold:     &threshold,
			},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result dto.DuplicatesResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("want 0 pairs (below threshold), got %d", len(result.Data))
	}
}

func TestDuplicates_MissingRequestBody_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.POSTRaw("/api/v1/search/duplicates", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
