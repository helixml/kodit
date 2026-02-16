package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
)

const exampleSummarySystemPrompt = `
You are a professional software developer. You will be given a code example.
Please provide a concise explanation of what this example demonstrates and how it works.
`

// ExampleSummary handles the CREATE_EXAMPLE_SUMMARY_FOR_COMMIT operation.
type ExampleSummary struct {
	enrichCtx handler.EnrichmentContext
}

// NewExampleSummary creates a new ExampleSummary handler.
func NewExampleSummary(
	enrichCtx handler.EnrichmentContext,
) (*ExampleSummary, error) {
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewExampleSummary: nil Enricher")
	}
	return &ExampleSummary{
		enrichCtx: enrichCtx,
	}, nil
}

// Execute processes the CREATE_EXAMPLE_SUMMARY_FOR_COMMIT task.
func (h *ExampleSummary) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateExampleSummaryForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	hasSummaries, err := h.enrichCtx.Query.Exists(ctx, &service.EnrichmentExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeDevelopment, Subtype: enrichment.SubtypeExampleSummary})
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing example summaries", slog.String("error", err.Error()))
		return err
	}

	if hasSummaries {
		if skipErr := tracker.Skip(ctx, "Example summaries already exist for commit"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeExample
	examples, err := h.enrichCtx.Query.List(ctx, &service.EnrichmentListParams{CommitSHA: commitSHA, Type: &typ, Subtype: &sub})
	if err != nil {
		return fmt.Errorf("get examples: %w", err)
	}

	if len(examples) == 0 {
		if skipErr := tracker.Skip(ctx, "No examples to enrich"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(examples)); setTotalErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	exampleMap := make(map[string]enrichment.Enrichment, len(examples))
	requests := make([]domainservice.EnrichmentRequest, 0, len(examples))

	for _, example := range examples {
		id := fmt.Sprintf("%d", example.ID())
		exampleMap[id] = example
		requests = append(requests, domainservice.NewEnrichmentRequest(id, example.Content(), exampleSummarySystemPrompt))
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich examples: %w", err)
	}

	for i, resp := range responses {
		if currentErr := tracker.SetCurrent(ctx, i, "Enriching examples for commit"); currentErr != nil {
			h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		example, ok := exampleMap[resp.ID()]
		if !ok {
			continue
		}

		summaryEnrichment := enrichment.NewExampleSummary(resp.Text())
		saved, err := h.enrichCtx.Enrichments.Save(ctx, summaryEnrichment)
		if err != nil {
			return fmt.Errorf("save example summary enrichment: %w", err)
		}

		exampleAssoc := enrichment.NewAssociation(saved.ID(), fmt.Sprintf("%d", example.ID()), enrichment.EntityTypeSnippet)
		if _, err := h.enrichCtx.Associations.Save(ctx, exampleAssoc); err != nil {
			return fmt.Errorf("save example association: %w", err)
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

// Ensure ExampleSummary implements handler.Handler.
var _ handler.Handler = (*ExampleSummary)(nil)
