package repository

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// TestDelete_SharedCommitEnrichments covers two repositories sharing a git
// commit's chunk. Deleting either one keeps the chunk for the other and
// removes only the deleted repository's association.
func TestDelete_SharedCommitEnrichments(t *testing.T) {
	for _, deleteOwner := range []bool{false, true} {
		t.Run("delete owner="+strconv.FormatBool(deleteOwner), func(t *testing.T) {
			ctx := context.Background()
			db := testdb.New(t)
			logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
			repos := persistence.NewRepositoryStore(db)
			commits := persistence.NewCommitStore(db)
			enrichmentStore := persistence.NewEnrichmentStore(db)
			associations := persistence.NewAssociationStore(db)
			enrichments := service.NewEnrichment(enrichmentStore, associations, nil, nil, nil, nil, nil)
			h := NewDelete(handler.RepositoryStores{
				Repositories: repos, Commits: commits,
				Branches: persistence.NewBranchStore(db), Tags: persistence.NewTagStore(db), Files: persistence.NewFileStore(db),
			}, enrichments, service.NewQueue(persistence.NewTaskStore(db), logger), &fakeSyncTrackerFactory{}, logger)

			var saved []repository.Repository
			for _, url := range []string{"https://example.com/owner.git", "https://example.com/fork.git"} {
				r, err := repository.NewRepository(url)
				require.NoError(t, err)
				r, err = repos.Save(ctx, r)
				require.NoError(t, err)
				saved = append(saved, r)
			}
			owner, fork := saved[0], saved[1]
			author := repository.NewAuthor("Test", "test@example.com")
			_, err := commits.Save(ctx, repository.NewCommit("shared123", owner.ID(), "m", author, author, time.Now(), time.Now()))
			require.NoError(t, err)

			chunk, err := enrichmentStore.Save(ctx, enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit, "chunk"))
			require.NoError(t, err)
			for _, a := range []enrichment.Association{
				enrichment.CommitAssociation(chunk.ID(), "shared123"),
				enrichment.RepositoryAssociation(chunk.ID(), strconv.FormatInt(owner.ID(), 10)),
				enrichment.RepositoryAssociation(chunk.ID(), strconv.FormatInt(fork.ID(), 10)),
			} {
				_, err := associations.Save(ctx, a)
				require.NoError(t, err)
			}

			deleted, kept := fork, owner
			if deleteOwner {
				deleted, kept = owner, fork
			}
			require.NoError(t, h.Execute(ctx, map[string]any{"repository_id": deleted.ID()}))

			exists, err := enrichmentStore.Exists(ctx, repository.WithID(chunk.ID()))
			require.NoError(t, err)
			assert.True(t, exists, "the remaining repository keeps the shared chunk")
			owners, err := enrichments.RepositoryIDs(ctx, []int64{chunk.ID()})
			require.NoError(t, err)
			assert.Equal(t, []int64{kept.ID()}, owners[strconv.FormatInt(chunk.ID(), 10)])
		})
	}
}
