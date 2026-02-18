package search

import (
	"fmt"

	"github.com/helixml/kodit/domain/search"
	"gorm.io/gorm"
)

// applySearchFilters adds JOINs and WHERE clauses to a GORM query based on
// search filters. The calling table must have a snippet_id column that stores
// enrichment IDs as strings; the JOINs cast snippet_id to the appropriate
// integer type for the dialect.
func applySearchFilters(db *gorm.DB, filters search.Filters) *gorm.DB {
	if filters.IsEmpty() {
		return db
	}

	castType := "bigint"
	if db.Name() == "sqlite" {
		castType = "INTEGER"
	}

	needEnrichmentJoin := len(filters.Languages()) > 0 ||
		len(filters.EnrichmentTypes()) > 0 ||
		len(filters.EnrichmentSubtypes()) > 0

	if needEnrichmentJoin {
		db = db.Joins(fmt.Sprintf(
			"JOIN enrichments_v2 ON enrichments_v2.id = CAST(snippet_id AS %s)", castType))
		if langs := filters.Languages(); len(langs) > 0 {
			db = db.Where("enrichments_v2.language IN ?", langs)
		}
		if types := filters.EnrichmentTypes(); len(types) > 0 {
			db = db.Where("enrichments_v2.type IN ?", types)
		}
		if subtypes := filters.EnrichmentSubtypes(); len(subtypes) > 0 {
			db = db.Where("enrichments_v2.subtype IN ?", subtypes)
		}
	}

	if shas := filters.CommitSHAs(); len(shas) > 0 {
		db = db.Joins(fmt.Sprintf(
			"JOIN enrichment_associations ea_sha ON ea_sha.enrichment_id = CAST(snippet_id AS %s)", castType))
		db = db.Where("ea_sha.entity_type = ?", "git_commits")
		db = db.Where("ea_sha.entity_id IN ?", shas)
	}

	if repos := filters.SourceRepos(); len(repos) > 0 {
		db = db.Joins(fmt.Sprintf(
			"JOIN enrichment_associations ea_repo ON ea_repo.enrichment_id = CAST(snippet_id AS %s) AND ea_repo.entity_type = 'git_commits'", castType))
		db = db.Joins("JOIN git_commits gc_repo ON gc_repo.commit_sha = ea_repo.entity_id")
		db = db.Where("gc_repo.repo_id IN ?", repos)
	}

	return db
}
