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
) (*Cookbook, error) {
	if repoStore == nil {
		return nil, fmt.Errorf("NewCookbook: nil repoStore")
	}
	if fileStore == nil {
		return nil, fmt.Errorf("NewCookbook: nil fileStore")
	}
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewCookbook: nil Enricher")
	}
	if contextGatherer == nil {
		return nil, fmt.Errorf("NewCookbook: nil contextGatherer")
	}
	return &Cookbook{
		repoStore:       repoStore,
		fileStore:       fileStore,
		enrichCtx:       enrichCtx,
		contextGatherer: contextGatherer,
	}, nil
}

// Execute processes the CREATE_COOKBOOK_FOR_COMMIT task.
func (h *Cookbook) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateCookbookForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	count, err := h.enrichCtx.Enrichments.CountByCommitSHA(ctx, cp.CommitSHA(), enrichment.WithType(enrichment.TypeUsage), enrichment.WithSubtype(enrichment.SubtypeCookbook))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing cookbook", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "Cookbook already exists for commit")
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

	tracker.SetTotal(ctx, 4)
	tracker.SetCurrent(ctx, 1, "Getting files for cookbook generation")

	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		tracker.Skip(ctx, "No files to generate cookbook from")
		return nil
	}

	primaryLang := determinePrimaryLanguage(files)
	if primaryLang == "" {
		tracker.Skip(ctx, "No supported languages found for cookbook")
		return nil
	}

	tracker.SetCurrent(ctx, 2, fmt.Sprintf("Parsing %s code with AST", primaryLang))
	tracker.SetCurrent(ctx, 3, "Gathering repository context for cookbook")

	repoContext, err := h.contextGatherer.Gather(ctx, clonedPath, primaryLang)
	if err != nil {
		h.enrichCtx.Logger.Warn("failed to gather context", slog.String("error", err.Error()))
		repoContext = fmt.Sprintf("Repository at %s with primary language %s", clonedPath, primaryLang)
	}

	tracker.SetCurrent(ctx, 4, "Generating cookbook examples with LLM")

	taskPrompt := fmt.Sprintf(cookbookTaskPrompt, repoContext)
	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(cp.CommitSHA(), taskPrompt, cookbookSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich cookbook: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", cp.CommitSHA())
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

	commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
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
