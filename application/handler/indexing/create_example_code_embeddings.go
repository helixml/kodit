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

// CreateExampleCodeEmbeddings creates vector embeddings for extracted example code.
type CreateExampleCodeEmbeddings struct {
	codeIndex       handler.VectorIndex
	enrichmentStore enrichment.EnrichmentStore
	trackerFactory  handler.TrackerFactory
	logger          *slog.Logger
}

// NewCreateExampleCodeEmbeddings creates a new CreateExampleCodeEmbeddings handler.
func NewCreateExampleCodeEmbeddings(
	codeIndex handler.VectorIndex,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
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

	examples, err := h.enrichmentStore.FindByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeExample))
	if err != nil {
		h.logger.Error("failed to get example enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(examples) == 0 {
		tracker.Skip(ctx, "No example code to embed")
		return nil
	}

	newExamples, err := h.filterNewExamples(ctx, examples)
	if err != nil {
		h.logger.Error("failed to filter new examples", slog.String("error", err.Error()))
		return err
	}

	if len(newExamples) == 0 {
		tracker.Skip(ctx, "All examples already have code embeddings")
		return nil
	}

	tracker.SetTotal(ctx, len(newExamples))

	documents := make([]search.Document, 0, len(newExamples))
	for _, e := range newExamples {
		content := e.Content()
		if content != "" {
			doc := search.NewDocument(enrichmentDocID(e.ID()), content)
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		tracker.Skip(ctx, "No valid example documents to embed")
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.codeIndex.Embedding.Index(ctx, request); err != nil {
		h.logger.Error("failed to create example code embeddings", slog.String("error", err.Error()))
		tracker.Fail(ctx, err.Error())
		return err
	}

	tracker.SetCurrent(ctx, len(newExamples), "Creating example code embeddings")

	h.logger.Info("example code embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(cp.CommitSHA())),
	)

	return nil
}

func (h *CreateExampleCodeEmbeddings) filterNewExamples(ctx context.Context, examples []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	ids := make([]string, len(examples))
	for i, e := range examples {
		ids[i] = enrichmentDocID(e.ID())
	}

	existing, err := h.codeIndex.Store.HasEmbeddings(ctx, ids, search.EmbeddingTypeCode)
	if err != nil {
		return nil, err
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
