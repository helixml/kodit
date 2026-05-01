package persistence

import (
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// filterJoinOptions translates search.Filters into base repository options
// (JOINs + WHEREs) so that callers can compose them with their other Find
// options. Each search store invokes this from its Find override, passing
// its dialect's integer cast type ("bigint" for Postgres, "INTEGER" for SQLite).
//
// The calling table must have a snippet_id column that stores enrichment
// IDs as strings; the JOINs cast snippet_id to the appropriate integer type.
func filterJoinOptions(filters search.Filters, castType string) []repository.Option {
	if filters.IsEmpty() {
		return nil
	}

	var opts []repository.Option

	needEnrichmentJoin := len(filters.Languages()) > 0 ||
		len(filters.EnrichmentTypes()) > 0 ||
		len(filters.EnrichmentSubtypes()) > 0

	if needEnrichmentJoin {
		opts = append(opts, repository.WithJoin(fmt.Sprintf(
			"JOIN enrichments_v2 ON enrichments_v2.id = CAST(snippet_id AS %s)", castType)))
		if langs := filters.Languages(); len(langs) > 0 {
			opts = append(opts, repository.WithWhere("enrichments_v2.language IN ?", langs))
		}
		if types := filters.EnrichmentTypes(); len(types) > 0 {
			opts = append(opts, repository.WithWhere("enrichments_v2.type IN ?", types))
		}
		if subtypes := filters.EnrichmentSubtypes(); len(subtypes) > 0 {
			opts = append(opts, repository.WithWhere("enrichments_v2.subtype IN ?", subtypes))
		}
	}

	if shas := filters.CommitSHAs(); len(shas) > 0 {
		opts = append(opts,
			repository.WithJoin(fmt.Sprintf(
				"JOIN enrichment_associations ea_sha ON ea_sha.enrichment_id = CAST(snippet_id AS %s)", castType)),
			repository.WithWhere("ea_sha.entity_type = ?", "git_commits"),
			repository.WithWhere("ea_sha.entity_id IN ?", shas),
		)
	}

	if repos := filters.SourceRepos(); len(repos) > 0 {
		opts = append(opts,
			repository.WithJoin(fmt.Sprintf(
				"JOIN enrichment_associations ea_repo ON ea_repo.enrichment_id = CAST(snippet_id AS %s) AND ea_repo.entity_type = 'git_commits'", castType)),
			repository.WithJoin("JOIN git_commits gc_repo ON gc_repo.commit_sha = ea_repo.entity_id"),
			repository.WithWhere("gc_repo.repo_id IN ?", repos),
		)
	}

	return opts
}
