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

// Sync handles the SYNC_REPOSITORY task operation.
// It fetches the latest changes from the remote repository and optionally
// queues commit scanning tasks.
type Sync struct {
	repoStore      repository.RepositoryStore
	branchStore    repository.BranchStore
	cloner         domainservice.Cloner
	scanner        domainservice.Scanner
	queue          *service.Queue
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewSync creates a new Sync handler.
func NewSync(
	repoStore repository.RepositoryStore,
	branchStore repository.BranchStore,
	cloner domainservice.Cloner,
	scanner domainservice.Scanner,
	queue *service.Queue,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Sync {
	return &Sync{
		repoStore:      repoStore,
		branchStore:    branchStore,
		cloner:         cloner,
		scanner:        scanner,
		queue:          queue,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the SYNC_REPOSITORY task.
func (h *Sync) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationSyncRepository,
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

	if !repo.HasWorkingCopy() {
		if failErr := tracker.Fail(ctx, "Repository not cloned"); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("repository %d has not been cloned", repoID)
	}

	if setTotalErr := tracker.SetTotal(ctx, 3); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 0, "Fetching latest changes"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.cloner.Update(ctx, repo); err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("update repository: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Scanning branches"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	clonedPath := repo.WorkingCopy().Path()
	branches, err := h.scanner.ScanAllBranches(ctx, clonedPath, repoID)
	if err != nil {
		h.logger.Warn("failed to scan branches", slog.String("error", err.Error()))
	} else {
		if _, err := h.branchStore.SaveAll(ctx, branches); err != nil {
			h.logger.Warn("failed to save branches", slog.String("error", err.Error()))
		}
	}

	if currentErr := tracker.SetCurrent(ctx, 2, "Queueing commit scans"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.enqueueCommitScans(ctx, repo, branches); err != nil {
		h.logger.Warn("failed to enqueue commit scans", slog.String("error", err.Error()))
	}

	h.logger.Info("repository synced successfully",
		slog.Int64("repo_id", repoID),
		slog.Int("branches", len(branches)),
	)

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *Sync) enqueueCommitScans(ctx context.Context, repo repository.Repository, branches []repository.Branch) error {
	var commitSHA string

	if repo.HasTrackingConfig() {
		tc := repo.TrackingConfig()
		if tc.IsBranch() {
			for _, b := range branches {
				if b.Name() == tc.Branch() {
					commitSHA = b.HeadCommitSHA()
					break
				}
			}
		} else if tc.IsCommit() {
			commitSHA = tc.Commit()
		}
	}

	if commitSHA == "" {
		for _, b := range branches {
			if b.IsDefault() {
				commitSHA = b.HeadCommitSHA()
				break
			}
		}
	}

	if commitSHA == "" && len(branches) > 0 {
		commitSHA = branches[0].HeadCommitSHA()
	}

	if commitSHA == "" {
		h.logger.Debug("no commit to scan", slog.Int64("repo_id", repo.ID()))
		return nil
	}

	payload := map[string]any{
		"repository_id": repo.ID(),
		"commit_sha":    commitSHA,
	}

	operations := task.PrescribedOperations{}.ScanAndIndexCommit()
	return h.queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload)
}
