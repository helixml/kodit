package repository

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// Delete handles the DELETE_REPOSITORY task operation.
// It removes a repository and all its associated data from the system.
type Delete struct {
	repoStores     handler.RepositoryStores
	enrichments    *service.Enrichment
	queue          *service.Queue
	trackerFactory handler.TrackerFactory
	logger         zerolog.Logger
}

// NewDelete creates a new Delete handler.
func NewDelete(
	repoStores handler.RepositoryStores,
	enrichments *service.Enrichment,
	queue *service.Queue,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) *Delete {
	return &Delete{
		repoStores:     repoStores,
		enrichments:    enrichments,
		queue:          queue,
		trackerFactory: trackerFactory,
		logger:         logger,
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
		payload,
	)

	repo, err := h.repoStores.Repositories.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	// Drain any pending tasks for this repository (e.g. leftover rescan/indexing tasks)
	// so they don't block the worker after the repository data is gone.
	drained, err := h.queue.DrainForRepository(ctx, repoID)
	if err != nil {
		h.logger.Warn().Str("error", err.Error()).Msg("failed to drain pending tasks")
	}
	if drained > 0 {
		h.logger.Info().Int64("repo_id", repoID).Int("drained", drained).Msg("drained pending tasks for repository")
	}

	tracker.SetTotal(ctx, 3)

	// Clean up enrichment data (polymorphic relationship, not cascade-deleted)
	tracker.SetCurrent(ctx, 0, "Cleaning up enrichment data")

	commits, err := h.repoStores.Commits.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		h.logger.Warn().Str("error", err.Error()).Msg("failed to find commits")
	}

	if err := h.deleteEnrichments(ctx, commits); err != nil {
		h.logger.Warn().Str("error", err.Error()).Msg("failed to delete enrichment data")
	}

	// Remove working copy from disk
	tracker.SetCurrent(ctx, 1, "Removing working copy")

	if repo.HasWorkingCopy() {
		clonedPath := repo.WorkingCopy().Path()
		if err := os.RemoveAll(clonedPath); err != nil {
			h.logger.Warn().Str("path", clonedPath).Str("error", err.Error()).Msg("failed to remove working copy")
		}
	}

	// Delete repository — commits, branches, tags, and files cascade-delete via GORM constraints
	tracker.SetCurrent(ctx, 2, "Deleting repository record")

	if err := h.repoStores.Repositories.Delete(ctx, repo); err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}

	h.logger.Info().Int64("repo_id", repoID).Int("commits_cleaned", len(commits)).Msg("repository deleted successfully")

	return nil
}

func (h *Delete) deleteEnrichments(ctx context.Context, commits []repository.Commit) error {
	if len(commits) == 0 {
		return nil
	}

	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.SHA()
	}

	found, err := h.enrichments.Find(ctx, enrichment.WithCommitSHAs(shas))
	if err != nil {
		return fmt.Errorf("find enrichments: %w", err)
	}

	if len(found) == 0 {
		return nil
	}

	ids := make([]int64, len(found))
	for i, e := range found {
		ids[i] = e.ID()
	}

	if err := h.enrichments.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
		return fmt.Errorf("delete enrichments: %w", err)
	}

	return nil
}
