package handler

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

// PreviousCommit deletes data from old commits after new data has been created.
// Each handler uses this to clean up its own data type once the replacement
// data for the current commit is safely persisted.
type PreviousCommit struct {
	enrichments *service.Enrichment
	commits     repository.CommitStore
	files       repository.FileStore
}

// NewPreviousCommit creates a PreviousCommit.
func NewPreviousCommit(
	enrichments *service.Enrichment,
	commits repository.CommitStore,
	files repository.FileStore,
) *PreviousCommit {
	return &PreviousCommit{
		enrichments: enrichments,
		commits:     commits,
		files:       files,
	}
}

// DeleteEnrichments removes enrichments of the given type and subtype from
// previous commits for the repository. No-op when no old commits exist.
//
// Uses a two-step approach (Find then DeleteBy with IDs) because the
// enrichment store's DeleteBy does not support the commit SHA JOIN that
// Find uses.
func (p *PreviousCommit) DeleteEnrichments(
	ctx context.Context,
	repoID int64,
	currentSHA string,
	typ enrichment.Type,
	subtype enrichment.Subtype,
) error {
	shas, err := p.oldCommitSHAs(ctx, repoID, currentSHA)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		return nil
	}

	existing, err := p.enrichments.Find(ctx,
		enrichment.WithCommitSHAs(shas),
		enrichment.WithType(typ),
		enrichment.WithSubtype(subtype),
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

	if err := p.enrichments.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
		return fmt.Errorf("delete old enrichments: %w", err)
	}

	return nil
}

// DeleteFiles removes files associated with previous commits for the
// repository. No-op when no old commits exist.
func (p *PreviousCommit) DeleteFiles(ctx context.Context, repoID int64, currentSHA string) error {
	shas, err := p.oldCommitSHAs(ctx, repoID, currentSHA)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		return nil
	}

	if err := p.files.DeleteBy(ctx, repository.WithCommitSHAIn(shas)); err != nil {
		return fmt.Errorf("delete old files: %w", err)
	}

	return nil
}

// Handler wraps an inner handler with automatic deletion of old enrichments
// after the inner handler completes successfully.
func (p *PreviousCommit) Handler(typ enrichment.Type, subtype enrichment.Subtype, inner Handler) Handler {
	return &previousCommitHandler{
		inner:         inner,
		previous:      p,
		enrichType:    typ,
		enrichSubtype: subtype,
	}
}

// previousCommitHandler decorates a Handler to delete old enrichments after
// the inner handler succeeds.
type previousCommitHandler struct {
	inner         Handler
	previous      *PreviousCommit
	enrichType    enrichment.Type
	enrichSubtype enrichment.Subtype
}

// Execute runs the inner handler, then deletes old enrichments of the
// configured type/subtype for the same repository.
func (h *previousCommitHandler) Execute(ctx context.Context, payload map[string]any) error {
	if err := h.inner.Execute(ctx, payload); err != nil {
		return err
	}

	cp, err := ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	return h.previous.DeleteEnrichments(ctx, cp.RepoID(), cp.CommitSHA(), h.enrichType, h.enrichSubtype)
}

// oldCommitSHAs returns SHAs of all commits for the repo except the current one.
func (p *PreviousCommit) oldCommitSHAs(ctx context.Context, repoID int64, currentSHA string) ([]string, error) {
	commits, err := p.commits.Find(ctx, repository.WithRepoID(repoID))
	if err != nil {
		return nil, fmt.Errorf("find commits: %w", err)
	}

	var shas []string
	for _, c := range commits {
		if c.SHA() != currentSHA {
			shas = append(shas, c.SHA())
		}
	}
	return shas, nil
}
