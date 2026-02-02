package repository

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/queue"
	"github.com/stretchr/testify/assert"
)

// FakeRepoRepository implements git.RepoRepository for testing.
type FakeRepoRepository struct {
	repos      map[int64]git.Repo
	reposByURL map[string]git.Repo
	nextID     int64
	getErr     error
	saveErr    error
}

func NewFakeRepoRepository() *FakeRepoRepository {
	return &FakeRepoRepository{
		repos:      make(map[int64]git.Repo),
		reposByURL: make(map[string]git.Repo),
		nextID:     1,
	}
}

func (r *FakeRepoRepository) Get(ctx context.Context, id int64) (git.Repo, error) {
	if r.getErr != nil {
		return git.Repo{}, r.getErr
	}
	repo, ok := r.repos[id]
	if !ok {
		return git.Repo{}, errors.New("not found")
	}
	return repo, nil
}

func (r *FakeRepoRepository) Find(ctx context.Context, query database.Query) ([]git.Repo, error) {
	var repos []git.Repo
	for _, repo := range r.repos {
		repos = append(repos, repo)
	}
	return repos, nil
}

func (r *FakeRepoRepository) FindAll(ctx context.Context) ([]git.Repo, error) {
	var repos []git.Repo
	for _, repo := range r.repos {
		repos = append(repos, repo)
	}
	return repos, nil
}

func (r *FakeRepoRepository) Save(ctx context.Context, repo git.Repo) (git.Repo, error) {
	if r.saveErr != nil {
		return git.Repo{}, r.saveErr
	}
	if repo.ID() == 0 {
		repo = repo.WithID(r.nextID)
		r.nextID++
	}
	r.repos[repo.ID()] = repo
	r.reposByURL[repo.RemoteURL()] = repo
	return repo, nil
}

func (r *FakeRepoRepository) Delete(ctx context.Context, repo git.Repo) error {
	delete(r.repos, repo.ID())
	delete(r.reposByURL, repo.RemoteURL())
	return nil
}

func (r *FakeRepoRepository) GetByRemoteURL(ctx context.Context, url string) (git.Repo, error) {
	repo, ok := r.reposByURL[url]
	if !ok {
		return git.Repo{}, errors.New("not found")
	}
	return repo, nil
}

func (r *FakeRepoRepository) ExistsByRemoteURL(ctx context.Context, url string) (bool, error) {
	_, ok := r.reposByURL[url]
	return ok, nil
}

func (r *FakeRepoRepository) AddRepo(repo git.Repo) {
	if repo.ID() == 0 {
		repo = repo.WithID(r.nextID)
		r.nextID++
	}
	r.repos[repo.ID()] = repo
	r.reposByURL[repo.RemoteURL()] = repo
}

// FakeCommitRepository implements git.CommitRepository for testing.
type FakeCommitRepository struct {
	commits map[int64]git.Commit
}

func NewFakeCommitRepository() *FakeCommitRepository {
	return &FakeCommitRepository{
		commits: make(map[int64]git.Commit),
	}
}

func (r *FakeCommitRepository) Get(ctx context.Context, id int64) (git.Commit, error) {
	commit, ok := r.commits[id]
	if !ok {
		return git.Commit{}, errors.New("not found")
	}
	return commit, nil
}

func (r *FakeCommitRepository) Find(ctx context.Context, query database.Query) ([]git.Commit, error) {
	return nil, nil
}

func (r *FakeCommitRepository) Save(ctx context.Context, commit git.Commit) (git.Commit, error) {
	return commit, nil
}

func (r *FakeCommitRepository) SaveAll(ctx context.Context, commits []git.Commit) ([]git.Commit, error) {
	return commits, nil
}

func (r *FakeCommitRepository) Delete(ctx context.Context, commit git.Commit) error {
	return nil
}

func (r *FakeCommitRepository) GetByRepoAndSHA(ctx context.Context, repoID int64, sha string) (git.Commit, error) {
	return git.Commit{}, errors.New("not found")
}

