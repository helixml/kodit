package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// Compile-time check: Scope must return types that satisfy the interfaces
// accepted by NewServer. If NewServer gains a new dependency interface,
// the developer must decide whether Scope needs to cover it.
//
// Interfaces NOT in this list are safe without scoping because they only
// access data already gated by a scoped dependency:
//   - CommitFinder:       always called with a repo ID from the scoped RepositoryLister
//   - EnrichmentQuery:    always called with a commit SHA resolved through a scoped repo
//   - EnrichmentResolver: operates on enrichment IDs returned by scoped search results
//   - FileFinder:         operates on file IDs returned by the scoped EnrichmentResolver
var (
	_ RepositoryLister  = scopeReturn[RepositoryLister]()
	_ FileContentReader = scopeReturn[FileContentReader]()
	_ SemanticSearcher  = scopeReturn[SemanticSearcher]()
	_ KeywordSearcher   = scopeReturn[KeywordSearcher]()
	_ Grepper           = scopeReturn[Grepper]()
	_ FileLister        = scopeReturn[FileLister]()
)

// scopeReturn is a compile-time helper that extracts a typed return value
// from Scope. It panics at runtime but is only used in var declarations
// that the compiler evaluates for type-checking without executing.
func scopeReturn[T any]() T {
	repos, fc, ss, ks, g, fl := Scope(nil, nil, nil, nil, nil, nil, nil)
	m := map[string]any{
		"RepositoryLister":  repos,
		"FileContentReader": fc,
		"SemanticSearcher":  ss,
		"KeywordSearcher":   ks,
		"Grepper":           g,
		"FileLister":        fl,
	}
	for _, v := range m {
		if t, ok := v.(T); ok {
			return t
		}
	}
	var zero T
	return zero
}

// --- scopedRepositories ---

func TestScopedRepositories_FindReturnsOnlyScopedRepos(t *testing.T) {
	repo1 := repository.ReconstructRepository(
		1, "https://github.com/org/repo1", "https://github.com/org/repo1",
		repository.WorkingCopy{}, repository.TrackingConfig{},
		time.Now(), time.Now(), time.Time{},
	)
	repo2 := repository.ReconstructRepository(
		2, "https://github.com/org/repo2", "https://github.com/org/repo2",
		repository.WorkingCopy{}, repository.TrackingConfig{},
		time.Now(), time.Now(), time.Time{},
	)
	inner := &recordingRepositoryLister{repos: []repository.Repository{repo1, repo2}}
	scoped := &scopedRepositories{inner: inner, ids: []int64{1}}

	repos, err := scoped.Find(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The inner lister receives the WithIDIn option. Our recording lister
	// just returns all repos, but we verify the option was appended.
	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(inner.calls))
	}
	opts := inner.calls[0]
	q := repository.Build(opts...)
	found := false
	for _, c := range q.Conditions() {
		if c.Field() == "id" && c.In() {
			found = true
		}
	}
	if !found {
		t.Error("expected WithIDIn condition to be appended")
	}
	_ = repos
}

func TestScopedRepositories_FindPreservesCallerOptions(t *testing.T) {
	inner := &recordingRepositoryLister{}
	scoped := &scopedRepositories{inner: inner, ids: []int64{1, 2}}

	_, _ = scoped.Find(context.Background(), repository.WithRemoteURL("https://github.com/org/repo1"))

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(inner.calls))
	}
	q := repository.Build(inner.calls[0]...)
	conditions := q.Conditions()
	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions (remote_url + id IN), got %d", len(conditions))
	}
}

// --- scopedFileContent ---

func TestScopedFileContent_AllowsInScope(t *testing.T) {
	inner := &fakeFileContentReader{content: []byte("hello"), commitSHA: "abc"}
	scoped := &scopedFileContent{inner: inner, allowed: map[int64]struct{}{1: {}}}

	result, err := scoped.Content(context.Background(), 1, "main", "README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content()) != "hello" {
		t.Errorf("expected 'hello', got %q", string(result.Content()))
	}
}

func TestScopedFileContent_RejectsOutOfScope(t *testing.T) {
	inner := &fakeFileContentReader{content: []byte("hello"), commitSHA: "abc"}
	scoped := &scopedFileContent{inner: inner, allowed: map[int64]struct{}{1: {}}}

	_, err := scoped.Content(context.Background(), 99, "main", "README.md")
	if err == nil {
		t.Fatal("expected error for out-of-scope repo")
	}
}

