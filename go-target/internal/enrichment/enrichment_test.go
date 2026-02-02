package enrichment

import (
	"testing"
	"time"
)

func TestNewEnrichment(t *testing.T) {
	content := "Test enrichment content"
	e := NewEnrichment(TypeArchitecture, SubtypePhysical, EntityTypeCommit, content)

	if e.ID() != 0 {
		t.Errorf("expected ID 0, got %d", e.ID())
	}
	if e.Content() != content {
		t.Errorf("expected content %q, got %q", content, e.Content())
	}
	if e.Type() != TypeArchitecture {
		t.Errorf("expected type %q, got %q", TypeArchitecture, e.Type())
	}
	if e.Subtype() != SubtypePhysical {
		t.Errorf("expected subtype %q, got %q", SubtypePhysical, e.Subtype())
	}
	if e.EntityTypeKey() != EntityTypeCommit {
		t.Errorf("expected entity key %q, got %q", EntityTypeCommit, e.EntityTypeKey())
	}
	if e.Language() != "" {
		t.Errorf("expected empty language, got %q", e.Language())
	}
	if e.CreatedAt().IsZero() {
		t.Error("expected non-zero created_at")
	}
	if e.UpdatedAt().IsZero() {
		t.Error("expected non-zero updated_at")
	}
}

func TestNewEnrichmentWithLanguage(t *testing.T) {
	e := NewEnrichmentWithLanguage(TypeUsage, SubtypeAPIDocs, EntityTypeCommit, "API docs", "golang")

	if e.Language() != "golang" {
		t.Errorf("expected language 'golang', got %q", e.Language())
	}
	if e.Type() != TypeUsage {
		t.Errorf("expected type %q, got %q", TypeUsage, e.Type())
	}
	if e.Subtype() != SubtypeAPIDocs {
		t.Errorf("expected subtype %q, got %q", SubtypeAPIDocs, e.Subtype())
	}
}

func TestReconstructEnrichment(t *testing.T) {
	now := time.Now()
	before := now.Add(-time.Hour)

	e := ReconstructEnrichment(42, TypeDevelopment, SubtypeSnippet, EntityTypeCommit, "content", "python", before, now)

	if e.ID() != 42 {
		t.Errorf("expected ID 42, got %d", e.ID())
	}
	if e.Type() != TypeDevelopment {
		t.Errorf("expected type %q, got %q", TypeDevelopment, e.Type())
	}
	if e.Subtype() != SubtypeSnippet {
		t.Errorf("expected subtype %q, got %q", SubtypeSnippet, e.Subtype())
	}
	if e.Language() != "python" {
		t.Errorf("expected language 'python', got %q", e.Language())
	}
	if !e.CreatedAt().Equal(before) {
		t.Errorf("expected created_at %v, got %v", before, e.CreatedAt())
	}
	if !e.UpdatedAt().Equal(now) {
		t.Errorf("expected updated_at %v, got %v", now, e.UpdatedAt())
	}
}

func TestEnrichment_IsCommitEnrichment(t *testing.T) {
	commit := NewEnrichment(TypeArchitecture, SubtypePhysical, EntityTypeCommit, "content")
	snippet := NewEnrichment(TypeDevelopment, SubtypeSnippet, EntityTypeSnippet, "content")

	if !commit.IsCommitEnrichment() {
		t.Error("expected commit enrichment to return true for IsCommitEnrichment")
	}
	if snippet.IsCommitEnrichment() {
		t.Error("expected snippet enrichment to return false for IsCommitEnrichment")
	}
}

func TestEnrichment_WithID(t *testing.T) {
	e := NewEnrichment(TypeArchitecture, SubtypePhysical, EntityTypeCommit, "content")
	e2 := e.WithID(123)

	if e.ID() != 0 {
		t.Error("original enrichment should not be mutated")
	}
	if e2.ID() != 123 {
		t.Errorf("expected ID 123, got %d", e2.ID())
	}
}

func TestEnrichment_WithContent(t *testing.T) {
	e := NewEnrichment(TypeArchitecture, SubtypePhysical, EntityTypeCommit, "original")
	originalUpdatedAt := e.UpdatedAt()

	time.Sleep(time.Millisecond)
	e2 := e.WithContent("updated")

	if e.Content() != "original" {
		t.Error("original enrichment should not be mutated")
	}
	if e2.Content() != "updated" {
		t.Errorf("expected content 'updated', got %q", e2.Content())
	}
	if !e2.UpdatedAt().After(originalUpdatedAt) {
		t.Error("updated_at should be newer after WithContent")
	}
}

