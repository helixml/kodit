package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

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
	resolver       handler.CommitOperationResolver
	trackerFactory handler.TrackerFactory
	logger         zerolog.Logger
}

// NewSync creates a new Sync handler.
func NewSync(
	repoStore repository.RepositoryStore,
	branchStore repository.BranchStore,
	cloner domainservice.Cloner,
	scanner domainservice.Scanner,
	queue *service.Queue,
	resolver handler.CommitOperationResolver,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) *Sync {
	return &Sync{
		repoStore:      repoStore,
		branchStore:    branchStore,
		cloner:         cloner,
		scanner:        scanner,
		queue:          queue,
		resolver:       resolver,
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
		payload,
	)

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return repository.ErrNotCloned
	}

	// Mark the repository as scanned now so the periodic sync does not
	// re-enqueue while this (potentially slow) sync is still running.
	repo = repo.WithLastScannedAt(time.Now())
	if _, err := h.repoStore.Save(ctx, repo); err != nil {
		return fmt.Errorf("save last scanned at: %w", err)
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
		h.logger.Info().Int64("repo_id", repoID).Str("old_path", repo.WorkingCopy().Path()).Str("new_path", clonedPath).Msg("repository clone path changed")
		repo = repo.WithWorkingCopy(repository.NewWorkingCopy(clonedPath, repo.RemoteURL()))
		if _, err := h.repoStore.Save(ctx, repo); err != nil {
			return fmt.Errorf("save relocated repository: %w", err)
		}
	}

	tracker.SetCurrent(ctx, 1, "Scanning branches")
	branches, err := h.scanner.ScanAllBranches(ctx, clonedPath, repoID)
	if err != nil {
		h.logger.Warn().Str("error", err.Error()).Msg("failed to scan branches")
	}

	if err == nil {
		if _, err := h.branchStore.SaveAll(ctx, branches); err != nil {
			h.logger.Warn().Str("error", err.Error()).Msg("failed to save branches")
		}
	}

	if !repo.HasTrackingConfig() {
		if tc, ok := defaultTrackingConfig(branches); ok {
			repo = repo.WithTrackingConfig(tc)
			if _, err := h.repoStore.Save(ctx, repo); err != nil {
				return fmt.Errorf("save default tracking config: %w", err)
			}
			h.logger.Info().Int64("repo_id", repoID).Str("branch", tc.Branch()).Msg("set default tracking config")
		}
	}

	tracker.SetCurrent(ctx, 2, "Queueing commit scans")

	if err := h.enqueueCommitScans(ctx, repo, branches); err != nil {
		h.logger.Warn().Str("error", err.Error()).Msg("failed to enqueue commit scans")
	}

	repo = repo.WithLastScannedAt(time.Now())
	if _, err := h.repoStore.Save(ctx, repo); err != nil {
		return fmt.Errorf("save last synced at: %w", err)
	}

	h.logger.Info().Int64("repo_id", repoID).Int("branches", len(branches)).Msg("repository synced successfully")

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
		h.logger.Debug().Int64("repo_id", repo.ID()).Msg("no commit to scan")
		return nil
	}

	payload := map[string]any{
		"repository_id": repo.ID(),
		"commit_sha":    commitSHA,
	}

	operations, err := h.resolver.Operations(ctx, repo.PipelineID())
	if err != nil {
		return fmt.Errorf("resolve pipeline operations: %w", err)
	}

	return h.queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload)
}

// defaultTrackingConfig returns a branch tracking config derived from the
// scanned branches. It picks the branch marked as default.
func defaultTrackingConfig(branches []repository.Branch) (repository.TrackingConfig, bool) {
	for _, b := range branches {
		if b.IsDefault() {
			return repository.NewTrackingConfigForBranch(b.Name()), true
		}
	}
	return repository.TrackingConfig{}, false
}
