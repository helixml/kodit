package indexing

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/slicing"
)

// ExtractSnippets extracts code snippets from commit files using AST parsing.
type ExtractSnippets struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	fileStore        repository.FileStore
	slicer           *slicing.Slicer
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewExtractSnippets creates a new ExtractSnippets handler.
func NewExtractSnippets(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	fileStore repository.FileStore,
	slicerInstance *slicing.Slicer,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *ExtractSnippets {
	return &ExtractSnippets{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		fileStore:        fileStore,
		slicer:           slicerInstance,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the EXTRACT_SNIPPETS_FOR_COMMIT task.
func (h *ExtractSnippets) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationExtractSnippetsForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	existing, err := h.enrichmentStore.FindByCommitSHA(ctx, commitSHA, enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
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

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	// Load files from database (which have IDs from SCAN_COMMIT step)
	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(commitSHA))
	if err != nil {
		return fmt.Errorf("get commit files from database: %w", err)
	}

	if len(files) == 0 {
		h.logger.Info("no files found for commit, skipping",
			slog.String("commit", handler.ShortSHA(commitSHA)),
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

	cfg := slicing.DefaultSliceConfig()
	var allSnippets []snippet.Snippet

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
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	for _, s := range uniqueSnippets {
		e := enrichment.NewSnippetEnrichmentWithLanguage(s.Content(), s.Extension())
		saved, err := h.enrichmentStore.Save(ctx, e)
		if err != nil {
			return fmt.Errorf("save snippet enrichment: %w", err)
		}

		assoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.associationStore.Save(ctx, assoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *ExtractSnippets) groupFilesByExtension(files []repository.File) map[string][]repository.File {
	result := make(map[string][]repository.File)

	for _, f := range files {
		ext := filepath.Ext(f.Path())
		result[ext] = append(result[ext], f)
	}

	return result
}

func (h *ExtractSnippets) deduplicateSnippets(snippets []snippet.Snippet) []snippet.Snippet {
	seen := make(map[string]bool)
	result := make([]snippet.Snippet, 0, len(snippets))

	for _, s := range snippets {
		if !seen[s.SHA()] {
			seen[s.SHA()] = true
			result = append(result, s)
		}
	}

	return result
}
