package persistence_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// n is large enough so that n × columns_per_row exceeds SQLite's
// SQLITE_MAX_VARIABLE_NUMBER (32766) when all rows are inserted in a
// single statement. The fix (CreateInBatches with gitBatchSize=1000)
// keeps each batch well under the limit.
const bulkN = 10_000

// TestFileStore_SaveAll_Large verifies that saving a large number of files
// does not exceed the database engine's bind-parameter limit.
// FileModel has 8 bound columns → 10 000 × 8 = 80 000 params (fails without batching).
func TestFileStore_SaveAll_Large(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewFileStore(db)
	ctx := context.Background()

	files := make([]repository.File, bulkN)
	for i := range files {
		files[i] = repository.NewFileWithDetails(
			fmt.Sprintf("abc%04d", i),
			fmt.Sprintf("path/file%d.go", i),
			fmt.Sprintf("blob%04d", i),
			"text/plain",
			".go",
			int64(i),
		)
	}

	_, err := store.SaveAll(ctx, files)
	require.NoError(t, err)
}

// TestCommitStore_SaveAll_Large verifies that saving a large number of commits
// does not exceed the bind-parameter limit.
// CommitModel has 8 bound columns → 10 000 × 8 = 80 000 params.
func TestCommitStore_SaveAll_Large(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewCommitStore(db)
	ctx := context.Background()

	now := time.Now()
	author := repository.NewAuthor("alice", "alice@example.com")

	commits := make([]repository.Commit, bulkN)
	for i := range commits {
		commits[i] = repository.NewCommit(
			fmt.Sprintf("commit%06d", i),
			1,
			fmt.Sprintf("commit message %d", i),
			author,
			author,
			now,
			now,
		)
	}

	_, err := store.SaveAll(ctx, commits)
	require.NoError(t, err)
}

// TestBranchStore_SaveAll_Large verifies that saving a large number of branches
// does not exceed the bind-parameter limit.
// BranchModel has 6 bound columns → 10 000 × 6 = 60 000 params.
func TestBranchStore_SaveAll_Large(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewBranchStore(db)
	ctx := context.Background()

	branches := make([]repository.Branch, bulkN)
	for i := range branches {
		branches[i] = repository.NewBranch(
			1,
			fmt.Sprintf("branch-%d", i),
			fmt.Sprintf("head%06d", i),
			i == 0,
		)
	}

	_, err := store.SaveAll(ctx, branches)
	require.NoError(t, err)
}

// TestTagStore_SaveAll_Large verifies that saving a large number of tags
// does not exceed the bind-parameter limit.
// TagModel has 9 bound columns → 10 000 × 9 = 90 000 params.
func TestTagStore_SaveAll_Large(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTagStore(db)
	ctx := context.Background()

	tags := make([]repository.Tag, bulkN)
	for i := range tags {
		tags[i] = repository.NewTag(
			1,
			fmt.Sprintf("v0.0.%d", i),
			fmt.Sprintf("commit%06d", i),
		)
	}

	_, err := store.SaveAll(ctx, tags)
	require.NoError(t, err)
}
