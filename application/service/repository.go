package service

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// RepositoryAddParams configures adding a new repository.
type RepositoryAddParams struct {
	URL         string
	UpstreamURL string
	Branch      string
	Tag         string
	Commit      string
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

// ChunkingConfigParams holds the parameters for updating a repository's chunking config.
type ChunkingConfigParams struct {
	Size    int
	Overlap int
	MinSize int
}

// CommitOperationResolver resolves the operation sequence for a pipeline.
type CommitOperationResolver interface {
	Operations(ctx context.Context, pipelineID *int64) ([]task.Operation, error)
}

// Repository provides repository management and query operations.
// Embeds Collection for Find/Get; bespoke methods handle writes and lifecycle.
type Repository struct {
	repository.Collection[repository.Repository]
	repoStore     repository.RepositoryStore
	pipelineStore repository.PipelineStore
	commitStore   repository.CommitStore
	branchStore   repository.BranchStore
	tagStore      repository.TagStore
	queue         *Queue
	resolver      CommitOperationResolver
	logger        zerolog.Logger
}

// NewRepository creates a new Repository service.
func NewRepository(
	repoStore repository.RepositoryStore,
	pipelineStore repository.PipelineStore,
	commitStore repository.CommitStore,
	branchStore repository.BranchStore,
	tagStore repository.TagStore,
	queue *Queue,
	resolver CommitOperationResolver,
	logger zerolog.Logger,
) *Repository {
	return &Repository{
		Collection:    repository.NewCollection[repository.Repository](repoStore),
		repoStore:     repoStore,
		pipelineStore: pipelineStore,
		commitStore:   commitStore,
		branchStore:   branchStore,
		tagStore:      tagStore,
		queue:         queue,
		resolver:      resolver,
		logger:        logger,
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

	if params.UpstreamURL != "" {
		found, err := s.repoStore.Exists(ctx, repository.WithUpstreamURL(params.UpstreamURL))
		if err != nil {
			return repository.Source{}, false, fmt.Errorf("check upstream: %w", err)
		}
		if found {
			repo, err := s.repoStore.FindOne(ctx, repository.WithUpstreamURL(params.UpstreamURL))
			if err != nil {
				return repository.Source{}, false, fmt.Errorf("find existing by upstream: %w", err)
			}
			return repository.NewSource(repo), false, nil
		}
	}

	repo, err := repository.NewRepository(params.URL)
	if err != nil {
		return repository.Source{}, false, fmt.Errorf("create repository: %w", err)
	}
	if params.UpstreamURL != "" {
		repo = repo.WithUpstreamURL(params.UpstreamURL)
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
	operations := []task.Operation{task.OperationCloneRepository}

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		s.logger.Warn().Int64("repo_id", repo.ID()).Str("error", err.Error()).Msg("failed to enqueue clone task")
	}

	s.logger.Info().Int64("repo_id", savedRepo.ID()).Str("url", savedRepo.SanitizedURL()).Str("tracking", savedRepo.TrackingConfig().Reference()).Str("local_path", savedRepo.WorkingCopy().Path()).Msg("repository added")

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

	s.logger.Info().Int64("repo_id", id).Msg("delete requested")

	return nil
}

// Sync triggers a sync (git fetch + branch scan + commit indexing) for a repository.
func (s *Repository) Sync(ctx context.Context, id int64) error {
	_, err := s.repoStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	payload := map[string]any{"repository_id": id}
	operations := []task.Operation{task.OperationCloneRepository, task.OperationSyncRepository}

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		return fmt.Errorf("enqueue sync: %w", err)
	}

	s.logger.Info().Int64("repo_id", id).Msg("sync requested")

	return nil
}

// Rescan triggers a rescan of a specific commit.
func (s *Repository) Rescan(ctx context.Context, params *RescanParams) error {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(params.RepositoryID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}
	return s.enqueueRescan(ctx, repo, params.CommitSHA)
}

func (s *Repository) RescanAll(ctx context.Context) error {
	repos, err := s.repoStore.Find(ctx)
	if err != nil {
		return fmt.Errorf("find repositories: %w", err)
	}
	for _, repo := range repos {
		if err := s.enqueueRescan(ctx, repo, ""); err != nil {
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

	s.logger.Info().Int64("repo_id", id).Str("tracking", trackingConfig.Reference()).Msg("tracking config updated")

	return repository.NewSource(savedRepo), nil
}

// UpdateChunkingConfig updates a repository's chunking configuration.
func (s *Repository) UpdateChunkingConfig(ctx context.Context, id int64, params *ChunkingConfigParams) (repository.Repository, error) {
	cc, err := repository.NewChunkingConfig(params.Size, params.Overlap, params.MinSize)
	if err != nil {
		return repository.Repository{}, fmt.Errorf("invalid chunking config: %w", err)
	}

	repo, err := s.repoStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return repository.Repository{}, fmt.Errorf("get repository: %w", err)
	}

	updated := repo.WithChunkingConfig(cc)

	saved, err := s.repoStore.Save(ctx, updated)
	if err != nil {
		return repository.Repository{}, fmt.Errorf("save repository: %w", err)
	}

	s.logger.Info().Int64("repo_id", id).Msg("chunking config updated")

	return saved, nil
}

// AssignPipeline links a pipeline to a repository.
func (s *Repository) AssignPipeline(ctx context.Context, repoID, pipelineID int64) (repository.Source, error) {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return repository.Source{}, fmt.Errorf("get repository: %w", err)
	}

	_, err = s.pipelineStore.FindOne(ctx, repository.WithID(pipelineID))
	if err != nil {
		return repository.Source{}, fmt.Errorf("get pipeline: %w", err)
	}

	updated := repo.WithPipelineID(pipelineID)

	saved, err := s.repoStore.Save(ctx, updated)
	if err != nil {
		return repository.Source{}, fmt.Errorf("save repository: %w", err)
	}

	s.logger.Info().Int64("repo_id", repoID).Int64("pipeline_id", pipelineID).Msg("pipeline assigned")

	return repository.NewSource(saved), nil
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

func (s *Repository) enqueueRescan(ctx context.Context, repo repository.Repository, commitSHA string) error {
	pipelineOps, err := s.resolver.Operations(ctx, repo.PipelineID())
	if err != nil {
		return fmt.Errorf("resolve pipeline operations: %w", err)
	}

	operations := append([]task.Operation{task.OperationRescanCommit}, pipelineOps...)
	payload := map[string]any{
		"repository_id": repo.ID(),
		"commit_sha":    commitSHA,
	}

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		return fmt.Errorf("enqueue rescan: %w", err)
	}

	s.logger.Info().Int64("repo_id", repo.ID()).Str("commit_sha", commitSHA).Msg("rescan requested")

	return nil
}
