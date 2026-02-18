package enrichment

import "github.com/helixml/kodit/domain/repository"

// WithType filters by the "type" column.
func WithType(typ Type) repository.Option {
	return repository.WithCondition("type", string(typ))
}

// WithSubtype filters by the "subtype" column.
func WithSubtype(subtype Subtype) repository.Option {
	return repository.WithCondition("subtype", string(subtype))
}

// WithEnrichmentID filters by the "enrichment_id" column.
func WithEnrichmentID(id int64) repository.Option {
	return repository.WithCondition("enrichment_id", id)
}

// WithEntityID filters by the "entity_id" column.
func WithEntityID(entityID string) repository.Option {
	return repository.WithCondition("entity_id", entityID)
}

// WithEntityType filters by the "entity_type" column.
func WithEntityType(entityType EntityTypeKey) repository.Option {
	return repository.WithCondition("entity_type", string(entityType))
}

// WithEntityIDIn filters by multiple entity IDs.
func WithEntityIDIn(entityIDs []string) repository.Option {
	return repository.WithConditionIn("entity_id", entityIDs)
}

// WithEnrichmentIDIn filters by multiple enrichment IDs.
func WithEnrichmentIDIn(ids []int64) repository.Option {
	return repository.WithConditionIn("enrichment_id", ids)
}

// WithCommitSHA filters enrichments associated with a single commit SHA
// via the enrichment_associations table.
func WithCommitSHA(sha string) repository.Option {
	return repository.WithParam("enrichment_commit_sha", sha)
}

// WithCommitSHAs filters enrichments associated with multiple commit SHAs
// via the enrichment_associations table.
func WithCommitSHAs(shas []string) repository.Option {
	return repository.WithParam("enrichment_commit_shas", shas)
}

// CommitSHAFrom extracts a single commit SHA filter from a query.
func CommitSHAFrom(q repository.Query) (string, bool) {
	v, ok := q.Param("enrichment_commit_sha")
	if !ok {
		return "", false
	}
	sha, ok := v.(string)
	return sha, ok && sha != ""
}

// CommitSHAsFrom extracts multiple commit SHA filters from a query.
func CommitSHAsFrom(q repository.Query) ([]string, bool) {
	v, ok := q.Param("enrichment_commit_shas")
	if !ok {
		return nil, false
	}
	shas, ok := v.([]string)
	return shas, ok && len(shas) > 0
}
