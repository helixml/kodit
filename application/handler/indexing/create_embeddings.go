package indexing

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// CreateCodeEmbeddings creates vector embeddings for commit snippets.
type CreateCodeEmbeddings struct {
	codeIndex      handler.VectorIndex
	snippetStore   snippet.SnippetStore
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewCreateCodeEmbeddings creates a new CreateCodeEmbeddings handler.
func NewCreateCodeEmbeddings(
	codeIndex handler.VectorIndex,
	snippetStore snippet.SnippetStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) (*CreateCodeEmbeddings, error) {
	if codeIndex.Embedding == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil Embedding")
	}
	if codeIndex.Store == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil Store")
	}
	if snippetStore == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil snippetStore")
	}
	if trackerFactory == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil trackerFactory")
	}
	return &CreateCodeEmbeddings{
		codeIndex:      codeIndex,
		snippetStore:   snippetStore,
		trackerFactory: trackerFactory,
		logger:         logger,
	}, nil
}

// Execute processes the CREATE_CODE_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateCodeEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateCodeEmbeddingsForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	snippets, err := h.snippetStore.SnippetsForCommit(ctx, commitSHA)
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

	documents := make([]search.Document, 0, len(newSnippets))
	for _, s := range newSnippets {
		if s.SHA() != "" && s.Content() != "" {
			doc := search.NewDocument(s.SHA(), s.Content())
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		if skipErr := tracker.Skip(ctx, "No valid documents to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.codeIndex.Embedding.Index(ctx, request); err != nil {
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
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	return nil
}

func (h *CreateCodeEmbeddings) filterNewSnippets(ctx context.Context, snippets []snippet.Snippet) ([]snippet.Snippet, error) {
	newSnippets := make([]snippet.Snippet, 0, len(snippets))

	for _, s := range snippets {
		hasEmbedding, err := h.codeIndex.Store.HasEmbedding(ctx, s.SHA(), snippet.EmbeddingTypeCode)
		if err != nil {
			return nil, err
		}

		if !hasEmbedding {
			newSnippets = append(newSnippets, s)
		}
	}

	return newSnippets, nil
}
