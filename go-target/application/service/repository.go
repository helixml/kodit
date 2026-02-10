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
func (s *Repository) Add(ctx context.Context, params *RepositoryAddParams) (Source, error) {
	if params.Branch != "" || params.Tag != "" || params.Commit != "" {
		tc := repository.NewTrackingConfig(params.Branch, params.Tag, params.Commit)
		return s.addWithTracking(ctx, params.URL, tc)
	}
	return s.add(ctx, params.URL)
}

// Delete removes a repository and all associated data.
func (s *Repository) Delete(ctx context.Context, id int64) error {
	return s.requestDelete(ctx, id)
}

// Sync triggers re-indexing of a repository.
func (s *Repository) Sync(ctx context.Context, id int64) error {
	return s.requestSync(ctx, id)
}

// Rescan triggers a rescan of a specific commit.
func (s *Repository) Rescan(ctx context.Context, params *RescanParams) error {
	return s.requestRescan(ctx, params.RepositoryID, params.CommitSHA)
}

// UpdateTrackingConfig updates a repository's tracking configuration.
func (s *Repository) UpdateTrackingConfig(ctx context.Context, id int64, params *TrackingConfigParams) (Source, error) {
	tc := repository.NewTrackingConfig(params.Branch, params.Tag, params.Commit)
	return s.updateTrackingConfig(ctx, id, tc)
}

// Exists checks if a repository exists by remote URL.
func (s *Repository) Exists(ctx context.Context, url string) (bool, error) {
	return s.repoStore.Exists(ctx, repository.WithRemoteURL(url))
}

// SummaryByID returns a detailed summary for a repository.
func (s *Repository) SummaryByID(ctx context.Context, id int64) (RepositorySummary, error) {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("get repository: %w", err)
	}

	branches, err := s.branchStore.Find(ctx, repository.WithRepoID(id))
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find branches: %w", err)
	}

	tags, err := s.tagStore.Find(ctx, repository.WithRepoID(id))
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find tags: %w", err)
	}

	commits, err := s.commitStore.Find(ctx, repository.WithRepoID(id))
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find commits: %w", err)
	}

	var defaultBranch string
	for _, branch := range branches {
		if branch.IsDefault() {
			defaultBranch = branch.Name()
			break
		}
	}

	return NewRepositorySummary(
		NewSource(repo),
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

// SyncAll queues sync operations for all cloned repositories.
func (s *Repository) SyncAll(ctx context.Context) (int, error) {
	repos, err := s.repoStore.Find(ctx)
	if err != nil {
		return 0, fmt.Errorf("find all repositories: %w", err)
	}

	syncCount := 0
	for _, repo := range repos {
		if !repo.HasWorkingCopy() {
			continue
		}

		if err := s.requestSync(ctx, repo.ID()); err != nil {
			s.logger.Warn("failed to request sync",
				slog.Int64("repo_id", repo.ID()),
				slog.String("error", err.Error()),
			)
			continue
		}
		syncCount++
	}

	s.logger.Info("sync all requested",
		slog.Int("repositories", syncCount),
	)

	return syncCount, nil
}

// --- internal write operations (from RepositorySync) ---

func (s *Repository) add(ctx context.Context, remoteURL string) (Source, error) {
	existing, err := s.repoStore.Exists(ctx, repository.WithRemoteURL(remoteURL))
	if err != nil {
		return Source{}, fmt.Errorf("check existing: %w", err)
	}
	if existing {
		return Source{}, fmt.Errorf("repository already exists: %s", remoteURL)
	}

	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		return Source{}, fmt.Errorf("create repository: %w", err)
	}

	savedRepo, err := s.repoStore.Save(ctx, repo)
	if err != nil {
		return Source{}, fmt.Errorf("save repository: %w", err)
	}

	s.enqueueNewRepository(ctx, savedRepo)

	s.logger.Info("repository added",
		slog.Int64("repo_id", savedRepo.ID()),
		slog.String("url", remoteURL),
	)

	return NewSource(savedRepo), nil
}

