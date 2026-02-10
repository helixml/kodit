package handler

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
)

// DeleteRepository handles the DELETE_REPOSITORY task operation.
// It removes a repository and all its associated data from the system.
type DeleteRepository struct {
	repoRepo       repository.RepositoryStore
	commitRepo     repository.CommitStore
	branchRepo     repository.BranchStore
	tagRepo        repository.TagStore
	fileRepo       repository.FileStore
	snippetRepo    indexing.SnippetRepository
	trackerFactory TrackerFactory
	logger         *slog.Logger
}

// NewDeleteRepository creates a new DeleteRepository handler.
func NewDeleteRepository(
	repoRepo repository.RepositoryStore,
	commitRepo repository.CommitStore,
	branchRepo repository.BranchStore,
	tagRepo repository.TagStore,
	fileRepo repository.FileStore,
	snippetRepo indexing.SnippetRepository,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *DeleteRepository {
	return &DeleteRepository{
		repoRepo:       repoRepo,
		commitRepo:     commitRepo,
		branchRepo:     branchRepo,
		tagRepo:        tagRepo,
		fileRepo:       fileRepo,
		snippetRepo:    snippetRepo,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the DELETE_REPOSITORY task.
func (h *DeleteRepository) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationDeleteRepository,
		domain.TrackableTypeRepository,
		repoID,
	)

	repo, err := h.repoRepo.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("get repository: %w", err)
	}

	if setTotalErr := tracker.SetTotal(ctx, 6); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 0, "Deleting commits"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	commits, err := h.commitRepo.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		h.logger.Warn("failed to find commits", slog.String("error", err.Error()))
	}

	for _, commit := range commits {
		if err := h.deleteCommitData(ctx, commit.SHA()); err != nil {
			h.logger.Warn("failed to delete commit data",
				slog.String("sha", commit.SHA()),
				slog.String("error", err.Error()),
			)
		}
		if err := h.commitRepo.Delete(ctx, commit); err != nil {
			h.logger.Warn("failed to delete commit",
				slog.String("sha", commit.SHA()),
				slog.String("error", err.Error()),
			)
		}
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Deleting branches"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.deleteBranches(ctx, repoID); err != nil {
		h.logger.Warn("failed to delete branches", slog.String("error", err.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 2, "Deleting tags"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.deleteTags(ctx, repoID); err != nil {
		h.logger.Warn("failed to delete tags", slog.String("error", err.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Removing working copy"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if repo.HasWorkingCopy() {
		clonedPath := repo.WorkingCopy().Path()
		if err := os.RemoveAll(clonedPath); err != nil {
			h.logger.Warn("failed to remove working copy",
				slog.String("path", clonedPath),
				slog.String("error", err.Error()),
			)
		}
	}

	if currentErr := tracker.SetCurrent(ctx, 4, "Deleting repository record"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if err := h.repoRepo.Delete(ctx, repo); err != nil {
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return fmt.Errorf("delete repository: %w", err)
	}

	h.logger.Info("repository deleted successfully",
		slog.Int64("repo_id", repoID),
		slog.Int("commits_deleted", len(commits)),
	)

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *DeleteRepository) deleteCommitData(ctx context.Context, commitSHA string) error {
	if err := h.fileRepo.DeleteBy(ctx, repository.WithCommitSHA(commitSHA)); err != nil {
		return fmt.Errorf("delete files: %w", err)
	}

	if err := h.snippetRepo.DeleteForCommit(ctx, commitSHA); err != nil {
		return fmt.Errorf("delete snippets: %w", err)
	}

	return nil
}

func (h *DeleteRepository) deleteBranches(ctx context.Context, repoID int64) error {
	branches, err := h.branchRepo.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		return fmt.Errorf("find branches: %w", err)
	}

	for _, branch := range branches {
		if err := h.branchRepo.Delete(ctx, branch); err != nil {
			h.logger.Warn("failed to delete branch",
				slog.String("name", branch.Name()),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

func (h *DeleteRepository) deleteTags(ctx context.Context, repoID int64) error {
	tags, err := h.tagRepo.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		return fmt.Errorf("find tags: %w", err)
	}

	for _, tag := range tags {
		if err := h.tagRepo.Delete(ctx, tag); err != nil {
			h.logger.Warn("failed to delete tag",
				slog.String("name", tag.Name()),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}
