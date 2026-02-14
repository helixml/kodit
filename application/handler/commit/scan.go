package commit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
)

// Scan handles the SCAN_COMMIT task operation.
// It scans a specific commit to extract its metadata and files.
type Scan struct {
	repoStore      repository.RepositoryStore
	commitStore    repository.CommitStore
	fileStore      repository.FileStore
	scanner        domainservice.Scanner
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewScan creates a new Scan handler.
func NewScan(
	repoStore repository.RepositoryStore,
	commitStore repository.CommitStore,
	fileStore repository.FileStore,
	scanner domainservice.Scanner,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Scan {
	return &Scan{
		repoStore:      repoStore,
		commitStore:    commitStore,
		fileStore:      fileStore,
		scanner:        scanner,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the SCAN_COMMIT task.
func (h *Scan) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationScanCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	existing, err := h.commitStore.Exists(ctx, repository.WithRepoID(repoID), repository.WithSHA(commitSHA))
	if err != nil {
		h.logger.Warn("failed to check existing commit", slog.String("error", err.Error()))
	}

	if existing {
		h.logger.Info("commit already scanned",
			slog.Int64("repo_id", repoID),
			slog.String("commit", handler.ShortSHA(commitSHA)),
		)
		if skipErr := tracker.Skip(ctx, "Commit already scanned"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(repoID))
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

	if setTotalErr := tracker.SetTotal(ctx, 2); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 0, "Scanning commit"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	clonedPath := repo.WorkingCopy().Path()
	result, err := h.scanner.ScanCommit(ctx, clonedPath, commitSHA, repoID)
	if err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("scan commit: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Saving commit data"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	commit := result.Commit()
	savedCommit, err := h.commitStore.Save(ctx, commit)
	if err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("save commit: %w", err)
	}

	files := result.Files()
	if len(files) > 0 {
		if _, err := h.fileStore.SaveAll(ctx, files); err != nil {
			h.logger.Warn("failed to save files",
				slog.String("commit", handler.ShortSHA(commitSHA)),
				slog.String("error", err.Error()),
			)
		}
	}

	h.logger.Info("commit scanned successfully",
		slog.Int64("repo_id", repoID),
		slog.String("commit", handler.ShortSHA(commitSHA)),
		slog.Int64("commit_id", savedCommit.ID()),
		slog.Int("files", len(files)),
	)

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}
