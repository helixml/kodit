package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

// PaginationParams holds pagination parameters parsed from query strings.
type PaginationParams struct {
	page     int
	pageSize int
}

// DefaultPageSize is the default number of items per page.
const DefaultPageSize = 20

// MaxPageSize is the maximum allowed page size.
const MaxPageSize = 100

// NewPaginationParams creates pagination params with defaults.
func NewPaginationParams() PaginationParams {
	return PaginationParams{
		page:     1,
		pageSize: DefaultPageSize,
	}
}

// ParsePagination parses pagination parameters from an HTTP request.
// Default: page=1, page_size=20
// Max page_size: 100
func ParsePagination(r *http.Request) PaginationParams {
	params := NewPaginationParams()

	// Parse page parameter
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page >= 1 {
			params.page = page
		}
	}

	// Parse page_size parameter
	if sizeStr := r.URL.Query().Get("page_size"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size >= 1 {
			params.pageSize = size
			if params.pageSize > MaxPageSize {
				params.pageSize = MaxPageSize
			}
		}
	}

	return params
}

// Page returns the page number (1-indexed).
func (p PaginationParams) Page() int { return p.page }

// PageSize returns the page size.
func (p PaginationParams) PageSize() int { return p.pageSize }

// Offset returns the offset for database queries.
func (p PaginationParams) Offset() int {
	return (p.page - 1) * p.pageSize
}

// Limit returns the limit for database queries.
func (p PaginationParams) Limit() int {
	return p.pageSize
}

// WithPage returns a copy with the specified page.
func (p PaginationParams) WithPage(page int) PaginationParams {
	if page < 1 {
		page = 1
	}
	p.page = page
	return p
}

// WithPageSize returns a copy with the specified page size.
func (p PaginationParams) WithPageSize(size int) PaginationParams {
	if size < 1 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	p.pageSize = size
	return p
}

// Options returns repository options for database pagination.
func (p PaginationParams) Options() []repository.Option {
	return repository.WithPagination(p.Limit(), p.Offset())
}

// PaginationMeta builds a JSON:API meta object from pagination params and total count.
func PaginationMeta(params PaginationParams, totalCount int64) *jsonapi.Meta {
	totalPages := 0
	if params.PageSize() > 0 {
		totalPages = (int(totalCount) + params.PageSize() - 1) / params.PageSize()
	}
	return &jsonapi.Meta{
		"page":        params.Page(),
		"page_size":   params.PageSize(),
		"total_count": totalCount,
		"total_pages": totalPages,
	}
}

// PaginationLinks builds JSON:API links from the request, params, and total count.
func PaginationLinks(r *http.Request, params PaginationParams, totalCount int64) *jsonapi.Links {
	totalPages := 0
	if params.PageSize() > 0 {
		totalPages = (int(totalCount) + params.PageSize() - 1) / params.PageSize()
	}

	buildURL := func(page int) string {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", strconv.Itoa(params.PageSize()))
		return fmt.Sprintf("%s?%s", r.URL.Path, q.Encode())
	}

	links := jsonapi.Links{
		Self:  buildURL(params.Page()),
		First: buildURL(1),
	}

	if totalPages > 0 {
		links.Last = buildURL(totalPages)
	}

	if params.Page() > 1 {
		links.Prev = buildURL(params.Page() - 1)
	}

	if params.Page() < totalPages {
		links.Next = buildURL(params.Page() + 1)
	}

	return &links
}
