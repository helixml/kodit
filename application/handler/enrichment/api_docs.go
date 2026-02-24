package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// APIDocExtractor extracts API documentation from files.
type APIDocExtractor interface {
	Extract(ctx context.Context, files []repository.File, language string, includePrivate bool) ([]enrichment.Enrichment, error)
}

// APIDocs handles the CREATE_PUBLIC_API_DOCS_FOR_COMMIT operation.
type APIDocs struct {
	fileStore repository.FileStore
	enrichCtx handler.EnrichmentContext
	extractor APIDocExtractor
}

// NewAPIDocs creates a new APIDocs handler.
func NewAPIDocs(
	fileStore repository.FileStore,
	enrichCtx handler.EnrichmentContext,
	extractor APIDocExtractor,
) *APIDocs {
	return &APIDocs{
		fileStore: fileStore,
		enrichCtx: enrichCtx,
		extractor: extractor,
	}
}

// Execute processes the CREATE_PUBLIC_API_DOCS_FOR_COMMIT task.
func (h *APIDocs) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreatePublicAPIDocsForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	count, err := h.enrichCtx.Enrichments.Count(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeUsage), enrichment.WithSubtype(enrichment.SubtypeAPIDocs))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing API docs", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "API docs already exist for commit")
		return nil
	}

	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		tracker.Skip(ctx, "No files to extract API docs from")
		return nil
	}

	langFiles := groupFilesByLanguage(files)

	tracker.SetTotal(ctx, len(langFiles))

	languages := make([]string, 0, len(langFiles))
	for lang := range langFiles {
		languages = append(languages, lang)
	}
	sort.Strings(languages)

	var allEnrichments []enrichment.Enrichment

	i := 0
	for _, lang := range languages {
		langFileList := langFiles[lang]
		tracker.SetCurrent(ctx, i, fmt.Sprintf("Extracting API docs for %s", lang))

		enrichments, err := h.extractor.Extract(ctx, langFileList, lang, false)
		if err != nil {
			h.enrichCtx.Logger.Warn("failed to extract API docs",
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
		saved, err := h.enrichCtx.Enrichments.Save(ctx, e)
		if err != nil {
			return fmt.Errorf("save API docs enrichment: %w", err)
		}

		commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
		if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	return nil
}

func groupFilesByLanguage(files []repository.File) map[string][]repository.File {
	result := make(map[string][]repository.File)

	for _, f := range files {
		lang := f.Language()
		if lang != "" {
			result[lang] = append(result[lang], f)
		}
	}

	return result
}

// Ensure APIDocs implements handler.Handler.
var _ handler.Handler = (*APIDocs)(nil)
