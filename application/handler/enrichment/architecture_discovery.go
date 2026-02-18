package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/handler"
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
	repoStore  repository.RepositoryStore
	enrichCtx  handler.EnrichmentContext
	discoverer ArchitectureDiscoverer
}

// NewArchitectureDiscovery creates a new ArchitectureDiscovery handler.
func NewArchitectureDiscovery(
	repoStore repository.RepositoryStore,
	enrichCtx handler.EnrichmentContext,
	discoverer ArchitectureDiscoverer,
) (*ArchitectureDiscovery, error) {
	if repoStore == nil {
		return nil, fmt.Errorf("NewArchitectureDiscovery: nil repoStore")
	}
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewArchitectureDiscovery: nil Enricher")
	}
	if discoverer == nil {
		return nil, fmt.Errorf("NewArchitectureDiscovery: nil discoverer")
	}
	return &ArchitectureDiscovery{
		repoStore:  repoStore,
		enrichCtx:  enrichCtx,
		discoverer: discoverer,
	}, nil
}

// Execute processes the CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT task.
func (h *ArchitectureDiscovery) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateArchitectureEnrichmentForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	tracker.SetTotal(ctx, 3)

	count, err := h.enrichCtx.Enrichments.Count(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeArchitecture), enrichment.WithSubtype(enrichment.SubtypePhysical))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing architecture", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "Architecture enrichment already exists for commit")
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", cp.RepoID())
	}

	tracker.SetCurrent(ctx, 1, "Discovering physical architecture")

	architectureNarrative, err := h.discoverer.Discover(ctx, clonedPath)
	if err != nil {
		return fmt.Errorf("discover architecture: %w", err)
	}

	tracker.SetCurrent(ctx, 2, "Enriching architecture notes with LLM")

	taskPrompt := fmt.Sprintf(architectureTaskPrompt, architectureNarrative)
	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(cp.CommitSHA(), taskPrompt, architectureSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich architecture: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", cp.CommitSHA())
	}

	archEnrichment := enrichment.NewEnrichment(
		enrichment.TypeArchitecture,
		enrichment.SubtypePhysical,
		enrichment.EntityTypeCommit,
		responses[0].Text(),
	)
	saved, err := h.enrichCtx.Enrichments.Save(ctx, archEnrichment)
	if err != nil {
		return fmt.Errorf("save architecture enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	tracker.SetCurrent(ctx, 3, "Architecture enrichment completed")

	return nil
}

// Ensure ArchitectureDiscovery implements handler.Handler.
var _ handler.Handler = (*ArchitectureDiscovery)(nil)
