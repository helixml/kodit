package mcp

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// WithRepositoryScope restricts the server to the given repository IDs.
//
// NewServer wraps every repository-bound dependency (repositories, commits,
// file content, every read mode's disk path resolver, semantic, keyword and
// visual search, grep and file listing) after all other options are applied,
// so no dependency can reach the server unscoped. Search results are also
// post-filtered against the scope.
//
// An empty or nil repoIDs scopes the server to no repositories: every tool
// sees nothing. Build the server without this option to see all repositories.
func WithRepositoryScope(repoIDs []int64) ServerOption {
	return func(s *Server) {
		scope := newRepoScope(repoIDs)
		s.scope = &scope
	}
}

// repoScope is a fixed set of repository IDs that a scoped server may access.
type repoScope struct {
	ids     []int64
	allowed map[int64]struct{}
}

func newRepoScope(repoIDs []int64) repoScope {
	allowed := make(map[int64]struct{}, len(repoIDs))
	ids := make([]int64, 0, len(repoIDs))
	for _, id := range repoIDs {
		if _, dup := allowed[id]; dup {
			continue
		}
		allowed[id] = struct{}{}
		ids = append(ids, id)
	}
	return repoScope{ids: ids, allowed: allowed}
}

func (s repoScope) contains(repoID int64) bool {
	_, ok := s.allowed[repoID]
	return ok
}

func (s repoScope) check(repoID int64) error {
	if !s.contains(repoID) {
		return fmt.Errorf("repository %d is not in scope", repoID)
	}
	return nil
}

// filters restricts the source repositories of original to the scope. If the
// caller already named source repos, the result is the intersection. The
// boolean is false when no repository remains, in which case the search must
// not run: search stores treat an empty source-repo list as "all repositories".
func (s repoScope) filters(original search.Filters) (search.Filters, bool) {
	repos := s.ids
	if requested := original.SourceRepos(); len(requested) > 0 {
		repos = make([]int64, 0, len(requested))
		for _, id := range requested {
			if s.contains(id) {
				repos = append(repos, id)
			}
		}
	}
	if len(repos) == 0 {
		return original, false
	}
	return original.With(search.WithSourceRepos(repos)), true
}

// apply wraps every repository-bound dependency of s with a scoping decorator.
// Dependencies that are not repository-bound are listed in
// TestRepositoryScope_WrapsEveryRepositoryBoundDependency with the reason.
func (s repoScope) apply(srv *Server) {
	srv.repositories = &scopedRepositories{inner: srv.repositories, scope: s}
	srv.commits = &scopedCommits{inner: srv.commits, scope: s}
	srv.enrichmentQuery = &scopedEnrichmentQuery{inner: srv.enrichmentQuery, commits: srv.commits}
	srv.fileContent = &scopedFileContent{inner: srv.fileContent, scope: s}
	srv.semanticSearch = &scopedSemanticSearch{inner: srv.semanticSearch, scope: s}
	srv.keywordSearch = &scopedKeywordSearch{inner: srv.keywordSearch, scope: s}
	srv.grepper = &scopedGrepper{inner: srv.grepper, scope: s}
	srv.fileLister = &scopedFileLister{inner: srv.fileLister, scope: s}
	// Optional dependencies stay nil so handlers keep reporting them as
	// unavailable.
	if srv.diskPathResolver != nil {
		srv.diskPathResolver = &scopedDiskPaths{inner: srv.diskPathResolver, scope: s}
	}
	if srv.visualSearch != nil {
		srv.visualSearch = &scopedVisualSearch{inner: srv.visualSearch, scope: s}
	}
}

// scopedRepositories restricts repository lookups to the scope.
type scopedRepositories struct {
	inner RepositoryLister
	scope repoScope
}

func (s *scopedRepositories) Find(ctx context.Context, options ...repository.Option) ([]repository.Repository, error) {
	if len(s.scope.ids) == 0 {
		return nil, nil
	}
	return s.inner.Find(ctx, append(options, repository.WithIDIn(s.scope.ids))...)
}

// scopedCommits restricts commit lookups to repositories in the scope.
type scopedCommits struct {
	inner CommitFinder
	scope repoScope
}

func (s *scopedCommits) Find(ctx context.Context, options ...repository.Option) ([]repository.Commit, error) {
	if len(s.scope.ids) == 0 {
		return nil, nil
	}
	return s.inner.Find(ctx, append(options, repository.WithRepoIDIn(s.scope.ids))...)
}

