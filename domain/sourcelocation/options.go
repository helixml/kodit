package sourcelocation

import "github.com/helixml/kodit/domain/repository"

// WithEnrichmentID filters by the "enrichment_id" column.
func WithEnrichmentID(id int64) repository.Option {
	return repository.WithCondition("enrichment_id", id)
}
