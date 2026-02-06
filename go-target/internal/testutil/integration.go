// Package testutil provides common test utilities and fixtures.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/tracking"
)

// TestSchema contains the SQL to create all tables for integration tests.
const TestSchema = `
CREATE TABLE IF NOT EXISTS git_repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sanitized_remote_uri TEXT NOT NULL UNIQUE,
    remote_uri TEXT NOT NULL,
    cloned_path TEXT,
    last_scanned_at DATETIME,
    num_commits INTEGER DEFAULT 0,
    num_branches INTEGER DEFAULT 0,
    num_tags INTEGER DEFAULT 0,
    tracking_type TEXT DEFAULT '',
    tracking_name TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS git_commits (
    commit_sha TEXT PRIMARY KEY,
    repo_id INTEGER NOT NULL,
    date DATETIME NOT NULL,
    message TEXT,
    parent_commit_sha TEXT,
    author TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_branches (
    repo_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    head_commit_sha TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repo_id, name),
    FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_tags (
    repo_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    target_commit_sha TEXT NOT NULL,
    message TEXT,
    tagger_name TEXT,
    tagger_email TEXT,
    tagged_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repo_id, name),
    FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_commit_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    commit_sha TEXT NOT NULL,
    path TEXT NOT NULL,
    blob_sha TEXT NOT NULL,
    mime_type TEXT DEFAULT '',
    extension TEXT DEFAULT '',
    size INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (commit_sha, path)
);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dedup_key TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    payload TEXT,
    priority INTEGER NOT NULL DEFAULT 100,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_status (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    operation TEXT NOT NULL,
    trackable_id INTEGER,
    trackable_type TEXT,
    parent TEXT,
    message TEXT DEFAULT '',
    state TEXT DEFAULT '',
    error TEXT DEFAULT '',
    total INTEGER DEFAULT 0,
    current INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS snippets_v2 (
    sha TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    extension TEXT NOT NULL,
    snippet_type TEXT NOT NULL,
    name TEXT DEFAULT '',
    language TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS commit_index (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    commit_sha TEXT NOT NULL UNIQUE,
    repo_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    snippet_count INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS commit_snippet_associations (
    commit_sha TEXT NOT NULL,
    snippet_sha TEXT NOT NULL,
    file_path TEXT NOT NULL,
    start_line INTEGER DEFAULT 0,
    end_line INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commit_sha, snippet_sha, file_path)
);

CREATE TABLE IF NOT EXISTS enrichments_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    subtype TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enrichment_associations (
    enrichment_id INTEGER NOT NULL,
    snippet_sha TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (enrichment_id, snippet_sha),
    FOREIGN KEY (enrichment_id) REFERENCES enrichments_v2(id)
);
`

// GitTestRepo provides a real Git repository for integration tests.
// It creates a temporary directory with an initialized Git repo.
type GitTestRepo struct {
	path   string
	repo   *git.Repository
	t      *testing.T
	author *object.Signature
}

// NewGitTestRepo creates a new test Git repository in a temporary directory.
func NewGitTestRepo(t *testing.T) *GitTestRepo {
	t.Helper()

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("init git repo: %v", err)
	}

	gtr := &GitTestRepo{
		path: repoPath,
		repo: repo,
		t:    t,
		author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}

	return gtr
}

// Path returns the path to the repository.
func (g *GitTestRepo) Path() string {
	return g.path
}

// WriteFile writes a file to the repository at the given relative path.
func (g *GitTestRepo) WriteFile(relativePath, content string) {
	g.t.Helper()

	fullPath := filepath.Join(g.path, relativePath)

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		g.t.Fatalf("create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		g.t.Fatalf("write file %s: %v", relativePath, err)
	}
}

// StageFile stages a file for commit.
func (g *GitTestRepo) StageFile(relativePath string) {
	g.t.Helper()

	wt, err := g.repo.Worktree()
	if err != nil {
		g.t.Fatalf("get worktree: %v", err)
	}

	if _, err := wt.Add(relativePath); err != nil {
		g.t.Fatalf("stage file %s: %v", relativePath, err)
	}
}