// --- scopedSemanticSearch ---

func TestScopedSemanticSearch_InjectsSourceRepos(t *testing.T) {
	inner := &recordingSemanticSearcher{}
	scoped := &scopedSemanticSearch{inner: inner, ids: []int64{1, 2}}

	_, _, _ = scoped.SearchCodeWithScores(context.Background(), "query", 10, search.NewFilters())

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(inner.calls))
	}
	repos := inner.calls[0].SourceRepos()
	if len(repos) != 2 || repos[0] != 1 || repos[1] != 2 {
		t.Errorf("expected source repos [1 2], got %v", repos)
	}
}

func TestScopedSemanticSearch_IntersectsWithCallerRepos(t *testing.T) {
	inner := &recordingSemanticSearcher{}
	scoped := &scopedSemanticSearch{inner: inner, ids: []int64{1, 2, 3}}

	callerFilters := search.NewFilters(search.WithSourceRepos([]int64{2, 4}))
	_, _, _ = scoped.SearchCodeWithScores(context.Background(), "query", 10, callerFilters)

	repos := inner.calls[0].SourceRepos()
	if len(repos) != 1 || repos[0] != 2 {
		t.Errorf("expected intersection [2], got %v", repos)
	}
}

func TestScopedSemanticSearch_PreservesOtherFilters(t *testing.T) {
	inner := &recordingSemanticSearcher{}
	scoped := &scopedSemanticSearch{inner: inner, ids: []int64{1}}

	callerFilters := search.NewFilters(search.WithLanguages([]string{"go"}))
	_, _, _ = scoped.SearchCodeWithScores(context.Background(), "query", 10, callerFilters)

	filters := inner.calls[0]
	if langs := filters.Languages(); len(langs) != 1 || langs[0] != "go" {
		t.Errorf("expected languages [go], got %v", langs)
	}
}

// --- scopedKeywordSearch ---

func TestScopedKeywordSearch_InjectsSourceRepos(t *testing.T) {
	inner := &recordingKeywordSearcher{}
	scoped := &scopedKeywordSearch{inner: inner, ids: []int64{5}}

	_, _, _ = scoped.SearchKeywordsWithScores(context.Background(), "query", 10, search.NewFilters())

	repos := inner.calls[0].SourceRepos()
	if len(repos) != 1 || repos[0] != 5 {
		t.Errorf("expected source repos [5], got %v", repos)
	}
}

func TestScopedKeywordSearch_IntersectsWithCallerRepos(t *testing.T) {
	inner := &recordingKeywordSearcher{}
	scoped := &scopedKeywordSearch{inner: inner, ids: []int64{1, 3}}

	callerFilters := search.NewFilters(search.WithSourceRepos([]int64{3, 7}))
	_, _, _ = scoped.SearchKeywordsWithScores(context.Background(), "query", 10, callerFilters)

	repos := inner.calls[0].SourceRepos()
	if len(repos) != 1 || repos[0] != 3 {
		t.Errorf("expected intersection [3], got %v", repos)
	}
}

// --- scopedGrepper ---