func TestArchitectureEnrichments(t *testing.T) {
	physical := NewPhysicalArchitecture("System has 3 microservices")
	dbSchema := NewDatabaseSchema("Users table has 5 columns")

	if !IsArchitectureEnrichment(physical) {
		t.Error("expected physical to be architecture enrichment")
	}
	if !IsArchitectureEnrichment(dbSchema) {
		t.Error("expected dbSchema to be architecture enrichment")
	}
	if !IsPhysicalArchitecture(physical) {
		t.Error("expected physical to be physical architecture")
	}
	if IsPhysicalArchitecture(dbSchema) {
		t.Error("expected dbSchema to not be physical architecture")
	}
	if !IsDatabaseSchema(dbSchema) {
		t.Error("expected dbSchema to be database schema")
	}
	if IsDatabaseSchema(physical) {
		t.Error("expected physical to not be database schema")
	}
}

func TestDevelopmentEnrichments(t *testing.T) {
	snippet := NewSnippetEnrichment("Code snippet")
	snippetSummary := NewSnippetSummary("Summary of snippet")
	example := NewExample("Example code")
	exampleSummary := NewExampleSummary("Summary of example")

	tests := []struct {
		name        string
		enrichment  Enrichment
		isDev       bool
		isSnippet   bool
		isSummary   bool
		isExample   bool
		isExSummary bool
	}{
		{"snippet", snippet, true, true, false, false, false},
		{"snippetSummary", snippetSummary, true, false, true, false, false},
		{"example", example, true, false, false, true, false},
		{"exampleSummary", exampleSummary, true, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsDevelopmentEnrichment(tt.enrichment) != tt.isDev {
				t.Errorf("IsDevelopmentEnrichment() = %v, want %v", !tt.isDev, tt.isDev)
			}
			if IsSnippetEnrichment(tt.enrichment) != tt.isSnippet {
				t.Errorf("IsSnippetEnrichment() = %v, want %v", !tt.isSnippet, tt.isSnippet)
			}
			if IsSnippetSummary(tt.enrichment) != tt.isSummary {
				t.Errorf("IsSnippetSummary() = %v, want %v", !tt.isSummary, tt.isSummary)
			}
			if IsExample(tt.enrichment) != tt.isExample {
				t.Errorf("IsExample() = %v, want %v", !tt.isExample, tt.isExample)
			}
			if IsExampleSummary(tt.enrichment) != tt.isExSummary {
				t.Errorf("IsExampleSummary() = %v, want %v", !tt.isExSummary, tt.isExSummary)
			}
		})
	}
}

func TestHistoryEnrichments(t *testing.T) {
	commitDesc := NewCommitDescription("This commit adds logging")

	if !IsHistoryEnrichment(commitDesc) {
		t.Error("expected commitDesc to be history enrichment")
	}
	if !IsCommitDescription(commitDesc) {
		t.Error("expected commitDesc to be commit description")
	}

	physical := NewPhysicalArchitecture("content")
	if IsHistoryEnrichment(physical) {
		t.Error("expected physical to not be history enrichment")
	}
	if IsCommitDescription(physical) {
		t.Error("expected physical to not be commit description")
	}
}

func TestUsageEnrichments(t *testing.T) {
	cookbook := NewCookbook("How to use feature X")
	apiDocs := NewAPIDocs("Function Foo takes 2 params", "golang")

	if !IsUsageEnrichment(cookbook) {
		t.Error("expected cookbook to be usage enrichment")
	}
	if !IsUsageEnrichment(apiDocs) {
		t.Error("expected apiDocs to be usage enrichment")
	}
	if !IsCookbook(cookbook) {
		t.Error("expected cookbook to be cookbook subtype")
	}
	if IsCookbook(apiDocs) {
		t.Error("expected apiDocs to not be cookbook subtype")
	}
	if !IsAPIDocs(apiDocs) {
		t.Error("expected apiDocs to be api_docs subtype")
	}
	if IsAPIDocs(cookbook) {
		t.Error("expected cookbook to not be api_docs subtype")
	}

	if apiDocs.Language() != "golang" {
		t.Errorf("expected language 'golang', got %q", apiDocs.Language())
	}
}

func TestNewAssociation(t *testing.T) {
	assoc := NewAssociation(42, "commit-sha-123", EntityTypeCommit)

	if assoc.ID() != 0 {
		t.Errorf("expected ID 0, got %d", assoc.ID())
	}
	if assoc.EnrichmentID() != 42 {
		t.Errorf("expected enrichment ID 42, got %d", assoc.EnrichmentID())
	}
	if assoc.EntityID() != "commit-sha-123" {
		t.Errorf("expected entity ID 'commit-sha-123', got %q", assoc.EntityID())
	}
	if assoc.EntityType() != EntityTypeCommit {
		t.Errorf("expected entity type %q, got %q", EntityTypeCommit, assoc.EntityType())
	}
}

