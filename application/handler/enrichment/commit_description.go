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
	infraGit "github.com/helixml/kodit/infrastructure/git"
)

const commitDescriptionSystemPrompt = `
You are a professional software developer. You will be given a git commit diff.
Please provide a concise description of what changes were made and why.
`

// CommitDescription handles the CREATE_COMMIT_DESCRIPTION_FOR_COMMIT operation.
type CommitDescription struct {
	repoStore repository.RepositoryStore
	enrichCtx handler.EnrichmentContext
	adapter   infraGit.Adapter
}

// NewCommitDescription creates a new CommitDescription handler.
func NewCommitDescription(
	repoStore repository.RepositoryStore,
	enrichCtx handler.EnrichmentContext,
	adapter infraGit.Adapter,
) (*CommitDescription, error) {
	if repoStore == nil {
		return nil, fmt.Errorf("NewCommitDescription: nil repoStore")
	}
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewCommitDescription: nil Enricher")
	}
	if adapter == nil {
		return nil, fmt.Errorf("NewCommitDescription: nil adapter")
	}
	return &CommitDescription{
		repoStore: repoStore,
		enrichCtx: enrichCtx,
		adapter:   adapter,
	}, nil
}

// Execute processes the CREATE_COMMIT_DESCRIPTION_FOR_COMMIT task.
func (h *CommitDescription) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateCommitDescriptionForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	count, err := h.enrichCtx.Enrichments.CountByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeHistory), enrichment.WithSubtype(enrichment.SubtypeCommitDescription))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing commit description", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "Commit description already exists for commit")
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

	tracker.SetTotal(ctx, 3)
	tracker.SetCurrent(ctx, 1, "Getting commit diff")

	diff, err := h.adapter.CommitDiff(ctx, clonedPath, cp.CommitSHA())
	if err != nil {
		return fmt.Errorf("get commit diff: %w", err)
	}

	if diff == "" {
		tracker.Skip(ctx, "No diff found for commit")
		return nil
	}

	tracker.SetCurrent(ctx, 2, "Enriching commit description with LLM")

	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(cp.CommitSHA(), TruncateDiff(diff, MaxDiffLength), commitDescriptionSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich commit description: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", cp.CommitSHA())
	}

	descEnrichment := enrichment.NewEnrichment(
		enrichment.TypeHistory,
		enrichment.SubtypeCommitDescription,
		enrichment.EntityTypeCommit,
		responses[0].Text(),
	)
	saved, err := h.enrichCtx.Enrichments.Save(ctx, descEnrichment)
	if err != nil {
		return fmt.Errorf("save commit description enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	tracker.SetCurrent(ctx, 3, "Commit description enrichment completed")

	return nil
}

// Ensure CommitDescription implements handler.Handler.
var _ handler.Handler = (*CommitDescription)(nil)
