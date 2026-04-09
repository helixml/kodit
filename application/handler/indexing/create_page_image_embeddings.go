package indexing

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/rasterization"
)

// imageBatchSize controls how many images are embedded per call.
const imageBatchSize = 8

// CreatePageImageEmbeddings rasterizes page-image enrichments and stores
// their vision embeddings in a dedicated vector store.
type CreatePageImageEmbeddings struct {
	repoStore        repository.RepositoryStore
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
	sourceLocStore   sourcelocation.Store
	fileStore        repository.FileStore
	rasterizers      *rasterization.Registry
	embedder         search.Embedder
	store            search.EmbeddingStore
	trackerFactory   handler.TrackerFactory
	logger           zerolog.Logger
}

// NewCreatePageImageEmbeddings creates a new CreatePageImageEmbeddings handler.
func NewCreatePageImageEmbeddings(
	repoStore repository.RepositoryStore,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	sourceLocStore sourcelocation.Store,
	fileStore repository.FileStore,
	rasterizers *rasterization.Registry,
	embedder search.Embedder,
	store search.EmbeddingStore,
	trackerFactory handler.TrackerFactory,
	logger zerolog.Logger,
) (*CreatePageImageEmbeddings, error) {
	if embedder == nil {
		return nil, fmt.Errorf("NewCreatePageImageEmbeddings: nil embedder")
	}
	if store == nil {
		return nil, fmt.Errorf("NewCreatePageImageEmbeddings: nil store")
	}
	if enrichmentStore == nil {
		return nil, fmt.Errorf("NewCreatePageImageEmbeddings: nil enrichmentStore")
	}
	if trackerFactory == nil {
		return nil, fmt.Errorf("NewCreatePageImageEmbeddings: nil trackerFactory")
	}
	return &CreatePageImageEmbeddings{
		repoStore:        repoStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		sourceLocStore:   sourceLocStore,
		fileStore:        fileStore,
		rasterizers:      rasterizers,
		embedder:         embedder,
		store:            store,
		trackerFactory:   trackerFactory,
		logger:           logger,
	}, nil
}

// Execute processes page-image enrichments for a commit: rasterize each page,
// embed the resulting image, and persist the vision embedding.
func (h *CreatePageImageEmbeddings) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		task.OperationCreatePageImageEmbeddingsForCommit,
		payload,
	)

	enrichments, err := h.enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(cp.CommitSHA()),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
		repository.WithOrderAsc("enrichments_v2.id"),
	)
	if err != nil {
		return fmt.Errorf("find page image enrichments: %w", err)
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "No page images to embed")
		return nil
	}

	enrichments, err = h.filterNew(ctx, enrichments)
	if err != nil {
		return fmt.Errorf("filter new enrichments: %w", err)
	}

	if len(enrichments) == 0 {
		tracker.Skip(ctx, "All page images already have vision embeddings")
		return nil
	}

	// Resolve source locations and file associations for all enrichments.
	ids := enrichmentIDs(enrichments)

	sourceLocations, err := h.sourceLocations(ctx, ids)
	if err != nil {
		return err
	}

	filesByEnrichment, err := h.filesByEnrichment(ctx, ids)
	if err != nil {
		return err
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", cp.RepoID())
	}

	tracker.SetTotal(ctx, len(enrichments))

	// Rasterize and embed in batches.
	var batchIDs []string
	var batchImages [][]byte

	for i, e := range enrichments {
		idStr := strconv.FormatInt(e.ID(), 10)

		sl, ok := sourceLocations[idStr]
		if !ok {
			h.logger.Warn().Str("enrichment_id", idStr).Msg("no source location for page image enrichment")
			continue
		}

		f, ok := filesByEnrichment[idStr]
		if !ok {
			h.logger.Warn().Str("enrichment_id", idStr).Msg("no file association for page image enrichment")
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Path()))
		rast, supported := h.rasterizers.For(ext)
		if !supported {
			continue
		}

		relPath := relativeFilePath(f.Path(), clonedPath)
		diskPath, safe := safeDiskPath(clonedPath, relPath)
		if !safe {
			h.logger.Warn().Str("path", f.Path()).Msg("file path escapes clone directory, skipping")
			continue
		}

		img, renderErr := rast.Render(diskPath, sl.Page())
		if renderErr != nil {
			h.logger.Warn().Str("path", f.Path()).Int("page", sl.Page()).Err(renderErr).Msg("failed to render page")
			continue
		}

		var buf bytes.Buffer
		if encErr := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); encErr != nil {
			h.logger.Warn().Str("path", f.Path()).Int("page", sl.Page()).Err(encErr).Msg("failed to encode page as JPEG")
			continue
		}

		batchIDs = append(batchIDs, idStr)
		batchImages = append(batchImages, buf.Bytes())

		if len(batchImages) >= imageBatchSize || i == len(enrichments)-1 {
			if err := h.embedAndSave(ctx, batchIDs, batchImages); err != nil {
				return err
			}
			tracker.SetCurrent(ctx, i+1, "Creating vision embeddings")
			batchIDs = batchIDs[:0]
			batchImages = batchImages[:0]
		}
	}

	h.logger.Info().Int("enrichments", len(enrichments)).Str("commit", handler.ShortSHA(cp.CommitSHA())).Msg("page image embeddings created")

	return nil
}