// Commit creates a commit with the staged files and returns the commit info.
func (g *GitTestRepo) Commit(message string) CommitResult {
	g.t.Helper()

	wt, err := g.repo.Worktree()
	if err != nil {
		g.t.Fatalf("get worktree: %v", err)
	}

	// Stage all changes
	if _, err := wt.Add("."); err != nil {
		g.t.Fatalf("stage all: %v", err)
	}

	hash, err := wt.Commit(message, &git.CommitOptions{
		Author: g.author,
	})
	if err != nil {
		g.t.Fatalf("commit: %v", err)
	}

	return CommitResult{
		SHA:     hash.String(),
		Message: message,
	}
}

// CreateBranch creates a new branch pointing to the current HEAD.
func (g *GitTestRepo) CreateBranch(name string) {
	g.t.Helper()

	headRef, err := g.repo.Head()
	if err != nil {
		g.t.Fatalf("get head: %v", err)
	}

	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), headRef.Hash())
	if err := g.repo.Storer.SetReference(ref); err != nil {
		g.t.Fatalf("create branch %s: %v", name, err)
	}
}

// CreateTag creates a new tag pointing to the current HEAD.
func (g *GitTestRepo) CreateTag(name string) {
	g.t.Helper()

	headRef, err := g.repo.Head()
	if err != nil {
		g.t.Fatalf("get head: %v", err)
	}

	_, err = g.repo.CreateTag(name, headRef.Hash(), nil)
	if err != nil {
		g.t.Fatalf("create tag %s: %v", name, err)
	}
}

// CreateAnnotatedTag creates an annotated tag with a message.
func (g *GitTestRepo) CreateAnnotatedTag(name, message string) {
	g.t.Helper()

	headRef, err := g.repo.Head()
	if err != nil {
		g.t.Fatalf("get head: %v", err)
	}

	_, err = g.repo.CreateTag(name, headRef.Hash(), &git.CreateTagOptions{
		Tagger:  g.author,
		Message: message,
	})
	if err != nil {
		g.t.Fatalf("create annotated tag %s: %v", name, err)
	}
}

// HeadSHA returns the SHA of HEAD.
func (g *GitTestRepo) HeadSHA() string {
	g.t.Helper()

	headRef, err := g.repo.Head()
	if err != nil {
		g.t.Fatalf("get head: %v", err)
	}

	return headRef.Hash().String()
}

// CommitResult holds information about a created commit.
type CommitResult struct {
	SHA     string
	Message string
}

// FakeEmbedder generates deterministic embeddings for testing.
// Embeddings are based on text characteristics for reproducibility.
type FakeEmbedder struct {
	mu         sync.RWMutex
	dimension  int
	embeddings map[string][]float64
}

// NewFakeEmbedder creates a new fake embedder with the given dimension.
func NewFakeEmbedder(dimension int) *FakeEmbedder {
	return &FakeEmbedder{
		dimension:  dimension,
		embeddings: make(map[string][]float64),
	}
}

// Embed generates a deterministic embedding for the given text.
func (f *FakeEmbedder) Embed(text string) []float64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	if emb, ok := f.embeddings[text]; ok {
		return emb
	}

	// Generate deterministic embedding based on text
	emb := make([]float64, f.dimension)
	for i := range f.dimension {
		// Use text hash and position to generate values
		hash := 0
		for j, c := range text {
			hash += int(c) * (j + 1)
		}
		emb[i] = float64((hash+i)%100) / 100.0
	}

	f.embeddings[text] = emb
	return emb
}

// SetEmbedding sets a specific embedding for a text.
func (f *FakeEmbedder) SetEmbedding(text string, embedding []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.embeddings[text] = embedding
}

// Dimension returns the embedding dimension.
func (f *FakeEmbedder) Dimension() int {
	return f.dimension
}

// FakeBM25Repository is a controllable BM25 repository for testing.
type FakeBM25Repository struct {
	mu      sync.RWMutex
	indexed map[string]string // snippetID -> text
	results []domain.SearchResult
}

// NewFakeBM25Repository creates a new fake BM25 repository.
func NewFakeBM25Repository() *FakeBM25Repository {
	return &FakeBM25Repository{
		indexed: make(map[string]string),
		results: make([]domain.SearchResult, 0),
	}
}

// Index stores documents for later search.
func (f *FakeBM25Repository) Index(_ context.Context, req domain.IndexRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, doc := range req.Documents() {
		f.indexed[doc.SnippetID()] = doc.Text()
	}
	return nil
}

