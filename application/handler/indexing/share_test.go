package indexing

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// TestChunkFiles_AlreadyChunkedCommit covers a repository whose commit SHA
// another repository already chunked. A git commit is shared, so the second
// repository gains a repository association on the existing chunk. A
// directory snapshot is never shared.
func TestChunkFiles_AlreadyChunkedCommit(t *testing.T) {
	tests := []struct {
		name      string
		commit    func(sha string, repoID int64) repository.Commit
		wantOwner []string
	}{
		{
			name: "git commit is shared",
			commit: func(sha string, repoID int64) repository.Commit {
				author := repository.NewAuthor("Dev", "dev@example.com")
				return repository.NewCommit(sha, repoID, "m", author, author, time.Now(), time.Now())
			},
			wantOwner: []string{"1", "2"},
		},
		{
			name: "directory snapshot is not shared",
			commit: func(sha string, repoID int64) repository.Commit {
				return repository.NewDirectorySnapshot(sha, repoID, time.Now())
			},
			wantOwner: []string{"1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := testdb.New(t)
			enrichments := persistence.NewEnrichmentStore(db)
			associations := persistence.NewAssociationStore(db)
			commits := persistence.NewCommitStore(db)

			const sha = "shared123"
			_, err := commits.Save(ctx, tt.commit(sha, 1))
			require.NoError(t, err)
			chunk, err := enrichments.Save(ctx, enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit, "chunk"))
			require.NoError(t, err)
			for _, a := range []enrichment.Association{
				enrichment.CommitAssociation(chunk.ID(), sha),
				enrichment.RepositoryAssociation(chunk.ID(), "1"),
			} {
				_, err := associations.Save(ctx, a)
				require.NoError(t, err)
			}

			h := NewChunkFiles(nil, enrichments, associations, nil, nil, commits, nil, nil, nil, nil,
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
