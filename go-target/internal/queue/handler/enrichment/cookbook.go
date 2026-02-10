package enrichment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/queue"
)

const cookbookSystemPrompt = `
You are a technical writer who creates excellent code cookbook documentation.
You will be given information about a codebase and should produce practical,
task-oriented examples that show how to accomplish common tasks.
`

const cookbookTaskPrompt = `
Based on the following repository context, create a cookbook with practical examples:

<repository_context>
%s
</repository_context>

Provide 3-5 practical cookbook examples showing common use cases.
Each example should include:
1. A descriptive title
2. When to use this pattern
3. Complete, working code example
4. Brief explanation of key points
`

// CookbookContextGatherer gathers context for cookbook generation.
type CookbookContextGatherer interface {
	Gather(ctx context.Context, repoPath, language string) (string, error)
}

// Cookbook handles the CREATE_COOKBOOK_FOR_COMMIT operation.
type Cookbook struct {
	repoRepo        git.RepoRepository
	fileRepo        git.FileRepository
	enrichmentRepo  enrichment.EnrichmentRepository
	associationRepo enrichment.AssociationRepository
	queryService    *enrichment.QueryService
	contextGatherer CookbookContextGatherer
	enricher        enrichment.Enricher
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewCookbook creates a new Cookbook handler.
func NewCookbook(
	repoRepo git.RepoRepository,
	fileRepo git.FileRepository,
	enrichmentRepo enrichment.EnrichmentRepository,
	associationRepo enrichment.AssociationRepository,
	queryService *enrichment.QueryService,
	contextGatherer CookbookContextGatherer,
	enricher enrichment.Enricher,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *Cookbook {
	return &Cookbook{
		repoRepo:        repoRepo,
		fileRepo:        fileRepo,
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		contextGatherer: contextGatherer,
		enricher:        enricher,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_COOKBOOK_FOR_COMMIT task.
func (h *Cookbook) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreateCookbookForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasCookbook, err := h.queryService.Exists(ctx, &enrichment.ExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeUsage, Subtype: enrichment.SubtypeCookbook})
	if err != nil {
		h.logger.Error("failed to check existing cookbook", slog.String("error", err.Error()))
		return err
	}

	if hasCookbook {
		if skipErr := tracker.Skip(ctx, "Cookbook already exists for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	repo, err := h.repoRepo.Get(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	if setTotalErr := tracker.SetTotal(ctx, 4); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Getting files for cookbook generation"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	files, err := h.fileRepo.FindByCommitSHA(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		if skipErr := tracker.Skip(ctx, "No files to generate cookbook from"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	primaryLang := h.determinePrimaryLanguage(files)
	if primaryLang == "" {
		if skipErr := tracker.Skip(ctx, "No supported languages found for cookbook"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if currentErr := tracker.SetCurrent(ctx, 2, fmt.Sprintf("Parsing %s code with AST", primaryLang)); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Gathering repository context for cookbook"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	repoContext, err := h.contextGatherer.Gather(ctx, clonedPath, primaryLang)
	if err != nil {
		h.logger.Warn("failed to gather context", slog.String("error", err.Error()))
		repoContext = fmt.Sprintf("Repository at %s with primary language %s", clonedPath, primaryLang)
	}

	if currentErr := tracker.SetCurrent(ctx, 4, "Generating cookbook examples with LLM"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	taskPrompt := fmt.Sprintf(cookbookTaskPrompt, repoContext)
	requests := []enrichment.Request{
		enrichment.NewRequest(commitSHA, taskPrompt, cookbookSystemPrompt),
	}

	responses, err := h.enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich cookbook: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", commitSHA)
	}

	cookbookEnrichment := enrichment.NewCookbook(responses[0].Text())
	saved, err := h.enrichmentRepo.Save(ctx, cookbookEnrichment)
	if err != nil {
		return fmt.Errorf("save cookbook enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
	if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *Cookbook) determinePrimaryLanguage(files []git.File) string {
	langCounts := make(map[string]int)

	for _, f := range files {
		lang := f.Language()
		if lang != "" {
			langCounts[lang]++
		}
	}

	var primaryLang string
	var maxCount int
	for lang, count := range langCounts {
		if count > maxCount {
			maxCount = count
			primaryLang = lang
		}
	}

	return primaryLang
}

// Ensure Cookbook implements queue.Handler.
var _ queue.Handler = (*Cookbook)(nil)
