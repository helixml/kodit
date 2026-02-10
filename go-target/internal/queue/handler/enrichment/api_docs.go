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

// APIDocExtractor extracts API documentation from files.
type APIDocExtractor interface {
	Extract(ctx context.Context, files []git.File, language string, includePrivate bool) ([]enrichment.Enrichment, error)
}

// APIDocs handles the CREATE_PUBLIC_API_DOCS_FOR_COMMIT operation.
type APIDocs struct {
	repoRepo        git.RepoRepository
	fileRepo        git.FileRepository
	enrichmentRepo  enrichment.EnrichmentRepository
	associationRepo enrichment.AssociationRepository
	queryService    *enrichment.QueryService
	extractor       APIDocExtractor
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewAPIDocs creates a new APIDocs handler.
func NewAPIDocs(
	repoRepo git.RepoRepository,
	fileRepo git.FileRepository,
	enrichmentRepo enrichment.EnrichmentRepository,
	associationRepo enrichment.AssociationRepository,
	queryService *enrichment.QueryService,
	extractor APIDocExtractor,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *APIDocs {
	return &APIDocs{
		repoRepo:        repoRepo,
		fileRepo:        fileRepo,
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		extractor:       extractor,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_PUBLIC_API_DOCS_FOR_COMMIT task.
func (h *APIDocs) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreatePublicAPIDocsForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasAPIDocs, err := h.queryService.Exists(ctx, &enrichment.ExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeUsage, Subtype: enrichment.SubtypeAPIDocs})
	if err != nil {
		h.logger.Error("failed to check existing API docs", slog.String("error", err.Error()))
		return err
	}

	if hasAPIDocs {
		if skipErr := tracker.Skip(ctx, "API docs already exist for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	_, err = h.repoRepo.Get(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	files, err := h.fileRepo.FindByCommitSHA(ctx, commitSHA)
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		if skipErr := tracker.Skip(ctx, "No files to extract API docs from"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	langFiles := h.groupFilesByLanguage(files)

	if setTotalErr := tracker.SetTotal(ctx, len(langFiles)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	var allEnrichments []enrichment.Enrichment

	i := 0
	for lang, langFileList := range langFiles {
		if currentErr := tracker.SetCurrent(ctx, i, fmt.Sprintf("Extracting API docs for %s", lang)); currentErr != nil {
			h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		enrichments, err := h.extractor.Extract(ctx, langFileList, lang, false)
		if err != nil {
			h.logger.Warn("failed to extract API docs",
				slog.String("language", lang),
				slog.String("error", err.Error()),
			)
			i++
			continue
		}

		allEnrichments = append(allEnrichments, enrichments...)
		i++
	}

	for _, e := range allEnrichments {
		saved, err := h.enrichmentRepo.Save(ctx, e)
		if err != nil {
			return fmt.Errorf("save API docs enrichment: %w", err)
		}

		commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *APIDocs) groupFilesByLanguage(files []git.File) map[string][]git.File {
	result := make(map[string][]git.File)

	for _, f := range files {
		lang := f.Language()
		if lang != "" {
			result[lang] = append(result[lang], f)
		}
	}

	return result
}

// Ensure APIDocs implements queue.Handler.
var _ queue.Handler = (*APIDocs)(nil)
