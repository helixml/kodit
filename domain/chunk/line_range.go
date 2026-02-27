// Package chunk provides domain types for chunk-level metadata.
package chunk

// LineRange records the 1-based start and end line of a chunk enrichment
// within its source file. Immutable value object.
type LineRange struct {
	id           int64
	enrichmentID int64
	startLine    int
	endLine      int
}

// NewLineRange creates a LineRange for a new enrichment (not yet persisted).
func NewLineRange(enrichmentID int64, startLine, endLine int) LineRange {
	return LineRange{
		enrichmentID: enrichmentID,
		startLine:    startLine,
		endLine:      endLine,
	}
}

// ReconstructLineRange recreates a LineRange from persistence.
func ReconstructLineRange(id, enrichmentID int64, startLine, endLine int) LineRange {
	return LineRange{
		id:           id,
		enrichmentID: enrichmentID,
		startLine:    startLine,
		endLine:      endLine,
	}
}

// ID returns the database identifier.
func (r LineRange) ID() int64 { return r.id }

// EnrichmentID returns the associated enrichment's ID.
func (r LineRange) EnrichmentID() int64 { return r.enrichmentID }

// StartLine returns the 1-based first line.
func (r LineRange) StartLine() int { return r.startLine }

// EndLine returns the 1-based last line.
func (r LineRange) EndLine() int { return r.endLine }