func (r *FakeCommitRepository) FindByRepoID(ctx context.Context, repoID int64) ([]git.Commit, error) {
	return nil, nil
}

func (r *FakeCommitRepository) ExistsBySHA(ctx context.Context, repoID int64, sha string) (bool, error) {
	return false, nil
}

// FakeBranchRepository implements git.BranchRepository for testing.
type FakeBranchRepository struct {
	branches []git.Branch
}

func NewFakeBranchRepository() *FakeBranchRepository {
	return &FakeBranchRepository{
		branches: make([]git.Branch, 0),
	}
}

func (r *FakeBranchRepository) Get(ctx context.Context, id int64) (git.Branch, error) {
	return git.Branch{}, errors.New("not found")
}

func (r *FakeBranchRepository) Find(ctx context.Context, query database.Query) ([]git.Branch, error) {
	return r.branches, nil
}

func (r *FakeBranchRepository) Save(ctx context.Context, branch git.Branch) (git.Branch, error) {
	return branch, nil
}

func (r *FakeBranchRepository) SaveAll(ctx context.Context, branches []git.Branch) ([]git.Branch, error) {
	return branches, nil
}

func (r *FakeBranchRepository) Delete(ctx context.Context, branch git.Branch) error {
	return nil
}

func (r *FakeBranchRepository) GetByName(ctx context.Context, repoID int64, name string) (git.Branch, error) {
	return git.Branch{}, errors.New("not found")
}

func (r *FakeBranchRepository) FindByRepoID(ctx context.Context, repoID int64) ([]git.Branch, error) {
	return r.branches, nil
}

func (r *FakeBranchRepository) GetDefaultBranch(ctx context.Context, repoID int64) (git.Branch, error) {
	for _, b := range r.branches {
		if b.IsDefault() {
			return b, nil
		}
	}
	return git.Branch{}, errors.New("not found")
}

// FakeTagRepository implements git.TagRepository for testing.
type FakeTagRepository struct {
	tags []git.Tag
}

func NewFakeTagRepository() *FakeTagRepository {
	return &FakeTagRepository{
		tags: make([]git.Tag, 0),
	}
}

func (r *FakeTagRepository) Get(ctx context.Context, id int64) (git.Tag, error) {
	return git.Tag{}, errors.New("not found")
}

func (r *FakeTagRepository) Find(ctx context.Context, query database.Query) ([]git.Tag, error) {
	return r.tags, nil
}

func (r *FakeTagRepository) Save(ctx context.Context, tag git.Tag) (git.Tag, error) {
	return tag, nil
}

func (r *FakeTagRepository) SaveAll(ctx context.Context, tags []git.Tag) ([]git.Tag, error) {
	return tags, nil
}

func (r *FakeTagRepository) Delete(ctx context.Context, tag git.Tag) error {
	return nil
}

func (r *FakeTagRepository) GetByName(ctx context.Context, repoID int64, name string) (git.Tag, error) {
	return git.Tag{}, errors.New("not found")
}

func (r *FakeTagRepository) FindByRepoID(ctx context.Context, repoID int64) ([]git.Tag, error) {
	return r.tags, nil
}

func TestQueryService_ByID(t *testing.T) {
	ctx := context.Background()

	repoRepo := NewFakeRepoRepository()
	commitRepo := NewFakeCommitRepository()
	branchRepo := NewFakeBranchRepository()
	tagRepo := NewFakeTagRepository()

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	wc := git.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo.git")
	repo = repo.WithWorkingCopy(wc)
	repoRepo.AddRepo(repo)

	svc := NewQueryService(repoRepo, commitRepo, branchRepo, tagRepo)

	source, err := svc.ByID(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), source.ID())
	assert.Equal(t, "https://github.com/test/repo.git", source.RemoteURL())
	assert.Equal(t, StatusCloned, source.Status())
}

