package kodit

import (
	"context"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
)

// Enrichments provides enrichment query and mutation operations.
type Enrichments interface {
	// Get retrieves a specific enrichment by ID.
	Get(ctx context.Context, id int64) (enrichment.Enrichment, error)

	// List returns enrichments matching the given filter.
	List(ctx context.Context, filter enrichment.Filter) ([]enrichment.Enrichment, error)

	// Update replaces the content of an enrichment and returns the saved result.
	Update(ctx context.Context, id int64, content string) (enrichment.Enrichment, error)

	// Delete removes an enrichment by ID.
	Delete(ctx context.Context, id int64) error

	// ForCommit returns enrichments for a specific commit.
	ForCommit(ctx context.Context, commitSHA string, filter enrichment.Filter) ([]enrichment.Enrichment, error)

	// ForCommits returns enrichments across multiple commits.
	ForCommits(ctx context.Context, commitSHAs []string, filter enrichment.Filter) ([]enrichment.Enrichment, error)

	// DeleteForCommit removes all enrichments and associations for a commit.
	DeleteForCommit(ctx context.Context, commitSHA string) error
}

// enrichmentsImpl implements Enrichments as a thin forwarder to EnrichmentQuery.
type enrichmentsImpl struct {
	query *service.EnrichmentQuery
}

func (e *enrichmentsImpl) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	return e.query.Get(ctx, id)
}

func (e *enrichmentsImpl) List(ctx context.Context, filter enrichment.Filter) ([]enrichment.Enrichment, error) {
	return e.query.List(ctx, filter)
}

func (e *enrichmentsImpl) Update(ctx context.Context, id int64, content string) (enrichment.Enrichment, error) {
	return e.query.Update(ctx, id, content)
}

func (e *enrichmentsImpl) Delete(ctx context.Context, id int64) error {
	return e.query.Delete(ctx, id)
}

func (e *enrichmentsImpl) ForCommit(ctx context.Context, commitSHA string, filter enrichment.Filter) ([]enrichment.Enrichment, error) {
	return e.query.EnrichmentsForCommit(ctx, commitSHA, filter.FirstType(), filter.FirstSubtype())
}

func (e *enrichmentsImpl) ForCommits(ctx context.Context, commitSHAs []string, filter enrichment.Filter) ([]enrichment.Enrichment, error) {
	return e.query.EnrichmentsForCommits(ctx, commitSHAs, filter.FirstType(), filter.Limit())
}

func (e *enrichmentsImpl) DeleteForCommit(ctx context.Context, commitSHA string) error {
	return e.query.DeleteForCommit(ctx, commitSHA)
}
