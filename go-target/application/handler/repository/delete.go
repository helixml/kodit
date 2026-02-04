package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// Delete handles the DELETE_REPOSITORY task operation.
// It removes a repository and all its associated data from the system.
type Delete struct {
	repoStore      repository.RepositoryStore
	commitStore    repository.CommitStore
	branchStore    repository.BranchStore
	tagStore       repository.TagStore
	fileStore      repository.FileStore
	snippetStore   snippet.SnippetStore
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewDelete creates a new Delete handler.
func NewDelete(
	repoStore repository.RepositoryStore,
	commitStore repository.CommitStore,
	branchStore repository.BranchStore,
	tagStore repository.TagStore,
	fileStore repository.FileStore,
	snippetStore snippet.SnippetStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *Delete {
	return &Delete{
		repoStore:      repoStore,
		commitStore:    commitStore,
		branchStore:    branchStore,
		tagStore:       tagStore,
		fileStore:      fileStore,
		snippetStore:   snippetStore,
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
		task.TrackableTypeRepository,
		repoID,
	)

	repo, err := h.repoStore.Get(ctx, repoID)
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

	commits, err := h.commitStore.FindByRepoID(ctx, repoID)
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
		if err := h.commitStore.Delete(ctx, commit); err != nil {
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

	if err := h.repoStore.Delete(ctx, repo); err != nil {
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

func (h *Delete) deleteCommitData(ctx context.Context, commitSHA string) error {
	if err := h.fileStore.DeleteByCommitSHA(ctx, commitSHA); err != nil {
		return fmt.Errorf("delete files: %w", err)
	}

	if err := h.snippetStore.DeleteForCommit(ctx, commitSHA); err != nil {
		return fmt.Errorf("delete snippets: %w", err)
	}

	return nil
}

func (h *Delete) deleteBranches(ctx context.Context, repoID int64) error {
	branches, err := h.branchStore.FindByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("find branches: %w", err)
	}

	for _, branch := range branches {
		if err := h.branchStore.Delete(ctx, branch); err != nil {
			h.logger.Warn("failed to delete branch",
				slog.String("name", branch.Name()),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

func (h *Delete) deleteTags(ctx context.Context, repoID int64) error {
	tags, err := h.tagStore.FindByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("find tags: %w", err)
	}

	for _, tag := range tags {
		if err := h.tagStore.Delete(ctx, tag); err != nil {
			h.logger.Warn("failed to delete tag",
				slog.String("name", tag.Name()),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}
