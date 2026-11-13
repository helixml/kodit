package persistence

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// TestSourceRepoFilter_UsesRepositoryAssociationsOnly checks that the
// source-repository filter matches an enrichment only through its direct
// repository associations. git_commits records repository 3 for the SHA of
// enrichment 10 (as if repository 3 had been scanned with that SHA), but
// only repository 1 owns the enrichment, so a filter for repository 3 must
// not return it. Enrichment 20 is shared by repositories 2 and 4 and is
// returned once for either or both.
func TestSourceRepoFilter_UsesRepositoryAssociationsOnly(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, AutoMigrate(db))
	ctx := context.Background()

	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, zerolog.Nop())
	require.NoError(t, err)
	associations := NewAssociationStore(db)
	commits := NewCommitStore(db)

	now := time.Now()
	author := repository.NewAuthor("Test", "test@example.com")
	_, err = commits.Save(ctx, repository.NewCommit("sha10", 3, "m", author, author, now, now))
	require.NoError(t, err)

	for _, a := range []enrichment.Association{
		enrichment.CommitAssociation(10, "sha10"),
		enrichment.RepositoryAssociation(10, "1"),
		enrichment.CommitAssociation(20, "sha20"),
		enrichment.RepositoryAssociation(20, "2"),
		enrichment.RepositoryAssociation(20, "4"),
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

	require.Equal(t, []string{"10"}, find(1))
	require.Empty(t, find(3), "git_commits ownership of the SHA must not grant the enrichment")
	require.Equal(t, []string{"20"}, find(2))
	require.Equal(t, []string{"20"}, find(4))
	require.Equal(t, []string{"20"}, find(2, 4), "a shared enrichment is returned once")
	require.Equal(t, []string{"10", "20"}, find(1, 2))
}
