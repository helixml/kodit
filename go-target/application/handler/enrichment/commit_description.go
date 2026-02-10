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
) *CommitDescription {
	return &CommitDescription{
		repoStore: repoStore,
		enrichCtx: enrichCtx,
		adapter:   adapter,
	}
}

// Execute processes the CREATE_COMMIT_DESCRIPTION_FOR_COMMIT task.
func (h *CommitDescription) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateCommitDescriptionForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	hasDescription, err := h.enrichCtx.Query.Exists(ctx, &service.EnrichmentExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeHistory, Subtype: enrichment.SubtypeCommitDescription})
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing commit description", slog.String("error", err.Error()))
		return err
	}

	if hasDescription {
		if skipErr := tracker.Skip(ctx, "Commit description already exists for commit"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
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

	if setTotalErr := tracker.SetTotal(ctx, 3); setTotalErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Getting commit diff"); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	diff, err := h.adapter.CommitDiff(ctx, clonedPath, commitSHA)
	if err != nil {
		return fmt.Errorf("get commit diff: %w", err)
	}

	if diff == "" {
		if skipErr := tracker.Skip(ctx, "No diff found for commit"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if currentErr := tracker.SetCurrent(ctx, 2, "Enriching commit description with LLM"); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(commitSHA, TruncateDiff(diff, MaxDiffLength), commitDescriptionSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich commit description: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", commitSHA)
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

	commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Commit description enrichment completed"); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.enrichCtx.Logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure CommitDescription implements handler.Handler.
var _ handler.Handler = (*CommitDescription)(nil)