func (s *Repository) addWithTracking(ctx context.Context, remoteURL string, trackingConfig repository.TrackingConfig) (Source, error) {
	existing, err := s.repoStore.Exists(ctx, repository.WithRemoteURL(remoteURL))
	if err != nil {
		return Source{}, fmt.Errorf("check existing: %w", err)
	}
	if existing {
		return Source{}, fmt.Errorf("repository already exists: %s", remoteURL)
	}

	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		return Source{}, fmt.Errorf("create repository: %w", err)
	}

	repo = repo.WithTrackingConfig(trackingConfig)

	savedRepo, err := s.repoStore.Save(ctx, repo)
	if err != nil {
		return Source{}, fmt.Errorf("save repository: %w", err)
	}

	s.enqueueNewRepository(ctx, savedRepo)

	s.logger.Info("repository added with tracking",
		slog.Int64("repo_id", savedRepo.ID()),
		slog.String("url", remoteURL),
		slog.String("tracking", trackingConfig.Reference()),
	)

	return NewSource(savedRepo), nil
}

func (s *Repository) enqueueNewRepository(ctx context.Context, repo repository.Repository) {
	payload := map[string]any{"repository_id": repo.ID()}
	operations := task.PrescribedOperations{}.CreateNewRepository()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		s.logger.Warn("failed to enqueue clone task",
			slog.Int64("repo_id", repo.ID()),
			slog.String("error", err.Error()),
		)
	}
}

func (s *Repository) requestSync(ctx context.Context, repoID int64) error {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return fmt.Errorf("repository %d has not been cloned", repoID)
	}

	payload := map[string]any{"repository_id": repoID}
	operations := task.PrescribedOperations{}.SyncRepository()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload); err != nil {
		return fmt.Errorf("enqueue sync: %w", err)
	}

	s.logger.Info("sync requested",
		slog.Int64("repo_id", repoID),
	)

	return nil
}

func (s *Repository) requestDelete(ctx context.Context, repoID int64) error {
	_, err := s.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	payload := map[string]any{"repository_id": repoID}
	t := task.NewTask(task.OperationDeleteRepository, int(task.PriorityUserInitiated), payload)

	if err := s.queue.Enqueue(ctx, t); err != nil {
		return fmt.Errorf("enqueue delete: %w", err)
	}

	s.logger.Info("delete requested",
		slog.Int64("repo_id", repoID),
	)

	return nil
}

func (s *Repository) updateTrackingConfig(ctx context.Context, repoID int64, trackingConfig repository.TrackingConfig) (Source, error) {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return Source{}, fmt.Errorf("get repository: %w", err)
	}

	updatedRepo := repo.WithTrackingConfig(trackingConfig)

	savedRepo, err := s.repoStore.Save(ctx, updatedRepo)
	if err != nil {
		return Source{}, fmt.Errorf("save repository: %w", err)
	}

	s.logger.Info("tracking config updated",
		slog.Int64("repo_id", repoID),
		slog.String("tracking", trackingConfig.Reference()),
	)

	return NewSource(savedRepo), nil
}

func (s *Repository) requestRescan(ctx context.Context, repoID int64, commitSHA string) error {
	repo, err := s.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return fmt.Errorf("repository %d has not been cloned", repoID)
	}

	payload := map[string]any{
		"repository_id": repoID,
		"commit_sha":    commitSHA,
	}
	operations := task.PrescribedOperations{}.RescanCommit()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		return fmt.Errorf("enqueue rescan: %w", err)
	}

	s.logger.Info("rescan requested",
		slog.Int64("repo_id", repoID),
		slog.String("commit_sha", commitSHA),
	)

	return nil
}

// --- Source and RepositorySummary types ---

// SourceStatus represents the current state of a repository source.
type SourceStatus string

// Status values for a repository source.
const (
	StatusPending  SourceStatus = "pending"
	StatusCloning  SourceStatus = "cloning"
	StatusCloned   SourceStatus = "cloned"
	StatusSyncing  SourceStatus = "syncing"
	StatusFailed   SourceStatus = "failed"
	StatusDeleting SourceStatus = "deleting"
)

