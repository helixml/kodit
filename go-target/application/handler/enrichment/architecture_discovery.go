package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
)

const architectureSystemPrompt = `
You are an expert software architect. You will be given a description of a software system's structure.
Please provide a clear, structured explanation of the physical architecture, including:
1. Key components and services
2. How they interact
3. Any notable patterns or design decisions
`

const architectureTaskPrompt = `
Please analyze the following architecture discovery report and provide a clear summary:

<architecture_report>
%s
</architecture_report>
`

// ArchitectureDiscoverer discovers physical architecture from a repository.
type ArchitectureDiscoverer interface {
	Discover(ctx context.Context, repoPath string) (string, error)
}

// ArchitectureDiscovery handles the CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT operation.
type ArchitectureDiscovery struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	queryService     *service.EnrichmentQuery
	discoverer       ArchitectureDiscoverer
	enricher         domainservice.Enricher
	trackerFactory   handler.TrackerFactory
	logger           *slog.Logger
}

// NewArchitectureDiscovery creates a new ArchitectureDiscovery handler.
func NewArchitectureDiscovery(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	queryService *service.EnrichmentQuery,
	discoverer ArchitectureDiscoverer,
	enricher domainservice.Enricher,
	trackerFactory handler.TrackerFactory,
	logger *slog.Logger,
) *ArchitectureDiscovery {
	return &ArchitectureDiscovery{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		queryService:     queryService,
		discoverer:       discoverer,
		enricher:         enricher,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT task.
func (h *ArchitectureDiscovery) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreateArchitectureEnrichmentForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	if setTotalErr := tracker.SetTotal(ctx, 3); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	hasArchitecture, err := h.queryService.HasArchitectureForCommit(ctx, commitSHA)
	if err != nil {
		h.logger.Error("failed to check existing architecture", slog.String("error", err.Error()))
		return err
	}

	if hasArchitecture {
		if skipErr := tracker.Skip(ctx, "Architecture enrichment already exists for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	repo, err := h.repoStore.Get(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Discovering physical architecture"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	architectureNarrative, err := h.discoverer.Discover(ctx, clonedPath)
	if err != nil {
		return fmt.Errorf("discover architecture: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 2, "Enriching architecture notes with LLM"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	taskPrompt := fmt.Sprintf(architectureTaskPrompt, architectureNarrative)
	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(commitSHA, taskPrompt, architectureSystemPrompt),
	}

	responses, err := h.enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich architecture: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", commitSHA)
	}

	archEnrichment := enrichment.NewEnrichment(
		enrichment.TypeArchitecture,
		enrichment.SubtypePhysical,
		enrichment.EntityTypeCommit,
		responses[0].Text(),
	)
	saved, err := h.enrichmentStore.Save(ctx, archEnrichment)
	if err != nil {
		return fmt.Errorf("save architecture enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
	if _, err := h.associationStore.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Architecture enrichment completed"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure ArchitectureDiscovery implements handler.Handler.
var _ handler.Handler = (*ArchitectureDiscovery)(nil)
