package handler

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/indexing/slicer"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/tracking"
)

// ExtractSnippets extracts code snippets from commit files using AST parsing.
type ExtractSnippets struct {
	repoRepo       git.RepoRepository
	commitRepo     git.CommitRepository
	snippetRepo    indexing.SnippetRepository
	fileRepo       git.FileRepository
	slicer         *slicer.Slicer
	trackerFactory TrackerFactory
	logger         *slog.Logger
}

// TrackerFactory creates trackers for progress reporting.
type TrackerFactory interface {
	ForOperation(operation queue.TaskOperation, trackableType domain.TrackableType, trackableID int64) *tracking.Tracker
}

// NewExtractSnippets creates a new ExtractSnippets handler.
func NewExtractSnippets(
	repoRepo git.RepoRepository,
	commitRepo git.CommitRepository,
	snippetRepo indexing.SnippetRepository,
	fileRepo git.FileRepository,
	slicerInstance *slicer.Slicer,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *ExtractSnippets {
	return &ExtractSnippets{
		repoRepo:       repoRepo,
		commitRepo:     commitRepo,
		snippetRepo:    snippetRepo,
		fileRepo:       fileRepo,
		slicer:         slicerInstance,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the EXTRACT_SNIPPETS_FOR_COMMIT task.
func (h *ExtractSnippets) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationExtractSnippetsForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	existing, err := h.snippetRepo.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to check existing snippets", slog.String("error", err.Error()))
		return err
	}

	if len(existing) > 0 {
		if skipErr := tracker.Skip(ctx, "Snippets already extracted for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	repo, err := h.repoRepo.Get(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	// Load files from database (which have IDs from SCAN_COMMIT step)
	files, err := h.fileRepo.FindByCommitSHA(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get commit files from database: %w", err)
	}

	if len(files) == 0 {
		h.logger.Info("no files found for commit, skipping",
			slog.String("commit", commitSHA[:8]),
		)
		if skipErr := tracker.Skip(ctx, "No files found for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	langFiles := h.groupFilesByExtension(files)

	if setTotalErr := tracker.SetTotal(ctx, len(langFiles)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	cfg := slicer.DefaultSliceConfig()
	var allSnippets []indexing.Snippet

	processed := 0
	for ext, extFiles := range langFiles {
		message := fmt.Sprintf("Extracting snippets for %s", ext)
		if currentErr := tracker.SetCurrent(ctx, processed, message); currentErr != nil {
			h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		result, sliceErr := h.slicer.Slice(ctx, extFiles, clonedPath, cfg)
		if sliceErr != nil {
			h.logger.Warn("failed to slice files",
				slog.String("extension", ext),
				slog.String("error", sliceErr.Error()),
			)
			processed++
			continue
		}

		allSnippets = append(allSnippets, result.Snippets()...)
		processed++
	}

	uniqueSnippets := h.deduplicateSnippets(allSnippets)

	h.logger.Info("extracted snippets",
		slog.Int("total", len(allSnippets)),
		slog.Int("unique", len(uniqueSnippets)),
		slog.String("commit", commitSHA[:8]),
	)

	if err := h.snippetRepo.Save(ctx, commitSHA, uniqueSnippets); err != nil {
		return fmt.Errorf("save snippets: %w", err)
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *ExtractSnippets) groupFilesByExtension(files []git.File) map[string][]git.File {
	result := make(map[string][]git.File)

	for _, f := range files {
		ext := filepath.Ext(f.Path())
		result[ext] = append(result[ext], f)
	}

	return result
}

func (h *ExtractSnippets) deduplicateSnippets(snippets []indexing.Snippet) []indexing.Snippet {
	seen := make(map[string]bool)
	result := make([]indexing.Snippet, 0, len(snippets))

	for _, s := range snippets {
		if !seen[s.SHA()] {
			seen[s.SHA()] = true
			result = append(result, s)
		}
	}

	return result
}

func extractInt64(payload map[string]any, key string) (int64, error) {
	val, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("missing required field: %s", key)
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid type for %s: expected int64, got %T", key, val)
	}
}

func extractString(payload map[string]any, key string) (string, error) {
	val, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("missing required field: %s", key)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for %s: expected string, got %T", key, val)
	}

	return str, nil
}
