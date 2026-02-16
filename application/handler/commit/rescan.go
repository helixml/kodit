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
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewRescan creates a new Rescan handler.
func NewRescan(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Rescan {
	return &Rescan{
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the RESCAN_COMMIT task.
func (h *Rescan) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationRescanCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	if setTotalErr := tracker.SetTotal(ctx, 2); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 0, "Deleting enrichments for commit"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	// Find associations for this commit, collect enrichment IDs, delete enrichments, then associations
	associations, err := h.associationStore.Find(ctx, enrichment.WithEntityType(enrichment.EntityTypeCommit), enrichment.WithEntityID(commitSHA))
	if err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("find associations for commit: %w", err)
	}

	if len(associations) > 0 {
		ids := make([]int64, len(associations))
		for i, a := range associations {
			ids[i] = a.EnrichmentID()
		}
		if err := h.enrichmentStore.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
			if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
				h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
			}
			return fmt.Errorf("delete enrichments: %w", err)
		}
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Deleting enrichment associations"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.associationStore.DeleteBy(ctx, enrichment.WithEntityID(commitSHA)); err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("delete enrichment associations: %w", err)
	}

	h.logger.Info("commit data cleared for rescan",
		slog.Int64("repo_id", repoID),
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}
