package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	domainenrichment "github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/queue"
)

const exampleSummarySystemPrompt = `
You are a professional software developer. You will be given a code example.
Please provide a concise explanation of what this example demonstrates and how it works.
`

// ExampleSummary handles the CREATE_EXAMPLE_SUMMARY_FOR_COMMIT operation.
type ExampleSummary struct {
	enrichmentRepo  domainenrichment.EnrichmentStore
	associationRepo domainenrichment.AssociationStore
	queryService    *enrichment.QueryService
	enricher        enrichment.Enricher
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewExampleSummary creates a new ExampleSummary handler.
func NewExampleSummary(
	enrichmentRepo domainenrichment.EnrichmentStore,
	associationRepo domainenrichment.AssociationStore,
	queryService *enrichment.QueryService,
	enricher enrichment.Enricher,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *ExampleSummary {
	return &ExampleSummary{
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		enricher:        enricher,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_EXAMPLE_SUMMARY_FOR_COMMIT task.
func (h *ExampleSummary) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreateExampleSummaryForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasSummaries, err := h.queryService.Exists(ctx, &enrichment.ExistsParams{CommitSHA: commitSHA, Type: domainenrichment.TypeDevelopment, Subtype: domainenrichment.SubtypeExampleSummary})
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

	exTyp := domainenrichment.TypeDevelopment
	exSub := domainenrichment.SubtypeExample
	examples, err := h.queryService.List(ctx, &enrichment.ListParams{CommitSHA: commitSHA, Type: &exTyp, Subtype: &exSub})
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

	exampleMap := make(map[string]domainenrichment.Enrichment, len(examples))
	requests := make([]enrichment.Request, 0, len(examples))

	for _, example := range examples {
		id := fmt.Sprintf("%d", example.ID())
		exampleMap[id] = example
		requests = append(requests, enrichment.NewRequest(id, example.Content(), exampleSummarySystemPrompt))
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

		summaryEnrichment := domainenrichment.NewExampleSummary(resp.Text())
		saved, err := h.enrichmentRepo.Save(ctx, summaryEnrichment)
		if err != nil {
			return fmt.Errorf("save example summary enrichment: %w", err)
		}

		exampleAssoc := domainenrichment.NewAssociation(saved.ID(), fmt.Sprintf("%d", example.ID()), domainenrichment.EntityTypeSnippet)
		if _, err := h.associationRepo.Save(ctx, exampleAssoc); err != nil {
			return fmt.Errorf("save example association: %w", err)
		}

		commitAssoc := domainenrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure ExampleSummary implements queue.Handler.
var _ queue.Handler = (*ExampleSummary)(nil)
