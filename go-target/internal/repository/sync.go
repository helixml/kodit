package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
)

// SyncService orchestrates repository synchronization operations.
type SyncService struct {
	repoRepo     repository.RepositoryStore
	queueService *queue.Service
	logger       *slog.Logger
}

// NewSyncService creates a new SyncService.
func NewSyncService(
	repoRepo repository.RepositoryStore,
	queueService *queue.Service,
	logger *slog.Logger,
) *SyncService {
	return &SyncService{
		repoRepo:     repoRepo,
		queueService: queueService,
		logger:       logger,
	}
}

// AddRepository creates a new repository and queues cloning.
func (s *SyncService) AddRepository(ctx context.Context, remoteURL string) (Source, error) {
	existing, err := s.repoRepo.Exists(ctx, repository.WithRemoteURL(remoteURL))
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

	savedRepo, err := s.repoRepo.Save(ctx, repo)
	if err != nil {
		return Source{}, fmt.Errorf("save repository: %w", err)
	}

	payload := map[string]any{"repository_id": savedRepo.ID()}
	operations := queue.PrescribedOperations{}.CreateNewRepository()

	if err := s.queueService.EnqueueOperations(ctx, operations, domain.QueuePriorityUserInitiated, payload); err != nil {
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
func (s *SyncService) AddRepositoryWithTracking(
	ctx context.Context,
	remoteURL string,
	trackingConfig repository.TrackingConfig,
) (Source, error) {
	existing, err := s.repoRepo.Exists(ctx, repository.WithRemoteURL(remoteURL))
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

	savedRepo, err := s.repoRepo.Save(ctx, repo)
	if err != nil {
		return Source{}, fmt.Errorf("save repository: %w", err)
	}

	payload := map[string]any{"repository_id": savedRepo.ID()}
	operations := queue.PrescribedOperations{}.CreateNewRepository()

	if err := s.queueService.EnqueueOperations(ctx, operations, domain.QueuePriorityUserInitiated, payload); err != nil {
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
func (s *SyncService) RequestSync(ctx context.Context, repoID int64) error {
	repo, err := s.repoRepo.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return fmt.Errorf("repository %d has not been cloned", repoID)
	}

	payload := map[string]any{"repository_id": repoID}
	operations := queue.PrescribedOperations{}.SyncRepository()

	if err := s.queueService.EnqueueOperations(ctx, operations, domain.QueuePriorityNormal, payload); err != nil {
		return fmt.Errorf("enqueue sync: %w", err)
	}

	s.logger.Info("sync requested",
		slog.Int64("repo_id", repoID),
	)

	return nil
}

// RequestDelete queues a delete operation for a repository.
func (s *SyncService) RequestDelete(ctx context.Context, repoID int64) error {
	_, err := s.repoRepo.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	payload := map[string]any{"repository_id": repoID}
	task := queue.NewTask(queue.OperationDeleteRepository, int(domain.QueuePriorityUserInitiated), payload)

	if err := s.queueService.Enqueue(ctx, task); err != nil {
		return fmt.Errorf("enqueue delete: %w", err)
	}

	s.logger.Info("delete requested",
		slog.Int64("repo_id", repoID),
	)

	return nil
}

// UpdateTrackingConfig updates a repository's tracking configuration.
func (s *SyncService) UpdateTrackingConfig(
	ctx context.Context,
	repoID int64,
	trackingConfig repository.TrackingConfig,
) (Source, error) {
	repo, err := s.repoRepo.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return Source{}, fmt.Errorf("get repository: %w", err)
	}

	updatedRepo := repo.WithTrackingConfig(trackingConfig)

	savedRepo, err := s.repoRepo.Save(ctx, updatedRepo)
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
func (s *SyncService) RequestRescan(ctx context.Context, repoID int64, commitSHA string) error {
	repo, err := s.repoRepo.FindOne(ctx, repository.WithID(repoID))
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
	operations := queue.PrescribedOperations{}.RescanCommit()

	if err := s.queueService.EnqueueOperations(ctx, operations, domain.QueuePriorityUserInitiated, payload); err != nil {
		return fmt.Errorf("enqueue rescan: %w", err)
	}

	s.logger.Info("rescan requested",
		slog.Int64("repo_id", repoID),
		slog.String("commit_sha", commitSHA),
	)

	return nil
}

// SyncAll queues sync operations for all cloned repositories.
func (s *SyncService) SyncAll(ctx context.Context) (int, error) {
	repos, err := s.repoRepo.Find(ctx)
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
