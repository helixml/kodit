package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// Delete handles the DELETE_REPOSITORY task operation.
// It removes a repository and all its associated data from the system.
type Delete struct {
	repoStores      handler.RepositoryStores
	enrichmentStore enrichment.EnrichmentStore
	trackerFactory  handler.TrackerFactory
	logger          *slog.Logger
}

// NewDelete creates a new Delete handler.
func NewDelete(
	repoStores handler.RepositoryStores,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Delete {
	return &Delete{
		repoStores:      repoStores,
		enrichmentStore: enrichmentStore,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the DELETE_REPOSITORY task.
func (h *Delete) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationDeleteRepository,
		task.TrackableTypeRepository,
		repoID,
	)

	repo, err := h.repoStores.Repositories.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	tracker.SetTotal(ctx, 3)

	// Clean up enrichment data (polymorphic relationship, not cascade-deleted)
	tracker.SetCurrent(ctx, 0, "Cleaning up enrichment data")

	commits, err := h.repoStores.Commits.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		h.logger.Warn("failed to find commits", slog.String("error", err.Error()))
	}

	for _, commit := range commits {
		if err := h.deleteEnrichmentData(ctx, commit.SHA()); err != nil {
			h.logger.Warn("failed to delete enrichment data",
				slog.String("sha", commit.SHA()),
				slog.String("error", err.Error()),
			)
		}
	}

	// Remove working copy from disk
	tracker.SetCurrent(ctx, 1, "Removing working copy")

	if repo.HasWorkingCopy() {
		clonedPath := repo.WorkingCopy().Path()
		if err := os.RemoveAll(clonedPath); err != nil {
			h.logger.Warn("failed to remove working copy",
				slog.String("path", clonedPath),
				slog.String("error", err.Error()),
			)
		}
	}

	// Delete repository — commits, branches, tags, and files cascade-delete via GORM constraints
	tracker.SetCurrent(ctx, 2, "Deleting repository record")

	if err := h.repoStores.Repositories.Delete(ctx, repo); err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}

	h.logger.Info("repository deleted successfully",
		slog.Int64("repo_id", repoID),
		slog.Int("commits_cleaned", len(commits)),
	)

	return nil
}

func (h *Delete) deleteEnrichmentData(ctx context.Context, commitSHA string) error {
	// Find enrichments for this commit, then delete them.
	// Associations are cascade-deleted when their parent enrichment is removed.
	enrichments, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(commitSHA))
	if err != nil {
		return fmt.Errorf("find enrichments for commit: %w", err)
	}

	if len(enrichments) > 0 {
		ids := make([]int64, len(enrichments))
		for i, e := range enrichments {
			ids[i] = e.ID()
		}
		if err := h.enrichmentStore.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
			return fmt.Errorf("delete enrichments: %w", err)
		}
	}

	return nil
}
