package mcp

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// scopedRepositories decorates a RepositoryLister to restrict results
// to a fixed set of repository IDs.
type scopedRepositories struct {
	inner RepositoryLister
	ids   []int64
}

func (s *scopedRepositories) Find(ctx context.Context, options ...repository.Option) ([]repository.Repository, error) {
	return s.inner.Find(ctx, append(options, repository.WithIDIn(s.ids))...)
}

// scopedFileContent decorates a FileContentReader, rejecting requests
// for repositories outside the allowed set.
type scopedFileContent struct {
	inner   FileContentReader
	allowed map[int64]struct{}
}

func (s *scopedFileContent) Content(ctx context.Context, repoID int64, blobName, filePath string) (service.BlobContent, error) {
	if _, ok := s.allowed[repoID]; !ok {
		return service.BlobContent{}, fmt.Errorf("repository %d is not in scope", repoID)
	}
	return s.inner.Content(ctx, repoID, blobName, filePath)
}

// scopedSemanticSearch decorates a SemanticSearcher, injecting source-repo
// filters so results never leak outside the allowed set.
type scopedSemanticSearch struct {
	inner SemanticSearcher
	ids   []int64
}

func (s *scopedSemanticSearch) SearchCodeWithScores(ctx context.Context, query string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return s.inner.SearchCodeWithScores(ctx, query, topK, scopeFilters(filters, s.ids))
}

// scopedKeywordSearch decorates a KeywordSearcher, injecting source-repo
// filters so results never leak outside the allowed set.
type scopedKeywordSearch struct {
	inner KeywordSearcher
	ids   []int64
}

func (s *scopedKeywordSearch) SearchKeywordsWithScores(ctx context.Context, query string, limit int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return s.inner.SearchKeywordsWithScores(ctx, query, limit, scopeFilters(filters, s.ids))
}

// scopedGrepper decorates a Grepper, rejecting requests for
// repositories outside the allowed set.
type scopedGrepper struct {
	inner   Grepper
	allowed map[int64]struct{}
}

func (s *scopedGrepper) Search(ctx context.Context, repoID int64, pattern string, pathspec string, maxFiles int) ([]service.GrepResult, error) {
	if _, ok := s.allowed[repoID]; !ok {
		return nil, fmt.Errorf("repository %d is not in scope", repoID)
	}
	return s.inner.Search(ctx, repoID, pattern, pathspec, maxFiles)
}

// scopedFileLister decorates a FileLister, rejecting requests for
// repositories outside the allowed set.
type scopedFileLister struct {
	inner   FileLister
	allowed map[int64]struct{}
}

func (s *scopedFileLister) ListFiles(ctx context.Context, repoID int64, pattern string) ([]service.FileEntry, error) {
	if _, ok := s.allowed[repoID]; !ok {
		return nil, fmt.Errorf("repository %d is not in scope", repoID)
	}
	return s.inner.ListFiles(ctx, repoID, pattern)
}

// Scope wraps the given dependencies with scoping decorators that restrict
// access to only the specified repository IDs.
func Scope(
	repositories RepositoryLister,
	fileContent FileContentReader,
	semanticSearch SemanticSearcher,
	keywordSearch KeywordSearcher,
	grepper Grepper,
	fileLister FileLister,
	repoIDs []int64,
) (RepositoryLister, FileContentReader, SemanticSearcher, KeywordSearcher, Grepper, FileLister) {
	allowed := make(map[int64]struct{}, len(repoIDs))
	for _, id := range repoIDs {
		allowed[id] = struct{}{}
	}
	ids := make([]int64, len(repoIDs))
	copy(ids, repoIDs)

	return &scopedRepositories{inner: repositories, ids: ids},
		&scopedFileContent{inner: fileContent, allowed: allowed},
		&scopedSemanticSearch{inner: semanticSearch, ids: ids},
		&scopedKeywordSearch{inner: keywordSearch, ids: ids},
		&scopedGrepper{inner: grepper, allowed: allowed},
		&scopedFileLister{inner: fileLister, allowed: allowed}
}

// scopeFilters returns filters with source repos restricted to the allowed set.
// If the caller already specified source repos, the result is the intersection.
func scopeFilters(original search.Filters, allowed []int64) search.Filters {
	repos := allowed
	if existing := original.SourceRepos(); len(existing) > 0 {
		set := make(map[int64]struct{}, len(allowed))
		for _, id := range allowed {
			set[id] = struct{}{}
		}
		intersection := make([]int64, 0)
		for _, id := range existing {
			if _, ok := set[id]; ok {
				intersection = append(intersection, id)
			}
		}
		repos = intersection
	}
	return original.With(search.WithSourceRepos(repos))
}