// Search returns configured results or empty slice.
func (f *FakeBM25Repository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]domain.SearchResult, len(f.results))
	copy(result, f.results)
	return result, nil
}

// Delete removes documents from the index.
func (f *FakeBM25Repository) Delete(_ context.Context, req domain.DeleteRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, id := range req.SnippetIDs() {
		delete(f.indexed, id)
	}
	return nil
}

// SetResults configures the results returned by Search.
func (f *FakeBM25Repository) SetResults(results []domain.SearchResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = results
}

// IndexedDocuments returns the indexed documents.
func (f *FakeBM25Repository) IndexedDocuments() map[string]string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range f.indexed {
		result[k] = v
	}
	return result
}

// FakeVectorRepository is a controllable vector repository for testing.
type FakeVectorRepository struct {
	mu         sync.RWMutex
	embeddings map[string]indexing.EmbeddingInfo
	results    []domain.SearchResult
}

// NewFakeVectorRepository creates a new fake vector repository.
func NewFakeVectorRepository() *FakeVectorRepository {
	return &FakeVectorRepository{
		embeddings: make(map[string]indexing.EmbeddingInfo),
		results:    make([]domain.SearchResult, 0),
	}
}

// Index stores documents (no-op for fake).
func (f *FakeVectorRepository) Index(_ context.Context, _ domain.IndexRequest) error {
	return nil
}

// Search returns configured results or empty slice.
func (f *FakeVectorRepository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]domain.SearchResult, len(f.results))
	copy(result, f.results)
	return result, nil
}

// HasEmbedding checks if an embedding exists.
func (f *FakeVectorRepository) HasEmbedding(_ context.Context, snippetID string, embType indexing.EmbeddingType) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	info, ok := f.embeddings[snippetID]
	if !ok {
		return false, nil
	}
	return info.Type() == embType, nil
}

// Delete removes embeddings.
func (f *FakeVectorRepository) Delete(_ context.Context, req domain.DeleteRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, id := range req.SnippetIDs() {
		delete(f.embeddings, id)
	}
	return nil
}

// EmbeddingsForSnippets returns embeddings for the given snippet IDs.
func (f *FakeVectorRepository) EmbeddingsForSnippets(_ context.Context, snippetIDs []string) ([]indexing.EmbeddingInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var result []indexing.EmbeddingInfo
	for _, id := range snippetIDs {
		if info, ok := f.embeddings[id]; ok {
			result = append(result, info)
		}
	}
	return result, nil
}

// SetResults configures the results returned by Search.
func (f *FakeVectorRepository) SetResults(results []domain.SearchResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = results
}

// AddEmbedding adds an embedding for a snippet.
func (f *FakeVectorRepository) AddEmbedding(snippetID string, embType indexing.EmbeddingType, embedding []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.embeddings[snippetID] = indexing.NewEmbeddingInfo(snippetID, embType, embedding)
}

// FakeSnippetRepository is a controllable snippet repository for testing.
type FakeSnippetRepository struct {
	mu             sync.RWMutex
	snippets       map[string]indexing.Snippet // SHA -> Snippet
	commitSnippets map[string][]string         // commitSHA -> []snippetSHA
}

// NewFakeSnippetRepository creates a new fake snippet repository.
func NewFakeSnippetRepository() *FakeSnippetRepository {
	return &FakeSnippetRepository{
		snippets:       make(map[string]indexing.Snippet),
		commitSnippets: make(map[string][]string),
	}
}

// Save stores snippets for a commit.
func (f *FakeSnippetRepository) Save(_ context.Context, commitSHA string, snippets []indexing.Snippet) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	shas := make([]string, 0, len(snippets))
	for _, s := range snippets {
		f.snippets[s.SHA()] = s
		shas = append(shas, s.SHA())
	}
	f.commitSnippets[commitSHA] = shas
	return nil
}

// SnippetsForCommit returns snippets for a commit.
func (f *FakeSnippetRepository) SnippetsForCommit(_ context.Context, commitSHA string) ([]indexing.Snippet, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	shas, ok := f.commitSnippets[commitSHA]
	if !ok {
		return []indexing.Snippet{}, nil
	}

	result := make([]indexing.Snippet, 0, len(shas))
	for _, sha := range shas {
		if s, exists := f.snippets[sha]; exists {
			result = append(result, s)
		}
	}
	return result, nil
}

