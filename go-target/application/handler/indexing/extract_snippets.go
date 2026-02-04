package indexing

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/repository"
	domainsnippet "github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// ExtractSnippets extracts code snippets from commit files using AST parsing.
type ExtractSnippets struct {
	repoStore      repository.RepositoryStore
	snippetStore   domainsnippet.SnippetStore
	adapter        git.Adapter
	slicer         *slicer.Slicer
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewExtractSnippets creates a new ExtractSnippets handler.
func NewExtractSnippets(
	repoStore repository.RepositoryStore,
	snippetStore domainsnippet.SnippetStore,
	adapter git.Adapter,
	slicerInstance *slicer.Slicer,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *ExtractSnippets {
	return &ExtractSnippets{
		repoStore:      repoStore,
		snippetStore:   snippetStore,
		adapter:        adapter,
		slicer:         slicerInstance,
		trackerFactory: trackerFactory,
		logger:         logger,
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

	existing, err := h.snippetStore.SnippetsForCommit(ctx, commitSHA)
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

	repo, err := h.repoStore.Get(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	files, err := h.adapter.CommitFiles(ctx, clonedPath, commitSHA)
	if err != nil {
		return fmt.Errorf("get commit files: %w", err)
	}

	gitFiles := h.convertToGitFiles(files, commitSHA)
	langFiles := h.groupFilesByExtension(gitFiles)

	if setTotalErr := tracker.SetTotal(ctx, len(langFiles)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	cfg := slicer.DefaultSliceConfig()
	var allSnippets []domainsnippet.Snippet

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

		allSnippets = append(allSnippets, h.convertSnippets(result.Snippets())...)
		processed++
	}

	uniqueSnippets := h.deduplicateSnippets(allSnippets)

	h.logger.Info("extracted snippets",
		slog.Int("total", len(allSnippets)),
		slog.Int("unique", len(uniqueSnippets)),
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	if err := h.snippetStore.Save(ctx, commitSHA, uniqueSnippets); err != nil {
		return fmt.Errorf("save snippets: %w", err)
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *ExtractSnippets) convertToGitFiles(files []git.FileInfo, commitSHA string) []git.File {
	result := make([]git.File, 0, len(files))
	language := domainsnippet.Language{}

	for _, f := range files {
		ext := filepath.Ext(f.Path)
		lang, err := language.LanguageForExtension(ext)
		if err != nil {
			continue
		}

		gitFile := git.NewFile(commitSHA, f.Path, lang, f.Size)
		result = append(result, gitFile)
	}

	return result
}

func (h *ExtractSnippets) groupFilesByExtension(files []git.File) map[string][]git.File {
	result := make(map[string][]git.File)

	for _, f := range files {
		ext := filepath.Ext(f.Path())
		result[ext] = append(result[ext], f)
	}

	return result
}

func (h *ExtractSnippets) convertSnippets(internal []indexing.Snippet) []domainsnippet.Snippet {
	result := make([]domainsnippet.Snippet, 0, len(internal))
	for _, s := range internal {
		// Convert internal/git.File to domain/repository.File
		internalFiles := s.DerivesFrom()
		domainFiles := make([]repository.File, 0, len(internalFiles))
		for _, f := range internalFiles {
			domainFile := repository.NewFile(f.CommitSHA(), f.Path(), f.Language(), f.Size())
			domainFiles = append(domainFiles, domainFile)
		}

		domainSnippet := domainsnippet.NewSnippet(s.Content(), s.Extension(), domainFiles)
		result = append(result, domainSnippet)
	}
	return result
}

func (h *ExtractSnippets) deduplicateSnippets(snippets []domainsnippet.Snippet) []domainsnippet.Snippet {
	seen := make(map[string]bool)
	result := make([]domainsnippet.Snippet, 0, len(snippets))

	for _, s := range snippets {
		if !seen[s.SHA()] {
			seen[s.SHA()] = true
			result = append(result, s)
		}
	}

	return result
}
