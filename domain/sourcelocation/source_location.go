// Package sourcelocation provides metadata about where an enrichment's
// content originates within a source file.
package sourcelocation

// SourceLocation records the origin of an enrichment within its source file.
// For text chunks this is a line range; for page images this is a page number.
// Immutable value object.
type SourceLocation struct {
	id           int64
	enrichmentID int64
	page         int
	startLine    int
	endLine      int
}

// New creates a SourceLocation for a line range (not yet persisted).
func New(enrichmentID int64, startLine, endLine int) SourceLocation {
	return SourceLocation{
		enrichmentID: enrichmentID,
		startLine:    startLine,
		endLine:      endLine,
	}
}

// NewPage creates a SourceLocation for a page number (not yet persisted).
func NewPage(enrichmentID int64, page int) SourceLocation {
	return SourceLocation{
		enrichmentID: enrichmentID,
		page:         page,
	}
}

// NewWithPage creates a SourceLocation with both a page number and a line range.
func NewWithPage(enrichmentID int64, page, startLine, endLine int) SourceLocation {
	return SourceLocation{
		enrichmentID: enrichmentID,
		page:         page,
		startLine:    startLine,
		endLine:      endLine,
	}
}

// Reconstruct recreates a SourceLocation from persistence.
func Reconstruct(id, enrichmentID int64, page, startLine, endLine int) SourceLocation {
	return SourceLocation{
		id:           id,
		enrichmentID: enrichmentID,
		page:         page,
		startLine:    startLine,
		endLine:      endLine,
	}
}

// ID returns the database identifier.
func (s SourceLocation) ID() int64 { return s.id }

// EnrichmentID returns the associated enrichment's ID.
func (s SourceLocation) EnrichmentID() int64 { return s.enrichmentID }

// Page returns the 1-based page number (0 means not applicable).
func (s SourceLocation) Page() int { return s.page }

// StartLine returns the 1-based first line (0 means not applicable).
func (s SourceLocation) StartLine() int { return s.startLine }

// EndLine returns the 1-based last line (0 means not applicable).
func (s SourceLocation) EndLine() int { return s.endLine }
