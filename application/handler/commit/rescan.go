package commit

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// Rescan handles the RESCAN_COMMIT task operation.
// It clears all existing data for a commit to prepare for a full re-scan and re-index.
type Rescan struct {
	enrichments      *service.Enrichment
	associationStore enrichment.AssociationStore
	commitStore      repository.CommitStore
	fileStore        repository.FileStore
	statusStore      task.StatusStore
	trackerFactory   handler.TrackerFactory
	logger           zerolog.Logger
}

// NewRescan creates a new Rescan handler.
func NewRescan(
	enrichments *service.Enrichment,
	associationStore enrichment.AssociationStore,
	commitStore repository.CommitStore,
	fileStore repository.FileStore,
	statusStore task.StatusStore,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) *Rescan {
	return &Rescan{
		enrichments:      enrichments,
		associationStore: associationStore,
		commitStore:      commitStore,
		fileStore:        fileStore,
		statusStore:      statusStore,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the RESCAN_COMMIT task.
func (h *Rescan) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	// Delete old task statuses so previous errors no longer appear.
	if err := h.statusStore.DeleteBy(ctx, task.WithTrackable(task.TrackableTypeRepository, cp.RepoID())...); err != nil {
		return fmt.Errorf("delete old task statuses: %w", err)
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationRescanCommit,
		payload,
	)

	// A repository may only rescan a commit recorded for it. git_commits keeps
	// one row per SHA, and the commit's enrichments may belong to other
	// repositories as well.
	commit, err := h.commitStore.FindOne(ctx, repository.WithRepoID(cp.RepoID()), repository.WithSHA(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("commit %s is not recorded for repository %d: %w", handler.ShortSHA(cp.CommitSHA()), cp.RepoID(), err)
	}

	tracker.SetTotal(ctx, 3)
	tracker.SetCurrent(ctx, 0, "Deleting enrichments for commit")

	// Clear this repository's data for the commit so the scan and index
	// pipeline can re-create it. Enrichments another repository shares (the
	// same git commit) keep existing; only this repository's claim on them is
	// removed, and indexing restores it.
	associations, err := h.associationStore.Find(ctx, enrichment.WithEntityType(enrichment.EntityTypeCommit), enrichment.WithEntityID(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("find associations for commit: %w", err)
	}

	ids := make([]int64, len(associations))
	for i, a := range associations {
		ids[i] = a.EnrichmentID()
	}
	deleted, err := h.enrichments.ReleaseForRepository(ctx, ids, cp.RepoID())
	if err != nil {
		return fmt.Errorf("delete enrichments: %w", err)
	}

	tracker.SetCurrent(ctx, 1, "Deleting enrichment associations")

	if len(deleted) > 0 {
		if err := h.associationStore.DeleteBy(ctx, enrichment.WithEnrichmentIDIn(deleted)); err != nil {
			return fmt.Errorf("delete enrichment associations: %w", err)
		}
	}

	if shared := len(ids) - len(deleted); shared > 0 {
		// Other repositories still use the commit's files and row.
		h.logger.Info().Int64("repo_id", cp.RepoID()).Str("commit", handler.ShortSHA(cp.CommitSHA())).Int("shared", shared).
			Msg("commit shared with other repositories; kept its files and shared enrichments")
		return nil
	}

	tracker.SetCurrent(ctx, 2, "Deleting commit and files")

	if err := h.fileStore.DeleteBy(ctx, repository.WithCommitSHA(cp.CommitSHA())); err != nil {
		return fmt.Errorf("delete commit files: %w", err)
	}

	if err := h.commitStore.Delete(ctx, commit); err != nil {
		return fmt.Errorf("delete commit: %w", err)
	}

	h.logger.Info().Int64("repo_id", cp.RepoID()).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("commit data cleared for rescan")

	return nil
}