// scopedEnrichmentQuery only lists enrichments of commits that belong to a
// repository in the scope. commits must already be scoped.
type scopedEnrichmentQuery struct {
	inner   EnrichmentQuery
	commits CommitFinder
}

func (s *scopedEnrichmentQuery) List(ctx context.Context, params *service.EnrichmentListParams) ([]enrichment.Enrichment, error) {
	if params == nil {
		return nil, fmt.Errorf("enrichment listing requires a commit SHA on a scoped server")
	}
	shas := params.CommitSHAs
	if params.CommitSHA != "" {
		shas = append([]string{params.CommitSHA}, shas...)
	}
	if len(shas) == 0 {
		return nil, fmt.Errorf("enrichment listing requires a commit SHA on a scoped server")
	}
	for _, sha := range shas {
		commits, err := s.commits.Find(ctx, repository.WithSHA(sha), repository.WithLimit(1))
		if err != nil {
			return nil, fmt.Errorf("find commit %s: %w", sha, err)
		}
		if len(commits) == 0 {
			return nil, fmt.Errorf("commit %s is not in scope", sha)
		}
	}
	return s.inner.List(ctx, params)
}

// scopedFileContent rejects plain file reads outside the scope.
type scopedFileContent struct {
	inner FileContentReader
	scope repoScope
}

func (s *scopedFileContent) Content(ctx context.Context, repoID int64, blobName, filePath string) (service.BlobContent, error) {
	if err := s.scope.check(repoID); err != nil {
		return service.BlobContent{}, err
	}
	return s.inner.Content(ctx, repoID, blobName, filePath)
}

// scopedDiskPaths rejects raster and text reads outside the scope.
type scopedDiskPaths struct {
	inner DiskPathResolver
	scope repoScope
}

func (s *scopedDiskPaths) DiskPath(ctx context.Context, repoID int64, blobName, filePath string) (string, string, error) {
	if err := s.scope.check(repoID); err != nil {
		return "", "", err
	}
	return s.inner.DiskPath(ctx, repoID, blobName, filePath)
}

// scopedSemanticSearch restricts semantic search to the scope.
type scopedSemanticSearch struct {
	inner SemanticSearcher
	scope repoScope
}

func (s *scopedSemanticSearch) SearchCodeWithScores(ctx context.Context, query string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	scoped, ok := s.scope.filters(filters)
	if !ok {
		return nil, nil, nil
	}
	return s.inner.SearchCodeWithScores(ctx, query, topK, scoped)
}

// scopedKeywordSearch restricts keyword search to the scope.
type scopedKeywordSearch struct {
	inner KeywordSearcher
	scope repoScope
}

func (s *scopedKeywordSearch) SearchKeywordsWithScores(ctx context.Context, query string, limit int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	scoped, ok := s.scope.filters(filters)
	if !ok {
		return nil, nil, nil
	}
	return s.inner.SearchKeywordsWithScores(ctx, query, limit, scoped)
}

// scopedVisualSearch restricts visual search to the scope.
type scopedVisualSearch struct {
	inner VisualSearcher
	scope repoScope
}

func (s *scopedVisualSearch) SearchVisualWithScores(ctx context.Context, query string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	scoped, ok := s.scope.filters(filters)
	if !ok {
		return nil, nil, nil
	}
	return s.inner.SearchVisualWithScores(ctx, query, topK, scoped)
}

// scopedGrepper rejects grep requests outside the scope.
type scopedGrepper struct {
	inner Grepper
	scope repoScope
}

func (s *scopedGrepper) Search(ctx context.Context, repoID int64, pattern string, pathspec string, maxFiles int) ([]service.GrepResult, error) {
	if err := s.scope.check(repoID); err != nil {
		return nil, err
	}
	return s.inner.Search(ctx, repoID, pattern, pathspec, maxFiles)
}

// scopedFileLister rejects file listings outside the scope.
type scopedFileLister struct {
	inner FileLister
	scope repoScope
}

func (s *scopedFileLister) ListFiles(ctx context.Context, repoID int64, pattern string) ([]service.FileEntry, error) {
	if err := s.scope.check(repoID); err != nil {
		return nil, err
	}
	return s.inner.ListFiles(ctx, repoID, pattern)
}
