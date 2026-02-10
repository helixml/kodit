package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/queue"
)

// CloneRepository handles the CLONE_REPOSITORY task operation.
// It clones a Git repository to the local filesystem and updates the repo record.
type CloneRepository struct {
	repoRepo       repository.RepositoryStore
	cloner         git.Cloner
	queueService   *queue.Service
	trackerFactory TrackerFactory
	logger         *slog.Logger
}

// NewCloneRepository creates a new CloneRepository handler.
func NewCloneRepository(
	repoRepo repository.RepositoryStore,
	cloner git.Cloner,
	queueService *queue.Service,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *CloneRepository {
	return &CloneRepository{
		repoRepo:       repoRepo,
		cloner:         cloner,
		queueService:   queueService,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the CLONE_REPOSITORY task.
func (h *CloneRepository) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCloneRepository,
		domain.TrackableTypeRepository,
		repoID,
	)

	repo, err := h.repoRepo.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("get repository: %w", err)
	}

	if repo.HasWorkingCopy() {
		h.logger.Info("repository already cloned",
			slog.Int64("repo_id", repoID),
			slog.String("path", repo.WorkingCopy().Path()),
		)
		if skipErr := tracker.Skip(ctx, "Repository already cloned"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, 1); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 0, "Cloning repository"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	clonedPath, err := h.cloner.Clone(ctx, repo.RemoteURL())
	if err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("clone repository: %w", err)
	}

	wc := repository.NewWorkingCopy(clonedPath, repo.RemoteURL())
	updatedRepo := repo.WithWorkingCopy(wc)

	if _, err := h.repoRepo.Save(ctx, updatedRepo); err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("save repository: %w", err)
	}

	h.logger.Info("repository cloned successfully",
		slog.Int64("repo_id", repoID),
		slog.String("path", clonedPath),
	)

	if err := h.enqueueFollowUpTasks(ctx, repoID); err != nil {
		h.logger.Warn("failed to enqueue follow-up tasks", slog.String("error", err.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *CloneRepository) enqueueFollowUpTasks(ctx context.Context, repoID int64) error {
	payload := map[string]any{"repository_id": repoID}

	return h.queueService.EnqueueOperations(
		ctx,
		[]queue.TaskOperation{queue.OperationSyncRepository},
		domain.QueuePriorityNormal,
		payload,
	)
}
