package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
)

const summarizationSystemPrompt = `
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
`

// CreateSummary handles the CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT operation.
type CreateSummary struct {
	snippetRepo     indexing.SnippetRepository
	enrichmentRepo  enrichment.EnrichmentRepository
	associationRepo enrichment.AssociationRepository
	queryService    *enrichment.QueryService
	enricher        enrichment.Enricher
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewCreateSummary creates a new CreateSummary handler.
func NewCreateSummary(
	snippetRepo indexing.SnippetRepository,
	enrichmentRepo enrichment.EnrichmentRepository,
	associationRepo enrichment.AssociationRepository,
	queryService *enrichment.QueryService,
	enricher enrichment.Enricher,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *CreateSummary {
	return &CreateSummary{
		snippetRepo:     snippetRepo,
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		enricher:        enricher,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT task.
func (h *CreateSummary) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreateSummaryEnrichmentForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasSummaries, err := h.queryService.HasSummariesForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to check existing summaries", slog.String("error", err.Error()))
		return err
	}

	if hasSummaries {
		if skipErr := tracker.Skip(ctx, "Summary enrichments already exist for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	snippets, err := h.snippetRepo.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get snippets: %w", err)
	}

	if len(snippets) == 0 {
		if skipErr := tracker.Skip(ctx, "No snippets to enrich"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(snippets)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	snippetMap := make(map[string]indexing.Snippet, len(snippets))
	requests := make([]enrichment.Request, 0, len(snippets))

	for _, snippet := range snippets {
		id := snippet.SHA()
		snippetMap[id] = snippet
		requests = append(requests, enrichment.NewRequest(id, snippet.Content(), summarizationSystemPrompt))
	}

	responses, err := h.enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich snippets: %w", err)
	}

	for i, resp := range responses {
		if currentErr := tracker.SetCurrent(ctx, i, "Enriching snippets for commit"); currentErr != nil {
			h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		snippet, ok := snippetMap[resp.ID()]
		if !ok {
			continue
		}

		summaryEnrichment := enrichment.NewSnippetSummary(resp.Text())
		saved, err := h.enrichmentRepo.Save(ctx, summaryEnrichment)
		if err != nil {
			return fmt.Errorf("save summary enrichment: %w", err)
		}

		snippetAssoc := enrichment.SnippetAssociation(saved.ID(), snippet.SHA())
		if _, err := h.associationRepo.Save(ctx, snippetAssoc); err != nil {
			return fmt.Errorf("save snippet association: %w", err)
		}

		commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure CreateSummary implements queue.Handler.
var _ queue.Handler = (*CreateSummary)(nil)
