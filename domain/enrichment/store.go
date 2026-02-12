package enrichment

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// EnrichmentStore defines operations for persisting and retrieving enrichments.
type EnrichmentStore interface {
	repository.Store[Enrichment]
	DeleteBy(ctx context.Context, options ...repository.Option) error

	// FindByEntityKey returns all enrichments for a specific entity type (requires JOIN).
	FindByEntityKey(ctx context.Context, key EntityTypeKey) ([]Enrichment, error)

	// FindByCommitSHA returns enrichments associated with a commit.
	FindByCommitSHA(ctx context.Context, commitSHA string, options ...repository.Option) ([]Enrichment, error)

	// CountByCommitSHA returns the count of enrichments for a commit.
	CountByCommitSHA(ctx context.Context, commitSHA string, options ...repository.Option) (int64, error)

	// FindByCommitSHAs returns enrichments across multiple commits.
	FindByCommitSHAs(ctx context.Context, commitSHAs []string, options ...repository.Option) ([]Enrichment, error)

	// CountByCommitSHAs returns the count of enrichments across multiple commits.
	CountByCommitSHAs(ctx context.Context, commitSHAs []string, options ...repository.Option) (int64, error)
}

// AssociationStore defines operations for persisting and retrieving enrichment associations.
type AssociationStore interface {
	repository.Store[Association]
	DeleteBy(ctx context.Context, options ...repository.Option) error
}
