package indexing

import (
	"context"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
)

// CreateBM25Index creates BM25 keyword index for commit enrichments.
type CreateBM25Index struct {
	bm25Service     *domainservice.BM25
	enrichmentStore enrichment.EnrichmentStore
	subtype         enrichment.Subtype
	trackerFactory  handler.TrackerFactory
	logger          zerolog.Logger
}

// NewCreateBM25Index creates a new CreateBM25Index handler.
// The subtype parameter controls which enrichments to index (e.g. SubtypeSnippet or SubtypeChunk).
func NewCreateBM25Index(
	bm25Service *domainservice.BM25,
	enrichmentStore enrichment.EnrichmentStore,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
	subtype enrichment.Subtype,
) *CreateBM25Index {
	return &CreateBM25Index{
		bm25Service:     bm25Service,
		enrichmentStore: enrichmentStore,
		subtype:         subtype,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_BM25_INDEX_FOR_COMMIT task.
func (h *CreateBM25Index) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateBM25IndexForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	enrichments, err := h.enrichmentStore.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(h.subtype))
	if err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to get snippet enrichments for commit")
		return err
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "No snippets to index")
		return nil
	}

	tracker.SetTotal(ctx, len(enrichments))

	documents := make([]search.Document, 0, len(enrichments))
	for _, e := range enrichments {
		if e.Content() != "" {
			doc := search.NewDocument(strconv.FormatInt(e.ID(), 10), e.Content())
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		tracker.Skip(ctx, "No valid documents to index")
		return nil
	}

	request := search.NewIndexRequest(documents)
	if err := h.bm25Service.Index(ctx, request); err != nil {
		h.logger.Error().Str("error", err.Error()).Msg("failed to index documents")
		return err
	}

	tracker.SetCurrent(ctx, len(enrichments), "BM25 index created for commit")

	h.logger.Info().Int("documents", len(documents)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("BM25 index created")

	return nil
}
