package indexing

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/extraction"
	"github.com/helixml/kodit/infrastructure/slicing"
)

// ExtractSnippets extracts code snippets from commit files using AST parsing.
// When documentText is non-nil, binary document files (PDF, DOCX, etc.) are
// extracted via tabula, chunked, and saved as snippet enrichments alongside
// the AST-parsed source code snippets.
type ExtractSnippets struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	fileStore        repository.FileStore
	slicer           *slicing.Slicer
	documentText     DocumentTextSource
	trackerFactory   handler.TrackerFactory
	logger           zerolog.Logger
}

// NewExtractSnippets creates a new ExtractSnippets handler.
// When documentText is nil, document files are skipped.
func NewExtractSnippets(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	fileStore repository.FileStore,
	slicerInstance *slicing.Slicer,
	documentText DocumentTextSource,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) *ExtractSnippets {
	return &ExtractSnippets{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		fileStore:        fileStore,
		slicer:           slicerInstance,
		documentText:     documentText,
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
		payload,
	)

	existing, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
	if err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to check existing snippets")
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
	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(cp.CommitSHA()), repository.WithOrderAsc("path"))
	if err != nil {
		return fmt.Errorf("get commit files from database: %w", err)
	}

	if len(files) == 0 {
		h.logger.Info().Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("no files found for commit, skipping")
		tracker.Skip(ctx, "No files found for commit")
		return nil
	}

	// Separate document files (PDF, DOCX, etc.) from source code files.
	// Documents are extracted via tabula and chunked; source code goes through the AST slicer.
	var codeFiles []repository.File
	if h.documentText != nil {
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f.Path()))
			if !extraction.IsDocument(ext) {
				codeFiles = append(codeFiles, f)
				continue
			}
			relPath := relativeFilePath(f.Path(), clonedPath)
			diskPath := filepath.Join(clonedPath, relPath)
			text, extractErr := h.documentText.Text(diskPath)
			if extractErr != nil {
				h.logger.Warn().Str("path", f.Path()).Str("error", extractErr.Error()).Msg("failed to extract document text")
				continue
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			textChunks, chunkErr := chunking.NewTextChunks(text, chunking.DefaultChunkParams())
			if chunkErr != nil {
				h.logger.Warn().Str("path", f.Path()).Str("error", chunkErr.Error()).Msg("failed to chunk document")
				continue
			}
			for _, ch := range textChunks.All() {
				e := enrichment.NewSnippetEnrichmentWithLanguage(ch.Content(), f.Extension())
				saved, saveErr := h.enrichmentStore.Save(ctx, e)
				if saveErr != nil {
					return fmt.Errorf("save document snippet: %w", saveErr)
				}
				if _, err := h.associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())); err != nil {
					return fmt.Errorf("save commit association: %w", err)
				}
				if f.ID() != 0 {
					if _, err := h.associationStore.Save(ctx, enrichment.FileAssociation(saved.ID(), strconv.FormatInt(f.ID(), 10))); err != nil {
						return fmt.Errorf("save file association: %w", err)
					}
				}
			}
		}
	} else {
		codeFiles = files
	}

	langFiles := h.groupFilesByExtension(codeFiles)

	tracker.SetTotal(ctx, len(langFiles))

	cfg := slicing.DefaultSliceConfig()
	var allSnippets []snippet.Snippet

	extensions := make([]string, 0, len(langFiles))
	for ext := range langFiles {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)

	processed := 0
	for _, ext := range extensions {
		extFiles := langFiles[ext]
		message := fmt.Sprintf("Extracting snippets for %s", ext)
		tracker.SetCurrent(ctx, processed, message)

		result, sliceErr := h.slicer.Slice(ctx, extFiles, clonedPath, cfg)
		if sliceErr != nil {
			h.logger.Warn().Str("extension", ext).Str("error", sliceErr.Error()).Msg("failed to slice files")
			processed++
			continue
		}

		allSnippets = append(allSnippets, result.Snippets()...)
		processed++
	}

	uniqueSnippets := h.deduplicateSnippets(allSnippets)

	h.logger.Info().Int("total", len(allSnippets)).Int("unique", len(uniqueSnippets)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("extracted snippets")

	for _, s := range uniqueSnippets {
		if s.Content() == "" {
			continue
		}
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
