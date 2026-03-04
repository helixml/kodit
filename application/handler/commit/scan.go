package commit

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

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
	logger         zerolog.Logger
}

// NewScan creates a new Scan handler.
func NewScan(
	repoStore repository.RepositoryStore,
	commitStore repository.CommitStore,
	fileStore repository.FileStore,
	scanner domainservice.Scanner,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
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
		return fmt.Errorf("check existing commit: %w", err)
	}

	if existing {
		h.logger.Info().Int64("repo_id", cp.RepoID()).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("commit already scanned")
		tracker.Skip(ctx, "Commit already scanned")
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return repository.ErrNotCloned
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
			h.logger.Warn().Str("commit", handler.ShortSHA(cp.CommitSHA())).Str("error", err.Error()).Msg("failed to save files")
		}
	}

	h.logger.Info().Int64("repo_id", cp.RepoID()).Str("commit", handler.ShortSHA(cp.CommitSHA())).Int64("commit_id", savedCommit.ID()).Int("files", len(files)).Msg("commit scanned successfully")

	return nil
}
