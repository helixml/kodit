package indexing

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/extraction"
)

// FileContentSource reads file content at a specific commit.
type FileContentSource interface {
	FileContent(ctx context.Context, localPath string, commitSHA string, filePath string) ([]byte, error)
}

// DocumentTextSource extracts text from binary document files.
type DocumentTextSource interface {
	Text(path string) (string, error)
}

// ChunkFiles creates fixed-size text chunks from commit files.
type ChunkFiles struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	lineRangeStore   chunk.LineRangeStore
	fileStore        repository.FileStore
	fileContent      FileContentSource
	documentText     DocumentTextSource
	extractors       *extraction.Extractors
	params           chunking.ChunkParams
	trackerFactory   handler.TrackerFactory
	logger           zerolog.Logger
}

// NewChunkFiles creates a new ChunkFiles handler.
// When documentText is nil, document files are skipped.
func NewChunkFiles(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	lineRangeStore chunk.LineRangeStore,
	fileStore repository.FileStore,
	fileContent FileContentSource,
	documentText DocumentTextSource,
	extractors *extraction.Extractors,
	params chunking.ChunkParams,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) *ChunkFiles {
	return &ChunkFiles{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		lineRangeStore:   lineRangeStore,
		fileStore:        fileStore,
		fileContent:      fileContent,
		documentText:     documentText,
		extractors:       extractors,
		params:           params,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the EXTRACT_SNIPPETS_FOR_COMMIT task using fixed-size text chunking.
func (h *ChunkFiles) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationExtractSnippetsForCommit,
		payload,
	)

	existing, err := h.enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(cp.CommitSHA()),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	if err != nil {
		return fmt.Errorf("check existing chunks: %w", err)
	}

	if len(existing) > 0 {
		tracker.Skip(ctx, "Chunks already created for commit")
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	params := chunking.ChunkParams{
		Size:    repo.ChunkingConfig().Size(),
		Overlap: repo.ChunkingConfig().Overlap(),
		MinSize: repo.ChunkingConfig().MinSize(),
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", cp.RepoID())
	}

	files, err := h.fileStore.Find(ctx,
		repository.WithCommitSHA(cp.CommitSHA()),
		repository.WithOrderAsc("path"),
	)
	if err != nil {
		return fmt.Errorf("get commit files: %w", err)
	}

	if len(files) == 0 {
		tracker.Skip(ctx, "No files found for commit")
		return nil
	}

	tracker.SetTotal(ctx, len(files))
	repoIDStr := strconv.FormatInt(cp.RepoID(), 10)

	processed := 0
	for _, f := range files {
		tracker.SetCurrent(ctx, processed, fmt.Sprintf("Chunking %s", f.Path()))

		if !isIndexable(f.Path()) {
			processed++
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Path()))
		relPath := relativeFilePath(f.Path(), clonedPath)

		var text string

		if extraction.IsDocument(ext) {
			if h.documentText == nil {
				processed++
				continue
			}
			diskPath := filepath.Join(clonedPath, relPath)
			var extractErr error
			text, extractErr = h.documentText.Text(diskPath)
			if extractErr != nil {
				h.logger.Warn().Str("path", f.Path()).Str("error", extractErr.Error()).Msg("failed to extract document text")
				processed++
				continue
			}
		} else {
			content, readErr := h.fileContent.FileContent(ctx, clonedPath, cp.CommitSHA(), relPath)
			if readErr != nil {
				h.logger.Warn().Str("path", f.Path()).Str("error", readErr.Error()).Msg("failed to read file content")
				processed++
				continue
			}
			var extractErr error
			text, extractErr = h.extractors.For(ext).Text(content)
			if extractErr != nil {
				h.logger.Warn().Str("path", f.Path()).Str("error", extractErr.Error()).Msg("failed to extract text")
				processed++
				continue
			}
		}

		if strings.TrimSpace(text) == "" {
			processed++
			continue
		}

		textChunks, chunkErr := chunking.NewTextChunks(text, params)
		if chunkErr != nil {
			h.logger.Warn().Str("path", f.Path()).Str("error", chunkErr.Error()).Msg("failed to chunk file")
			processed++
			continue
		}

		if err := h.persistChunks(ctx, textChunks, f, cp.CommitSHA(), repoIDStr); err != nil {
			return err
		}

		processed++
	}

	h.logger.Info().Int("files", len(files)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("text chunks created")

	return nil
}

// persistChunks saves enrichments, line ranges, and associations for the given chunks.
func (h *ChunkFiles) persistChunks(ctx context.Context, textChunks chunking.TextChunks, f repository.File, commitSHA string, repoIDStr string) error {
	for _, ch := range textChunks.All() {
		e := enrichment.NewChunkEnrichmentWithLanguage(ch.Content(), f.Extension())
		saved, saveErr := h.enrichmentStore.Save(ctx, e)
		if saveErr != nil {
			return fmt.Errorf("save chunk enrichment: %w", saveErr)
		}

		lr := chunk.NewLineRange(saved.ID(), ch.StartLine(), ch.EndLine())
		if _, err := h.lineRangeStore.Save(ctx, lr); err != nil {
			return fmt.Errorf("save chunk line range: %w", err)
		}

		if _, err := h.associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA)); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}

		if f.ID() != 0 {
			if _, err := h.associationStore.Save(ctx, enrichment.FileAssociation(saved.ID(), strconv.FormatInt(f.ID(), 10))); err != nil {
				return fmt.Errorf("save file association: %w", err)
			}
		}

		if _, err := h.associationStore.Save(ctx, enrichment.RepositoryAssociation(saved.ID(), repoIDStr)); err != nil {
			return fmt.Errorf("save repository association: %w", err)
		}
	}
	return nil
}

