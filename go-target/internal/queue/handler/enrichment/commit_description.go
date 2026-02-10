package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	domainenrichment "github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/queue"
)

const commitDescriptionSystemPrompt = `
You are a professional software developer. You will be given a git commit diff.
Please provide a concise description of what changes were made and why.
`

// CommitDescription handles the CREATE_COMMIT_DESCRIPTION_FOR_COMMIT operation.
type CommitDescription struct {
	repoRepo        repository.RepositoryStore
	enrichmentRepo  domainenrichment.EnrichmentStore
	associationRepo domainenrichment.AssociationStore
	queryService    *enrichment.QueryService
	adapter         git.Adapter
	enricher        enrichment.Enricher
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewCommitDescription creates a new CommitDescription handler.
func NewCommitDescription(
	repoRepo repository.RepositoryStore,
	enrichmentRepo domainenrichment.EnrichmentStore,
	associationRepo domainenrichment.AssociationStore,
	queryService *enrichment.QueryService,
	adapter git.Adapter,
	enricher enrichment.Enricher,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *CommitDescription {
	return &CommitDescription{
		repoRepo:        repoRepo,
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		adapter:         adapter,
		enricher:        enricher,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_COMMIT_DESCRIPTION_FOR_COMMIT task.
func (h *CommitDescription) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreateCommitDescriptionForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasDescription, err := h.queryService.Exists(ctx, &enrichment.ExistsParams{CommitSHA: commitSHA, Type: domainenrichment.TypeHistory, Subtype: domainenrichment.SubtypeCommitDescription})
	if err != nil {
		h.logger.Error("failed to check existing commit description", slog.String("error", err.Error()))
		return err
	}

	if hasDescription {
		if skipErr := tracker.Skip(ctx, "Commit description already exists for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	repo, err := h.repoRepo.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	if setTotalErr := tracker.SetTotal(ctx, 3); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Getting commit diff"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	diff, err := h.adapter.CommitDiff(ctx, clonedPath, commitSHA)
	if err != nil {
		return fmt.Errorf("get commit diff: %w", err)
	}

	if diff == "" {
		if skipErr := tracker.Skip(ctx, "No diff found for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if currentErr := tracker.SetCurrent(ctx, 2, "Enriching commit description with LLM"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	requests := []enrichment.Request{
		enrichment.NewRequest(commitSHA, TruncateDiff(diff, MaxDiffLength), commitDescriptionSystemPrompt),
	}

	responses, err := h.enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich commit description: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", commitSHA)
	}

	descEnrichment := domainenrichment.NewCommitDescription(responses[0].Text())
	saved, err := h.enrichmentRepo.Save(ctx, descEnrichment)
	if err != nil {
		return fmt.Errorf("save commit description enrichment: %w", err)
	}

	commitAssoc := domainenrichment.CommitAssociation(saved.ID(), commitSHA)
	if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Commit description enrichment completed"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure CommitDescription implements queue.Handler.
var _ queue.Handler = (*CommitDescription)(nil)
