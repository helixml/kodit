//go:build integration

package persistence

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
)

// TestVectorChordEmbeddingStore_ExistingSnippetIDsAcrossChunks confirms that
// search.ExistingSnippetIDs correctly returns ALL existing IDs even when the
// input set exceeds search.MaxSnippetIDsPerFind. This guards the regression
// from issue #554, where a single Find with WithLimit(N) silently dropped
// matches beyond N and re-embedded the overflow on every cycle.
//
//	VECTORCHORD_TEST_URL="postgresql://postgres:mysecretpassword@localhost:5432/kodit" \
//	  go test -tags integration -v -run TestVectorChordEmbeddingStore_ExistingSnippetIDsAcrossChunks \
//	  ./infrastructure/persistence/
func TestVectorChordEmbeddingStore_ExistingSnippetIDsAcrossChunks(t *testing.T) {
	dsn := os.Getenv("VECTORCHORD_TEST_URL")
	if dsn == "" {
		t.Skip("VECTORCHORD_TEST_URL not set")
	}

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t))

	db, err := database.NewDatabase(ctx, dsn)
	require.NoError(t, err)

	taskName := TaskName("dedupe_live_test")
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)
	dropTable := func() { db.Session(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)) }

	dropTable()
	defer func() {
		dropTable()
		_ = db.Close()
	}()

	store := NewVectorChordEmbeddingStore(db, taskName, nil, logger)

	// Save more embeddings than the per-Find chunk size so dedupe must span
	// multiple lookups. The exact count mirrors the production scenario from
	// the bug report (a repo with thousands of chunk enrichments).
	total := search.MaxSnippetIDsPerFind*2 + 25
	embeddings := make([]search.Document, total)
	ids := make([]string, total)
	for i := range total {
		ids[i] = strconv.Itoa(i + 1)
		embeddings[i] = search.NewVectorDocument(ids[i], []float64{0.1, 0.2, 0.3, 0.4})
	}
	require.NoError(t, store.Index(ctx, embeddings))

	existing, err := search.ExistingSnippetIDs(ctx, store, ids)
	require.NoError(t, err)

	assert.Len(t, existing, total,
		"all %d snippet IDs must be reported as existing — bug #554 regression check", total)
	for _, id := range ids {
		_, ok := existing[id]
		assert.True(t, ok, "id %s must be in the existing set", id)
	}
}
