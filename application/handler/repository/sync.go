package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
	prescribedOps  task.PrescribedOperations
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
	prescribedOps task.PrescribedOperations,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Sync {
	return &Sync{
		repoStore:      repoStore,
		branchStore:    branchStore,
		cloner:         cloner,
		scanner:        scanner,
		queue:          queue,
		prescribedOps:  prescribedOps,
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

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return repository.ErrNotCloned
	}

	tracker.SetTotal(ctx, 3)
	tracker.SetCurrent(ctx, 0, "Fetching latest changes")

	clonedPath, err := h.cloner.Update(ctx, repo)
	if err != nil {
		return fmt.Errorf("update repository: %w", err)
	}

	// The clone may have been relocated (e.g. stale path from a previous
	// container). Persist the new working copy so future syncs use it.
	if clonedPath != repo.WorkingCopy().Path() {
		h.logger.Info("repository clone path changed",
			slog.Int64("repo_id", repoID),
			slog.String("old_path", repo.WorkingCopy().Path()),
			slog.String("new_path", clonedPath),
		)
		repo = repo.WithWorkingCopy(repository.NewWorkingCopy(clonedPath, repo.RemoteURL()))
		if _, err := h.repoStore.Save(ctx, repo); err != nil {
			return fmt.Errorf("save relocated repository: %w", err)
		}
	}

	tracker.SetCurrent(ctx, 1, "Scanning branches")
	branches, err := h.scanner.ScanAllBranches(ctx, clonedPath, repoID)
	if err != nil {
		h.logger.Warn("failed to scan branches", slog.String("error", err.Error()))
	}

	if err == nil {
		if _, err := h.branchStore.SaveAll(ctx, branches); err != nil {
			h.logger.Warn("failed to save branches", slog.String("error", err.Error()))
		}
	}

	tracker.SetCurrent(ctx, 2, "Queueing commit scans")

	if err := h.enqueueCommitScans(ctx, repo, branches); err != nil {
		h.logger.Warn("failed to enqueue commit scans", slog.String("error", err.Error()))
	}

	repo = repo.WithLastScannedAt(time.Now())
	if _, err := h.repoStore.Save(ctx, repo); err != nil {
		return fmt.Errorf("save last synced at: %w", err)
	}

	h.logger.Info("repository synced successfully",
		slog.Int64("repo_id", repoID),
		slog.Int("branches", len(branches)),
	)

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

	operations := h.prescribedOps.ScanAndIndexCommit()
	return h.queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload)
}