func TestReconstructAssociation(t *testing.T) {
	assoc := ReconstructAssociation(1, 42, "sha-456", EntityTypeSnippet)

	if assoc.ID() != 1 {
		t.Errorf("expected ID 1, got %d", assoc.ID())
	}
	if assoc.EnrichmentID() != 42 {
		t.Errorf("expected enrichment ID 42, got %d", assoc.EnrichmentID())
	}
	if assoc.EntityID() != "sha-456" {
		t.Errorf("expected entity ID 'sha-456', got %q", assoc.EntityID())
	}
	if assoc.EntityType() != EntityTypeSnippet {
		t.Errorf("expected entity type %q, got %q", EntityTypeSnippet, assoc.EntityType())
	}
}

func TestAssociation_WithID(t *testing.T) {
	assoc := NewAssociation(42, "entity-1", EntityTypeCommit)
	assoc2 := assoc.WithID(99)

	if assoc.ID() != 0 {
		t.Error("original association should not be mutated")
	}
	if assoc2.ID() != 99 {
		t.Errorf("expected ID 99, got %d", assoc2.ID())
	}
}

func TestCommitAssociation(t *testing.T) {
	assoc := CommitAssociation(42, "abc123")

	if assoc.EnrichmentID() != 42 {
		t.Errorf("expected enrichment ID 42, got %d", assoc.EnrichmentID())
	}
	if assoc.EntityID() != "abc123" {
		t.Errorf("expected entity ID 'abc123', got %q", assoc.EntityID())
	}
	if assoc.EntityType() != EntityTypeCommit {
		t.Errorf("expected entity type %q, got %q", EntityTypeCommit, assoc.EntityType())
	}
}

func TestSnippetAssociation(t *testing.T) {
	assoc := SnippetAssociation(42, "snippet-hash")

	if assoc.EnrichmentID() != 42 {
		t.Errorf("expected enrichment ID 42, got %d", assoc.EnrichmentID())
	}
	if assoc.EntityID() != "snippet-hash" {
		t.Errorf("expected entity ID 'snippet-hash', got %q", assoc.EntityID())
	}
	if assoc.EntityType() != EntityTypeSnippet {
		t.Errorf("expected entity type %q, got %q", EntityTypeSnippet, assoc.EntityType())
	}
}

func TestSnippetSummaryLink(t *testing.T) {
	summary := NewAssociation(1, "snippet-1", EntityTypeSnippet)
	snippet := NewAssociation(2, "snippet-1", EntityTypeSnippet)

	link := NewSnippetSummaryLink(summary, snippet)

	if link.Summary().EnrichmentID() != 1 {
		t.Errorf("expected summary enrichment ID 1, got %d", link.Summary().EnrichmentID())
	}
	if link.Snippet().EnrichmentID() != 2 {
		t.Errorf("expected snippet enrichment ID 2, got %d", link.Snippet().EnrichmentID())
	}
}

func TestTypeConstants(t *testing.T) {
	types := []Type{TypeArchitecture, TypeDevelopment, TypeHistory, TypeUsage}
	expected := []string{"architecture", "development", "history", "usage"}

	for i, typ := range types {
		if string(typ) != expected[i] {
			t.Errorf("expected type %q, got %q", expected[i], typ)
		}
	}
}

func TestSubtypeConstants(t *testing.T) {
	subtypes := []Subtype{
		SubtypePhysical, SubtypeDatabaseSchema,
		SubtypeSnippet, SubtypeSnippetSummary, SubtypeExample, SubtypeExampleSummary,
		SubtypeCommitDescription,
		SubtypeCookbook, SubtypeAPIDocs,
	}
	expected := []string{
		"physical", "database_schema",
		"snippet", "snippet_summary", "example", "example_summary",
		"commit_description",
		"cookbook", "api_docs",
	}

	for i, subtype := range subtypes {
		if string(subtype) != expected[i] {
			t.Errorf("expected subtype %q, got %q", expected[i], subtype)
		}
	}
}

func TestEntityTypeKeyConstants(t *testing.T) {
	if string(EntityTypeCommit) != "git_commits" {
		t.Errorf("expected 'git_commits', got %q", EntityTypeCommit)
	}
	if string(EntityTypeSnippet) != "snippets" {
		t.Errorf("expected 'snippets', got %q", EntityTypeSnippet)
	}
}
