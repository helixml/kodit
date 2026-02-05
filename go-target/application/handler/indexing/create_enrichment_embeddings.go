package indexing

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// CreateSummaryEmbeddings creates vector embeddings for snippet summary enrichments.
type CreateSummaryEmbeddings struct {
	embeddingService domainservice.Embedding
	queryService     *service.EnrichmentQuery
	associationStore enrichment.AssociationStore
	vectorStore      search.VectorStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewCreateSummaryEmbeddings creates a new CreateSummaryEmbeddings handler.
func NewCreateSummaryEmbeddings(
	embeddingService domainservice.Embedding,
	queryService *service.EnrichmentQuery,
	associationStore enrichment.AssociationStore,
	vectorStore search.VectorStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *CreateSummaryEmbeddings {
	return &CreateSummaryEmbeddings{
		embeddingService: embeddingService,
		queryService:     queryService,
		associationStore: associationStore,
		vectorStore:      vectorStore,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateSummaryEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateSummaryEmbeddingsForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeSnippetSummary
	enrichments, err := h.queryService.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		h.logger.Error("failed to get summary enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(enrichments) == 0 {
		if skipErr := tracker.Skip(ctx, "No summary enrichments to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	newEnrichments, err := h.filterNewEnrichments(ctx, enrichments)
	if err != nil {
		h.logger.Error("failed to filter new enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(newEnrichments) == 0 {
		if skipErr := tracker.Skip(ctx, "All summary enrichments already have embeddings"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(newEnrichments)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
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
		if skipErr := tracker.Skip(ctx, "No valid documents to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.embeddingService.Index(ctx, request); err != nil {
		h.logger.Error("failed to create summary embeddings", slog.String("error", err.Error()))
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return err
	}

	if currentErr := tracker.SetCurrent(ctx, len(newEnrichments), "Creating summary embeddings"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	h.logger.Info("summary embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	return nil
}

func (h *CreateSummaryEmbeddings) filterNewEnrichments(ctx context.Context, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	result := make([]enrichment.Enrichment, 0, len(enrichments))

	for _, e := range enrichments {
		// Find the snippet SHA associated with this enrichment
		snippetSHA, err := h.findSnippetSHA(ctx, e.ID())
		if err != nil {
			return nil, err
		}
		if snippetSHA == "" {
			// No snippet association, skip
			continue
		}

		hasEmbedding, err := h.vectorStore.HasEmbedding(ctx, snippetSHA, snippet.EmbeddingTypeSummary)
		if err != nil {
			return nil, err
		}

		if !hasEmbedding {
			result = append(result, e)
		}
	}

	return result, nil
}

// findSnippetSHA finds the snippet SHA associated with an enrichment.
func (h *CreateSummaryEmbeddings) findSnippetSHA(ctx context.Context, enrichmentID int64) (string, error) {
	associations, err := h.associationStore.FindByEnrichmentID(ctx, enrichmentID)
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

// CreateExampleCodeEmbeddings creates vector embeddings for extracted example code.
type CreateExampleCodeEmbeddings struct {
	embeddingService domainservice.Embedding
	queryService     *service.EnrichmentQuery
	vectorStore      search.VectorStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewCreateExampleCodeEmbeddings creates a new CreateExampleCodeEmbeddings handler.
func NewCreateExampleCodeEmbeddings(
	embeddingService domainservice.Embedding,
	queryService *service.EnrichmentQuery,
	vectorStore search.VectorStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *CreateExampleCodeEmbeddings {
	return &CreateExampleCodeEmbeddings{
		embeddingService: embeddingService,
		queryService:     queryService,
		vectorStore:      vectorStore,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the CREATE_EXAMPLE_CODE_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateExampleCodeEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateExampleCodeEmbeddingsForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	examples, err := h.queryService.ExamplesForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to get example enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(examples) == 0 {
		if skipErr := tracker.Skip(ctx, "No example code to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	newExamples, err := h.filterNewExamples(ctx, examples)
	if err != nil {
		h.logger.Error("failed to filter new examples", slog.String("error", err.Error()))
		return err
	}

	if len(newExamples) == 0 {
		if skipErr := tracker.Skip(ctx, "All examples already have code embeddings"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(newExamples)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	documents := make([]search.Document, 0, len(newExamples))
	for _, e := range newExamples {
		content := e.Content()
		if content != "" {
			doc := search.NewDocument(enrichmentDocID(e.ID()), content)
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		if skipErr := tracker.Skip(ctx, "No valid example documents to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.embeddingService.Index(ctx, request); err != nil {
		h.logger.Error("failed to create example code embeddings", slog.String("error", err.Error()))
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return err
	}

	if currentErr := tracker.SetCurrent(ctx, len(newExamples), "Creating example code embeddings"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	h.logger.Info("example code embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	return nil
}

func (h *CreateExampleCodeEmbeddings) filterNewExamples(ctx context.Context, examples []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	result := make([]enrichment.Enrichment, 0, len(examples))

	for _, e := range examples {
		hasEmbedding, err := h.vectorStore.HasEmbedding(ctx, enrichmentDocID(e.ID()), snippet.EmbeddingTypeCode)
		if err != nil {
			return nil, err
		}

		if !hasEmbedding {
			result = append(result, e)
		}
	}

	return result, nil
}

// Ensure CreateExampleCodeEmbeddings implements handler.Handler.
var _ handler.Handler = (*CreateExampleCodeEmbeddings)(nil)

// CreateExampleSummaryEmbeddings creates vector embeddings for example summary enrichments.
type CreateExampleSummaryEmbeddings struct {
	embeddingService domainservice.Embedding
	queryService     *service.EnrichmentQuery
	vectorStore      search.VectorStore
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewCreateExampleSummaryEmbeddings creates a new CreateExampleSummaryEmbeddings handler.
func NewCreateExampleSummaryEmbeddings(
	embeddingService domainservice.Embedding,
	queryService *service.EnrichmentQuery,
	vectorStore search.VectorStore,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *CreateExampleSummaryEmbeddings {
	return &CreateExampleSummaryEmbeddings{
		embeddingService: embeddingService,
		queryService:     queryService,
		vectorStore:      vectorStore,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the CREATE_EXAMPLE_SUMMARY_EMBEDDINGS_FOR_COMMIT task.
func (h *CreateExampleSummaryEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateExampleSummaryEmbeddingsForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeExampleSummary
	enrichments, err := h.queryService.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		h.logger.Error("failed to get example summary enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(enrichments) == 0 {
		if skipErr := tracker.Skip(ctx, "No example summaries to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	newEnrichments, err := h.filterNewEnrichments(ctx, enrichments)
	if err != nil {
		h.logger.Error("failed to filter new enrichments", slog.String("error", err.Error()))
		return err
	}

	if len(newEnrichments) == 0 {
		if skipErr := tracker.Skip(ctx, "All example summaries already have embeddings"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(newEnrichments)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	documents := make([]search.Document, 0, len(newEnrichments))
	for _, e := range newEnrichments {
		content := e.Content()
		if content != "" {
			doc := search.NewDocument(enrichmentDocID(e.ID()), content)
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		if skipErr := tracker.Skip(ctx, "No valid example summary documents to embed"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.embeddingService.Index(ctx, request); err != nil {
		h.logger.Error("failed to create example summary embeddings", slog.String("error", err.Error()))
		if failErr := tracker.Fail(ctx, err.Error()); failErr != nil {
			h.logger.Warn("failed to mark tracker as failed", slog.String("error", failErr.Error()))
		}
		return err
	}

	if currentErr := tracker.SetCurrent(ctx, len(newEnrichments), "Creating example summary embeddings"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	h.logger.Info("example summary embeddings created",
		slog.Int("documents", len(documents)),
		slog.String("commit", handler.ShortSHA(commitSHA)),
	)

	return nil
}

func (h *CreateExampleSummaryEmbeddings) filterNewEnrichments(ctx context.Context, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	result := make([]enrichment.Enrichment, 0, len(enrichments))

	for _, e := range enrichments {
		hasEmbedding, err := h.vectorStore.HasEmbedding(ctx, enrichmentDocID(e.ID()), snippet.EmbeddingTypeSummary)
		if err != nil {
			return nil, err
		}

		if !hasEmbedding {
			result = append(result, e)
		}
	}

	return result, nil
}

// Ensure CreateExampleSummaryEmbeddings implements handler.Handler.
var _ handler.Handler = (*CreateExampleSummaryEmbeddings)(nil)

// enrichmentDocID converts an enrichment ID to a document ID string.
// This ensures enrichment embeddings use a consistent ID format that can be
// differentiated from snippet SHA IDs.
func enrichmentDocID(enrichmentID int64) string {
	return fmt.Sprintf("enrichment:%d", enrichmentID)
}

// ParseEnrichmentDocID extracts the enrichment ID from a document ID string.
// Returns 0 and false if the document ID is not an enrichment ID.
func ParseEnrichmentDocID(docID string) (int64, bool) {
	var id int64
	if _, err := fmt.Sscanf(docID, "enrichment:%d", &id); err != nil {
		return 0, false
	}
	return id, true
}

// IsEnrichmentDocID checks if a document ID is an enrichment document ID.
func IsEnrichmentDocID(docID string) bool {
	if len(docID) < 11 {
		return false
	}
	return docID[:11] == "enrichment:"
}

