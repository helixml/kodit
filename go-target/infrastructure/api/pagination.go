package api

import (
	"net/http"
	"strconv"
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

// PaginatedResponse wraps a list response with pagination metadata.
type PaginatedResponse struct {
	Data       any                `json:"data"`
	Meta       PaginationMeta     `json:"meta,omitempty"`
	Links      *PaginationLinks   `json:"links,omitempty"`
}

// PaginationMeta contains pagination metadata.
type PaginationMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalCount int `json:"total_count,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// PaginationLinks contains pagination links.
type PaginationLinks struct {
	Self  string `json:"self,omitempty"`
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
}

// NewPaginatedResponse creates a paginated response.
func NewPaginatedResponse(data any, params PaginationParams, totalCount int) PaginatedResponse {
	totalPages := 0
	if params.pageSize > 0 {
		totalPages = (totalCount + params.pageSize - 1) / params.pageSize
	}

	return PaginatedResponse{
		Data: data,
		Meta: PaginationMeta{
			Page:       params.page,
			PageSize:   params.pageSize,
			TotalCount: totalCount,
			TotalPages: totalPages,
		},
	}
}
