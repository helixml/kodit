package repository

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	repositorydomain "github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/queue"
	"github.com/stretchr/testify/assert"
)

// FakeRepoRepository implements repositorydomain.RepositoryStore for testing.
type FakeRepoRepository struct {
	repos      map[int64]repositorydomain.Repository
	reposByURL map[string]repositorydomain.Repository
	nextID     int64
	findOneErr error
	saveErr    error
}

func NewFakeRepoRepository() *FakeRepoRepository {
	return &FakeRepoRepository{
		repos:      make(map[int64]repositorydomain.Repository),
		reposByURL: make(map[string]repositorydomain.Repository),
		nextID:     1,
	}
}

func (r *FakeRepoRepository) Find(ctx context.Context, options ...repositorydomain.Option) ([]repositorydomain.Repository, error) {
	var repos []repositorydomain.Repository
	for _, repo := range r.repos {
		repos = append(repos, repo)
	}
	return repos, nil
}

func (r *FakeRepoRepository) FindOne(ctx context.Context, options ...repositorydomain.Option) (repositorydomain.Repository, error) {
	if r.findOneErr != nil {
		return repositorydomain.Repository{}, r.findOneErr
	}
	q := repositorydomain.Build(options...)
	for _, cond := range q.Conditions() {
		switch cond.Field() {
		case "id":
			id, ok := cond.Value().(int64)
			if !ok {
				continue
			}
			repo, found := r.repos[id]
			if !found {
				return repositorydomain.Repository{}, errors.New("not found")
			}
			return repo, nil
		case "sanitized_remote_uri":
			url, ok := cond.Value().(string)
			if !ok {
				continue
			}
			repo, found := r.reposByURL[url]
			if !found {
				return repositorydomain.Repository{}, errors.New("not found")
			}
			return repo, nil
		}
	}
	return repositorydomain.Repository{}, errors.New("not found")
}

