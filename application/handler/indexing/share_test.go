package indexing

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// TestChunkFiles_AlreadyChunkedCommit covers a repository indexing a commit
// SHA another repository already chunked. The repository gains a repository
// association on the existing chunk only when its own git history holds the
// commit; a SHA it does not hold (named by a rescan, a tracking config or a
// task) shares nothing.
func TestChunkFiles_AlreadyChunkedCommit(t *testing.T) {
	const sha = "shared123"
	tests := []struct {
		name      string
		holds     commitReader
		wantOwner []string
	}{
		{"repository holds the commit", commitReader{sha: true}, []string{"1", "2"}},
		{"repository does not hold the commit", commitReader{}, []string{"1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := testdb.New(t)
			repos := persistence.NewRepositoryStore(db)
			enrichments := persistence.NewEnrichmentStore(db)
			associations := persistence.NewAssociationStore(db)

			var repoIDs []int64
			for _, url := range []string{"https://example.com/a.git", "https://example.com/b.git"} {
				r, err := repository.NewRepository(url)
				require.NoError(t, err)
				r = r.WithWorkingCopy(repository.NewWorkingCopy(t.TempDir(), url))
				saved, err := repos.Save(ctx, r)
				require.NoError(t, err)
				repoIDs = append(repoIDs, saved.ID())
			}
			require.Equal(t, []int64{1, 2}, repoIDs)

			chunk, err := enrichments.Save(ctx, enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit, "chunk"))
			require.NoError(t, err)
			for _, a := range []enrichment.Association{
				enrichment.CommitAssociation(chunk.ID(), sha),
				enrichment.RepositoryAssociation(chunk.ID(), "1"),
			} {
				_, err := associations.Save(ctx, a)
				require.NoError(t, err)
			}

			h := NewChunkFiles(repos, enrichments, associations, nil, nil, tt.holds, nil, nil, nil, nil,
				chunking.ChunkParams{}, &fakeTrackerFactory{}, zerolog.Nop())
			for range 2 { // running twice must not add a second association
				require.NoError(t, h.Execute(ctx, map[string]any{"repository_id": int64(2), "commit_sha": sha}))
			}

			assocs, err := associations.Find(ctx,
				enrichment.WithEnrichmentIDIn([]int64{chunk.ID()}),
				enrichment.WithEntityType(enrichment.EntityTypeRepository),
			)
			require.NoError(t, err)
			var owners []string
			for _, a := range assocs {
				owners = append(owners, a.EntityID())
			}
			assert.ElementsMatch(t, tt.wantOwner, owners)
		})
	}
}
