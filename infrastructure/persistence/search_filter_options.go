package persistence

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"

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

	// An enrichment belongs to the repositories it is directly associated
	// with. git_commits is not consulted: it keeps one row per SHA, so any
	// repository scanned with a matching SHA would otherwise gain the
	// enrichments of another repository. Enrichment.RepositoryIDs
	// (application/service/enrichment.go) applies the same rule to search
	// results; change both together.
	if repos := filters.SourceRepos(); len(repos) > 0 {
		repoKeys := make([]string, len(repos))
		for i, id := range repos {
			repoKeys[i] = strconv.FormatInt(id, 10)
		}
		opts = append(opts, repository.WithWhere(fmt.Sprintf(
			"CAST(snippet_id AS %s) IN ("+
				"SELECT ea_repo.enrichment_id FROM enrichment_associations ea_repo "+
				"WHERE ea_repo.entity_type = 'git_repos' AND ea_repo.entity_id IN ?)", castType),
			repoKeys))
	}

	return opts
}

// appendSearchFilters appends the filter-and-snippet options derived from q
// to opts. Used by store Find overrides to consolidate the common pattern of
// translating search.Filters into JOIN/WHERE options and forwarding any
// snippet-ID restrictions.
func appendSearchFilters(opts []repository.Option, q repository.Query, castType string) []repository.Option {
	if filters, ok := search.FiltersFrom(q); ok {
		opts = append(opts, filterJoinOptions(filters, castType)...)
	}
	if snippetIDs := search.SnippetIDsFrom(q); len(snippetIDs) > 0 {
		opts = append(opts, search.WithSnippetIDs(snippetIDs))
	}
	return opts
}

// filterNewDocuments returns documents that have non-empty snippet IDs and
// non-empty text and are not already present in the store. Used by BM25
// stores to skip duplicates before INSERT — re-indexing the same passage
// would either error or pointlessly re-tokenize.
func filterNewDocuments(ctx context.Context, store search.Store, docs []search.Document, logger zerolog.Logger) ([]search.Document, error) {
	valid := make([]search.Document, 0, len(docs))
	for _, doc := range docs {
		if doc.SnippetID() != "" && doc.Text() != "" {
			valid = append(valid, doc)
		}
	}
	if len(valid) == 0 {
		logger.Warn().Msg("corpus is empty, skipping bm25 index")
		return nil, nil
	}

	ids := make([]string, len(valid))
	for i, doc := range valid {
		ids[i] = doc.SnippetID()
	}

	existing, err := search.ExistingSnippetIDs(ctx, store, ids)
	if err != nil {
		return nil, err
	}

	out := make([]search.Document, 0, len(valid))
	for _, doc := range valid {
		if _, dup := existing[doc.SnippetID()]; !dup {
			out = append(out, doc)
		}
	}
	return out, nil
}
