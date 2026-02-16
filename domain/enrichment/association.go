package enrichment

// Association links an enrichment to an entity.
// This is an immutable value object.
type Association struct {
	id           int64
	enrichmentID int64
	entityID     string
	entityType   EntityTypeKey
}

// NewAssociation creates a new enrichment-entity association.
func NewAssociation(enrichmentID int64, entityID string, entityType EntityTypeKey) Association {
	return Association{
		id:           0,
		enrichmentID: enrichmentID,
		entityID:     entityID,
		entityType:   entityType,
	}
}

// ReconstructAssociation recreates an association from persistence.
func ReconstructAssociation(id, enrichmentID int64, entityID string, entityType EntityTypeKey) Association {
	return Association{
		id:           id,
		enrichmentID: enrichmentID,
		entityID:     entityID,
		entityType:   entityType,
	}
}

// ID returns the association's database identifier.
func (a Association) ID() int64 {
	return a.id
}

// EnrichmentID returns the ID of the associated enrichment.
func (a Association) EnrichmentID() int64 {
	return a.enrichmentID
}

// EntityID returns the ID of the associated entity.
func (a Association) EntityID() string {
	return a.entityID
}

// EntityType returns the type of entity this association links to.
func (a Association) EntityType() EntityTypeKey {
	return a.entityType
}

// WithID returns a copy of the association with the specified ID.
func (a Association) WithID(id int64) Association {
	a.id = id
	return a
}

// CommitAssociation creates a new association linking an enrichment to a commit.
func CommitAssociation(enrichmentID int64, commitSHA string) Association {
	return NewAssociation(enrichmentID, commitSHA, EntityTypeCommit)
}

// SnippetAssociation creates a new association linking an enrichment to a snippet.
func SnippetAssociation(enrichmentID int64, snippetID string) Association {
	return NewAssociation(enrichmentID, snippetID, EntityTypeSnippet)
}

// FileAssociation creates a new association linking an enrichment to a file.
func FileAssociation(enrichmentID int64, fileID string) Association {
	return NewAssociation(enrichmentID, fileID, EntityTypeFile)
}

// SnippetSummaryLink pairs a snippet summary enrichment with its corresponding snippet enrichment.
// This is used to track which summary belongs to which snippet.
type SnippetSummaryLink struct {
	summary Association
	snippet Association
}

// NewSnippetSummaryLink creates a new link between a snippet summary and snippet association.
func NewSnippetSummaryLink(summary, snippet Association) SnippetSummaryLink {
	return SnippetSummaryLink{
		summary: summary,
		snippet: snippet,
	}
}

// Summary returns the snippet summary association.
func (l SnippetSummaryLink) Summary() Association {
	return l.summary
}

// Snippet returns the snippet association.
func (l SnippetSummaryLink) Snippet() Association {
	return l.snippet
}