// DeleteForCommit removes snippets for a commit.
func (f *FakeSnippetRepository) DeleteForCommit(_ context.Context, commitSHA string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	shas, ok := f.commitSnippets[commitSHA]
	if !ok {
		return nil
	}

	for _, sha := range shas {
		delete(f.snippets, sha)
	}
	delete(f.commitSnippets, commitSHA)
	return nil
}

// Search returns matching snippets (basic implementation).
func (f *FakeSnippetRepository) Search(_ context.Context, _ domain.MultiSearchRequest) ([]indexing.Snippet, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]indexing.Snippet, 0, len(f.snippets))
	for _, s := range f.snippets {
		result = append(result, s)
	}
	return result, nil
}

// ByIDs returns snippets by their IDs (SHAs).
func (f *FakeSnippetRepository) ByIDs(_ context.Context, ids []string) ([]indexing.Snippet, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]indexing.Snippet, 0, len(ids))
	for _, id := range ids {
		if s, ok := f.snippets[id]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}

// BySHA returns a single snippet by its SHA.
func (f *FakeSnippetRepository) BySHA(_ context.Context, sha string) (indexing.Snippet, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if s, ok := f.snippets[sha]; ok {
		return s, nil
	}
	return indexing.Snippet{}, nil
}

// AddSnippet adds a snippet directly (for test setup).
func (f *FakeSnippetRepository) AddSnippet(snippet indexing.Snippet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snippets[snippet.SHA()] = snippet
}

// ProgressEvent records a progress tracking event.
type ProgressEvent struct {
	Operation queue.TaskOperation
	State     domain.ReportingState
	Current   int
	Total     int
	Message   string
	Error     string
}

// FakeProgressTracker records progress events for assertions.
type FakeProgressTracker struct {
	mu     sync.Mutex
	events []ProgressEvent
}

// NewFakeProgressTracker creates a new fake progress tracker.
func NewFakeProgressTracker() *FakeProgressTracker {
	return &FakeProgressTracker{
		events: make([]ProgressEvent, 0),
	}
}

// Events returns all recorded events.
func (f *FakeProgressTracker) Events() []ProgressEvent {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make([]ProgressEvent, len(f.events))
	copy(result, f.events)
	return result
}

// Clear removes all recorded events.
func (f *FakeProgressTracker) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = make([]ProgressEvent, 0)
}

// OnChange implements tracking.Reporter.
func (f *FakeProgressTracker) OnChange(_ context.Context, status queue.TaskStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.events = append(f.events, ProgressEvent{
		Operation: status.Operation(),
		State:     status.State(),
		Current:   status.Current(),
		Total:     status.Total(),
		Message:   status.Message(),
		Error:     status.Error(),
	})
	return nil
}

// FakeTrackerFactory creates trackers that report to a FakeProgressTracker.
type FakeTrackerFactory struct {
	reporter *FakeProgressTracker
}

// NewFakeTrackerFactory creates a new factory with a shared reporter.
func NewFakeTrackerFactory() *FakeTrackerFactory {
	return &FakeTrackerFactory{
		reporter: NewFakeProgressTracker(),
	}
}

// ForOperation creates a tracker for the given operation.
func (f *FakeTrackerFactory) ForOperation(
	operation queue.TaskOperation,
	trackableType domain.TrackableType,
	trackableID int64,
) *tracking.Tracker {
	tracker := tracking.TrackerForOperation(operation, nil, trackableType, trackableID)
	tracker.Subscribe(f.reporter)
	return tracker
}

// Events returns all recorded events from all trackers.
func (f *FakeTrackerFactory) Events() []ProgressEvent {
	return f.reporter.Events()
}

// Clear removes all recorded events.
func (f *FakeTrackerFactory) Clear() {
	f.reporter.Clear()
}

// FakeGitAdapter is a controllable Git adapter for testing.
type FakeGitAdapter struct {
	mu           sync.RWMutex
	branches     map[string][]BranchInfoFake
	commits      map[string]CommitInfoFake
	files        map[string][]FileInfoFake
	tags         map[string][]TagInfoFake
	cloneErr error
	fetchErr error
}

