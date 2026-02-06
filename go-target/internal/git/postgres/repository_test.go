package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schema defines the SQLite schema for testing.
const schema = `
CREATE TABLE IF NOT EXISTS git_repos (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	sanitized_remote_uri TEXT NOT NULL UNIQUE,
	remote_uri TEXT NOT NULL,
	cloned_path TEXT,
	last_scanned_at DATETIME,
	num_commits INTEGER DEFAULT 0,
	num_branches INTEGER DEFAULT 0,
	num_tags INTEGER DEFAULT 0,
	tracking_type TEXT,
	tracking_name TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS git_commits (
	commit_sha TEXT PRIMARY KEY,
	repo_id INTEGER NOT NULL,
	date DATETIME NOT NULL,
	message TEXT,
	parent_commit_sha TEXT,
	author TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_branches (
	repo_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	head_commit_sha TEXT NOT NULL,
	is_default BOOLEAN DEFAULT 0,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	PRIMARY KEY (repo_id, name),
	FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_tags (
	repo_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	target_commit_sha TEXT NOT NULL,
	message TEXT,
	tagger_name TEXT,
	tagger_email TEXT,
	tagged_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	PRIMARY KEY (repo_id, name),
	FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_commit_files (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	commit_sha TEXT NOT NULL,
	path TEXT NOT NULL,
	blob_sha TEXT NOT NULL,
	mime_type TEXT,
	extension TEXT,
	size INTEGER,
	created_at DATETIME NOT NULL,
	UNIQUE (commit_sha, path),
	FOREIGN KEY (commit_sha) REFERENCES git_commits(commit_sha)
);
`

func testDB(t *testing.T) database.Database {
	t.Helper()
	return testutil.TestDatabaseWithSchema(t, schema)
}

func sampleRepo(t *testing.T) git.Repo {
	t.Helper()
	repo, err := git.NewRepo("https://github.com/test/repo")
	require.NoError(t, err)
	return repo
}

func sampleCommit(t *testing.T, repoID int64) git.Commit {
	t.Helper()
	author := git.NewAuthor("Test User", "test@example.com")
	now := time.Now()
	return git.NewCommit("abc123def456", repoID, "Test commit message", author, author, now, now)
}

