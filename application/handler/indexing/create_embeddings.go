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

// CreateCodeEmbeddings creates vector embeddings for commit snippets.
type CreateCodeEmbeddings struct {
	codeIndex       handler.VectorIndex
	enrichmentStore enrichment.EnrichmentStore
	trackerFactory  handler.TrackerFactory
	logger          *slog.Logger
}

// NewCreateCodeEmbeddings creates a new CreateCodeEmbeddings handler.
func NewCreateCodeEmbeddings(
	codeIndex handler.VectorIndex,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) (*CreateCodeEmbeddings, error) {
	if codeIndex.Embedding == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil Embedding")
	}
	if codeIndex.Store == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil Store")
	}
	if enrichmentStore == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil enrichmentStore")
	}
	if trackerFactory == nil {
		return nil, fmt.Errorf("NewCreateCodeEmbeddings: nil trackerFactory")
	}
	return &CreateCodeEmbeddings{
		codeIndex:       codeIndex,
		enrichmentStore: enrichmentStore,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}, nil
}

// Execute processes the CREATE_CODE_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateCodeEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateCodeEmbeddingsForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	enrichments, err := h.enrichmentStore.FindByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
	if err != nil {
		h.logger.Error("failed to get snippet enrichments for commit", slog.String("error", err.Error()))
		return err
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "No snippets to create embeddings for")
		return nil
	}

	newEnrichments, err := h.filterNew(ctx, enrichments)
	if err != nil {
		h.logger.Error("failed to filter new enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(newEnrichments) == 0 {
		tracker.Skip(ctx, "All snippets already have code embeddings")
		return nil
	}

	tracker.SetTotal(ctx, len(newEnrichments))

	documents := make([]search.Document, 0, len(newEnrichments))
	for _, e := range newEnrichments {
		if e.Content() != "" {
			doc := search.NewDocument(strconv.FormatInt(e.ID(), 10), e.Content())
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		tracker.Skip(ctx, "No valid documents to embed")
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.codeIndex.Embedding.Index(ctx, request); err != nil {
		h.logger.Error("failed to create embeddings", slog.String("error", err.Error()))
		tracker.Fail(ctx, err.Error())
		return err
	}

	tracker.SetCurrent(ctx, len(newEnrichments), "Creating code embeddings for commit")

	h.logger.Info("code embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}

func (h *CreateCodeEmbeddings) filterNew(ctx context.Context, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	ids := make([]string, len(enrichments))
	for i, e := range enrichments {
		ids[i] = strconv.FormatInt(e.ID(), 10)
	}

	existing, err := h.codeIndex.Store.HasEmbeddings(ctx, ids, search.EmbeddingTypeCode)
	if err != nil {
		return nil, err
	}

	result := make([]enrichment.Enrichment, 0, len(enrichments))
	for i, e := range enrichments {
		if !existing[ids[i]] {
			result = append(result, e)
		}
	}

	return result, nil
}
