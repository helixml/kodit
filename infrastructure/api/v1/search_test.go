package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
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
	bm25Store, err := persistence.NewSQLiteBM25Store(db, zerolog.New(os.Stderr).With().Timestamp().Logger())
	if err != nil {
		t.Fatalf("create bm25 store: %v", err)
	}
	snippetID := fmt.Sprintf("%d", saved.ID())
	doc := search.NewDocument(snippetID, "func hello")
	err = bm25Store.Index(ctx, []search.Document{doc})
	if err != nil {
		t.Fatalf("index bm25: %v", err)
	}

	// Seed line range.
	lineRangeStore := persistence.NewSourceLocationStore(db)
	lr := sourcelocation.New(saved.ID(), 5, 12)
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

// TestSearchRouter_WarnsOnEmptyContent verifies that when a search hit has
// empty Content (e.g. because chunk persistence failed silently for a PDF —
// see issue #553) the search path logs a warning identifying the source file
// rather than just propagating the empty string.
func TestSearchRouter_WarnsOnEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	// Seed a chunk enrichment with EMPTY content. This models the bug where
	// chunk persistence failed silently but BM25 still returned the hit.
	enrichmentStore := persistence.NewEnrichmentStore(db)
	e := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"",
	)
	saved, err := enrichmentStore.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	// Seed BM25 with searchable text so the empty-content enrichment is hit.
	bm25Store, err := persistence.NewSQLiteBM25Store(db, zerolog.New(os.Stderr))
	if err != nil {
		t.Fatalf("create bm25 store: %v", err)
	}
	snippetID := strconv.FormatInt(saved.ID(), 10)
	doc := search.NewDocument(snippetID, "broken pdf content")
	if err := bm25Store.Index(ctx, []search.Document{doc}); err != nil {
		t.Fatalf("index bm25: %v", err)
	}

	// Seed file + commit + repository so the warning has a path to identify.
	commitSHA := "deadbeef00000000000000000000000000000000"
	repoStore := persistence.NewRepositoryStore(db)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo"))
	savedRepo, err := repoStore.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	commitStore := persistence.NewCommitStore(db)
	author := repository.NewAuthor("phil", "phil@winder.ai")
	now := time.Now()
	commit := repository.NewCommit(commitSHA, savedRepo.ID(), "msg", author, author, now, now)
	if _, err := commitStore.Save(ctx, commit); err != nil {
		t.Fatalf("save commit: %v", err)
	}

	fileStore := persistence.NewFileStore(db)
	f := repository.NewFileWithDetails(commitSHA, "broken.pdf", "abc123", "application/pdf", ".pdf", 5000)
	savedFile, err := fileStore.Save(ctx, f)
	if err != nil {
		t.Fatalf("save file: %v", err)
	}

	associationStore := persistence.NewAssociationStore(db)
	if _, err := associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA)); err != nil {
		t.Fatalf("save commit association: %v", err)
	}
	if _, err := associationStore.Save(ctx, enrichment.FileAssociation(saved.ID(), strconv.FormatInt(savedFile.ID(), 10))); err != nil {
		t.Fatalf("save file association: %v", err)
	}

	_ = db.Close()

	// Capture logger output so we can assert the warning fires.
	var logBuf bytes.Buffer
	testLogger := zerolog.New(&logBuf)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithLogger(testLogger),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	router := v1.NewSearchRouter(client)
	routes := router.Routes()

	body := `{"data":{"type":"search","attributes":{"keywords":["broken"]}}}`
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
		t.Fatal("expected at least one search hit so we can verify the warning")
	}
	if response.Data[0].Attributes.Content.Value != "" {
		t.Fatalf("test setup wrong: expected empty content, got %q", response.Data[0].Attributes.Content.Value)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "broken.pdf") {
		t.Errorf("warning should identify source file path. log output: %s", logOutput)
	}
	if !strings.Contains(logOutput, fmt.Sprintf("%q", "warn")) && !strings.Contains(logOutput, `"level":"warn"`) {
		t.Errorf("expected a warn-level log entry. log output: %s", logOutput)
	}
	if !strings.Contains(logOutput, "empty content") {
		t.Errorf("expected warning to mention empty content. log output: %s", logOutput)
	}
}
