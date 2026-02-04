package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
)

// Clone handles the CLONE_REPOSITORY task operation.
// It clones a Git repository to the local filesystem and updates the repo record.
type Clone struct {
	repoStore      repository.RepositoryStore
	cloner         domainservice.Cloner
	queue          *service.Queue
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewClone creates a new Clone handler.
func NewClone(
	repoStore repository.RepositoryStore,
	cloner domainservice.Cloner,
	queue *service.Queue,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Clone {
	return &Clone{
		repoStore:      repoStore,
		cloner:         cloner,
		queue:          queue,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the CLONE_REPOSITORY task.
func (h *Clone) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCloneRepository,
		task.TrackableTypeRepository,
		repoID,
	)

	repo, err := h.repoStore.Get(ctx, repoID)
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

	if _, err := h.repoStore.Save(ctx, updatedRepo); err != nil {
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

func (h *Clone) enqueueFollowUpTasks(ctx context.Context, repoID int64) error {
	payload := map[string]any{"repository_id": repoID}

	return h.queue.EnqueueOperations(
		ctx,
		[]task.Operation{task.OperationSyncRepository},
		task.PriorityNormal,
		payload,
	)
}

