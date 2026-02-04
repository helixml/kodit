package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

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

// RepositorySync orchestrates repository synchronization operations.
type RepositorySync struct {
	repoStore repository.RepositoryStore
	queue     *Queue
	logger    *slog.Logger
}

// NewRepositorySync creates a new RepositorySync.
func NewRepositorySync(
	repoStore repository.RepositoryStore,
	queue *Queue,
	logger *slog.Logger,
) *RepositorySync {
	return &RepositorySync{
		repoStore: repoStore,
		queue:     queue,
		logger:    logger,
	}
}

// AddRepository creates a new repository and queues cloning.
func (s *RepositorySync) AddRepository(ctx context.Context, remoteURL string) (Source, error) {
	existing, err := s.repoStore.ExistsByRemoteURL(ctx, remoteURL)
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

	payload := map[string]any{"repository_id": savedRepo.ID()}
	operations := task.PrescribedOperations{}.CreateNewRepository()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		s.logger.Warn("failed to enqueue clone task",
			slog.Int64("repo_id", savedRepo.ID()),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("repository added",
		slog.Int64("repo_id", savedRepo.ID()),
		slog.String("url", remoteURL),
	)

	return NewSource(savedRepo), nil
}

// AddRepositoryWithTracking creates a new repository with a tracking configuration.
func (s *RepositorySync) AddRepositoryWithTracking(
	ctx context.Context,
	remoteURL string,
	trackingConfig repository.TrackingConfig,
) (Source, error) {
	existing, err := s.repoStore.ExistsByRemoteURL(ctx, remoteURL)
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

	payload := map[string]any{"repository_id": savedRepo.ID()}
	operations := task.PrescribedOperations{}.CreateNewRepository()

	if err := s.queue.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		s.logger.Warn("failed to enqueue clone task",
			slog.Int64("repo_id", savedRepo.ID()),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("repository added with tracking",
		slog.Int64("repo_id", savedRepo.ID()),
		slog.String("url", remoteURL),
		slog.String("tracking", trackingConfig.Reference()),
	)

	return NewSource(savedRepo), nil
}

// RequestSync queues a sync operation for a repository.
func (s *RepositorySync) RequestSync(ctx context.Context, repoID int64) error {
	repo, err := s.repoStore.Get(ctx, repoID)
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

// RequestDelete queues a delete operation for a repository.
func (s *RepositorySync) RequestDelete(ctx context.Context, repoID int64) error {
	_, err := s.repoStore.Get(ctx, repoID)
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

// UpdateTrackingConfig updates a repository's tracking configuration.
func (s *RepositorySync) UpdateTrackingConfig(
	ctx context.Context,
	repoID int64,
	trackingConfig repository.TrackingConfig,
) (Source, error) {
	repo, err := s.repoStore.Get(ctx, repoID)
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

// RequestRescan queues a rescan operation for a specific commit.
func (s *RepositorySync) RequestRescan(ctx context.Context, repoID int64, commitSHA string) error {
	repo, err := s.repoStore.Get(ctx, repoID)
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

// SyncAll queues sync operations for all cloned repositories.
func (s *RepositorySync) SyncAll(ctx context.Context) (int, error) {
	repos, err := s.repoStore.FindAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("find all repositories: %w", err)
	}

	syncCount := 0
	for _, repo := range repos {
		if !repo.HasWorkingCopy() {
			continue
		}

		if err := s.RequestSync(ctx, repo.ID()); err != nil {
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
