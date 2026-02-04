package indexing

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// CreateBM25Index creates BM25 keyword index for commit snippets.
type CreateBM25Index struct {
	bm25Service    *domainservice.BM25
	snippetStore   snippet.SnippetStore
	trackerFactory handler.TrackerFactory
	logger         *slog.Logger
}

// NewCreateBM25Index creates a new CreateBM25Index handler.
func NewCreateBM25Index(
	bm25Service *domainservice.BM25,
	snippetStore snippet.SnippetStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *CreateBM25Index {
	return &CreateBM25Index{
		bm25Service:    bm25Service,
		snippetStore:   snippetStore,
		trackerFactory: trackerFactory,
		logger:         logger,
	}
}

// Execute processes the CREATE_BM25_INDEX_FOR_COMMIT task.
func (h *CreateBM25Index) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateBM25IndexForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	snippets, err := h.snippetStore.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to get snippets for commit", slog.String("error", err.Error()))
		return err
	}

	if len(snippets) == 0 {
		if skipErr := tracker.Skip(ctx, "No snippets to index"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(snippets)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	documents := make([]search.Document, 0, len(snippets))
	for _, s := range snippets {
		if s.SHA() != "" && s.Content() != "" {
			doc := search.NewDocument(s.SHA(), s.Content())
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		if skipErr := tracker.Skip(ctx, "No valid documents to index"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.bm25Service.Index(ctx, request); err != nil {
		h.logger.Error("failed to index documents", slog.String("error", err.Error()))
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return err
	}

	if currentErr := tracker.SetCurrent(ctx, len(snippets), "BM25 index created for commit"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	h.logger.Info("BM25 index created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	return nil
}
