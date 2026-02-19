package indexing

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

const (
	defaultChunkLines   = 60
	defaultOverlapLines = 10
)

// ExtractSnippets extracts code snippets from commit files using line-based chunking.
type ExtractSnippets struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	fileStore        repository.FileStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewExtractSnippets creates a new ExtractSnippets handler.
func NewExtractSnippets(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	fileStore repository.FileStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *ExtractSnippets {
	return &ExtractSnippets{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		fileStore:        fileStore,
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

	existing, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
	if err != nil {
		return fmt.Errorf("check existing snippets: %w", err)
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

	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("get commit files: %w", err)
	}

	if len(files) == 0 {
		tracker.Skip(ctx, "No files found for commit")
		return nil
	}

	tracker.SetTotal(ctx, len(files))

	saved := 0
	for i, f := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tracker.SetCurrent(ctx, i, "Chunking "+f.Path())

		chunks, lang, err := h.chunkFile(f, clonedPath)
		if err != nil {
			h.logger.Warn("failed to read file for chunking",
				slog.String("path", f.Path()),
				slog.String("error", err.Error()),
			)
			continue
		}

		for _, chunk := range chunks {
			if err := h.saveChunk(ctx, chunk, lang, cp.CommitSHA(), f); err != nil {
				return err
			}
			saved++
		}
	}

	h.logger.Info("extracted snippets",
		slog.Int("snippets", saved),
		slog.Int("files", len(files)),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}

func (h *ExtractSnippets) chunkFile(f repository.File, basePath string) ([]string, string, error) {
	fullPath := filepath.Join(basePath, f.Path())

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", err
	}

	lang := fileLanguage(f)
	text := string(content)
	if strings.TrimSpace(text) == "" {
		return nil, lang, nil
	}

	lines := strings.Split(text, "\n")
	return splitLines(lines, defaultChunkLines, defaultOverlapLines), lang, nil
}

func (h *ExtractSnippets) saveChunk(ctx context.Context, chunk, language, commitSHA string, f repository.File) error {
	e := enrichment.NewSnippetEnrichmentWithLanguage(chunk, language)
	saved, err := h.enrichmentStore.Save(ctx, e)
	if err != nil {
		return fmt.Errorf("save snippet enrichment: %w", err)
	}

	if _, err := h.associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA)); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if f.ID() != 0 {
		if _, err := h.associationStore.Save(ctx, enrichment.FileAssociation(saved.ID(), strconv.FormatInt(f.ID(), 10))); err != nil {
			return fmt.Errorf("save file association: %w", err)
		}
	}

	return nil
}

// splitLines splits lines into overlapping chunks.
func splitLines(lines []string, size, overlap int) []string {
	if len(lines) == 0 {
		return nil
	}

	if len(lines) <= size {
		return []string{strings.Join(lines, "\n")}
	}

	step := size - overlap
	if step < 1 {
		step = 1
	}

	var chunks []string
	for start := 0; start < len(lines); start += step {
		end := start + size
		if end > len(lines) {
			end = len(lines)
		}
		chunk := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(lines) {
			break
		}
	}

	return chunks
}

// fileLanguage derives the language name from a file's extension or language field.
func fileLanguage(f repository.File) string {
	if f.Language() != "" {
		return f.Language()
	}
	ext := filepath.Ext(f.Path())
	languages := map[string]string{
		".py":    "python",
		".go":    "go",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".cc":    "cpp",
		".cxx":   "cpp",
		".rs":    "rust",
		".js":    "javascript",
		".ts":    "typescript",
		".tsx":   "tsx",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".kt":    "kotlin",
		".swift": "swift",
		".sh":    "shell",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".md":    "markdown",
		".sql":   "sql",
	}
	if lang, ok := languages[ext]; ok {
		return lang
	}
	return ""
}