func (r *FakeRepoRepository) Exists(ctx context.Context, options ...repositorydomain.Option) (bool, error) {
	_, err := r.FindOne(ctx, options...)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (r *FakeRepoRepository) Save(ctx context.Context, repo repositorydomain.Repository) (repositorydomain.Repository, error) {
	if r.saveErr != nil {
		return repositorydomain.Repository{}, r.saveErr
	}
	if repo.ID() == 0 {
		repo = repo.WithID(r.nextID)
		r.nextID++
	}
	r.repos[repo.ID()] = repo
	r.reposByURL[repo.RemoteURL()] = repo
	return repo, nil
}

func (r *FakeRepoRepository) Delete(ctx context.Context, repo repositorydomain.Repository) error {
	delete(r.repos, repo.ID())
	delete(r.reposByURL, repo.RemoteURL())
	return nil
}

func (r *FakeRepoRepository) AddRepo(repo repositorydomain.Repository) {
	if repo.ID() == 0 {
		repo = repo.WithID(r.nextID)
		r.nextID++
	}
	r.repos[repo.ID()] = repo
	r.reposByURL[repo.RemoteURL()] = repo
}

// FakeCommitRepository implements repositorydomain.CommitStore for testing.
type FakeCommitRepository struct {
	commits map[int64]repositorydomain.Commit
}

func NewFakeCommitRepository() *FakeCommitRepository {
	return &FakeCommitRepository{
		commits: make(map[int64]repositorydomain.Commit),
	}
}

func (r *FakeCommitRepository) Find(ctx context.Context, options ...repositorydomain.Option) ([]repositorydomain.Commit, error) {
	var commits []repositorydomain.Commit
	for _, commit := range r.commits {
		commits = append(commits, commit)
	}
	return commits, nil
}

func (r *FakeCommitRepository) FindOne(ctx context.Context, options ...repositorydomain.Option) (repositorydomain.Commit, error) {
	for _, commit := range r.commits {
		return commit, nil
	}
	return repositorydomain.Commit{}, errors.New("not found")
}

func (r *FakeCommitRepository) Exists(ctx context.Context, options ...repositorydomain.Option) (bool, error) {
	return len(r.commits) > 0, nil
}

func (r *FakeCommitRepository) Save(ctx context.Context, commit repositorydomain.Commit) (repositorydomain.Commit, error) {
	return commit, nil
}

func (r *FakeCommitRepository) SaveAll(ctx context.Context, commits []repositorydomain.Commit) ([]repositorydomain.Commit, error) {
	return commits, nil
}

func (r *FakeCommitRepository) Delete(ctx context.Context, commit repositorydomain.Commit) error {
	return nil
}

// FakeBranchRepository implements repositorydomain.BranchStore for testing.
type FakeBranchRepository struct {
	branches []repositorydomain.Branch
}

func NewFakeBranchRepository() *FakeBranchRepository {
	return &FakeBranchRepository{
		branches: make([]repositorydomain.Branch, 0),
	}
}

func (r *FakeBranchRepository) Find(ctx context.Context, options ...repositorydomain.Option) ([]repositorydomain.Branch, error) {
	return r.branches, nil
}

func (r *FakeBranchRepository) FindOne(ctx context.Context, options ...repositorydomain.Option) (repositorydomain.Branch, error) {
	if len(r.branches) == 0 {
		return repositorydomain.Branch{}, errors.New("not found")
	}
	return r.branches[0], nil
}

func (r *FakeBranchRepository) Save(ctx context.Context, branch repositorydomain.Branch) (repositorydomain.Branch, error) {
	return branch, nil
}

func (r *FakeBranchRepository) SaveAll(ctx context.Context, branches []repositorydomain.Branch) ([]repositorydomain.Branch, error) {
	return branches, nil
}

func (r *FakeBranchRepository) Delete(ctx context.Context, branch repositorydomain.Branch) error {
	return nil
}

// FakeTagRepository implements repositorydomain.TagStore for testing.
type FakeTagRepository struct {
	tags []repositorydomain.Tag
}

func NewFakeTagRepository() *FakeTagRepository {
	return &FakeTagRepository{
		tags: make([]repositorydomain.Tag, 0),
	}
}

func (r *FakeTagRepository) Find(ctx context.Context, options ...repositorydomain.Option) ([]repositorydomain.Tag, error) {
	return r.tags, nil
}

func (r *FakeTagRepository) FindOne(ctx context.Context, options ...repositorydomain.Option) (repositorydomain.Tag, error) {
	if len(r.tags) == 0 {
		return repositorydomain.Tag{}, errors.New("not found")
	}
	return r.tags[0], nil
}

func (r *FakeTagRepository) Save(ctx context.Context, tag repositorydomain.Tag) (repositorydomain.Tag, error) {
	return tag, nil
}

func (r *FakeTagRepository) SaveAll(ctx context.Context, tags []repositorydomain.Tag) ([]repositorydomain.Tag, error) {
	return tags, nil
}

func (r *FakeTagRepository) Delete(ctx context.Context, tag repositorydomain.Tag) error {
	return nil
}

func TestQueryService_ByID(t *testing.T) {
	ctx := context.Background()

	repoRepo := NewFakeRepoRepository()
	commitRepo := NewFakeCommitRepository()
	branchRepo := NewFakeBranchRepository()
	tagRepo := NewFakeTagRepository()

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	wc := repositorydomain.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo.git")
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

	repo1, _ := repositorydomain.NewRepository("https://github.com/test/repo1.git")
	repoRepo.AddRepo(repo1)

	repo2, _ := repositorydomain.NewRepository("https://github.com/test/repo2.git")
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

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
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

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
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

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	wc := repositorydomain.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo.git")
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

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
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

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
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

	repo, _ := repositorydomain.NewRepository("https://github.com/test/repo.git")
	repo = repo.WithID(1)
	repoRepo.AddRepo(repo)

	svc := NewSyncService(repoRepo, queueService, logger)

	tc := repositorydomain.NewTrackingConfigForBranch("develop")
	source, err := svc.UpdateTrackingConfig(ctx, 1, tc)

	assert.NoError(t, err)
	assert.Equal(t, "develop", source.TrackingConfig().Branch())
}
