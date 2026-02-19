package enrichment

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// EnrichmentStore defines operations for persisting and retrieving enrichments.
// Commit SHA filtering is supported via WithCommitSHA / WithCommitSHAs options
// passed to Find and Count.
type EnrichmentStore interface {
	repository.Store[Enrichment]
	DeleteBy(ctx context.Context, options ...repository.Option) error
}

// AssociationStore defines operations for persisting and retrieving enrichment associations.
type AssociationStore interface {
	repository.Store[Association]
	DeleteBy(ctx context.Context, options ...repository.Option) error
}
