package search

import "github.com/helixml/kodit/domain/repository"

// WithSnippetID filters by a single snippet ID.
func WithSnippetID(id string) repository.Option {
	return repository.WithCondition("snippet_id", id)
}

// WithSnippetIDs filters by multiple snippet IDs.
func WithSnippetIDs(ids []string) repository.Option {
	return repository.WithConditionIn("snippet_id", ids)
}

// WithEmbedding passes a pre-computed embedding vector through options.
func WithEmbedding(embedding []float64) repository.Option {
	return repository.WithParam("embedding", embedding)
}

// WithQuery passes a search query string through options.
func WithQuery(query string) repository.Option {
	return repository.WithParam("search_query", query)
}

// EmbeddingFrom extracts the embedding vector from a built query.
func EmbeddingFrom(q repository.Query) ([]float64, bool) {
	v, ok := q.Param("embedding")
	if !ok {
		return nil, false
	}
	emb, ok := v.([]float64)
	return emb, ok
}

// QueryFrom extracts the search query text from a built query.
func QueryFrom(q repository.Query) (string, bool) {
	v, ok := q.Param("search_query")
	if !ok {
		return "", false
	}
	text, ok := v.(string)
	return text, ok
}

// WithFilters passes search filters through the option system.
func WithFilters(filters Filters) repository.Option {
	return repository.WithParam("search_filters", filters)
}

// FiltersFrom extracts search filters from a built query.
func FiltersFrom(q repository.Query) (Filters, bool) {
	v, ok := q.Param("search_filters")
	if !ok {
		return Filters{}, false
	}
	f, ok := v.(Filters)
	return f, ok
}

// SnippetIDsFrom extracts snippet IDs from conditions on a built query.
func SnippetIDsFrom(q repository.Query) []string {
	for _, cond := range q.Conditions() {
		if cond.Field() == "snippet_id" && cond.In() {
			if ids, ok := cond.Value().([]string); ok {
				return ids
			}
		}
	}
	return nil
}
