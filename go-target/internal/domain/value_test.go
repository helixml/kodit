package domain

import (
	"testing"
	"time"
)

func TestReportingState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    ReportingState
		terminal bool
	}{
		{ReportingStateStarted, false},
		{ReportingStateInProgress, false},
		{ReportingStateCompleted, true},
		{ReportingStateFailed, true},
		{ReportingStateSkipped, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}

func TestEnrichment(t *testing.T) {
	e := NewEnrichment("summary", "This is a summary")

	if e.Type() != "summary" {
		t.Errorf("Type() = %v, want summary", e.Type())
	}
	if e.Content() != "This is a summary" {
		t.Errorf("Content() = %v, want 'This is a summary'", e.Content())
	}
}

func TestSnippetContent(t *testing.T) {
	sc := NewSnippetContent(SnippetContentTypeOriginal, "func main() {}")

	if sc.Type() != SnippetContentTypeOriginal {
		t.Errorf("Type() = %v, want original", sc.Type())
	}
	if sc.Value() != "func main() {}" {
		t.Errorf("Value() = %v, want 'func main() {}'", sc.Value())
	}
}

func TestSnippetSearchResult(t *testing.T) {
	authors := []string{"alice", "bob"}
	result := NewSnippetSearchResult(123, "content", "summary", 0.95, "/path/file.go", "go", authors)

	if result.SnippetID() != 123 {
		t.Errorf("SnippetID() = %v, want 123", result.SnippetID())
	}
	if result.Content() != "content" {
		t.Errorf("Content() = %v, want 'content'", result.Content())
	}
	if result.Summary() != "summary" {
		t.Errorf("Summary() = %v, want 'summary'", result.Summary())
	}
	if result.Score() != 0.95 {
		t.Errorf("Score() = %v, want 0.95", result.Score())
	}
	if result.FilePath() != "/path/file.go" {
		t.Errorf("FilePath() = %v, want '/path/file.go'", result.FilePath())
	}
	if result.Language() != "go" {
		t.Errorf("Language() = %v, want 'go'", result.Language())
	}

	returnedAuthors := result.Authors()
	if len(returnedAuthors) != 2 {
		t.Errorf("Authors() length = %v, want 2", len(returnedAuthors))
	}

	// Verify returned slice is a copy
	returnedAuthors[0] = "modified"
	if result.Authors()[0] == "modified" {
		t.Error("Authors() should return a copy, not the original slice")
	}
}

func TestSnippetSearchResult_NilAuthors(t *testing.T) {
	result := NewSnippetSearchResult(1, "c", "s", 0.5, "/p", "go", nil)
	authors := result.Authors()
	if authors == nil {
		t.Error("Authors() should return empty slice, not nil")
	}
	if len(authors) != 0 {
		t.Errorf("Authors() length = %v, want 0", len(authors))
	}
}

func TestDocument(t *testing.T) {
	doc := NewDocument("snippet-123", "function hello() {}")

	if doc.SnippetID() != "snippet-123" {
		t.Errorf("SnippetID() = %v, want 'snippet-123'", doc.SnippetID())
	}
	if doc.Text() != "function hello() {}" {
		t.Errorf("Text() = %v, want 'function hello() {}'", doc.Text())
	}
}

func TestSearchRequest(t *testing.T) {
	ids := []string{"id1", "id2"}
	req := NewSearchRequest("test query", 20, ids)

	if req.Query() != "test query" {
		t.Errorf("Query() = %v, want 'test query'", req.Query())
	}
	if req.TopK() != 20 {
		t.Errorf("TopK() = %v, want 20", req.TopK())
	}

	returnedIDs := req.SnippetIDs()
	if len(returnedIDs) != 2 {
		t.Errorf("SnippetIDs() length = %v, want 2", len(returnedIDs))
	}

	// Verify returned slice is a copy
	returnedIDs[0] = "modified"
	if req.SnippetIDs()[0] == "modified" {
		t.Error("SnippetIDs() should return a copy")
	}
}

func TestSearchRequest_NilSnippetIDs(t *testing.T) {
	req := NewSearchRequest("query", 10, nil)
	if req.SnippetIDs() != nil {
		t.Error("SnippetIDs() should return nil when not set")
	}
}

func TestSnippetSearchFilters(t *testing.T) {
	now := time.Now()
	after := now.Add(-24 * time.Hour)
	before := now

	filters := NewSnippetSearchFilters(
		WithLanguage("python"),
		WithAuthor("alice"),
		WithCreatedAfter(after),
		WithCreatedBefore(before),
		WithSourceRepo("github.com/test/repo"),
		WithFilePath("/src/main.py"),
		WithEnrichmentTypes([]string{"development", "usage"}),
		WithEnrichmentSubtypes([]string{"snippet", "example"}),
		WithCommitSHAs([]string{"abc123", "def456"}),
	)

	if filters.Language() != "python" {
		t.Errorf("Language() = %v, want 'python'", filters.Language())
	}
	if filters.Author() != "alice" {
		t.Errorf("Author() = %v, want 'alice'", filters.Author())
	}
	if !filters.CreatedAfter().Equal(after) {
		t.Errorf("CreatedAfter() = %v, want %v", filters.CreatedAfter(), after)
	}
	if !filters.CreatedBefore().Equal(before) {
		t.Errorf("CreatedBefore() = %v, want %v", filters.CreatedBefore(), before)
	}
	if filters.SourceRepo() != "github.com/test/repo" {
		t.Errorf("SourceRepo() = %v, want 'github.com/test/repo'", filters.SourceRepo())
	}
	if filters.FilePath() != "/src/main.py" {
		t.Errorf("FilePath() = %v, want '/src/main.py'", filters.FilePath())
	}
	if len(filters.EnrichmentTypes()) != 2 {
		t.Errorf("EnrichmentTypes() length = %v, want 2", len(filters.EnrichmentTypes()))
	}
	if len(filters.EnrichmentSubtypes()) != 2 {
		t.Errorf("EnrichmentSubtypes() length = %v, want 2", len(filters.EnrichmentSubtypes()))
	}
	if len(filters.CommitSHAs()) != 2 {
		t.Errorf("CommitSHAs() length = %v, want 2", len(filters.CommitSHAs()))
	}
	if filters.IsEmpty() {
		t.Error("IsEmpty() should be false when filters are set")
	}
}

func TestSnippetSearchFilters_IsEmpty(t *testing.T) {
	filters := NewSnippetSearchFilters()
	if !filters.IsEmpty() {
		t.Error("IsEmpty() should be true for empty filters")
	}
}

func TestSnippetSearchFilters_SlicesAreCopied(t *testing.T) {
	types := []string{"dev"}
	filters := NewSnippetSearchFilters(WithEnrichmentTypes(types))

	returnedTypes := filters.EnrichmentTypes()
	returnedTypes[0] = "modified"

	if filters.EnrichmentTypes()[0] == "modified" {
		t.Error("EnrichmentTypes() should return a copy")
	}
}

func TestProgressState(t *testing.T) {
	state := NewProgressState(50, 100, "indexing", "Processing files...")

	if state.Current() != 50 {
		t.Errorf("Current() = %v, want 50", state.Current())
	}
	if state.Total() != 100 {
		t.Errorf("Total() = %v, want 100", state.Total())
	}
	if state.Operation() != "indexing" {
		t.Errorf("Operation() = %v, want 'indexing'", state.Operation())
	}
	if state.Message() != "Processing files..." {
		t.Errorf("Message() = %v, want 'Processing files...'", state.Message())
	}
	if state.Percentage() != 50.0 {
		t.Errorf("Percentage() = %v, want 50.0", state.Percentage())
	}
}

func TestProgressState_ZeroTotal(t *testing.T) {
	state := NewProgressState(10, 0, "op", "msg")
	if state.Percentage() != 0.0 {
		t.Errorf("Percentage() = %v, want 0.0 when total is 0", state.Percentage())
	}
}

func TestEmbeddingResponse(t *testing.T) {
	embedding := []float64{0.1, 0.2, 0.3}
	resp := NewEmbeddingResponse("snippet-1", embedding)

	if resp.SnippetID() != "snippet-1" {
		t.Errorf("SnippetID() = %v, want 'snippet-1'", resp.SnippetID())
	}

	returnedEmb := resp.Embedding()
	if len(returnedEmb) != 3 {
		t.Errorf("Embedding() length = %v, want 3", len(returnedEmb))
	}

	// Verify returned slice is a copy
	returnedEmb[0] = 999.0
	if resp.Embedding()[0] == 999.0 {
		t.Error("Embedding() should return a copy")
	}
}

func TestFusionResult(t *testing.T) {
	originalScores := []float64{0.8, 0.7}
	result := NewFusionResult("doc-1", 0.75, originalScores)

	if result.ID() != "doc-1" {
		t.Errorf("ID() = %v, want 'doc-1'", result.ID())
	}
	if result.Score() != 0.75 {
		t.Errorf("Score() = %v, want 0.75", result.Score())
	}

	returnedScores := result.OriginalScores()
	if len(returnedScores) != 2 {
		t.Errorf("OriginalScores() length = %v, want 2", len(returnedScores))
	}

	// Verify returned slice is a copy
	returnedScores[0] = 999.0
	if result.OriginalScores()[0] == 999.0 {
		t.Error("OriginalScores() should return a copy")
	}
}

func TestIndexView(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	view := NewIndexView(42, created, 1000, updated, "github.com/test/repo")

	if view.ID() != 42 {
		t.Errorf("ID() = %v, want 42", view.ID())
	}
	if !view.CreatedAt().Equal(created) {
		t.Errorf("CreatedAt() = %v, want %v", view.CreatedAt(), created)
	}
	if view.NumSnippets() != 1000 {
		t.Errorf("NumSnippets() = %v, want 1000", view.NumSnippets())
	}
	if !view.UpdatedAt().Equal(updated) {
		t.Errorf("UpdatedAt() = %v, want %v", view.UpdatedAt(), updated)
	}
	if view.Source() != "github.com/test/repo" {
		t.Errorf("Source() = %v, want 'github.com/test/repo'", view.Source())
	}
}

func TestFunctionDefinition(t *testing.T) {
	fd := NewFunctionDefinition("hello", "main.hello", 100, 200)

	if fd.Name() != "hello" {
		t.Errorf("Name() = %v, want 'hello'", fd.Name())
	}
	if fd.QualifiedName() != "main.hello" {
		t.Errorf("QualifiedName() = %v, want 'main.hello'", fd.QualifiedName())
	}
	if fd.StartByte() != 100 {
		t.Errorf("StartByte() = %v, want 100", fd.StartByte())
	}
	if fd.EndByte() != 200 {
		t.Errorf("EndByte() = %v, want 200", fd.EndByte())
	}
}

func TestLanguageMapping_ExtensionsForLanguage(t *testing.T) {
	mapping := LanguageMapping{}

	extensions, err := mapping.ExtensionsForLanguage("python")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(extensions) == 0 {
		t.Error("expected non-empty extensions for python")
	}

	// Verify "py" is in the list
	found := false
	for _, ext := range extensions {
		if ext == "py" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'py' in python extensions")
	}

	// Verify returned slice is a copy
	extensions[0] = "modified"
	newExtensions, _ := mapping.ExtensionsForLanguage("python")
	if newExtensions[0] == "modified" {
		t.Error("ExtensionsForLanguage should return a copy")
	}
}

func TestLanguageMapping_ExtensionsForLanguage_CaseInsensitive(t *testing.T) {
	mapping := LanguageMapping{}

	_, err := mapping.ExtensionsForLanguage("PYTHON")
	if err != nil {
		t.Errorf("ExtensionsForLanguage should be case-insensitive: %v", err)
	}

	_, err = mapping.ExtensionsForLanguage("Python")
	if err != nil {
		t.Errorf("ExtensionsForLanguage should be case-insensitive: %v", err)
	}
}

func TestLanguageMapping_ExtensionsForLanguage_Unsupported(t *testing.T) {
	mapping := LanguageMapping{}

	_, err := mapping.ExtensionsForLanguage("cobol")
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

func TestLanguageMapping_LanguageForExtension(t *testing.T) {
	mapping := LanguageMapping{}

	tests := []struct {
		ext      string
		expected string
	}{
		{"py", "python"},
		{".py", "python"},
		{"go", "go"},
		{".go", "go"},
		{"ts", "typescript"},
		{"jsx", "javascript"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			lang, err := mapping.LanguageForExtension(tt.ext)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if lang != tt.expected {
				t.Errorf("LanguageForExtension(%q) = %v, want %v", tt.ext, lang, tt.expected)
			}
		})
	}
}

func TestLanguageMapping_LanguageForExtension_Unsupported(t *testing.T) {
	mapping := LanguageMapping{}

	_, err := mapping.LanguageForExtension(".xyz")
	if err == nil {
		t.Error("expected error for unsupported extension")
	}
}

func TestLanguageMapping_ExtensionToLanguageMap(t *testing.T) {
	mapping := LanguageMapping{}
	extMap := mapping.ExtensionToLanguageMap()

	if extMap["py"] != "python" {
		t.Errorf("extMap[py] = %v, want 'python'", extMap["py"])
	}
	if extMap["go"] != "go" {
		t.Errorf("extMap[go] = %v, want 'go'", extMap["go"])
	}
}

func TestLanguageMapping_SupportedLanguages(t *testing.T) {
	mapping := LanguageMapping{}
	languages := mapping.SupportedLanguages()

	if len(languages) == 0 {
		t.Error("expected non-empty supported languages list")
	}

	// Check a few expected languages exist
	expectedLangs := []string{"python", "go", "javascript", "typescript"}
	for _, expected := range expectedLangs {
		found := false
		for _, lang := range languages {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in supported languages", expected)
		}
	}
}

func TestLanguageMapping_SupportedExtensions(t *testing.T) {
	mapping := LanguageMapping{}
	extensions := mapping.SupportedExtensions()

	if len(extensions) == 0 {
		t.Error("expected non-empty supported extensions list")
	}
}

func TestLanguageMapping_IsLanguageSupported(t *testing.T) {
	mapping := LanguageMapping{}

	if !mapping.IsLanguageSupported("python") {
		t.Error("python should be supported")
	}
	if !mapping.IsLanguageSupported("PYTHON") {
		t.Error("PYTHON should be supported (case insensitive)")
	}
	if mapping.IsLanguageSupported("cobol") {
		t.Error("cobol should not be supported")
	}
}

func TestLanguageMapping_IsExtensionSupported(t *testing.T) {
	mapping := LanguageMapping{}

	if !mapping.IsExtensionSupported("py") {
		t.Error("py should be supported")
	}
	if !mapping.IsExtensionSupported(".py") {
		t.Error(".py should be supported")
	}
	if mapping.IsExtensionSupported(".xyz") {
		t.Error(".xyz should not be supported")
	}
}

func TestLanguageMapping_ExtensionsWithFallback(t *testing.T) {
	mapping := LanguageMapping{}

	// Supported language returns extensions
	extensions := mapping.ExtensionsWithFallback("python")
	if len(extensions) == 0 {
		t.Error("expected extensions for python")
	}

	// Unsupported language returns itself
	extensions = mapping.ExtensionsWithFallback("cobol")
	if len(extensions) != 1 || extensions[0] != "cobol" {
		t.Errorf("expected [cobol], got %v", extensions)
	}
}

func TestMultiSearchRequest(t *testing.T) {
	filters := NewSnippetSearchFilters(WithLanguage("go"))
	keywords := []string{"error", "handler"}
	req := NewMultiSearchRequest(25, "text query", "code query", keywords, filters)

	if req.TopK() != 25 {
		t.Errorf("TopK() = %v, want 25", req.TopK())
	}
	if req.TextQuery() != "text query" {
		t.Errorf("TextQuery() = %v, want 'text query'", req.TextQuery())
	}
	if req.CodeQuery() != "code query" {
		t.Errorf("CodeQuery() = %v, want 'code query'", req.CodeQuery())
	}
	if req.Filters().Language() != "go" {
		t.Errorf("Filters().Language() = %v, want 'go'", req.Filters().Language())
	}

	returnedKw := req.Keywords()
	if len(returnedKw) != 2 {
		t.Errorf("Keywords() length = %v, want 2", len(returnedKw))
	}

	// Verify returned slice is a copy
	returnedKw[0] = "modified"
	if req.Keywords()[0] == "modified" {
		t.Error("Keywords() should return a copy")
	}
}

func TestSnippetQuery(t *testing.T) {
	filters := NewSnippetSearchFilters(WithAuthor("bob"))
	query := NewSnippetQuery("search term", SearchTypeHybrid, filters, 15)

	if query.Text() != "search term" {
		t.Errorf("Text() = %v, want 'search term'", query.Text())
	}
	if query.SearchType() != SearchTypeHybrid {
		t.Errorf("SearchType() = %v, want hybrid", query.SearchType())
	}
	if query.Filters().Author() != "bob" {
		t.Errorf("Filters().Author() = %v, want 'bob'", query.Filters().Author())
	}
	if query.TopK() != 15 {
		t.Errorf("TopK() = %v, want 15", query.TopK())
	}
}