func (h *CreatePageImageEmbeddings) embedAndSave(ctx context.Context, ids []string, images [][]byte) error {
	if len(images) == 0 {
		return nil
	}

	vectors, err := h.embedder.Embed(ctx, images)
	if err != nil {
		return fmt.Errorf("embed page images: %w", err)
	}

	embeddings := make([]search.Embedding, len(vectors))
	for i, vec := range vectors {
		embeddings[i] = search.NewEmbedding(ids[i], vec)
	}

	if err := h.store.SaveAll(ctx, embeddings); err != nil {
		return fmt.Errorf("save vision embeddings: %w", err)
	}

	return nil
}

func (h *CreatePageImageEmbeddings) filterNew(ctx context.Context, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	ids := make([]string, len(enrichments))
	for i, e := range enrichments {
		ids[i] = strconv.FormatInt(e.ID(), 10)
	}

	found, err := h.store.Find(ctx, search.WithSnippetIDs(ids), repository.WithLimit(search.MaxSnippetIDsPerFind))
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(found))
	for _, emb := range found {
		existing[emb.SnippetID()] = true
	}

	result := make([]enrichment.Enrichment, 0, len(enrichments))
	for i, e := range enrichments {
		if !existing[ids[i]] {
			result = append(result, e)
		}
	}

	return result, nil
}

func (h *CreatePageImageEmbeddings) sourceLocations(ctx context.Context, ids []int64) (map[string]sourcelocation.SourceLocation, error) {
	locations, err := h.sourceLocStore.Find(ctx, repository.WithConditionIn("enrichment_id", ids))
	if err != nil {
		return nil, fmt.Errorf("find source locations: %w", err)
	}

	result := make(map[string]sourcelocation.SourceLocation, len(locations))
	for _, loc := range locations {
		result[strconv.FormatInt(loc.EnrichmentID(), 10)] = loc
	}

	return result, nil
}

func (h *CreatePageImageEmbeddings) filesByEnrichment(ctx context.Context, ids []int64) (map[string]repository.File, error) {
	associations, err := h.associationStore.Find(ctx,
		enrichment.WithEnrichmentIDIn(ids),
		enrichment.WithEntityType(enrichment.EntityTypeFile),
	)
	if err != nil {
		return nil, fmt.Errorf("find file associations: %w", err)
	}

	fileIDs := make([]int64, 0, len(associations))
	enrichmentToFileID := make(map[string]int64, len(associations))
	for _, a := range associations {
		fileID, parseErr := strconv.ParseInt(a.EntityID(), 10, 64)
		if parseErr != nil {
			continue
		}
		key := strconv.FormatInt(a.EnrichmentID(), 10)
		enrichmentToFileID[key] = fileID
		fileIDs = append(fileIDs, fileID)
	}

	if len(fileIDs) == 0 {
		return map[string]repository.File{}, nil
	}

	files, err := h.fileStore.Find(ctx, repository.WithIDIn(fileIDs))
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}

	filesByID := make(map[int64]repository.File, len(files))
	for _, f := range files {
		filesByID[f.ID()] = f
	}

	result := make(map[string]repository.File, len(enrichmentToFileID))
	for enrichmentID, fileID := range enrichmentToFileID {
		if f, ok := filesByID[fileID]; ok {
			result[enrichmentID] = f
		}
	}

	return result, nil
}

func enrichmentIDs(enrichments []enrichment.Enrichment) []int64 {
	ids := make([]int64, len(enrichments))
	for i, e := range enrichments {
		ids[i] = e.ID()
	}
	return ids
}
