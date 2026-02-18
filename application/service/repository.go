package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// RepositoryAddParams configures adding a new repository.
type RepositoryAddParams struct {
	URL    string
	Branch string
	Tag    string
	Commit string
}

// RescanParams configures a commit rescan operation.
type RescanParams struct {
	RepositoryID int64
	CommitSHA    string
}

// TrackingConfigParams configures a tracking config update.
type TrackingConfigParams struct {
	Branch string
	Tag    string
	Commit string
}

// Repository provides repository management and query operations.
// Embeds Collection for Find/Get; bespoke methods handle writes and lifecycle.
type Repository struct {
	repository.Collection[repository.Repository]
	repoStore   repository.RepositoryStore
	commitStore repository.CommitStore
	branchStore repository.BranchStore
	tagStore    repository.TagStore
	queue       *Queue
	logger      *slog.Logger
}

// NewRepository creates a new Repository service.
func NewRepository(
	repoStore repository.RepositoryStore,
	commitStore repository.CommitStore,
	branchStore repository.BranchStore,
	tagStore repository.TagStore,
	queue *Queue,
	logger *slog.Logger,
) *Repository {
	return &Repository{
		Collection:  repository.NewCollection[repository.Repository](repoStore),
		repoStore:   repoStore,
		commitStore: commitStore,
		branchStore: branchStore,
		tagStore:    tagStore,
		queue:       queue,
		logger:      logger,
	}
}

// Add creates a new repository and queues it for indexing.
// Returns the source, whether it was newly created, and any error.
// If the repository already exists, it returns the existing one with created=false.
func (s *Repository) Add(ctx context.Context, params *RepositoryAddParams) (repository.Source, bool, error) {
	existing, err := s.repoStore.Exists(ctx, repository.WithRemoteURL(params.URL))
	if err != nil {
		return repository.Source{}, false, fmt.Errorf("check existing: %w", err)
	}
	if existing {
		repo, err := s.repoStore.FindOne(ctx, repository.WithRemoteURL(params.URL))
		if err != nil {
			return repository.Source{}, false, fmt.Errorf("find existing repository: %w", err)
		}
		return repository.NewSource(repo), false, nil
	}

	repo, err := repository.NewRepository(params.URL)
	if err != nil {
		return repository.Source{}, false, fmt.Errorf("create repository: %w", err)
	}
	if params.Branch != "" || params.Tag != "" || params.Commit != "" {
		repo = repo.WithTrackingConfig(
			repository.NewTrackingConfig(params.Branch, params.Tag, params.Commit),
		)
	}

	savedRepo, err := s.repoStore.Save(ctx, repo)
	if err != nil {
		return repository.Source{}, false, fmt.Errorf("save repository: %w", err)
	}

	payload := map[string]any{"repository_id": savedRepo.ID()}
	operations := task.PrescribedOperations{}.CreateNewRepository()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		s.logger.Warn("failed to enqueue clone task",
			slog.Int64("repo_id", repo.ID()),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("repository added",
		slog.Int64("repo_id", savedRepo.ID()),
		slog.String("url", savedRepo.RemoteURL()),
		slog.String("tracking", savedRepo.TrackingConfig().Reference()),
		slog.String("local_path", savedRepo.WorkingCopy().Path()),
	)

	return repository.NewSource(savedRepo), true, nil
}

// Delete removes a repository and all associated data.
func (s *Repository) Delete(ctx context.Context, id int64) error {
	_, err := s.repoStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	payload := map[string]any{"repository_id": id}
	t := task.NewTask(task.OperationDeleteRepository, int(task.PriorityCritical), payload)

	if err := s.queue.Enqueue(ctx, t); err != nil {
		return fmt.Errorf("enqueue delete: %w", err)
	}

	s.logger.Info("delete requested",
		slog.Int64("repo_id", id),
	)

	return nil
}

// Rescan triggers a rescan of a specific commit.
func (s *Repository) Rescan(ctx context.Context, params *RescanParams) error {
	_, err := s.repoStore.FindOne(ctx, repository.WithID(params.RepositoryID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}
	return s.enqueueRescan(ctx, params)
}

func (s *Repository) RescanAll(ctx context.Context) error {
	repos, err := s.repoStore.Find(ctx)
	if err != nil {
		return fmt.Errorf("find repositories: %w", err)
	}
	for _, repo := range repos {
		if err := s.enqueueRescan(ctx, &RescanParams{RepositoryID: repo.ID()}); err != nil {
			return fmt.Errorf("enqueue rescan: %w", err)
		}
	}
	return nil
}

// UpdateTrackingConfig updates a repository's tracking configuration.
func (s *Repository) UpdateTrackingConfig(ctx context.Context, id int64, params *TrackingConfigParams) (repository.Source, error) {
	trackingConfig := repository.NewTrackingConfig(params.Branch, params.Tag, params.Commit)
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return repository.Source{}, fmt.Errorf("get repository: %w", err)
	}

	updatedRepo := repo.WithTrackingConfig(trackingConfig)

	savedRepo, err := s.repoStore.Save(ctx, updatedRepo)
	if err != nil {
		return repository.Source{}, fmt.Errorf("save repository: %w", err)
	}

	s.logger.Info("tracking config updated",
		slog.Int64("repo_id", id),
		slog.String("tracking", trackingConfig.Reference()),
	)

	return repository.NewSource(savedRepo), nil
}

// SummaryByID returns a detailed summary for a repository.
func (s *Repository) SummaryByID(ctx context.Context, id int64) (repository.RepositorySummary, error) {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return repository.RepositorySummary{}, fmt.Errorf("get repository: %w", err)
	}

	branches, err := s.branchStore.Find(ctx, repository.WithRepoID(id))
	if err != nil {
		return repository.RepositorySummary{}, fmt.Errorf("find branches: %w", err)
	}

	tags, err := s.tagStore.Find(ctx, repository.WithRepoID(id))
	if err != nil {
		return repository.RepositorySummary{}, fmt.Errorf("find tags: %w", err)
	}

	commits, err := s.commitStore.Find(ctx, repository.WithRepoID(id))
	if err != nil {
		return repository.RepositorySummary{}, fmt.Errorf("find commits: %w", err)
	}

	var defaultBranch string
	for _, branch := range branches {
		if branch.IsDefault() {
			defaultBranch = branch.Name()
			break
		}
	}

	return repository.NewRepositorySummary(
		repository.NewSource(repo),
		len(branches),
		len(tags),
		len(commits),
		defaultBranch,
	), nil
}

// BranchesForRepository returns all branches for a repository.
func (s *Repository) BranchesForRepository(ctx context.Context, repoID int64) ([]repository.Branch, error) {
	branches, err := s.branchStore.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		return nil, fmt.Errorf("find branches: %w", err)
	}
	return branches, nil
}

// --- internal write operations ---

func (s *Repository) enqueueRescan(ctx context.Context, params *RescanParams) error {
	payload := map[string]any{
		"repository_id": params.RepositoryID,
		"commit_sha":    params.CommitSHA,
	}
	operations := task.PrescribedOperations{}.RescanCommit()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		return fmt.Errorf("enqueue rescan: %w", err)
	}

	s.logger.Info("rescan requested",
		slog.Int64("repo_id", params.RepositoryID),
		slog.String("commit_sha", params.CommitSHA),
	)

	return nil
}