// TestRepoRepository tests the RepoRepository implementation.
func TestRepoRepository(t *testing.T) {
	t.Run("save and get", func(t *testing.T) {
		db := testDB(t)
		repo := NewRepoRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		saved, err := repo.Save(ctx, sample)
		require.NoError(t, err)
		assert.NotZero(t, saved.ID())
		assert.Equal(t, sample.RemoteURL(), saved.RemoteURL())

		retrieved, err := repo.Get(ctx, saved.ID())
		require.NoError(t, err)
		assert.Equal(t, saved.ID(), retrieved.ID())
		assert.Equal(t, saved.RemoteURL(), retrieved.RemoteURL())
	})

	t.Run("get by remote URL", func(t *testing.T) {
		db := testDB(t)
		repo := NewRepoRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		saved, err := repo.Save(ctx, sample)
		require.NoError(t, err)

		retrieved, err := repo.GetByRemoteURL(ctx, sample.RemoteURL())
		require.NoError(t, err)
		assert.Equal(t, saved.ID(), retrieved.ID())
	})

	t.Run("find all", func(t *testing.T) {
		db := testDB(t)
		repo := NewRepoRepository(db)
		ctx := context.Background()

		repo1, _ := git.NewRepo("https://github.com/test/repo1")
		repo2, _ := git.NewRepo("https://github.com/test/repo2")

		_, err := repo.Save(ctx, repo1)
		require.NoError(t, err)
		_, err = repo.Save(ctx, repo2)
		require.NoError(t, err)

		all, err := repo.FindAll(ctx)
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("delete", func(t *testing.T) {
		db := testDB(t)
		repo := NewRepoRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		saved, err := repo.Save(ctx, sample)
		require.NoError(t, err)

		err = repo.Delete(ctx, saved)
		require.NoError(t, err)

		_, err = repo.Get(ctx, saved.ID())
		assert.Error(t, err)
	})

	t.Run("exists by remote URL", func(t *testing.T) {
		db := testDB(t)
		repo := NewRepoRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		exists, err := repo.ExistsByRemoteURL(ctx, sample.RemoteURL())
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = repo.Save(ctx, sample)
		require.NoError(t, err)

		exists, err = repo.ExistsByRemoteURL(ctx, sample.RemoteURL())
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

// TestCommitRepository tests the CommitRepository implementation.
func TestCommitRepository(t *testing.T) {
	t.Run("save and get by repo and SHA", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		ctx := context.Background()

		// Create repo first (foreign key requirement)
		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		saved, err := commitRepo.Save(ctx, commit)
		require.NoError(t, err)
		assert.Equal(t, commit.SHA(), saved.SHA())

		retrieved, err := commitRepo.GetByRepoAndSHA(ctx, savedRepo.ID(), commit.SHA())
		require.NoError(t, err)
		assert.Equal(t, commit.SHA(), retrieved.SHA())
		assert.Equal(t, savedRepo.ID(), retrieved.RepoID())
	})

	t.Run("save all", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		author := git.NewAuthor("Test", "test@example.com")
		now := time.Now()
		commits := []git.Commit{
			git.NewCommit("sha1111", savedRepo.ID(), "Message 1", author, author, now, now),
			git.NewCommit("sha2222", savedRepo.ID(), "Message 2", author, author, now, now),
		}

		saved, err := commitRepo.SaveAll(ctx, commits)
		require.NoError(t, err)
		assert.Len(t, saved, 2)
	})

	t.Run("find by repo ID", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		author := git.NewAuthor("Test", "test@example.com")
		now := time.Now()
		commits := []git.Commit{
			git.NewCommit("sha1111", savedRepo.ID(), "Message 1", author, author, now, now),
			git.NewCommit("sha2222", savedRepo.ID(), "Message 2", author, author, now, now),
		}

		_, err = commitRepo.SaveAll(ctx, commits)
		require.NoError(t, err)

		found, err := commitRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Len(t, found, 2)
	})

	t.Run("exists by SHA", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		exists, err := commitRepo.ExistsBySHA(ctx, savedRepo.ID(), "nonexistent")
		require.NoError(t, err)
		assert.False(t, exists)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		exists, err = commitRepo.ExistsBySHA(ctx, savedRepo.ID(), commit.SHA())
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

// TestBranchRepository tests the BranchRepository implementation.
func TestBranchRepository(t *testing.T) {
	t.Run("save and find by repo ID", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		branchRepo := NewBranchRepository(db)
		ctx := context.Background()

		// Create repo and commit first
		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		branch := git.NewBranch(savedRepo.ID(), "main", commit.SHA(), true)
		_, err = branchRepo.Save(ctx, branch)
		require.NoError(t, err)

		branches, err := branchRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Len(t, branches, 1)
		assert.Equal(t, "main", branches[0].Name())
	})

	t.Run("save multiple branches", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		branchRepo := NewBranchRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		branches := []git.Branch{
			git.NewBranch(savedRepo.ID(), "main", commit.SHA(), true),
			git.NewBranch(savedRepo.ID(), "develop", commit.SHA(), false),
		}

		_, err = branchRepo.SaveAll(ctx, branches)
		require.NoError(t, err)

		found, err := branchRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Len(t, found, 2)

		names := make(map[string]bool)
		for _, b := range found {
			names[b.Name()] = true
		}
		assert.True(t, names["main"])
		assert.True(t, names["develop"])
	})

	t.Run("get by name", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		branchRepo := NewBranchRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		branch := git.NewBranch(savedRepo.ID(), "feature", commit.SHA(), false)
		_, err = branchRepo.Save(ctx, branch)
		require.NoError(t, err)

		retrieved, err := branchRepo.GetByName(ctx, savedRepo.ID(), "feature")
		require.NoError(t, err)
		assert.Equal(t, "feature", retrieved.Name())
	})

	t.Run("get default branch", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		branchRepo := NewBranchRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		branches := []git.Branch{
			git.NewBranch(savedRepo.ID(), "main", commit.SHA(), true),
			git.NewBranch(savedRepo.ID(), "develop", commit.SHA(), false),
		}

		_, err = branchRepo.SaveAll(ctx, branches)
		require.NoError(t, err)

		defaultBranch, err := branchRepo.GetDefaultBranch(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Equal(t, "main", defaultBranch.Name())
		assert.True(t, defaultBranch.IsDefault())
	})

	t.Run("empty repository returns empty list", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		branchRepo := NewBranchRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		branches, err := branchRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Empty(t, branches)
	})
}

