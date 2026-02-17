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
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationScanCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	existing, err := h.commitStore.Exists(ctx, repository.WithRepoID(cp.RepoID()), repository.WithSHA(cp.CommitSHA()))
	if err != nil {
		h.logger.Warn("failed to check existing commit", slog.String("error", err.Error()))
	}

	if existing {
		h.logger.Info("commit already scanned",
			slog.Int64("repo_id", cp.RepoID()),
			slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
		)
		tracker.Skip(ctx, "Commit already scanned")
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return fmt.Errorf("repository %d has not been cloned", cp.RepoID())
	}

	tracker.SetTotal(ctx, 2)
	tracker.SetCurrent(ctx, 0, "Scanning commit")

	clonedPath := repo.WorkingCopy().Path()
	result, err := h.scanner.ScanCommit(ctx, clonedPath, cp.CommitSHA(), cp.RepoID())
	if err != nil {
		return fmt.Errorf("scan commit: %w", err)
	}

	tracker.SetCurrent(ctx, 1, "Saving commit data")

	commit := result.Commit()
	savedCommit, err := h.commitStore.Save(ctx, commit)
	if err != nil {
		return fmt.Errorf("save commit: %w", err)
	}

	files := result.Files()
	if len(files) > 0 {
		if _, err := h.fileStore.SaveAll(ctx, files); err != nil {
			h.logger.Warn("failed to save files",
				slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
				slog.String("error", err.Error()),
			)
		}
	}

	h.logger.Info("commit scanned successfully",
		slog.Int64("repo_id", cp.RepoID()),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
		slog.Int64("commit_id", savedCommit.ID()),
		slog.Int("files", len(files)),
	)

	return nil
}
