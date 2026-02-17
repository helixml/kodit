package commit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// Rescan handles the RESCAN_COMMIT task operation.
// It clears existing indexed data for a commit to prepare for re-indexing.
type Rescan struct {
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	statusStore      task.StatusStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewRescan creates a new Rescan handler.
func NewRescan(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	statusStore task.StatusStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Rescan {
	return &Rescan{
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
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
	if err := h.statusStore.DeleteByTrackable(ctx, task.TrackableTypeRepository, cp.RepoID()); err != nil {
		return fmt.Errorf("delete old task statuses: %w", err)
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationRescanCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	tracker.SetTotal(ctx, 2)
	tracker.SetCurrent(ctx, 0, "Deleting enrichments for commit")

	// Clear enrichments and associations for this commit to prepare for re-indexing.
	// The commit record itself is preserved — only derived data is removed.
	associations, err := h.associationStore.Find(ctx, enrichment.WithEntityType(enrichment.EntityTypeCommit), enrichment.WithEntityID(cp.CommitSHA()))
	if err != nil {
		tracker.Fail(ctx, err.Error())
		return fmt.Errorf("find associations for commit: %w", err)
	}

	if len(associations) > 0 {
		ids := make([]int64, len(associations))
		for i, a := range associations {
			ids[i] = a.EnrichmentID()
		}
		if err := h.enrichmentStore.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
			tracker.Fail(ctx, err.Error())
			return fmt.Errorf("delete enrichments: %w", err)
		}
	}

	tracker.SetCurrent(ctx, 1, "Deleting enrichment associations")

	if err := h.associationStore.DeleteBy(ctx, enrichment.WithEntityID(cp.CommitSHA())); err != nil {
		tracker.Fail(ctx, err.Error())
		return fmt.Errorf("delete enrichment associations: %w", err)
	}

	h.logger.Info("commit data cleared for rescan",
		slog.Int64("repo_id", cp.RepoID()),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}
