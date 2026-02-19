// Package jsonapi provides JSON:API specification compliant types for API responses.
package jsonapi

import (
	"encoding/json"
	"time"
)

// Document represents a JSON:API top-level document.
// See: https://jsonapi.org/format/#document-structure
type Document struct {
	Data     any     `json:"data"`
	Meta     *Meta   `json:"meta,omitempty"`
	Links    *Links  `json:"links,omitempty"`
	Included []any   `json:"included,omitempty"`
	Errors   []Error `json:"errors,omitempty"`
}

// Meta holds non-standard meta-information about a document.
type Meta map[string]any

// Links holds links associated with a document or resource.
type Links struct {
	Self  string `json:"self,omitempty"`
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
}

// Resource represents a JSON:API resource object.
// See: https://jsonapi.org/format/#document-resource-objects
type Resource struct {
	Type          string        `json:"type"`
	ID            string        `json:"id"`
	Attributes    any           `json:"attributes"`
	Relationships Relationships `json:"relationships,omitempty"`
	Links         *Links        `json:"links,omitempty"`
	Meta          *Meta         `json:"meta,omitempty"`
}

// Relationships maps relationship names to their data.
type Relationships map[string]*Relationship

// Relationship represents a JSON:API relationship.
type Relationship struct {
	Links *Links `json:"links,omitempty"`
	Data  any    `json:"data,omitempty"` // Can be ResourceIdentifier, []ResourceIdentifier, or nil
	Meta  *Meta  `json:"meta,omitempty"`
}

// ResourceIdentifier identifies a resource without full attributes.
type ResourceIdentifier struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Error represents a JSON:API error object.
// See: https://jsonapi.org/format/#error-objects
type Error struct {
	ID     string       `json:"id,omitempty"`
	Links  *ErrorLinks  `json:"links,omitempty"`
	Status string       `json:"status,omitempty"`
	Code   string       `json:"code,omitempty"`
	Title  string       `json:"title,omitempty"`
	Detail string       `json:"detail,omitempty"`
	Source *ErrorSource `json:"source,omitempty"`
	Meta   *Meta        `json:"meta,omitempty"`
}

// ErrorLinks holds links for error objects.
type ErrorLinks struct {
	About string `json:"about,omitempty"`
	Type  string `json:"type,omitempty"`
}

// ErrorSource holds references to the source of an error.
type ErrorSource struct {
	Pointer   string `json:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Header    string `json:"header,omitempty"`
}

// NewResource creates a new resource with the given type, id and attributes.
func NewResource(resourceType, id string, attrs any) *Resource {
	return &Resource{
		Type:       resourceType,
		ID:         id,
		Attributes: attrs,
	}
}

// NewSingleResponse creates a JSON:API document with a single resource.
func NewSingleResponse(resource *Resource) *Document {
	return &Document{
		Data: resource,
	}
}

// NewListResponse creates a JSON:API document with a list of resources.
func NewListResponse(resources []*Resource) *Document {
	return &Document{
		Data: resources,
	}
}

// NewErrorResponse creates a JSON:API document with errors.
func NewErrorResponse(errors ...Error) *Document {
	return &Document{
		Errors: errors,
	}
}

// NewError creates a simple error with status, title and detail.
func NewError(status, title, detail string) Error {
	return Error{
		Status: status,
		Title:  title,
		Detail: detail,
	}
}

// DateTime handles JSON serialization of time.Time to ISO8601 format.
type DateTime time.Time

// MarshalJSON serializes the DateTime to ISO8601 format.
func (dt DateTime) MarshalJSON() ([]byte, error) {
	t := time.Time(dt)
	if t.IsZero() {
		return json.Marshal(nil)
	}
	return json.Marshal(t.Format(time.RFC3339))
}

// UnmarshalJSON deserializes ISO8601 format to DateTime.
func (dt *DateTime) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == nil {
		*dt = DateTime{}
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return err
	}
	*dt = DateTime(t)
	return nil
}

// Time returns the underlying time.Time.
func (dt DateTime) Time() time.Time {
	return time.Time(dt)
}

// NewDateTime creates a DateTime from a time.Time.
func NewDateTime(t time.Time) DateTime {
	return DateTime(t)
}

// Ptr returns a pointer to the DateTime.
func (dt DateTime) Ptr() *DateTime {
	return &dt
}