// relativeFilePath converts a file path to a path relative to a git repository.
// File records from legacy database migrations may contain absolute paths instead of
// repository-relative paths. This function normalizes both cases so that git show
// (which requires repo-relative paths) works correctly.
func relativeFilePath(filePath, clonedPath string) string {
	if !filepath.IsAbs(filePath) {
		return filePath
	}

	// If the path starts with the current clone directory, strip it.
	clonedPath = filepath.Clean(clonedPath)
	if rel, err := filepath.Rel(clonedPath, filePath); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}

	// Legacy absolute paths follow the pattern: /<data-dir>/<type>/<repo-name>/<relative-path>
	// where <type> is "clones" or "repos". Find the last such segment and extract the
	// relative portion after the repo directory.
	parts := strings.Split(filepath.Clean(filePath), string(filepath.Separator))
	lastIdx := -1
	for i, part := range parts {
		if part == "clones" || part == "repos" {
			lastIdx = i
		}
	}
	if lastIdx >= 0 && lastIdx+2 < len(parts) {
		return filepath.Join(parts[lastIdx+2:]...)
	}

	return filePath
}

// indexableExtensions lists file extensions that contain human-written source
// code or documentation worth indexing. Everything else (lock files, images,
// binary formats, data files) is skipped.
var indexableExtensions = map[string]bool{
	// Go
	".go": true,
	// Python
	".py": true, ".pyi": true, ".pyx": true,
	// JavaScript / TypeScript
	".js": true, ".mjs": true, ".cjs": true, ".jsx": true,
	".ts": true, ".mts": true, ".cts": true, ".tsx": true,
	// Ruby
	".rb": true, ".erb": true,
	// Rust
	".rs": true,
	// Java / Kotlin / Scala / Groovy
	".java": true, ".kt": true, ".kts": true, ".scala": true, ".groovy": true,
	// C / C++ / Objective-C
	".c": true, ".h": true, ".cpp": true, ".cc": true, ".cxx": true,
	".hpp": true, ".hxx": true, ".m": true, ".mm": true,
	// C# / F#
	".cs": true, ".fs": true, ".fsx": true,
	// PHP
	".php": true,
	// Swift
	".swift": true,
	// Shell
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	// SQL
	".sql": true,
	// R
	".r": true,
	// Lua
	".lua": true,
	// Perl
	".pl": true, ".pm": true,
	// Elixir / Erlang
	".ex": true, ".exs": true, ".erl": true, ".hrl": true,
	// Haskell
	".hs": true,
	// Clojure
	".clj": true, ".cljs": true, ".cljc": true,
	// Dart
	".dart": true,
	// Zig / Nim
	".zig": true, ".nim": true,
	// Julia
	".jl": true,
	// OCaml
	".ml": true, ".mli": true,
	// V / D
	".v": true, ".d": true,
	// Web
	".html": true, ".htm": true, ".css": true, ".scss": true,
	".sass": true, ".less": true, ".vue": true, ".svelte": true,
	// Documentation
	".md": true, ".mdx": true, ".rst": true, ".adoc": true, ".tex": true,
	// IDL / Schema
	".proto": true, ".graphql": true, ".gql": true, ".thrift": true,
	// Data
	".csv": true,
}

func init() {
	for _, ext := range extraction.Extensions() {
		indexableExtensions[ext] = true
	}
}

// isIndexable returns true if the file extension is in the whitelist of
// source code and documentation formats worth indexing.
func isIndexable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return indexableExtensions[ext]
}
