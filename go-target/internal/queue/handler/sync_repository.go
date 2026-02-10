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

// SyncRepository handles the SYNC_REPOSITORY task operation.
// It fetches the latest changes from the remote repository and optionally
// queues commit scanning tasks.
type SyncRepository struct {
	repoRepo       repository.RepositoryStore
	branchRepo     repository.BranchStore
	cloner         git.Cloner
	scanner        git.Scanner
	queueService   *queue.Service
	trackerFactory TrackerFactory
	logger         *slog.Logger
}

// NewSyncRepository creates a new SyncRepository handler.
func NewSyncRepository(
	repoRepo repository.RepositoryStore,
	branchRepo repository.BranchStore,
	cloner git.Cloner,
	scanner git.Scanner,
	queueService *queue.Service,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *SyncRepository {
	return &SyncRepository{
		repoRepo:       repoRepo,
		branchRepo:     branchRepo,
		cloner:         cloner,
		scanner:        scanner,
		queueService:   queueService,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the SYNC_REPOSITORY task.
func (h *SyncRepository) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationSyncRepository,
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
		if _, err := h.branchRepo.SaveAll(ctx, branches); err != nil {
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

func (h *SyncRepository) enqueueCommitScans(ctx context.Context, repo repository.Repository, branches []repository.Branch) error {
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

	operations := queue.PrescribedOperations{}.ScanAndIndexCommit()
	return h.queueService.EnqueueOperations(ctx, operations, domain.QueuePriorityNormal, payload)
}