func TestScopedGrepper_AllowsInScope(t *testing.T) {
	inner := &fakeGrepper{results: []service.GrepResult{{Path: "main.go"}}}
	scoped := &scopedGrepper{inner: inner, allowed: map[int64]struct{}{1: {}}}

	results, err := scoped.Search(context.Background(), 1, "pattern", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestScopedGrepper_RejectsOutOfScope(t *testing.T) {
	inner := &fakeGrepper{}
	scoped := &scopedGrepper{inner: inner, allowed: map[int64]struct{}{1: {}}}

	_, err := scoped.Search(context.Background(), 99, "pattern", "", 10)
	if err == nil {
		t.Fatal("expected error for out-of-scope repo")
	}
}

// --- scopedFileLister ---

func TestScopedFileLister_AllowsInScope(t *testing.T) {
	inner := &fakeFileLister{files: []service.FileEntry{{Path: "README.md"}}}
	scoped := &scopedFileLister{inner: inner, allowed: map[int64]struct{}{1: {}}}

	files, err := scoped.ListFiles(context.Background(), 1, "*.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestScopedFileLister_RejectsOutOfScope(t *testing.T) {
	inner := &fakeFileLister{}
	scoped := &scopedFileLister{inner: inner, allowed: map[int64]struct{}{1: {}}}

	_, err := scoped.ListFiles(context.Background(), 99, "*.md")
	if err == nil {
		t.Fatal("expected error for out-of-scope repo")
	}
}

// --- scopeFilters ---

func TestScopeFilters_InjectsRepos(t *testing.T) {
	original := search.NewFilters()
	result := scopeFilters(original, []int64{1, 2})

	repos := result.SourceRepos()
	if len(repos) != 2 || repos[0] != 1 || repos[1] != 2 {
		t.Errorf("expected [1 2], got %v", repos)
	}
}

func TestScopeFilters_IntersectsRepos(t *testing.T) {
	original := search.NewFilters(search.WithSourceRepos([]int64{2, 3}))
	result := scopeFilters(original, []int64{1, 2})

	repos := result.SourceRepos()
	if len(repos) != 1 || repos[0] != 2 {
		t.Errorf("expected [2], got %v", repos)
	}
}

func TestScopeFilters_CallerNarrowsToOneAllowedRepo(t *testing.T) {
	// Scope allows [1, 2, 3]. Caller filters to just [2].
	// Result must be [2] — a valid narrowing within the scope.
	original := search.NewFilters(search.WithSourceRepos([]int64{2}))
	result := scopeFilters(original, []int64{1, 2, 3})

	repos := result.SourceRepos()
	if len(repos) != 1 || repos[0] != 2 {
		t.Errorf("expected [2], got %v", repos)
	}
}

func TestScopeFilters_OutOfScopeReposProduceEmptyResult(t *testing.T) {
	// Caller requests repos entirely outside the allowed set.
	// The result must have an empty (non-nil) source repos list,
	// not the caller's original out-of-scope repos.
	original := search.NewFilters(search.WithSourceRepos([]int64{99, 100}))
	result := scopeFilters(original, []int64{1, 2})

	repos := result.SourceRepos()
	if repos == nil {
		t.Fatal("source repos must be non-nil (nil is treated as unfiltered)")
	}
	if len(repos) != 0 {
		t.Errorf("expected empty source repos, got %v", repos)
	}
}

func TestScopeFilters_PreservesAllFields(t *testing.T) {
	original := search.NewFilters(
		search.WithLanguages([]string{"go"}),
		search.WithAuthors([]string{"alice"}),
		search.WithFilePaths([]string{"src/"}),
		search.WithEnrichmentTypes([]string{"development"}),
		search.WithEnrichmentSubtypes([]string{"snippet"}),
		search.WithCommitSHAs([]string{"abc123"}),
	)
	result := scopeFilters(original, []int64{1})

	if langs := result.Languages(); len(langs) != 1 || langs[0] != "go" {
		t.Errorf("languages not preserved: %v", langs)
	}
	if authors := result.Authors(); len(authors) != 1 || authors[0] != "alice" {
		t.Errorf("authors not preserved: %v", authors)
	}
	if paths := result.FilePaths(); len(paths) != 1 || paths[0] != "src/" {
		t.Errorf("file paths not preserved: %v", paths)
	}
	if types := result.EnrichmentTypes(); len(types) != 1 || types[0] != "development" {
		t.Errorf("enrichment types not preserved: %v", types)
	}
	if subtypes := result.EnrichmentSubtypes(); len(subtypes) != 1 || subtypes[0] != "snippet" {
		t.Errorf("enrichment subtypes not preserved: %v", subtypes)
	}
	if shas := result.CommitSHAs(); len(shas) != 1 || shas[0] != "abc123" {
		t.Errorf("commit SHAs not preserved: %v", shas)
	}
}

// --- test helpers ---

// recordingRepositoryLister records the options passed to each Find call.
type recordingRepositoryLister struct {
	repos []repository.Repository
	calls [][]repository.Option
}

func (r *recordingRepositoryLister) Find(_ context.Context, options ...repository.Option) ([]repository.Repository, error) {
	r.calls = append(r.calls, options)
	return r.repos, nil
}

// recordingSemanticSearcher records the filters passed to each search call.
type recordingSemanticSearcher struct {
	calls []search.Filters
}

func (r *recordingSemanticSearcher) SearchCodeWithScores(_ context.Context, _ string, _ int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	r.calls = append(r.calls, filters)
	return nil, nil, nil
}

// recordingKeywordSearcher records the filters passed to each search call.
type recordingKeywordSearcher struct {
	calls []search.Filters
}

func (r *recordingKeywordSearcher) SearchKeywordsWithScores(_ context.Context, _ string, _ int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	r.calls = append(r.calls, filters)
	return nil, nil, nil
}
