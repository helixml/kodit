package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
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
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateExampleSummaryForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	count, err := h.enrichCtx.Enrichments.Count(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeExampleSummary))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing example summaries", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "Example summaries already exist for commit")
		return nil
	}

	examples, err := h.enrichCtx.Enrichments.Find(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeExample))
	if err != nil {
		return fmt.Errorf("get examples: %w", err)
	}

	if len(examples) == 0 {
		tracker.Skip(ctx, "No examples to enrich")
		return nil
	}

	tracker.SetTotal(ctx, len(examples))

	exampleMap := make(map[string]enrichment.Enrichment, len(examples))
	requests := make([]domainservice.EnrichmentRequest, 0, len(examples))

	for _, example := range examples {
		id := fmt.Sprintf("%d", example.ID())
		exampleMap[id] = example
		requests = append(requests, domainservice.NewEnrichmentRequest(id, example.Content(), exampleSummarySystemPrompt))
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests,
		domainservice.WithEnrichProgress(func(completed, total int) {
			tracker.SetCurrent(ctx, completed, "Enriching examples for commit")
		}),
		domainservice.WithRequestError(func(requestID string, err error) {
			h.enrichCtx.Logger.Error("enrichment request failed",
				slog.String("request_id", requestID),
				slog.String("error", err.Error()),
			)
		}),
	)
	if err != nil {
		return fmt.Errorf("enrich examples: %w", err)
	}

	for _, resp := range responses {

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

		commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
		if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	return nil
}

// Ensure ExampleSummary implements handler.Handler.
var _ handler.Handler = (*ExampleSummary)(nil)
