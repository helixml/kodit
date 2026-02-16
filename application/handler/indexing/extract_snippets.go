package indexing

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"

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
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationExtractSnippetsForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	existing, err := h.enrichmentStore.FindByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
	if err != nil {
		h.logger.Error("failed to check existing snippets", slog.String("error", err.Error()))
		return err
	}

	if len(existing) > 0 {
		tracker.Skip(ctx, "Snippets already extracted for commit")
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", cp.RepoID())
	}

	// Load files from database (which have IDs from SCAN_COMMIT step)
	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("get commit files from database: %w", err)
	}

	if len(files) == 0 {
		h.logger.Info("no files found for commit, skipping",
			slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
		)
		tracker.Skip(ctx, "No files found for commit")
		return nil
	}

	langFiles := h.groupFilesByExtension(files)

	tracker.SetTotal(ctx, len(langFiles))

	cfg := slicing.DefaultSliceConfig()
	var allSnippets []snippet.Snippet

	processed := 0
	for ext, extFiles := range langFiles {
		message := fmt.Sprintf("Extracting snippets for %s", ext)
		tracker.SetCurrent(ctx, processed, message)

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
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	for _, s := range uniqueSnippets {
		e := enrichment.NewSnippetEnrichmentWithLanguage(s.Content(), s.Extension())
		saved, err := h.enrichmentStore.Save(ctx, e)
		if err != nil {
			return fmt.Errorf("save snippet enrichment: %w", err)
		}

		assoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
		if _, err := h.associationStore.Save(ctx, assoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}

		for _, f := range s.DerivesFrom() {
			if f.ID() == 0 {
				continue
			}
			fileAssoc := enrichment.FileAssociation(saved.ID(), strconv.FormatInt(f.ID(), 10))
			if _, err := h.associationStore.Save(ctx, fileAssoc); err != nil {
				return fmt.Errorf("save file association: %w", err)
			}
		}
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
