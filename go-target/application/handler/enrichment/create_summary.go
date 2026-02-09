package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

const summarizationSystemPrompt = `
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
`

// CreateSummary handles the CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT operation.
type CreateSummary struct {
	snippetStore snippet.SnippetStore
	enrichCtx    handler.EnrichmentContext
}

// NewCreateSummary creates a new CreateSummary handler.
func NewCreateSummary(
	snippetStore snippet.SnippetStore,
	enrichCtx handler.EnrichmentContext,
) *CreateSummary {
	return &CreateSummary{
		snippetStore: snippetStore,
		enrichCtx:    enrichCtx,
	}
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

	hasSummaries, err := h.enrichCtx.Query.HasSummariesForCommit(ctx, commitSHA)
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

	snippets, err := h.snippetStore.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get snippets: %w", err)
	}

	if len(snippets) == 0 {
		if skipErr := tracker.Skip(ctx, "No snippets to enrich"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(snippets)); setTotalErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	snippetMap := make(map[string]snippet.Snippet, len(snippets))
	requests := make([]domainservice.EnrichmentRequest, 0, len(snippets))

	for _, s := range snippets {
		id := s.SHA()
		snippetMap[id] = s
		requests = append(requests, domainservice.NewEnrichmentRequest(id, s.Content(), summarizationSystemPrompt))
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich snippets: %w", err)
	}

	for i, resp := range responses {
		if currentErr := tracker.SetCurrent(ctx, i, "Enriching snippets for commit"); currentErr != nil {
			h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		s, ok := snippetMap[resp.ID()]
		if !ok {
			continue
		}

		summaryEnrichment := enrichment.NewEnrichment(
			enrichment.TypeDevelopment,
			enrichment.SubtypeSnippetSummary,
			enrichment.EntityTypeSnippet,
			resp.Text(),
		)
		saved, err := h.enrichCtx.Enrichments.Save(ctx, summaryEnrichment)
		if err != nil {
			return fmt.Errorf("save summary enrichment: %w", err)
		}

		snippetAssoc := enrichment.SnippetAssociation(saved.ID(), s.SHA())
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
