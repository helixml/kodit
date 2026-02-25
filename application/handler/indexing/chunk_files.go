package indexing

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/chunking"
)

// binaryProbeSize is the number of bytes checked for null bytes to detect binary files.
const binaryProbeSize = 8192

// FileContentSource reads file content at a specific commit.
type FileContentSource interface {
	FileContent(ctx context.Context, localPath string, commitSHA string, filePath string) ([]byte, error)
}

// ChunkFiles creates fixed-size text chunks from commit files.
type ChunkFiles struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	fileStore        repository.FileStore
	fileContent      FileContentSource
	params           chunking.ChunkParams
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewChunkFiles creates a new ChunkFiles handler.
func NewChunkFiles(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	fileStore repository.FileStore,
	fileContent FileContentSource,
	params chunking.ChunkParams,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *ChunkFiles {
	return &ChunkFiles{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		fileStore:        fileStore,
		fileContent:      fileContent,
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
		task.TrackableTypeRepository,
		cp.RepoID(),
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

		content, readErr := h.fileContent.FileContent(ctx, clonedPath, cp.CommitSHA(), f.Path())
		if readErr != nil {
			h.logger.Warn("failed to read file content",
				slog.String("path", f.Path()),
				slog.String("error", readErr.Error()),
			)
			processed++
			continue
		}

		if isBinary(content) {
			processed++
			continue
		}

		textChunks, chunkErr := chunking.NewTextChunks(string(content), h.params)
		if chunkErr != nil {
			h.logger.Warn("failed to chunk file",
				slog.String("path", f.Path()),
				slog.String("error", chunkErr.Error()),
			)
			processed++
			continue
		}

		for _, chunk := range textChunks.All() {
			e := enrichment.NewChunkEnrichmentWithLanguage(chunk.Content(), f.Extension())
			saved, saveErr := h.enrichmentStore.Save(ctx, e)
			if saveErr != nil {
				return fmt.Errorf("save chunk enrichment: %w", saveErr)
			}

			if _, err := h.associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())); err != nil {
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

		processed++
	}

	h.logger.Info("text chunks created",
		slog.Int("files", len(files)),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}

// isBinary returns true if the content contains null bytes in the first 8KB.
func isBinary(content []byte) bool {
	probe := content
	if len(probe) > binaryProbeSize {
		probe = probe[:binaryProbeSize]
	}
	return bytes.ContainsRune(probe, 0)
}
