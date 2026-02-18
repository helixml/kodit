package indexing

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/task"
)

// CreateExampleSummaryEmbeddings creates vector embeddings for example summary enrichments.
type CreateExampleSummaryEmbeddings struct {
	textIndex       handler.VectorIndex
	enrichmentStore enrichment.EnrichmentStore
	trackerFactory  handler.TrackerFactory
	logger          *slog.Logger
}

// NewCreateExampleSummaryEmbeddings creates a new CreateExampleSummaryEmbeddings handler.
func NewCreateExampleSummaryEmbeddings(
	textIndex handler.VectorIndex,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) (*CreateExampleSummaryEmbeddings, error) {
	if textIndex.Embedding == nil {
		return nil, fmt.Errorf("NewCreateExampleSummaryEmbeddings: nil Embedding")
	}
	if textIndex.Store == nil {
		return nil, fmt.Errorf("NewCreateExampleSummaryEmbeddings: nil Store")
	}
	if enrichmentStore == nil {
		return nil, fmt.Errorf("NewCreateExampleSummaryEmbeddings: nil enrichmentStore")
	}
	if trackerFactory == nil {
		return nil, fmt.Errorf("NewCreateExampleSummaryEmbeddings: nil trackerFactory")
	}
	return &CreateExampleSummaryEmbeddings{
		textIndex:       textIndex,
		enrichmentStore: enrichmentStore,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}, nil
}

// Execute processes the CREATE_EXAMPLE_SUMMARY_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateExampleSummaryEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateExampleSummaryEmbeddingsForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	enrichments, err := h.enrichmentStore.FindByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeExampleSummary))
	if err != nil {
		h.logger.Error("failed to get example summary enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "No example summaries to embed")
		return nil
	}

	newEnrichments, err := h.filterNewEnrichments(ctx, enrichments)
	if err != nil {
		h.logger.Error("failed to filter new enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(newEnrichments) == 0 {
		tracker.Skip(ctx, "All example summaries already have embeddings")
		return nil
	}

	tracker.SetTotal(ctx, len(newEnrichments))

	documents := make([]search.Document, 0, len(newEnrichments))
	for _, e := range newEnrichments {
		content := e.Content()
		if content != "" {
			doc := search.NewDocument(strconv.FormatInt(e.ID(), 10), content)
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		tracker.Skip(ctx, "No valid example summary documents to embed")
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.textIndex.Embedding.Index(ctx, request); err != nil {
		h.logger.Error("failed to create example summary embeddings", slog.String("error", err.Error()))
		return err
	}

	tracker.SetCurrent(ctx, len(newEnrichments), "Creating example summary embeddings")

	h.logger.Info("example summary embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}

func (h *CreateExampleSummaryEmbeddings) filterNewEnrichments(ctx context.Context, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	ids := make([]string, len(enrichments))
	for i, e := range enrichments {
		ids[i] = strconv.FormatInt(e.ID(), 10)
	}

	existingIDs, err := h.textIndex.Store.SnippetIDs(ctx, search.WithSnippetIDs(ids))
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = true
	}

	result := make([]enrichment.Enrichment, 0, len(enrichments))
	for i, e := range enrichments {
		if !existing[ids[i]] {
			result = append(result, e)
		}
	}

	return result, nil
}

// Ensure CreateExampleSummaryEmbeddings implements handler.Handler.
var _ handler.Handler = (*CreateExampleSummaryEmbeddings)(nil)
