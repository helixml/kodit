package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
)

const summarizationSystemPrompt = `
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
`

// CreateSummary handles the CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT operation.
type CreateSummary struct {
	enrichCtx handler.EnrichmentContext
}

// NewCreateSummary creates a new CreateSummary handler.
func NewCreateSummary(
	enrichCtx handler.EnrichmentContext,
) (*CreateSummary, error) {
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewCreateSummary: nil Enricher")
	}
	return &CreateSummary{
		enrichCtx: enrichCtx,
	}, nil
}

// Execute processes the CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT task.
func (h *CreateSummary) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateSummaryEnrichmentForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	hasSummaries, err := h.enrichCtx.Query.Exists(ctx, &service.EnrichmentExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeDevelopment, Subtype: enrichment.SubtypeSnippetSummary})
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing summaries", slog.String("error", err.Error()))
		return err
	}

	if hasSummaries {
		if skipErr := tracker.Skip(ctx, "Summary enrichments already exist for commit"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	typDev := enrichment.TypeDevelopment
	subSnippet := enrichment.SubtypeSnippet
	snippetEnrichments, err := h.enrichCtx.Query.List(ctx, &service.EnrichmentListParams{CommitSHA: commitSHA, Type: &typDev, Subtype: &subSnippet})
	if err != nil {
		return fmt.Errorf("get snippet enrichments: %w", err)
	}

	if len(snippetEnrichments) == 0 {
		if skipErr := tracker.Skip(ctx, "No snippets to enrich"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(snippetEnrichments)); setTotalErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	enrichmentMap := make(map[string]enrichment.Enrichment, len(snippetEnrichments))
	requests := make([]domainservice.EnrichmentRequest, 0, len(snippetEnrichments))

	for _, e := range snippetEnrichments {
		id := strconv.FormatInt(e.ID(), 10)
		enrichmentMap[id] = e
		requests = append(requests, domainservice.NewEnrichmentRequest(id, e.Content(), summarizationSystemPrompt))
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich snippets: %w", err)
	}

	for i, resp := range responses {
		if currentErr := tracker.SetCurrent(ctx, i, "Enriching snippets for commit"); currentErr != nil {
			h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		snippetEnrichment, ok := enrichmentMap[resp.ID()]
		if !ok {
			continue
		}

		summaryEnrichment := enrichment.NewEnrichment(
			enrichment.TypeDevelopment,
			enrichment.SubtypeSnippetSummary,
			enrichment.EntityTypeCommit,
			resp.Text(),
		)
		saved, err := h.enrichCtx.Enrichments.Save(ctx, summaryEnrichment)
		if err != nil {
			return fmt.Errorf("save summary enrichment: %w", err)
		}

		snippetAssoc := enrichment.SnippetAssociation(saved.ID(), strconv.FormatInt(snippetEnrichment.ID(), 10))
		if _, err := h.enrichCtx.Associations.Save(ctx, snippetAssoc); err != nil {
			return fmt.Errorf("save snippet association: %w", err)
		}

		commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.enrichCtx.Logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure CreateSummary implements handler.Handler.
var _ handler.Handler = (*CreateSummary)(nil)
