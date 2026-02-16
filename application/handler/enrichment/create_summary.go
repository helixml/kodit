package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/helixml/kodit/application/handler"
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
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateSummaryEnrichmentForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	count, err := h.enrichCtx.Enrichments.CountByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippetSummary))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing summaries", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "Summary enrichments already exist for commit")
		return nil
	}

	snippetEnrichments, err := h.enrichCtx.Enrichments.FindByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
	if err != nil {
		return fmt.Errorf("get snippet enrichments: %w", err)
	}

	if len(snippetEnrichments) == 0 {
		tracker.Skip(ctx, "No snippets to enrich")
		return nil
	}

	tracker.SetTotal(ctx, len(snippetEnrichments))

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
		tracker.SetCurrent(ctx, i, "Enriching snippets for commit")

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

		commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
		if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	return nil
}

// Ensure CreateSummary implements handler.Handler.
var _ handler.Handler = (*CreateSummary)(nil)
