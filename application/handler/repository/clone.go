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

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		tracker.Fail(ctx, err.Error())
		return fmt.Errorf("get repository: %w", err)
	}

	if repo.HasWorkingCopy() {
		h.logger.Info("repository already cloned",
			slog.Int64("repo_id", repoID),
			slog.String("path", repo.WorkingCopy().Path()),
		)
		tracker.Skip(ctx, "Repository already cloned")
		return nil
	}

	tracker.SetTotal(ctx, 1)
	tracker.SetCurrent(ctx, 0, "Cloning repository")

	clonedPath, err := h.cloner.Clone(ctx, repo.RemoteURL())
	if err != nil {
		tracker.Fail(ctx, err.Error())
		return fmt.Errorf("clone repository: %w", err)
	}

	wc := repository.NewWorkingCopy(clonedPath, repo.RemoteURL())
	updatedRepo := repo.WithWorkingCopy(wc)

	if _, err := h.repoStore.Save(ctx, updatedRepo); err != nil {
		tracker.Fail(ctx, err.Error())
		return fmt.Errorf("save repository: %w", err)
	}

	h.logger.Info("repository cloned successfully",
		slog.Int64("repo_id", repoID),
		slog.String("path", clonedPath),
	)

	if err := h.enqueueFollowUpTasks(ctx, repoID); err != nil {
		h.logger.Warn("failed to enqueue follow-up tasks", slog.String("error", err.Error()))
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

