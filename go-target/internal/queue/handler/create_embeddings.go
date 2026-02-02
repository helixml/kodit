package handler

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
)

// CreateCodeEmbeddings creates vector embeddings for commit snippets.
type CreateCodeEmbeddings struct {
	embeddingService *indexing.EmbeddingService
	snippetRepo      indexing.SnippetRepository
	vectorRepo       indexing.VectorSearchRepository
	trackerFactory   TrackerFactory
	logger           *slog.Logger
}

// NewCreateCodeEmbeddings creates a new CreateCodeEmbeddings handler.
func NewCreateCodeEmbeddings(
	embeddingService *indexing.EmbeddingService,
	snippetRepo indexing.SnippetRepository,
	vectorRepo indexing.VectorSearchRepository,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *CreateCodeEmbeddings {
	return &CreateCodeEmbeddings{
		embeddingService: embeddingService,
		snippetRepo:      snippetRepo,
		vectorRepo:       vectorRepo,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the CREATE_CODE_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateCodeEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreateCodeEmbeddingsForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	snippets, err := h.snippetRepo.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to get snippets for commit", slog.String("error", err.Error()))
		return err
	}

	if len(snippets) == 0 {
		if skipErr := tracker.Skip(ctx, "No snippets to create embeddings for"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	newSnippets, err := h.filterNewSnippets(ctx, snippets)
	if err != nil {
		h.logger.Error("failed to filter new snippets", slog.String("error", err.Error()))
		return err
	}

	if len(newSnippets) == 0 {
		if skipErr := tracker.Skip(ctx, "All snippets already have code embeddings"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(newSnippets)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	documents := make([]domain.Document, 0, len(newSnippets))
	for _, snippet := range newSnippets {
		if snippet.SHA() != "" && snippet.Content() != "" {
			doc := domain.NewDocument(snippet.SHA(), snippet.Content())
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		if skipErr := tracker.Skip(ctx, "No valid documents to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	request := domain.NewIndexRequest(documents)
	if err := h.embeddingService.Index(ctx, request); err != nil {
		h.logger.Error("failed to create embeddings", slog.String("error", err.Error()))
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return err
	}

	if currentErr := tracker.SetCurrent(ctx, len(newSnippets), "Creating code embeddings for commit"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	h.logger.Info("code embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", commitSHA[:8]),
	)

	return nil
}

func (h *CreateCodeEmbeddings) filterNewSnippets(ctx context.Context, snippets []indexing.Snippet) ([]indexing.Snippet, error) {
	newSnippets := make([]indexing.Snippet, 0, len(snippets))

	for _, snippet := range snippets {
		hasEmbedding, err := h.vectorRepo.HasEmbedding(ctx, snippet.SHA(), indexing.EmbeddingTypeCode)
		if err != nil {
			return nil, err
		}

		if !hasEmbedding {
			newSnippets = append(newSnippets, snippet)
		}
	}

	return newSnippets, nil
}
