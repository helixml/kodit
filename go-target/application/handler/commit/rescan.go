package commit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// Rescan handles the RESCAN_COMMIT task operation.
// It clears existing indexed data for a commit to prepare for re-indexing.
type Rescan struct {
	snippetStore     snippet.SnippetStore
	associationStore enrichment.AssociationStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewRescan creates a new Rescan handler.
func NewRescan(
	snippetStore snippet.SnippetStore,
	associationStore enrichment.AssociationStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Rescan {
	return &Rescan{
		snippetStore:     snippetStore,
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

	if currentErr := tracker.SetCurrent(ctx, 0, "Deleting snippet associations"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.snippetStore.DeleteForCommit(ctx, commitSHA); err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("delete snippet associations: %w", err)
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