// String returns the string representation of the status.
func (s SourceStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the status is a terminal state.
func (s SourceStatus) IsTerminal() bool {
	return s == StatusCloned || s == StatusFailed
}

// Source represents a repository being managed by the system.
// It wraps a Repository and provides lifecycle management operations.
type Source struct {
	repo      repository.Repository
	status    SourceStatus
	lastError string
}

// NewSource creates a new Source from a Repository.
func NewSource(repo repository.Repository) Source {
	status := StatusPending
	if repo.HasWorkingCopy() {
		status = StatusCloned
	}
	return Source{
		repo:   repo,
		status: status,
	}
}

// ReconstructSource reconstructs a Source from persistence.
func ReconstructSource(repo repository.Repository, status SourceStatus, lastError string) Source {
	return Source{
		repo:      repo,
		status:    status,
		lastError: lastError,
	}
}

// ID returns the repository ID.
func (s Source) ID() int64 {
	return s.repo.ID()
}

// RemoteURL returns the repository remote URL.
func (s Source) RemoteURL() string {
	return s.repo.RemoteURL()
}

// WorkingCopy returns the local working copy, if available.
func (s Source) WorkingCopy() repository.WorkingCopy {
	return s.repo.WorkingCopy()
}

// TrackingConfig returns the tracking configuration.
func (s Source) TrackingConfig() repository.TrackingConfig {
	return s.repo.TrackingConfig()
}

// Repository returns the underlying Repository.
func (s Source) Repository() repository.Repository {
	return s.repo
}

// Repo returns the underlying Repository (alias for Repository() for API compatibility).
func (s Source) Repo() repository.Repository {
	return s.repo
}

// Status returns the current source status.
func (s Source) Status() SourceStatus {
	return s.status
}

// LastError returns the last error message, if any.
func (s Source) LastError() string {
	return s.lastError
}

// IsCloned returns true if the repository has been cloned.
func (s Source) IsCloned() bool {
	return s.repo.HasWorkingCopy()
}

// ClonedPath returns the local filesystem path, or empty string if not cloned.
func (s Source) ClonedPath() string {
	if !s.repo.HasWorkingCopy() {
		return ""
	}
	return s.repo.WorkingCopy().Path()
}

// WithStatus returns a new Source with the given status.
func (s Source) WithStatus(status SourceStatus) Source {
	s.status = status
	return s
}

// WithError returns a new Source with an error status and message.
func (s Source) WithError(err error) Source {
	s.status = StatusFailed
	if err != nil {
		s.lastError = err.Error()
	}
	return s
}

// WithWorkingCopy returns a new Source with an updated working copy.
func (s Source) WithWorkingCopy(wc repository.WorkingCopy) Source {
	s.repo = s.repo.WithWorkingCopy(wc)
	s.status = StatusCloned
	s.lastError = ""
	return s
}

// WithTrackingConfig returns a new Source with an updated tracking config.
func (s Source) WithTrackingConfig(tc repository.TrackingConfig) Source {
	s.repo = s.repo.WithTrackingConfig(tc)
	return s
}

// WithRepository returns a new Source with an updated Repository.
func (s Source) WithRepository(repo repository.Repository) Source {
	s.repo = repo
	return s
}

// CanSync returns true if the repository can be synced.
func (s Source) CanSync() bool {
	return s.IsCloned() && s.status != StatusSyncing && s.status != StatusDeleting
}

// CanDelete returns true if the repository can be deleted.
func (s Source) CanDelete() bool {
	return s.status != StatusDeleting
}

// RepositorySummary provides a summary view of a repository.
type RepositorySummary struct {
	source        Source
	branchCount   int
	tagCount      int
	commitCount   int
	defaultBranch string
}

// NewRepositorySummary creates a new RepositorySummary.
func NewRepositorySummary(
	source Source,
	branchCount, tagCount, commitCount int,
	defaultBranch string,
) RepositorySummary {
	return RepositorySummary{
		source:        source,
		branchCount:   branchCount,
		tagCount:      tagCount,
		commitCount:   commitCount,
		defaultBranch: defaultBranch,
	}
}

// Source returns the repository source.
func (s RepositorySummary) Source() Source { return s.source }

// BranchCount returns the number of branches.
func (s RepositorySummary) BranchCount() int { return s.branchCount }

// TagCount returns the number of tags.
func (s RepositorySummary) TagCount() int { return s.tagCount }

// CommitCount returns the number of indexed commits.
func (s RepositorySummary) CommitCount() int { return s.commitCount }

// DefaultBranch returns the default branch name.
func (s RepositorySummary) DefaultBranch() string { return s.defaultBranch }
