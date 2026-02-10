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
	repoStore       repository.RepositoryStore
	fileStore       repository.FileStore
	enrichCtx       handler.EnrichmentContext
	contextGatherer CookbookContextGatherer
}

// NewCookbook creates a new Cookbook handler.
func NewCookbook(
	repoStore repository.RepositoryStore,
	fileStore repository.FileStore,
	enrichCtx handler.EnrichmentContext,
	contextGatherer CookbookContextGatherer,
) *Cookbook {
	return &Cookbook{
		repoStore:       repoStore,
		fileStore:       fileStore,
		enrichCtx:       enrichCtx,
		contextGatherer: contextGatherer,
	}
}

// Execute processes the CREATE_COOKBOOK_FOR_COMMIT task.
func (h *Cookbook) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := handler.ExtractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := handler.ExtractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateCookbookForCommit,
		task.TrackableTypeRepository,
		repoID,
	)

	hasCookbook, err := h.enrichCtx.Query.Exists(ctx, &service.EnrichmentExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeUsage, Subtype: enrichment.SubtypeCookbook})
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing cookbook", slog.String("error", err.Error()))
		return err
	}

	if hasCookbook {
		if skipErr := tracker.Skip(ctx, "Cookbook already exists for commit"); skipErr != nil {
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

	if setTotalErr := tracker.SetTotal(ctx, 4); setTotalErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Getting files for cookbook generation"); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	files, err := h.fileStore.FindByCommitSHA(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		if skipErr := tracker.Skip(ctx, "No files to generate cookbook from"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	primaryLang := determinePrimaryLanguage(files)
	if primaryLang == "" {
		if skipErr := tracker.Skip(ctx, "No supported languages found for cookbook"); skipErr != nil {
			h.enrichCtx.Logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if currentErr := tracker.SetCurrent(ctx, 2, fmt.Sprintf("Parsing %s code with AST", primaryLang)); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Gathering repository context for cookbook"); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	repoContext, err := h.contextGatherer.Gather(ctx, clonedPath, primaryLang)
	if err != nil {
		h.enrichCtx.Logger.Warn("failed to gather context", slog.String("error", err.Error()))
		repoContext = fmt.Sprintf("Repository at %s with primary language %s", clonedPath, primaryLang)
	}

	if currentErr := tracker.SetCurrent(ctx, 4, "Generating cookbook examples with LLM"); currentErr != nil {
		h.enrichCtx.Logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	taskPrompt := fmt.Sprintf(cookbookTaskPrompt, repoContext)
	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(commitSHA, taskPrompt, cookbookSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich cookbook: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", commitSHA)
	}

	cookbookEnrichment := enrichment.NewEnrichment(
		enrichment.TypeUsage,
		enrichment.SubtypeCookbook,
		enrichment.EntityTypeCommit,
		responses[0].Text(),
	)
	saved, err := h.enrichCtx.Enrichments.Save(ctx, cookbookEnrichment)
	if err != nil {
		return fmt.Errorf("save cookbook enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.enrichCtx.Logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func determinePrimaryLanguage(files []repository.File) string {
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

// Ensure Cookbook implements handler.Handler.
var _ handler.Handler = (*Cookbook)(nil)
