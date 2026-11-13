package commit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// TestScan_DoesNotTakeOverAnotherRepositorysCommit checks that scanning a
// commit SHA another repository already recorded leaves that repository as
// the commit's owner. The scanner is nil: it must not be reached.
func TestScan_DoesNotTakeOverAnotherRepositorysCommit(t *testing.T) {
	ctx := context.Background()
	db := testdb.New(t)
	commits := persistence.NewCommitStore(db)

	author := repository.NewAuthor("test", "test@example.com")
	now := time.Now()
	_, err := commits.Save(ctx, repository.NewCommit("shared123", 1, "m", author, author, now, now))
	require.NoError(t, err)

	h := NewScan(persistence.NewRepositoryStore(db), commits, persistence.NewFileStore(db), nil,
		&fakeTrackerFactory{}, zerolog.New(os.Stdout).Level(zerolog.ErrorLevel))
	require.NoError(t, h.Execute(ctx, map[string]any{"repository_id": int64(2), "commit_sha": "shared123"}))

	owner, err := commits.FindOne(ctx, repository.WithSHA("shared123"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), owner.RepoID())
}
