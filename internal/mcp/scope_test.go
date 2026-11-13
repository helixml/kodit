package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/infrastructure/extraction"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/rasterization"
)

// The two-repository fixture: a scoped server may see repository A only.
// Everything that belongs to repository B carries the marker "SECRET-B", so a
// response that reaches B is visible in the response text, and every backend
// call that names B is recorded by twoRepoWorld.
const (
	repoAID   int64 = 1
	repoBID   int64 = 2
	repoAURL        = "https://example.com/org/repo-a"
	repoBURL        = "https://example.com/org/repo-b"
	commitA         = "aaaa111122223333"
	commitB         = "bbbb444455556666"
	enrichA   int64 = 10
	enrichB   int64 = 20
	fileA     int64 = 100
	fileB     int64 = 200
	markerB         = "SECRET-B"
	contentA        = "repo-A-content"
	allowedID       = repoAID
)

// twoRepoWorld holds in-memory backends for repositories A and B. Each
// backend honours the repository options it receives, the way the real
// stores do, and records every call that would touch repository B.
type twoRepoWorld struct {
	mu       sync.Mutex
	touchesB []string
}

func (w *twoRepoWorld) touch(repoID int64, what string) {
	if repoID != repoBID {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.touchesB = append(w.touchesB, what)
}

func (w *twoRepoWorld) reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.touchesB = nil
}

func (w *twoRepoWorld) touchedB() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.touchesB...)
}

func repoIDForSHA(sha string) int64 {
	switch sha {
	case commitA:
		return repoAID
	case commitB:
		return repoBID
	}
	return 0
}

func worldRepos() []repository.Repository {
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id int64, url string) repository.Repository {
		return repository.ReconstructRepository(
			id, 0, url, url, "",
			repository.WorkingCopy{}, repository.NewTrackingConfigForBranch("main"),
			repository.DefaultChunkingConfig(), at, at, time.Time{},
		)
	}
	return []repository.Repository{mk(repoAID, repoAURL), mk(repoBID, repoBURL)}
}

func worldCommits() []repository.Commit {
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	author := repository.NewAuthor("Test", "test@example.com")
	return []repository.Commit{
		repository.ReconstructCommit(1, commitA, repoAID, "a", author, author, at, at, at, ""),
		repository.ReconstructCommit(2, commitB, repoBID, "b", author, author, at, at, at, ""),
	}
}

