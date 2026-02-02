package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snippetSchema defines the SQLite schema for snippet testing.
const snippetSchema = `
CREATE TABLE IF NOT EXISTS git_commit_files (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	commit_sha TEXT NOT NULL,
	path TEXT NOT NULL,
	extension TEXT,
	size INTEGER,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS snippets (
	sha TEXT PRIMARY KEY,
	content TEXT NOT NULL,
	extension TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS snippet_commit_associations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	snippet_sha TEXT NOT NULL,
	commit_sha TEXT NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS snippet_file_derivations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	snippet_sha TEXT NOT NULL,
	file_id INTEGER NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS enrichments_v2 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	subtype TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS enrichment_associations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	enrichment_id INTEGER NOT NULL,
	entity_type TEXT NOT NULL,
	entity_id TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);
`

func snippetTestDB(t *testing.T) database.Database {
	t.Helper()
	return testutil.TestDatabaseWithSchema(t, snippetSchema)
}

func sampleSnippet(content string) indexing.Snippet {
	return indexing.NewSnippet(content, "go", nil)
}

func TestSnippetRepository_SaveAndByIDs(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	snippet1 := sampleSnippet("func foo() { return 1 }")
	snippet2 := sampleSnippet("func bar() { return 2 }")

	// Save snippets for a commit
	err := repo.Save(ctx, "commit123", []indexing.Snippet{snippet1, snippet2})
	require.NoError(t, err)

	// Retrieve by IDs
	retrieved, err := repo.ByIDs(ctx, []string{snippet1.SHA(), snippet2.SHA()})
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Verify content
	shaToContent := make(map[string]string)
	for _, s := range retrieved {
		shaToContent[s.SHA()] = s.Content()
	}
	assert.Equal(t, snippet1.Content(), shaToContent[snippet1.SHA()])
	assert.Equal(t, snippet2.Content(), shaToContent[snippet2.SHA()])
}

func TestSnippetRepository_SnippetsForCommit(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	snippet1 := sampleSnippet("func commit1_func1() {}")
	snippet2 := sampleSnippet("func commit1_func2() {}")
	snippet3 := sampleSnippet("func commit2_func1() {}")

	// Save snippets for different commits
	err := repo.Save(ctx, "commit1", []indexing.Snippet{snippet1, snippet2})
	require.NoError(t, err)

	err = repo.Save(ctx, "commit2", []indexing.Snippet{snippet3})
	require.NoError(t, err)

	// Get snippets for commit1
	commit1Snippets, err := repo.SnippetsForCommit(ctx, "commit1")
	require.NoError(t, err)
	assert.Len(t, commit1Snippets, 2)

	// Get snippets for commit2
	commit2Snippets, err := repo.SnippetsForCommit(ctx, "commit2")
	require.NoError(t, err)
	assert.Len(t, commit2Snippets, 1)

	// Get snippets for non-existent commit
	noSnippets, err := repo.SnippetsForCommit(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, noSnippets)
}

func TestSnippetRepository_DeleteForCommit(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	snippet1 := sampleSnippet("func to_delete() {}")
	snippet2 := sampleSnippet("func to_keep() {}")

	// Save snippets for different commits
	err := repo.Save(ctx, "delete_commit", []indexing.Snippet{snippet1})
	require.NoError(t, err)

	err = repo.Save(ctx, "keep_commit", []indexing.Snippet{snippet2})
	require.NoError(t, err)

	// Delete associations for one commit
	err = repo.DeleteForCommit(ctx, "delete_commit")
	require.NoError(t, err)

	// Verify commit has no snippets
	deleted, err := repo.SnippetsForCommit(ctx, "delete_commit")
	require.NoError(t, err)
	assert.Empty(t, deleted)

	// Verify other commit still has snippets
	kept, err := repo.SnippetsForCommit(ctx, "keep_commit")
	require.NoError(t, err)
	assert.Len(t, kept, 1)

	// Note: The snippet itself should still exist (content-addressed)
	byID, err := repo.ByIDs(ctx, []string{snippet1.SHA()})
	require.NoError(t, err)
	assert.Len(t, byID, 1, "Snippet should still exist even though association was deleted")
}

