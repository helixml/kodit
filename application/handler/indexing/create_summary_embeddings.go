package indexing

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/task"
)

// CreateSummaryEmbeddings creates vector embeddings for snippet summary enrichments.
type CreateSummaryEmbeddings struct {
	textIndex        handler.VectorIndex
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewCreateSummaryEmbeddings creates a new CreateSummaryEmbeddings handler.
func NewCreateSummaryEmbeddings(
	textIndex handler.VectorIndex,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) (*CreateSummaryEmbeddings, error) {
	if textIndex.Embedding == nil {
		return nil, fmt.Errorf("NewCreateSummaryEmbeddings: nil Embedding")
	}
	if textIndex.Store == nil {
		return nil, fmt.Errorf("NewCreateSummaryEmbeddings: nil Store")
	}
	if enrichmentStore == nil {
		return nil, fmt.Errorf("NewCreateSummaryEmbeddings: nil enrichmentStore")
	}
	if associationStore == nil {
		return nil, fmt.Errorf("NewCreateSummaryEmbeddings: nil associationStore")
	}
	if trackerFactory == nil {
		return nil, fmt.Errorf("NewCreateSummaryEmbeddings: nil trackerFactory")
	}
	return &CreateSummaryEmbeddings{
		textIndex:        textIndex,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}, nil
}

// Execute processes the CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateSummaryEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateSummaryEmbeddingsForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	enrichments, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippetSummary))
	if err != nil {
		h.logger.Error("failed to get summary enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "No summary enrichments to embed")
		return nil
	}

	newEnrichments, err := h.filterNewEnrichments(ctx, enrichments)
	if err != nil {
		h.logger.Error("failed to filter new enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(newEnrichments) == 0 {
		tracker.Skip(ctx, "All summary enrichments already have embeddings")
		return nil
	}

	documents := make([]search.Document, 0, len(newEnrichments))
	for _, e := range newEnrichments {
		content := e.Content()
		if content == "" {
			continue
		}

		// Find the snippet SHA associated with this enrichment
		snippetSHA, err := h.findSnippetSHA(ctx, e.ID())
		if err != nil {
			h.logger.Warn("failed to find snippet SHA for enrichment", slog.Int64("enrichment_id", e.ID()), slog.String("error", err.Error()))
			continue
		}
		if snippetSHA == "" {
			h.logger.Warn("no snippet association found for enrichment", slog.Int64("enrichment_id", e.ID()))
			continue
		}

		doc := search.NewDocument(snippetSHA, content)
		documents = append(documents, doc)
	}

	if len(documents) == 0 {
		tracker.Skip(ctx, "No valid documents to embed")
		return nil
	}

	tracker.SetTotal(ctx, len(documents))

	request := search.NewIndexRequest(documents)
	if err := h.textIndex.Embedding.Index(ctx, request, search.WithProgress(func(completed, total int) {
		tracker.SetCurrent(ctx, completed, "Creating summary embeddings")
	})); err != nil {
		h.logger.Error("failed to create summary embeddings", slog.String("error", err.Error()))
		return err
	}

	h.logger.Info("summary embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}

func (h *CreateSummaryEmbeddings) filterNewEnrichments(ctx context.Context, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	// Collect snippet SHAs for all enrichments
	snippetSHAs := make([]string, 0, len(enrichments))
	shaToEnrichment := make(map[string][]enrichment.Enrichment, len(enrichments))

	for _, e := range enrichments {
		snippetSHA, err := h.findSnippetSHA(ctx, e.ID())
		if err != nil {
			return nil, err
		}
		if snippetSHA == "" {
			continue
		}
		snippetSHAs = append(snippetSHAs, snippetSHA)
		shaToEnrichment[snippetSHA] = append(shaToEnrichment[snippetSHA], e)
	}

	if len(snippetSHAs) == 0 {
		return nil, nil
	}

	found, err := h.textIndex.Store.Find(ctx, search.WithSnippetIDs(snippetSHAs))
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(found))
	for _, emb := range found {
		existing[emb.SnippetID()] = true
	}

	result := make([]enrichment.Enrichment, 0, len(enrichments))
	for sha, items := range shaToEnrichment {
		if !existing[sha] {
			result = append(result, items...)
		}
	}

	return result, nil
}

// findSnippetSHA finds the snippet SHA associated with an enrichment.
func (h *CreateSummaryEmbeddings) findSnippetSHA(ctx context.Context, enrichmentID int64) (string, error) {
	associations, err := h.associationStore.Find(ctx, enrichment.WithEnrichmentID(enrichmentID))
	if err != nil {
		return "", err
	}

	for _, assoc := range associations {
		if assoc.EntityType() == enrichment.EntityTypeSnippet {
			return assoc.EntityID(), nil
		}
	}

	return "", nil
}

// Ensure CreateSummaryEmbeddings implements handler.Handler.
var _ handler.Handler = (*CreateSummaryEmbeddings)(nil)