// BranchInfoFake holds fake branch data.
type BranchInfoFake struct {
	Name      string
	HeadSHA   string
	IsDefault bool
}

// CommitInfoFake holds fake commit data.
type CommitInfoFake struct {
	SHA           string
	Message       string
	AuthorName    string
	AuthorEmail   string
	CommitterName string
	CommitterEmail string
	AuthoredAt    time.Time
	CommittedAt   time.Time
	ParentSHA     string
}

// FileInfoFake holds fake file data.
type FileInfoFake struct {
	Path    string
	BlobSHA string
	Size    int64
}

// TagInfoFake holds fake tag data.
type TagInfoFake struct {
	Name            string
	TargetCommitSHA string
	Message         string
	TaggerName      string
	TaggerEmail     string
	TaggedAt        time.Time
}

// NewFakeGitAdapter creates a new fake Git adapter.
func NewFakeGitAdapter() *FakeGitAdapter {
	return &FakeGitAdapter{
		branches: make(map[string][]BranchInfoFake),
		commits:  make(map[string]CommitInfoFake),
		files:    make(map[string][]FileInfoFake),
		tags:     make(map[string][]TagInfoFake),
	}
}

// SetBranches sets the branches for a repository path.
func (f *FakeGitAdapter) SetBranches(path string, branches []BranchInfoFake) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.branches[path] = branches
}

// SetCommit sets a commit for a given SHA.
func (f *FakeGitAdapter) SetCommit(sha string, commit CommitInfoFake) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits[sha] = commit
}

// SetFiles sets the files for a commit.
func (f *FakeGitAdapter) SetFiles(commitSHA string, files []FileInfoFake) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[commitSHA] = files
}

// SetTags sets the tags for a repository path.
func (f *FakeGitAdapter) SetTags(path string, tags []TagInfoFake) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags[path] = tags
}

// SetCloneError sets an error to return from clone operations.
func (f *FakeGitAdapter) SetCloneError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cloneErr = err
}

// SetFetchError sets an error to return from fetch operations.
func (f *FakeGitAdapter) SetFetchError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchErr = err
}

// These methods satisfy the git.Adapter interface minimally for testing.
// Full implementations would be added as needed.

// CloneRepository simulates cloning.
func (f *FakeGitAdapter) CloneRepository(_ context.Context, _ string, _ string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cloneErr
}

// CheckoutCommit simulates checkout.
func (f *FakeGitAdapter) CheckoutCommit(_ context.Context, _ string, _ string) error {
	return nil
}

// CheckoutBranch simulates branch checkout.
func (f *FakeGitAdapter) CheckoutBranch(_ context.Context, _ string, _ string) error {
	return nil
}

// FetchRepository simulates fetch.
func (f *FakeGitAdapter) FetchRepository(_ context.Context, _ string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fetchErr
}

// PullRepository simulates pull.
func (f *FakeGitAdapter) PullRepository(_ context.Context, _ string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fetchErr
}

// AllBranches returns configured branches.
func (f *FakeGitAdapter) AllBranches(_ context.Context, path string) ([]BranchInfoFake, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	branches, ok := f.branches[path]
	if !ok {
		return []BranchInfoFake{}, nil
	}
	result := make([]BranchInfoFake, len(branches))
	copy(result, branches)
	return result, nil
}

// CommitDetails returns configured commit info.
func (f *FakeGitAdapter) CommitDetails(_ context.Context, _ string, sha string) (CommitInfoFake, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	commit, ok := f.commits[sha]
	if !ok {
		return CommitInfoFake{}, fmt.Errorf("commit not found: %s", sha)
	}
	return commit, nil
}

// CommitFiles returns configured files.
func (f *FakeGitAdapter) CommitFiles(_ context.Context, _ string, sha string) ([]FileInfoFake, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	files, ok := f.files[sha]
	if !ok {
		return []FileInfoFake{}, nil
	}
	result := make([]FileInfoFake, len(files))
	copy(result, files)
	return result, nil
}

// AllTags returns configured tags.
func (f *FakeGitAdapter) AllTags(_ context.Context, path string) ([]TagInfoFake, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	tags, ok := f.tags[path]
	if !ok {
		return []TagInfoFake{}, nil
	}
	result := make([]TagInfoFake, len(tags))
	copy(result, tags)
	return result, nil
}
