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

// CreateExampleCodeEmbeddings creates vector embeddings for extracted example code.
type CreateExampleCodeEmbeddings struct {
	codeIndex       handler.VectorIndex
	enrichmentStore enrichment.EnrichmentStore
	trackerFactory  handler.TrackerFactory
	logger          zerolog.Logger
}

// NewCreateExampleCodeEmbeddings creates a new CreateExampleCodeEmbeddings handler.
func NewCreateExampleCodeEmbeddings(
	codeIndex handler.VectorIndex,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) (*CreateExampleCodeEmbeddings, error) {
	if codeIndex.Embedding == nil {
		return nil, fmt.Errorf("NewCreateExampleCodeEmbeddings: nil Embedding")
	}
	if codeIndex.Store == nil {
		return nil, fmt.Errorf("NewCreateExampleCodeEmbeddings: nil Store")
	}
	if enrichmentStore == nil {
		return nil, fmt.Errorf("NewCreateExampleCodeEmbeddings: nil enrichmentStore")
	}
	if trackerFactory == nil {
		return nil, fmt.Errorf("NewCreateExampleCodeEmbeddings: nil trackerFactory")
	}
	return &CreateExampleCodeEmbeddings{
		codeIndex:       codeIndex,
		enrichmentStore: enrichmentStore,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}, nil
}

// Execute processes the CREATE_EXAMPLE_CODE_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateExampleCodeEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateExampleCodeEmbeddingsForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	examples, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeExample), repository.WithOrderAsc("enrichments_v2.id"))
	if err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to get example enrichments")
		return err
	}

	if len(examples) == 0 {
		tracker.Skip(ctx, "No example code to embed")
		return nil
	}

	newExamples, err := h.filterNewExamples(ctx, examples)
	if err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to filter new examples")
		return err
	}

	if len(newExamples) == 0 {
		tracker.Skip(ctx, "All examples already have code embeddings")
		return nil
	}

	documents := make([]search.Document, 0, len(newExamples))
	for _, e := range newExamples {
		content := e.Content()
		if content != "" {
			doc := search.NewDocument(strconv.FormatInt(e.ID(), 10), content)
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		tracker.Skip(ctx, "No valid example documents to embed")
		return nil
	}

	tracker.SetTotal(ctx, len(documents))

	request := search.NewIndexRequest(documents)
	if err := h.codeIndex.Embedding.Index(ctx, request,
		search.WithProgress(func(completed, total int) {
			tracker.SetCurrent(ctx, completed, "Creating example code embeddings")
		}),
		search.WithBatchError(func(batchStart, batchEnd int, err error) {
			h.logger.Error().Str("operation", "create_example_code_embeddings").Int("batch_start", batchStart).Int("batch_end", batchEnd).Str("error", err.Error()).Msg("embedding batch failed")
		}),
	); err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to create example code embeddings")
		return err
	}

	h.logger.Info().Int("documents", len(documents)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("example code embeddings created")

	return nil
}

func (h *CreateExampleCodeEmbeddings) filterNewExamples(ctx context.Context, examples []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	ids := make([]string, len(examples))
	for i, e := range examples {
		ids[i] = strconv.FormatInt(e.ID(), 10)
	}

	found, err := h.codeIndex.Store.Find(ctx, search.WithSnippetIDs(ids))
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(found))
	for _, emb := range found {
		existing[emb.SnippetID()] = true
	}

	result := make([]enrichment.Enrichment, 0, len(examples))
	for i, e := range examples {
		if !existing[ids[i]] {
			result = append(result, e)
		}
	}

	return result, nil
}

// Ensure CreateExampleCodeEmbeddings implements handler.Handler.
var _ handler.Handler = (*CreateExampleCodeEmbeddings)(nil)
