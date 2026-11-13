package mcp

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
)

// requestRecorder records the topK and filters each search receives.
type requestRecorder struct {
	topK    int
	filters search.Filters
}

func (r *requestRecorder) record(topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	r.topK = topK
	r.filters = filters
	return nil, nil, nil
}

func (r *requestRecorder) SearchCodeWithScores(_ context.Context, _ string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return r.record(topK, filters)
}

func (r *requestRecorder) SearchKeywordsWithScores(_ context.Context, _ string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return r.record(topK, filters)
}

func (r *requestRecorder) SearchVisualWithScores(_ context.Context, _ string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return r.record(topK, filters)
}

func TestSearchTools_SourceRepoFilterAndLimitCap(t *testing.T) {
	tests := []struct{ tool, queryArg string }{
		{"kodit_semantic_search", "query"},
		{"kodit_keyword_search", "keywords"},
		{"kodit_visual_search", "query"},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			rec := &requestRecorder{}
			srv := NewServer(
				&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
				&fakeCommitFinder{},
				&fakeEnrichmentQuery{},
				&fakeFileContentReader{},
				rec, rec, rec,
				&fakeEnrichmentResolver{
					sourceFiles:   map[string][]int64{},
					lineRanges:    map[string]sourcelocation.SourceLocation{},
					repositoryIDs: map[string]int64{},
				},
				&fakeFileLister{},
				&fakeFileFinder{},
				&fakeGrepper{},
				"test",
				zerolog.Nop(),
			)
			call(t, srv, "initialize", initializeParams())

			m, p := toolCall(tt.tool, map[string]any{
				tt.queryArg:   "q",
				"source_repo": testRepo().UpstreamURL(),
				"limit":       100000,
			})
			call(t, srv, m, p)

			if rec.topK != maxSearchLimit {
				t.Errorf("search ran with topK %d, want the cap %d", rec.topK, maxSearchLimit)
			}
			if repos := rec.filters.SourceRepos(); len(repos) != 1 || repos[0] != testRepo().ID() {
				t.Errorf("search ran with source repos %v, want [%d]", repos, testRepo().ID())
			}
		})
	}
}