// matchesConditions reports whether an entity with the given column values
// satisfies every equality and IN condition in options. Conditions on
// columns the entity does not have are ignored.
func matchesConditions(columns map[string]any, options []repository.Option) bool {
	for _, c := range repository.Build(options...).Conditions() {
		value, ok := columns[c.Field()]
		if !ok {
			continue
		}
		if !c.In() {
			if c.Value() != value {
				return false
			}
			continue
		}
		ids, isIDs := c.Value().([]int64)
		if !isIDs {
			continue
		}
		found := false
		for _, id := range ids {
			if id == value {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type worldRepositories struct{ w *twoRepoWorld }

func (r worldRepositories) Find(_ context.Context, options ...repository.Option) ([]repository.Repository, error) {
	var out []repository.Repository
	for _, repo := range worldRepos() {
		cols := map[string]any{
			"id":                   repo.ID(),
			"sanitized_remote_uri": repo.SanitizedURL(),
			"upstream_url":         repo.UpstreamURL(),
		}
		if matchesConditions(cols, options) {
			r.w.touch(repo.ID(), "repositories.Find")
			out = append(out, repo)
		}
	}
	return out, nil
}

type worldCommitFinder struct{ w *twoRepoWorld }

func (c worldCommitFinder) Find(_ context.Context, options ...repository.Option) ([]repository.Commit, error) {
	var out []repository.Commit
	for _, commit := range worldCommits() {
		cols := map[string]any{"repo_id": commit.RepoID(), "commit_sha": commit.SHA()}
		if matchesConditions(cols, options) {
			c.w.touch(commit.RepoID(), "commits.Find")
			out = append(out, commit)
		}
	}
	return out, nil
}

type worldEnrichmentQuery struct{ w *twoRepoWorld }

func (q worldEnrichmentQuery) List(_ context.Context, params *service.EnrichmentListParams) ([]enrichment.Enrichment, error) {
	repoID := repoIDForSHA(params.CommitSHA)
	q.w.touch(repoID, "enrichments.List")
	content := contentA
	if repoID == repoBID {
		content = markerB
	}
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []enrichment.Enrichment{enrichment.ReconstructEnrichment(
		1, *params.Type, *params.Subtype, enrichment.EntityTypeCommit, content, "", at, at,
	)}, nil
}

type worldFileContent struct{ w *twoRepoWorld }

func (f worldFileContent) Content(_ context.Context, repoID int64, _, _ string) (service.BlobContent, error) {
	f.w.touch(repoID, "fileContent.Content")
	if repoID == repoBID {
		return service.NewBlobContent([]byte(markerB), commitB), nil
	}
	return service.NewBlobContent([]byte(contentA), commitA), nil
}

type worldDiskPaths struct{ w *twoRepoWorld }

func (d worldDiskPaths) DiskPath(_ context.Context, repoID int64, _, filePath string) (string, string, error) {
	d.w.touch(repoID, "diskPaths.DiskPath")
	if repoID == repoBID {
		return "/disk/" + markerB + "/" + filePath, commitB, nil
	}
	return "/disk/" + contentA + "/" + filePath, commitA, nil
}

// worldDocument renders whatever disk path it is given, so a raster or text
// read reflects which repository's working copy was resolved.
type worldDocument struct{}

func (worldDocument) PageCount(_ string) (int, error) { return 3, nil }
func (worldDocument) Render(path string, _ int) (string, error) {
	return "text-from:" + path, nil
}
func (worldDocument) Close() error { return nil }

type worldRasterizer struct{}

func (worldRasterizer) PageCount(_ string) (int, error) { return 3, nil }
func (worldRasterizer) Render(_ string, _ int) (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}
func (worldRasterizer) Close() error { return nil }

// worldSearcher ignores the scope it is handed and returns hits from both
// repositories, so tests prove the server's own post-filtering. It records a
// touch of B whenever the filters it receives would let B through.
type worldSearcher struct {
	w    *twoRepoWorld
	name string
}

func (s worldSearcher) search(filters search.Filters) ([]enrichment.Enrichment, map[string]float64) {
	repos := filters.SourceRepos()
	reachesB := len(repos) == 0
	for _, id := range repos {
		if id == repoBID {
			reachesB = true
		}
	}
	if reachesB {
		s.w.touch(repoBID, s.name)
	}
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hits := []enrichment.Enrichment{
		enrichment.ReconstructEnrichment(enrichA, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, contentA, "go", at, at),
		enrichment.ReconstructEnrichment(enrichB, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, markerB, "go", at, at),
	}
	return hits, map[string]float64{strconv.FormatInt(enrichA, 10): 0.9, strconv.FormatInt(enrichB, 10): 0.8}
}

func (s worldSearcher) SearchCodeWithScores(_ context.Context, _ string, _ int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	hits, scores := s.search(filters)
	return hits, scores, nil
}

func (s worldSearcher) SearchKeywordsWithScores(_ context.Context, _ string, _ int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	hits, scores := s.search(filters)
	return hits, scores, nil
}

func (s worldSearcher) SearchVisualWithScores(_ context.Context, _ string, _ int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	hits, scores := s.search(filters)
	return hits, scores, nil
}

type worldResolver struct{}

func (worldResolver) SourceFiles(_ context.Context, _ []int64) (map[string][]int64, error) {
	return map[string][]int64{"10": {fileA}, "20": {fileB}}, nil
}

func (worldResolver) SourceLocations(_ context.Context, _ []int64) (map[string]sourcelocation.SourceLocation, error) {
	return map[string]sourcelocation.SourceLocation{}, nil
}

func (worldResolver) RepositoryIDs(_ context.Context, _ []int64) (map[string]int64, error) {
	return map[string]int64{"10": repoAID, "20": repoBID}, nil
}

type worldFiles struct{}

func (worldFiles) Find(_ context.Context, _ ...repository.Option) ([]repository.File, error) {
	return []repository.File{
		repository.ReconstructFile(fileA, commitA, "a.go", "", "", ".go", "go", 1, time.Time{}),
		repository.ReconstructFile(fileB, commitB, markerB+".go", "", "", ".go", "go", 1, time.Time{}),
	}, nil
}

type worldGrepper struct{ w *twoRepoWorld }

func (g worldGrepper) Search(_ context.Context, repoID int64, _ string, _ string, _ int) ([]service.GrepResult, error) {
	g.w.touch(repoID, "grep.Search")
	path := "a.go"
	if repoID == repoBID {
		path = markerB + ".go"
	}
	return []service.GrepResult{{
		RepoID: repoID, CommitSHA: commitA, Path: path, Language: "go",
		Matches: []git.GrepMatch{{Line: 1, Content: path}},
	}}, nil
}

type worldFileLister struct{ w *twoRepoWorld }

func (l worldFileLister) ListFiles(_ context.Context, repoID int64, _ string) ([]service.FileEntry, error) {
	l.w.touch(repoID, "fileLister.ListFiles")
	path := "a.go"
	if repoID == repoBID {
		path = markerB + ".go"
	}
	return []service.FileEntry{{Path: path, Size: 1}}, nil
}

// newWorldServer builds a server over both repositories with every optional
// read mode and visual search enabled.
func newWorldServer(w *twoRepoWorld, opts ...ServerOption) *Server {
	rasterizers := rasterization.NewRegistry()
	rasterizers.Register(".pdf", worldRasterizer{})
	rasterizers.Register(".png", worldRasterizer{})
	textRenderers := extraction.NewTextRendererRegistry()
	textRenderers.Register(".pdf", worldDocument{})
	textRenderers.Register(".docx", worldDocument{})

	all := append([]ServerOption{
		WithRasterization(worldDiskPaths{w: w}, rasterizers),
		WithTextRendering(worldDiskPaths{w: w}, textRenderers),
	}, opts...)

	return NewServer(
		worldRepositories{w: w},
		worldCommitFinder{w: w},
		worldEnrichmentQuery{w: w},
		worldFileContent{w: w},
		worldSearcher{w: w, name: "semantic"},
		worldSearcher{w: w, name: "keyword"},
		worldSearcher{w: w, name: "visual"},
		worldResolver{},
		worldFileLister{w: w},
		worldFiles{},
		worldGrepper{w: w},
		"test",
		zerolog.Nop(),
		all...,
	)
}

// call sends one JSON-RPC message and returns the whole response (result or
// error) as text, so a test can search it for repository B's marker.
func call(t *testing.T, srv *Server, method string, params map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := json.Marshal(srv.MCPServer().HandleMessage(context.Background(), raw))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return string(out)
}

func toolCall(name string, args map[string]any) (string, map[string]any) {
	return "tools/call", map[string]any{"name": name, "arguments": args}
}

func resourceRead(uri string) (string, map[string]any) {
	return "resources/read", map[string]any{"uri": uri}
}

type probe struct {
	name   string
	method string
	params map[string]any
}

func newProbe(name string, method string, params map[string]any) probe {
	return probe{name: name, method: method, params: params}
}

// repositoryBProbes reaches for repository B through every tool, every read
// mode (as a tool and as a resource) and every search.
func repositoryBProbes() []probe {
	var probes []probe
	add := func(name string, method string, params map[string]any) {
		probes = append(probes, newProbe(name, method, params))
	}

	m, p := toolCall("kodit_repositories", map[string]any{})
	add("repositories", m, p)

	for _, tool := range []string{
		"kodit_architecture_docs", "kodit_api_docs", "kodit_commit_description",
		"kodit_database_schema", "kodit_cookbook", "kodit_wiki",
	} {
		m, p = toolCall(tool, map[string]any{"repo_url": repoBURL})
		add(tool+"/repo B", m, p)
		m, p = toolCall(tool, map[string]any{"repo_url": repoAURL, "commit_sha": commitB})
		add(tool+"/repo A with B's commit", m, p)
	}
	m, p = toolCall("kodit_wiki_page", map[string]any{"repo_url": repoBURL, "page_slug": "x"})
	add("kodit_wiki_page/repo B", m, p)
	m, p = toolCall("kodit_wiki_page", map[string]any{"repo_url": repoAURL, "page_slug": "x", "commit_sha": commitB})
	add("kodit_wiki_page/repo A with B's commit", m, p)

	for _, tool := range []struct{ name, queryArg string }{
		{"kodit_semantic_search", "query"},
		{"kodit_keyword_search", "keywords"},
		{"kodit_visual_search", "query"},
	} {
		m, p = toolCall(tool.name, map[string]any{tool.queryArg: "anything"})
		add(tool.name+"/no source_repo", m, p)
		m, p = toolCall(tool.name, map[string]any{tool.queryArg: "anything", "source_repo": repoBURL})
		add(tool.name+"/source_repo B", m, p)
	}

	m, p = toolCall("kodit_grep", map[string]any{"repo_url": repoBURL, "pattern": "x"})
	add("kodit_grep/repo B", m, p)
	m, p = toolCall("kodit_ls", map[string]any{"repo_url": repoBURL, "pattern": "*"})
	add("kodit_ls/repo B", m, p)

	for _, uri := range []string{
		"file://2/main/secret.go",
		"file://2/main/secret.go?lines=L1-L2&line_numbers=true",
		"file://2/main/doc.pdf?mode=raster&page=1",
		"file://2/main/image.png?mode=raster&page=1",
		"file://2/main/doc.pdf?mode=text",
		"file://2/main/doc.pdf?mode=text&page=1",
		"file://2/main/doc.docx?mode=text&page=1",
	} {
		m, p = toolCall("kodit_read_resource", map[string]any{"uri": uri})
		add("kodit_read_resource "+uri, m, p)
		m, p = resourceRead(uri)
		add("resources/read "+uri, m, p)
	}
	return probes
}

func TestRepositoryScope_CannotReachOtherRepository(t *testing.T) {
	w := &twoRepoWorld{}
	srv := newWorldServer(w, WithRepositoryScope([]int64{allowedID}))
	call(t, srv, "initialize", initializeParams())

	for _, pr := range repositoryBProbes() {
		t.Run(pr.name, func(t *testing.T) {
			w.reset()
			out := call(t, srv, pr.method, pr.params)
			if strings.Contains(out, markerB) {
				t.Errorf("response reaches repository B: %s", out)
			}
			if touched := w.touchedB(); len(touched) > 0 {
				t.Errorf("backends were asked for repository B: %v", touched)
			}
		})
	}
}

// TestRepositoryScope_ProbesReachOtherRepositoryWhenUnscoped is the control
// for the test above: without a scope, every probe that names B through a
// repository URL, resource URI or unfiltered search does reach B, so the
// scoped test is not passing vacuously.
func TestRepositoryScope_ProbesReachOtherRepositoryWhenUnscoped(t *testing.T) {
	w := &twoRepoWorld{}
	srv := newWorldServer(w)
	call(t, srv, "initialize", initializeParams())

	for _, pr := range repositoryBProbes() {
		// A commit must belong to the repository named, scoped or not.
		if strings.HasSuffix(pr.name, "with B's commit") {
			continue
		}
		t.Run(pr.name, func(t *testing.T) {
			w.reset()
			_ = call(t, srv, pr.method, pr.params)
			if len(w.touchedB()) == 0 {
				t.Errorf("probe never reached repository B on an unscoped server")
			}
		})
	}
}

func TestRepositoryScope_RejectsCommitFromAnotherRepository(t *testing.T) {
	w := &twoRepoWorld{}
	srv := newWorldServer(w)
	call(t, srv, "initialize", initializeParams())

	m, p := toolCall("kodit_architecture_docs", map[string]any{"repo_url": repoAURL, "commit_sha": commitB})
	out := call(t, srv, m, p)
	if strings.Contains(out, markerB) {
		t.Errorf("repo A's docs returned repo B's commit enrichments: %s", out)
	}
	if !strings.Contains(out, "not found in repository") {
		t.Errorf("expected a commit-not-found error, got: %s", out)
	}

	m, p = toolCall("kodit_architecture_docs", map[string]any{"repo_url": repoAURL, "commit_sha": commitA})
	if out := call(t, srv, m, p); !strings.Contains(out, contentA) {
		t.Errorf("expected repo A's docs for its own commit, got: %s", out)
	}
}

func TestRepositoryScope_AllowsInScopeRepository(t *testing.T) {
	w := &twoRepoWorld{}
	srv := newWorldServer(w, WithRepositoryScope([]int64{allowedID}))
	call(t, srv, "initialize", initializeParams())

	tests := []struct {
		name string
		args map[string]any
		tool string
		want string
	}{
		{"repositories", map[string]any{}, "kodit_repositories", "repo-a"},
		{"docs", map[string]any{"repo_url": repoAURL}, "kodit_architecture_docs", contentA},
		{"docs at commit", map[string]any{"repo_url": repoAURL, "commit_sha": commitA}, "kodit_api_docs", contentA},
		{"semantic", map[string]any{"query": "q"}, "kodit_semantic_search", "a.go"},
		{"keyword", map[string]any{"keywords": "q"}, "kodit_keyword_search", "a.go"},
		{"visual", map[string]any{"query": "q"}, "kodit_visual_search", "a.go"},
		{"grep", map[string]any{"repo_url": repoAURL, "pattern": "x"}, "kodit_grep", "a.go"},
		{"ls", map[string]any{"repo_url": repoAURL, "pattern": "*"}, "kodit_ls", "a.go"},
		{"plain read", map[string]any{"uri": "file://1/main/a.go"}, "kodit_read_resource", contentA},
		{"text read", map[string]any{"uri": "file://1/main/doc.pdf?mode=text&page=1"}, "kodit_read_resource", contentA},
		{"page count", map[string]any{"uri": "file://1/main/doc.pdf?mode=text"}, "kodit_read_resource", "page_count"},
		{"raster read", map[string]any{"uri": "file://1/main/doc.pdf?mode=raster&page=1"}, "kodit_read_resource", "image/jpeg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, p := toolCall(tt.tool, tt.args)
			out := call(t, srv, m, p)
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in response, got: %s", tt.want, out)
			}
			if strings.Contains(out, markerB) {
				t.Errorf("response reaches repository B: %s", out)
			}
		})
	}
}

func TestRepositoryScope_EmptyScopeSeesNothing(t *testing.T) {
	for _, ids := range [][]int64{nil, {}} {
		t.Run(fmt.Sprintf("%v", ids), func(t *testing.T) {
			w := &twoRepoWorld{}
			srv := newWorldServer(w, WithRepositoryScope(ids))
			call(t, srv, "initialize", initializeParams())

			m, p := toolCall("kodit_repositories", map[string]any{})
			out := call(t, srv, m, p)
			if !strings.Contains(out, "No repositories found") {
				t.Errorf("empty scope listed repositories: %s", out)
			}
			for _, uri := range []string{"file://1/main/a.go", "file://1/main/doc.pdf?mode=text&page=1"} {
				m, p = toolCall("kodit_read_resource", map[string]any{"uri": uri})
				if out := call(t, srv, m, p); strings.Contains(out, contentA) {
					t.Errorf("empty scope read %s: %s", uri, out)
				}
			}
			m, p = toolCall("kodit_semantic_search", map[string]any{"query": "q"})
			if out := call(t, srv, m, p); out == "" || strings.Contains(out, "a.go") {
				t.Errorf("empty scope returned search results: %s", out)
			}
			if touched := w.touchedB(); len(touched) > 0 {
				t.Errorf("backends were asked for repository B: %v", touched)
			}
		})
	}
}

// TestRepositoryScope_WrapsEveryRepositoryBoundDependency fails when Server
// gains a dependency field that a scoped server leaves unwrapped. A new
// field must either be wrapped in repoScope.apply or added to notRepoBound
// with the reason it cannot reach another repository.
func TestRepositoryScope_WrapsEveryRepositoryBoundDependency(t *testing.T) {
	notRepoBound := map[string]string{
		"mcpServer":          "protocol server, not a data source",
		"enrichmentResolver": "only resolves enrichment IDs returned by scoped searches; results are post-filtered by scope",
		"files":              "only looks up file IDs from the enrichment resolver for post-filtered results",
		"rasterizers":        "renders a disk path returned by the scoped disk path resolver",
		"textRenderers":      "renders a disk path returned by the scoped disk path resolver",
		"scope":              "the scope itself",
		"version":            "constant",
		"logger":             "logging",
	}

	w := &twoRepoWorld{}
	srv := newWorldServer(w, WithRepositoryScope([]int64{allowedID}))

	v := reflect.ValueOf(srv).Elem()
	typ := v.Type()
	for i := range typ.NumField() {
		field := typ.Field(i)
		if _, ok := notRepoBound[field.Name]; ok {
			continue
		}
		value := v.Field(i)
		if value.Kind() != reflect.Interface {
			t.Errorf("field %s (%s) is neither wrapped by the scope nor listed as not repository-bound", field.Name, field.Type)
			continue
		}
		if value.IsNil() {
			t.Errorf("field %s is nil in the fixture; enable it so its scoping is checked", field.Name)
			continue
		}
		dynamic := value.Elem().Type().String()
		if !strings.HasPrefix(dynamic, "*mcp.scoped") {
			t.Errorf("field %s holds %s on a scoped server; wrap it in repoScope.apply", field.Name, dynamic)
		}
	}
}

func TestRepoScope_Filters(t *testing.T) {
	tests := []struct {
		name      string
		scope     []int64
		requested []int64
		want      []int64
		wantOK    bool
	}{
		{"injects scope", []int64{1, 2}, nil, []int64{1, 2}, true},
		{"intersects", []int64{1, 2}, []int64{2, 3}, []int64{2}, true},
		{"disjoint request runs no search", []int64{1, 2}, []int64{99}, nil, false},
		{"empty scope runs no search", nil, nil, nil, false},
		{"empty scope with request runs no search", []int64{}, []int64{1}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []search.FiltersOption
			if tt.requested != nil {
				opts = append(opts, search.WithSourceRepos(tt.requested))
			}
			got, ok := newRepoScope(tt.scope).filters(search.NewFilters(opts...))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !reflect.DeepEqual(got.SourceRepos(), tt.want) {
				t.Errorf("source repos = %v, want %v", got.SourceRepos(), tt.want)
			}
		})
	}
}

func TestRepoScope_FiltersPreservesOtherFields(t *testing.T) {
	original := search.NewFilters(
		search.WithLanguages([]string{"go"}),
		search.WithAuthors([]string{"alice"}),
		search.WithFilePaths([]string{"src/"}),
		search.WithEnrichmentTypes([]string{"development"}),
		search.WithEnrichmentSubtypes([]string{"snippet"}),
		search.WithCommitSHAs([]string{"abc123"}),
	)
	got, ok := newRepoScope([]int64{1}).filters(original)
	if !ok {
		t.Fatal("expected search to run")
	}
	if !reflect.DeepEqual(got.Languages(), []string{"go"}) ||
		!reflect.DeepEqual(got.Authors(), []string{"alice"}) ||
		!reflect.DeepEqual(got.FilePaths(), []string{"src/"}) ||
		!reflect.DeepEqual(got.EnrichmentTypes(), []string{"development"}) ||
		!reflect.DeepEqual(got.EnrichmentSubtypes(), []string{"snippet"}) ||
		!reflect.DeepEqual(got.CommitSHAs(), []string{"abc123"}) {
		t.Errorf("filters not preserved: %+v", got)
	}
}