func TestSnippetRepository_ContentAddressedDeduplication(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	// Same content will produce same SHA
	content := "func shared_function() {}"
	snippet1 := sampleSnippet(content)
	snippet2 := sampleSnippet(content) // Identical content

	// Save the same snippet for two different commits
	err := repo.Save(ctx, "commit_a", []indexing.Snippet{snippet1})
	require.NoError(t, err)

	err = repo.Save(ctx, "commit_b", []indexing.Snippet{snippet2})
	require.NoError(t, err)

	// Both commits should have the snippet
	snippetsA, err := repo.SnippetsForCommit(ctx, "commit_a")
	require.NoError(t, err)
	assert.Len(t, snippetsA, 1)

	snippetsB, err := repo.SnippetsForCommit(ctx, "commit_b")
	require.NoError(t, err)
	assert.Len(t, snippetsB, 1)

	// They should be the same snippet (same SHA)
	assert.Equal(t, snippetsA[0].SHA(), snippetsB[0].SHA())

	// Only one snippet should exist in the database
	allByID, err := repo.ByIDs(ctx, []string{snippet1.SHA()})
	require.NoError(t, err)
	assert.Len(t, allByID, 1)
}

func TestSnippetRepository_ByIDs_EmptyInput(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	// Empty input should return empty result
	snippets, err := repo.ByIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, snippets)

	snippets, err = repo.ByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, snippets)
}

func TestSnippetRepository_Save_EmptySnippets(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	// Saving empty snippets should succeed without error
	err := repo.Save(ctx, "commit_empty", []indexing.Snippet{})
	require.NoError(t, err)

	err = repo.Save(ctx, "commit_nil", nil)
	require.NoError(t, err)
}

func TestSnippetRepository_Search_ByCommitSHA(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	snippet1 := sampleSnippet("func search_test_1() {}")
	snippet2 := sampleSnippet("func search_test_2() {}")

	err := repo.Save(ctx, "search_commit_1", []indexing.Snippet{snippet1})
	require.NoError(t, err)

	err = repo.Save(ctx, "search_commit_2", []indexing.Snippet{snippet2})
	require.NoError(t, err)

	// Search with commit filter
	request := domain.NewMultiSearchRequest(
		10, "", "", nil,
		domain.NewSnippetSearchFilters(
			domain.WithCommitSHAs([]string{"search_commit_1"}),
		),
	)

	results, err := repo.Search(ctx, request)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, snippet1.SHA(), results[0].SHA())
}

func TestSnippetRepository_Search_ByLanguage(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	goSnippet := indexing.NewSnippet("func goFunc() {}", "go", nil)
	pySnippet := indexing.NewSnippet("def py_func():\n    pass", "py", nil)

	err := repo.Save(ctx, "lang_commit", []indexing.Snippet{goSnippet, pySnippet})
	require.NoError(t, err)

	// Search for Go snippets
	request := domain.NewMultiSearchRequest(
		10, "", "", nil,
		domain.NewSnippetSearchFilters(
			domain.WithLanguage("go"),
		),
	)

	results, err := repo.Search(ctx, request)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "go", results[0].Extension())
}

func TestSnippetRepository_Search_ByCreatedTime(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	snippet := sampleSnippet("func time_test() {}")

	err := repo.Save(ctx, "time_commit", []indexing.Snippet{snippet})
	require.NoError(t, err)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	// Search with time filter that includes the snippet
	request := domain.NewMultiSearchRequest(
		10, "", "", nil,
		domain.NewSnippetSearchFilters(
			domain.WithCreatedAfter(past),
			domain.WithCreatedBefore(future),
		),
	)

	results, err := repo.Search(ctx, request)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Search with time filter that excludes the snippet
	request = domain.NewMultiSearchRequest(
		10, "", "", nil,
		domain.NewSnippetSearchFilters(
			domain.WithCreatedAfter(future),
		),
	)

	results, err = repo.Search(ctx, request)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSnippetRepository_Search_TopK(t *testing.T) {
	db := snippetTestDB(t)
	repo := NewSnippetRepository(db.Session(context.Background()))
	ctx := context.Background()

	// Create multiple snippets
	snippets := make([]indexing.Snippet, 5)
	for i := range 5 {
		snippets[i] = sampleSnippet("func test" + string(rune('0'+i)) + "() {}")
	}

	err := repo.Save(ctx, "topk_commit", snippets)
	require.NoError(t, err)

	// Search with limit
	request := domain.NewMultiSearchRequest(
		2, "", "", nil,
		domain.NewSnippetSearchFilters(),
	)

	results, err := repo.Search(ctx, request)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
