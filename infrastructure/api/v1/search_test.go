package v1_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
	"github.com/helixml/kodit/infrastructure/persistence"
)

func TestSearchRouter_LineRanges(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open DB to seed data before creating the client.
	db := openTestDB(t, dbPath)

	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	// Seed enrichment.
	enrichmentStore := persistence.NewEnrichmentStore(db)
	e := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"func hello() { fmt.Println(\"hello\") }",
	)
	saved, err := enrichmentStore.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	// Seed BM25 index for the enrichment.
	bm25Store, err := persistence.NewSQLiteBM25Store(db, slog.Default())
	if err != nil {
		t.Fatalf("create bm25 store: %v", err)
	}
	snippetID := fmt.Sprintf("%d", saved.ID())
	doc := search.NewDocument(snippetID, "func hello")
	err = bm25Store.Index(ctx, search.NewIndexRequest([]search.Document{doc}))
	if err != nil {
		t.Fatalf("index bm25: %v", err)
	}

	// Seed line range.
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	lr := chunk.NewLineRange(saved.ID(), 5, 12)
	_, err = lineRangeStore.Save(ctx, lr)
	if err != nil {
		t.Fatalf("save line range: %v", err)
	}

	_ = db.Close()

	// Create client with the pre-seeded DB.
	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	router := v1.NewSearchRouter(client)
	routes := router.Routes()

	body := `{"data":{"type":"search","attributes":{"keywords":["hello"]}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response dto.SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Data) == 0 {
		t.Fatal("expected at least one search result")
	}

	content := response.Data[0].Attributes.Content
	if content.StartLine == nil {
		t.Error("content.start_line is nil, want non-nil")
	} else if *content.StartLine != 5 {
		t.Errorf("content.start_line = %d, want 5", *content.StartLine)
	}
	if content.EndLine == nil {
		t.Error("content.end_line is nil, want non-nil")
	} else if *content.EndLine != 12 {
		t.Errorf("content.end_line = %d, want 12", *content.EndLine)
	}
}
