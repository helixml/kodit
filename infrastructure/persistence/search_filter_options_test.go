package persistence

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// TestSourceRepoFilter_SharedCommit covers two repositories that share a
// commit SHA. git_commits keeps one row per SHA (the last repository
// scanned), while the commit's chunks carry a repository association for the
// repository that indexed them. Both repositories own the chunks, so a
// source-repo filter for either must find them, and a filter for an
// unrelated repository must not.
func TestSourceRepoFilter_SharedCommit(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, AutoMigrate(db))
	ctx := context.Background()

	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, zerolog.Nop())
	require.NoError(t, err)
	associations := NewAssociationStore(db)
	commits := NewCommitStore(db)

	const indexedBy, scannedLast, unrelated, ownRepo int64 = 1, 3, 2, 4
	now := time.Now()
	author := repository.NewAuthor("Test", "test@example.com")
	for _, c := range []repository.Commit{
		repository.NewCommit("shared", scannedLast, "m", author, author, now, now),
		repository.NewCommit("own", ownRepo, "m", author, author, now, now),
	} {
		_, err := commits.Save(ctx, c)
		require.NoError(t, err)
	}

	// Enrichment 10: chunk of the shared commit, indexed by repository 1.
	// Enrichment 20: chunk of repository 4's own commit.
	for _, a := range []enrichment.Association{
		enrichment.CommitAssociation(10, "shared"),
		enrichment.RepositoryAssociation(10, strconv.FormatInt(indexedBy, 10)),
		enrichment.CommitAssociation(20, "own"),
		enrichment.RepositoryAssociation(20, strconv.FormatInt(ownRepo, 10)),
	} {
		_, err := associations.Save(ctx, a)
		require.NoError(t, err)
	}
	require.NoError(t, store.Index(ctx, []search.Document{
		search.NewVectorDocument("10", []float64{1, 0}),
		search.NewVectorDocument("20", []float64{0, 1}),
	}))

	find := func(repos ...int64) []string {
		t.Helper()
		results, err := store.Find(ctx,
			search.WithEmbedding([]float64{1, 1}),
			search.WithFilters(search.NewFilters(search.WithSourceRepos(repos))),
			repository.WithLimit(10),
		)
		require.NoError(t, err)
		ids := make([]string, 0, len(results))
		for _, r := range results {
			ids = append(ids, r.SnippetID())
		}
		sort.Strings(ids)
		return ids
	}

	require.Equal(t, []string{"10"}, find(indexedBy), "repository that indexed the shared commit")
	require.Equal(t, []string{"10"}, find(scannedLast), "repository git_commits records for the shared commit")
	require.Empty(t, find(unrelated), "unrelated repository")
	require.Equal(t, []string{"20"}, find(ownRepo))
	require.Equal(t, []string{"10", "20"}, find(indexedBy, ownRepo))
}
