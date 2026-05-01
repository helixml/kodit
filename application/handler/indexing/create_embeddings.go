package indexing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/task"
)

// CreateCodeEmbeddings creates vector embeddings for commit enrichments.
type CreateCodeEmbeddings struct {
	codeIndex       handler.VectorIndex
	enrichmentStore enrichment.EnrichmentStore
	subtype         enrichment.Subtype
	trackerFactory  handler.TrackerFactory
	logger          zerolog.Logger
}

// NewCreateCodeEmbeddings creates a new CreateCodeEmbeddings handler.
// The subtype parameter controls which enrichments to embed (e.g. SubtypeSnippet or SubtypeChunk).
func NewCreateCodeEmbeddings(
	codeIndex handler.VectorIndex,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
	subtype enrichment.Subtype,
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
		subtype:         subtype,
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
		payload,
	)

	enrichments, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(h.subtype), repository.WithOrderAsc("enrichments_v2.id"))
	if err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to get snippet enrichments for commit")
		return err
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "No snippets to create embeddings for")
		return nil
	}

	newEnrichments, err := filterNewEnrichments(ctx, func(ctx context.Context, ids []string) (map[string]struct{}, error) {
		return search.ExistingSnippetIDs(ctx, h.codeIndex.Store, ids)
	}, enrichments)
	if err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to filter new enrichments")
		return err
	}

	if len(newEnrichments) == 0 {
		tracker.Skip(ctx, "All snippets already have code embeddings")
		return nil
	}

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

	tracker.SetTotal(ctx, len(documents))

	if err := h.codeIndex.Embedding.Index(ctx, documents,
		search.WithProgress(func(completed, total int) {
			tracker.SetCurrent(ctx, completed, "Creating code embeddings")
		}),
		search.WithBatchError(func(batchStart, batchEnd int, err error) {
			h.logger.Error().Str("operation", "create_code_embeddings").Int("batch_start", batchStart).Int("batch_end", batchEnd).Str("error", err.Error()).Msg("embedding batch failed")
		}),
	); err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to create embeddings")
		return err
	}

	h.logger.Info().Int("documents", len(documents)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("code embeddings created")

	return nil
}