// TestTagRepository tests the TagRepository implementation.
func TestTagRepository(t *testing.T) {
	t.Run("save and find by repo ID", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		tagRepo := NewTagRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		tag := git.NewTag(savedRepo.ID(), "v1.0.0", commit.SHA())
		_, err = tagRepo.Save(ctx, tag)
		require.NoError(t, err)

		tags, err := tagRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Len(t, tags, 1)
		assert.Equal(t, "v1.0.0", tags[0].Name())
	})

	t.Run("save multiple tags", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		tagRepo := NewTagRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		tags := []git.Tag{
			git.NewTag(savedRepo.ID(), "v1.0.0", commit.SHA()),
			git.NewTag(savedRepo.ID(), "v2.0.0", commit.SHA()),
		}

		_, err = tagRepo.SaveAll(ctx, tags)
		require.NoError(t, err)

		found, err := tagRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Len(t, found, 2)

		names := make(map[string]bool)
		for _, tag := range found {
			names[tag.Name()] = true
		}
		assert.True(t, names["v1.0.0"])
		assert.True(t, names["v2.0.0"])
	})

	t.Run("get by name", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		tagRepo := NewTagRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		tag := git.NewTag(savedRepo.ID(), "v3.0.0", commit.SHA())
		_, err = tagRepo.Save(ctx, tag)
		require.NoError(t, err)

		retrieved, err := tagRepo.GetByName(ctx, savedRepo.ID(), "v3.0.0")
		require.NoError(t, err)
		assert.Equal(t, "v3.0.0", retrieved.Name())
	})

	t.Run("empty repository returns empty list", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		tagRepo := NewTagRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		tags, err := tagRepo.FindByRepoID(ctx, savedRepo.ID())
		require.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("nonexistent repository returns empty list", func(t *testing.T) {
		db := testDB(t)
		tagRepo := NewTagRepository(db)
		ctx := context.Background()

		tags, err := tagRepo.FindByRepoID(ctx, 99999)
		require.NoError(t, err)
		assert.Empty(t, tags)
	})
}

// TestFileRepository tests the FileRepository implementation.
func TestFileRepository(t *testing.T) {
	t.Run("save and get", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		fileRepo := NewFileRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		file := git.NewFile(commit.SHA(), "src/main.go", "go", 1024)
		saved, err := fileRepo.Save(ctx, file)
		require.NoError(t, err)
		assert.Equal(t, "src/main.go", saved.Path())

		retrieved, err := fileRepo.GetByCommitAndPath(ctx, commit.SHA(), "src/main.go")
		require.NoError(t, err)
		assert.Equal(t, "src/main.go", retrieved.Path())
		assert.Equal(t, commit.SHA(), retrieved.CommitSHA())
	})

	t.Run("save all and find", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		fileRepo := NewFileRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		files := []git.File{
			git.NewFile(commit.SHA(), "src/main.go", "go", 1024),
			git.NewFile(commit.SHA(), "src/utils.go", "go", 512),
			git.NewFile(commit.SHA(), "README.md", "md", 2048),
		}

		saved, err := fileRepo.SaveAll(ctx, files)
		require.NoError(t, err)
		assert.Len(t, saved, 3)

		found, err := fileRepo.FindByCommitSHA(ctx, commit.SHA())
		require.NoError(t, err)
		assert.Len(t, found, 3)
	})

	t.Run("find by extension", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		fileRepo := NewFileRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		files := []git.File{
			git.NewFile(commit.SHA(), "src/main.go", "go", 1024),
			git.NewFile(commit.SHA(), "src/utils.go", "go", 512),
			git.NewFile(commit.SHA(), "README.md", "md", 2048),
		}

		_, err = fileRepo.SaveAll(ctx, files)
		require.NoError(t, err)

		// Find using query with filter
		query := database.NewQuery().Equal("extension", "go")
		goFiles, err := fileRepo.Find(ctx, query)
		require.NoError(t, err)
		assert.Len(t, goFiles, 2)
		for _, f := range goFiles {
			assert.Equal(t, "go", f.Language())
		}
	})

	t.Run("delete by commit SHA", func(t *testing.T) {
		db := testDB(t)
		repoRepo := NewRepoRepository(db)
		commitRepo := NewCommitRepository(db)
		fileRepo := NewFileRepository(db)
		ctx := context.Background()

		sample := sampleRepo(t)
		savedRepo, err := repoRepo.Save(ctx, sample)
		require.NoError(t, err)

		commit := sampleCommit(t, savedRepo.ID())
		_, err = commitRepo.Save(ctx, commit)
		require.NoError(t, err)

		files := []git.File{
			git.NewFile(commit.SHA(), "src/main.go", "go", 1024),
			git.NewFile(commit.SHA(), "src/utils.go", "go", 512),
		}

		_, err = fileRepo.SaveAll(ctx, files)
		require.NoError(t, err)

		err = fileRepo.DeleteByCommitSHA(ctx, commit.SHA())
		require.NoError(t, err)

		found, err := fileRepo.FindByCommitSHA(ctx, commit.SHA())
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}
