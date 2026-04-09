package indexing

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/rasterization"
)

// ExtractPageImages creates one enrichment per renderable page of each
// document file that the rasterization registry supports.
type ExtractPageImages struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	sourceLocStore   sourcelocation.Store
	fileStore        repository.FileStore
	rasterizers      *rasterization.Registry
	trackerFactory   handler.TrackerFactory
	logger           zerolog.Logger
}

// NewExtractPageImages creates a new ExtractPageImages handler.
// When rasterizers is nil, Execute returns immediately (no documents to rasterize).
func NewExtractPageImages(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	sourceLocStore sourcelocation.Store,
	fileStore repository.FileStore,
	rasterizers *rasterization.Registry,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) *ExtractPageImages {
	return &ExtractPageImages{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		sourceLocStore:   sourceLocStore,
		fileStore:        fileStore,
		rasterizers:      rasterizers,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}
}

// Execute processes the EXTRACT_PAGE_IMAGES_FOR_COMMIT task.
func (h *ExtractPageImages) Execute(ctx context.Context, payload map[string]any) error {
	if h.rasterizers == nil {
		return nil
	}

	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationExtractPageImagesForCommit,
		payload,
	)

	existing, err := h.enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(cp.CommitSHA()),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	if err != nil {
		return fmt.Errorf("check existing page images: %w", err)
	}

	if len(existing) > 0 {
		tracker.Skip(ctx, "Page images already created for commit")
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

	files, err := h.fileStore.Find(ctx,
		repository.WithCommitSHA(cp.CommitSHA()),
		repository.WithOrderAsc("path"),
	)
	if err != nil {
		return fmt.Errorf("get commit files: %w", err)
	}

	if len(files) == 0 {
		tracker.Skip(ctx, "No files found for commit")
		return nil
	}

	tracker.SetTotal(ctx, len(files))
	repoIDStr := strconv.FormatInt(cp.RepoID(), 10)

	processed := 0
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Path()))
		if !h.rasterizers.Supports(ext) {
			processed++
			continue
		}

		tracker.SetCurrent(ctx, processed, fmt.Sprintf("Extracting pages from %s", f.Path()))

		relPath := relativeFilePath(f.Path(), clonedPath)
		diskPath, safe := safeDiskPath(clonedPath, relPath)
		if !safe {
			h.logger.Warn().Str("path", f.Path()).Msg("file path escapes clone directory, skipping")
			processed++
			continue
		}

		rast, _ := h.rasterizers.For(ext)
		pageCount, countErr := rast.PageCount(diskPath)
		if countErr != nil {
			h.logger.Warn().Str("path", f.Path()).Str("error", countErr.Error()).Msg("failed to count pages")
			processed++
			continue
		}

		if err := h.persistPages(ctx, pageCount, f, cp.CommitSHA(), repoIDStr); err != nil {
			return err
		}

		processed++
	}

	h.logger.Info().Int("files", len(files)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("page images created")

	return nil
}

// persistPages saves enrichments, source locations, and associations for each page.
func (h *ExtractPageImages) persistPages(ctx context.Context, pageCount int, f repository.File, commitSHA string, repoIDStr string) error {
	for page := 1; page <= pageCount; page++ {
		e := enrichment.NewPageImage()
		saved, saveErr := h.enrichmentStore.Save(ctx, e)
		if saveErr != nil {
			return fmt.Errorf("save page image enrichment: %w", saveErr)
		}

		sl := sourcelocation.NewPage(saved.ID(), page)
		if _, err := h.sourceLocStore.Save(ctx, sl); err != nil {
			return fmt.Errorf("save page source location: %w", err)
		}

		if _, err := h.associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA)); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}

		if f.ID() != 0 {
			if _, err := h.associationStore.Save(ctx, enrichment.FileAssociation(saved.ID(), strconv.FormatInt(f.ID(), 10))); err != nil {
				return fmt.Errorf("save file association: %w", err)
			}
		}

		if _, err := h.associationStore.Save(ctx, enrichment.RepositoryAssociation(saved.ID(), repoIDStr)); err != nil {
			return fmt.Errorf("save repository association: %w", err)
		}
	}
	return nil
}
