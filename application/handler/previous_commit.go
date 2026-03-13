package handler

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

// Cleaner removes data from old commits after new data has been created.
type Cleaner interface {
	Clean(ctx context.Context, repoID int64, currentSHA string) error
}

// EnrichmentCleanup removes enrichments of a specific type and subtype from
// old commits. Each handler gets its own instance configured at construction.
type EnrichmentCleanup struct {
	enrichments *service.Enrichment
	commits     repository.CommitStore
	typ         enrichment.Type
	subtype     enrichment.Subtype
}

// NewEnrichmentCleanup creates an EnrichmentCleanup.
func NewEnrichmentCleanup(
	enrichments *service.Enrichment,
	commits repository.CommitStore,
	typ enrichment.Type,
	subtype enrichment.Subtype,
) *EnrichmentCleanup {
	return &EnrichmentCleanup{
		enrichments: enrichments,
		commits:     commits,
		typ:         typ,
		subtype:     subtype,
	}
}

// Clean removes enrichments of the configured type/subtype from previous
// commits for the repository. No-op when no old commits exist.
func (c *EnrichmentCleanup) Clean(ctx context.Context, repoID int64, currentSHA string) error {
	shas, err := oldCommitSHAs(ctx, c.commits, repoID, currentSHA)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		return nil
	}

	existing, err := c.enrichments.Find(ctx,
		enrichment.WithCommitSHAs(shas),
		enrichment.WithType(c.typ),
		enrichment.WithSubtype(c.subtype),
	)
	if err != nil {
		return fmt.Errorf("find old enrichments: %w", err)
	}
	if len(existing) == 0 {
		return nil
	}

	ids := make([]int64, len(existing))
	for i, e := range existing {
		ids[i] = e.ID()
	}

	if err := c.enrichments.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
		return fmt.Errorf("delete old enrichments: %w", err)
	}

	return nil
}

// FileCleanup removes files from old commits. Used by the scan handler
// via the WithCleanup decorator.
type FileCleanup struct {
	commits repository.CommitStore
	files   repository.FileStore
}

// NewFileCleanup creates a FileCleanup.
func NewFileCleanup(commits repository.CommitStore, files repository.FileStore) *FileCleanup {
	return &FileCleanup{
		commits: commits,
		files:   files,
	}
}

// Clean removes files associated with previous commits for the repository.
// No-op when no old commits exist.
func (c *FileCleanup) Clean(ctx context.Context, repoID int64, currentSHA string) error {
	shas, err := oldCommitSHAs(ctx, c.commits, repoID, currentSHA)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		return nil
	}

	if err := c.files.DeleteBy(ctx, repository.WithCommitSHAIn(shas)); err != nil {
		return fmt.Errorf("delete old files: %w", err)
	}

	return nil
}

// WithCleanup wraps a handler with automatic cleanup of old data after the
// inner handler completes successfully.
func WithCleanup(inner Handler, cleanup Cleaner) Handler {
	return &cleanupHandler{
		inner:   inner,
		cleanup: cleanup,
	}
}

// cleanupHandler decorates a Handler to run a Cleaner after the inner handler
// succeeds.
type cleanupHandler struct {
	inner   Handler
	cleanup Cleaner
}

// Execute runs the inner handler, then cleans up old data for the same
// repository.
func (h *cleanupHandler) Execute(ctx context.Context, payload map[string]any) error {
	if err := h.inner.Execute(ctx, payload); err != nil {
		return err
	}

	cp, err := ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	return h.cleanup.Clean(ctx, cp.RepoID(), cp.CommitSHA())
}

// oldCommitSHAs returns SHAs of all commits for the repo except the current one.
func oldCommitSHAs(ctx context.Context, commits repository.CommitStore, repoID int64, currentSHA string) ([]string, error) {
	all, err := commits.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		return nil, fmt.Errorf("find commits: %w", err)
	}

	var shas []string
	for _, c := range all {
		if c.SHA() != currentSHA {
			shas = append(shas, c.SHA())
		}
	}
	return shas, nil
}