func TestQueryService_ByID_NotFound(t *testing.T) {
	ctx := context.Background()

	repoRepo := NewFakeRepoRepository()
	commitRepo := NewFakeCommitRepository()
	branchRepo := NewFakeBranchRepository()
	tagRepo := NewFakeTagRepository()

	svc := NewQueryService(repoRepo, commitRepo, branchRepo, tagRepo)

	_, err := svc.ByID(ctx, 999)

	assert.Error(t, err)
}

func TestQueryService_All(t *testing.T) {
	ctx := context.Background()

	repoRepo := NewFakeRepoRepository()
	commitRepo := NewFakeCommitRepository()
	branchRepo := NewFakeBranchRepository()
	tagRepo := NewFakeTagRepository()

	repo1, _ := git.NewRepo("https://github.com/test/repo1.git")
	repoRepo.AddRepo(repo1)

	repo2, _ := git.NewRepo("https://github.com/test/repo2.git")
	repoRepo.AddRepo(repo2)

	svc := NewQueryService(repoRepo, commitRepo, branchRepo, tagRepo)

	sources, err := svc.All(ctx)

	assert.NoError(t, err)
	assert.Len(t, sources, 2)
}

func TestQueryService_Exists(t *testing.T) {
	ctx := context.Background()

	repoRepo := NewFakeRepoRepository()
	commitRepo := NewFakeCommitRepository()
	branchRepo := NewFakeBranchRepository()
	tagRepo := NewFakeTagRepository()

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repoRepo.AddRepo(repo)

	svc := NewQueryService(repoRepo, commitRepo, branchRepo, tagRepo)

	exists, err := svc.Exists(ctx, "https://github.com/test/repo.git")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = svc.Exists(ctx, "https://github.com/test/nonexistent.git")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestSyncService_AddRepository(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := NewFakeRepoRepository()
	queueService := queue.NewService(queue.NewFakeTaskRepository(), logger)

	svc := NewSyncService(repoRepo, queueService, logger)

	source, err := svc.AddRepository(ctx, "https://github.com/test/repo.git")

	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/test/repo.git", source.RemoteURL())
	assert.Equal(t, StatusPending, source.Status())
}

func TestSyncService_AddRepository_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := NewFakeRepoRepository()
	queueService := queue.NewService(queue.NewFakeTaskRepository(), logger)

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repoRepo.AddRepo(repo)

	svc := NewSyncService(repoRepo, queueService, logger)

	_, err := svc.AddRepository(ctx, "https://github.com/test/repo.git")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSyncService_RequestSync(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := NewFakeRepoRepository()
	queueService := queue.NewService(queue.NewFakeTaskRepository(), logger)

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	wc := git.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo.git")
	repo = repo.WithWorkingCopy(wc)
	repoRepo.AddRepo(repo)

	svc := NewSyncService(repoRepo, queueService, logger)

	err := svc.RequestSync(ctx, 1)

	assert.NoError(t, err)
}

func TestSyncService_RequestSync_NotCloned(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := NewFakeRepoRepository()
	queueService := queue.NewService(queue.NewFakeTaskRepository(), logger)

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	repoRepo.AddRepo(repo)

	svc := NewSyncService(repoRepo, queueService, logger)

	err := svc.RequestSync(ctx, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not been cloned")
}

func TestSyncService_RequestDelete(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := NewFakeRepoRepository()
	queueService := queue.NewService(queue.NewFakeTaskRepository(), logger)

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	repoRepo.AddRepo(repo)

	svc := NewSyncService(repoRepo, queueService, logger)

	err := svc.RequestDelete(ctx, 1)

	assert.NoError(t, err)
}

func TestSyncService_UpdateTrackingConfig(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := NewFakeRepoRepository()
	queueService := queue.NewService(queue.NewFakeTaskRepository(), logger)

	repo, _ := git.NewRepo("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	repoRepo.AddRepo(repo)

	svc := NewSyncService(repoRepo, queueService, logger)

	tc := git.NewTrackingConfigForBranch("develop")
	source, err := svc.UpdateTrackingConfig(ctx, 1, tc)

	assert.NoError(t, err)
	assert.Equal(t, "develop", source.TrackingConfig().Branch())
}
