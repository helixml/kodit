// Package search provides search domain types for hybrid code retrieval.
package search

// Type represents the type of search to perform.
type Type string

// Type values.
const (
	TypeBM25   Type = "bm25"
	TypeVector Type = "vector"
	TypeHybrid Type = "hybrid"
)

// Query represents a snippet search query.
type Query struct {
	text       string
	searchType Type
	filters    Filters
	topK       int
}

// NewQuery creates a new Query.
func NewQuery(text string, searchType Type, filters Filters, topK int) Query {
	return Query{
		text:       text,
		searchType: searchType,
		filters:    filters,
		topK:       topK,
	}
}

// Text returns the query text.
func (q Query) Text() string { return q.text }

// SearchType returns the search type.
func (q Query) SearchType() Type { return q.searchType }

// Filters returns the search filters.
func (q Query) Filters() Filters { return q.filters }

// TopK returns the number of results.
func (q Query) TopK() int { return q.topK }

// MultiRequest represents a multi-modal search request.
type MultiRequest struct {
	topK      int
	textQuery string
	codeQuery string
	keywords  []string
	filters   Filters
}

// NewMultiRequest creates a new MultiRequest.
func NewMultiRequest(
	topK int,
	textQuery, codeQuery string,
	keywords []string,
	filters Filters,
) MultiRequest {
	var kw []string
	if keywords != nil {
		kw = make([]string, len(keywords))
		copy(kw, keywords)
	}
	return MultiRequest{
		topK:      topK,
		textQuery: textQuery,
		codeQuery: codeQuery,
		keywords:  kw,
		filters:   filters,
	}
}

// TopK returns the number of results to return.
func (m MultiRequest) TopK() int { return m.topK }

// TextQuery returns the text query.
func (m MultiRequest) TextQuery() string { return m.textQuery }

// CodeQuery returns the code query.
func (m MultiRequest) CodeQuery() string { return m.codeQuery }

// Keywords returns the keywords.
func (m MultiRequest) Keywords() []string {
	if m.keywords == nil {
		return nil
	}
	kw := make([]string, len(m.keywords))
	copy(kw, m.keywords)
	return kw
}

// Filters returns the search filters.
func (m MultiRequest) Filters() Filters { return m.filters }
