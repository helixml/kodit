package enrichment

// NewPhysicalArchitecture creates a physical architecture enrichment for a commit.
// Physical architecture describes system structure discovery (containers, services, deployment).
func NewPhysicalArchitecture(content string) Enrichment {
	return NewEnrichment(TypeArchitecture, SubtypePhysical, EntityTypeCommit, content)
}

// NewDatabaseSchema creates a database schema enrichment for a commit.
// Database schema describes the data model and relationships.
func NewDatabaseSchema(content string) Enrichment {
	return NewEnrichment(TypeArchitecture, SubtypeDatabaseSchema, EntityTypeCommit, content)
}

// IsArchitectureEnrichment returns true if the enrichment is an architecture type.
func IsArchitectureEnrichment(e Enrichment) bool {
	return e.Type() == TypeArchitecture
}

// IsPhysicalArchitecture returns true if the enrichment is a physical architecture subtype.
func IsPhysicalArchitecture(e Enrichment) bool {
	return e.Type() == TypeArchitecture && e.Subtype() == SubtypePhysical
}

// IsDatabaseSchema returns true if the enrichment is a database schema subtype.
func IsDatabaseSchema(e Enrichment) bool {
	return e.Type() == TypeArchitecture && e.Subtype() == SubtypeDatabaseSchema
}
