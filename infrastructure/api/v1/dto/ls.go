package dto

import "github.com/helixml/kodit/infrastructure/api/jsonapi"

// LsFileLinks holds links for a file in an ls response.
type LsFileLinks struct {
	Self string `json:"self"`
}

// LsFileAttributes holds attributes for a file in an ls response.
type LsFileAttributes struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// LsFileData represents a single file entry in an ls response.
type LsFileData struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	Attributes LsFileAttributes `json:"attributes"`
	Links      LsFileLinks      `json:"links"`
}

// LsResponse represents the response for a file listing.
type LsResponse struct {
	Data  []LsFileData   `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
}
