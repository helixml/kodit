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
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	queryService     *service.EnrichmentQuery
	enricher         domainservice.Enricher
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewExampleSummary creates a new ExampleSummary handler.
func NewExampleSummary(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	queryService *service.EnrichmentQuery,
	enricher domainservice.Enricher,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *ExampleSummary {
	return &ExampleSummary{
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		queryService:     queryService,
		enricher:         enricher,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
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

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateExampleSummaryForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	hasSummaries, err := h.queryService.HasExampleSummariesForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to check existing example summaries", slog.String("error", err.Error()))
		return err
	}

	if hasSummaries {
		if skipErr := tracker.Skip(ctx, "Example summaries already exist for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	examples, err := h.queryService.ExamplesForCommit(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get examples: %w", err)
	}

	if len(examples) == 0 {
		if skipErr := tracker.Skip(ctx, "No examples to enrich"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if setTotalErr := tracker.SetTotal(ctx, len(examples)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	exampleMap := make(map[string]enrichment.Enrichment, len(examples))
	requests := make([]domainservice.EnrichmentRequest, 0, len(examples))

	for _, example := range examples {
		id := fmt.Sprintf("%d", example.ID())
		exampleMap[id] = example
		requests = append(requests, domainservice.NewEnrichmentRequest(id, example.Content(), exampleSummarySystemPrompt))
	}

	responses, err := h.enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich examples: %w", err)
	}

	for i, resp := range responses {
		if currentErr := tracker.SetCurrent(ctx, i, "Enriching examples for commit"); currentErr != nil {
			h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		example, ok := exampleMap[resp.ID()]
		if !ok {
			continue
		}

		summaryEnrichment := enrichment.NewEnrichment(
			enrichment.TypeDevelopment,
			enrichment.SubtypeExampleSummary,
			enrichment.EntityTypeSnippet,
			resp.Text(),
		)
		saved, err := h.enrichmentStore.Save(ctx, summaryEnrichment)
		if err != nil {
			return fmt.Errorf("save example summary enrichment: %w", err)
		}

		exampleAssoc := enrichment.NewAssociation(saved.ID(), fmt.Sprintf("%d", example.ID()), enrichment.EntityTypeSnippet)
		if _, err := h.associationStore.Save(ctx, exampleAssoc); err != nil {
			return fmt.Errorf("save example association: %w", err)
		}

		commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.associationStore.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure ExampleSummary implements handler.Handler.
var _ handler.Handler = (*ExampleSummary)(nil)
