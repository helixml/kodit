package enrichment

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/tracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTrackerFactory struct{}

func (f *fakeTrackerFactory) ForOperation(op queue.TaskOperation, tt domain.TrackableType, id int64) *tracking.Tracker {
	return tracking.NewTracker(
		queue.NewTaskStatus(op, nil, tt, id),
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
	)
}

type fakeEnricher struct {
	responses []enrichment.Response
	err       error
}

func (f *fakeEnricher) Enrich(_ context.Context, requests []enrichment.Request) ([]enrichment.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.responses != nil {
		return f.responses, nil
	}
	var responses []enrichment.Response
	for _, r := range requests {
		responses = append(responses, enrichment.NewResponse(r.ID(), "enriched content for "+r.ID()))
	}
	return responses, nil
}

type fakeEnrichmentRepo struct {
	enrichments map[int64]enrichment.Enrichment
	nextID      int64
}

func newFakeEnrichmentRepo() *fakeEnrichmentRepo {
	return &fakeEnrichmentRepo{
		enrichments: make(map[int64]enrichment.Enrichment),
		nextID:      1,
	}
}

func (f *fakeEnrichmentRepo) Get(_ context.Context, id int64) (enrichment.Enrichment, error) {
	e, ok := f.enrichments[id]
	if !ok {
		return enrichment.Enrichment{}, errors.New("not found")
	}
	return e, nil
}

func (f *fakeEnrichmentRepo) Save(_ context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	id := f.nextID
	f.nextID++
	saved := e.WithID(id)
	f.enrichments[id] = saved
	return saved, nil
}

func (f *fakeEnrichmentRepo) Delete(_ context.Context, e enrichment.Enrichment) error {
	delete(f.enrichments, e.ID())
	return nil
}

func (f *fakeEnrichmentRepo) FindByType(_ context.Context, t enrichment.Type) ([]enrichment.Enrichment, error) {
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		if e.Type() == t {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeEnrichmentRepo) FindByTypeAndSubtype(_ context.Context, t enrichment.Type, s enrichment.Subtype) ([]enrichment.Enrichment, error) {
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		if e.Type() == t && e.Subtype() == s {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeEnrichmentRepo) FindByEntityKey(_ context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		if e.EntityTypeKey() == key {
			result = append(result, e)
		}
	}
	return result, nil
}

type fakeAssociationRepo struct {
	associations map[int64]enrichment.Association
	nextID       int64
}

func newFakeAssociationRepo() *fakeAssociationRepo {
	return &fakeAssociationRepo{
		associations: make(map[int64]enrichment.Association),
		nextID:       1,
	}
}

func (f *fakeAssociationRepo) Get(_ context.Context, id int64) (enrichment.Association, error) {
	a, ok := f.associations[id]
	if !ok {
		return enrichment.Association{}, errors.New("not found")
	}
	return a, nil
}

func (f *fakeAssociationRepo) Save(_ context.Context, a enrichment.Association) (enrichment.Association, error) {
	id := f.nextID
	f.nextID++
	saved := a.WithID(id)
	f.associations[id] = saved
	return saved, nil
}

func (f *fakeAssociationRepo) Delete(_ context.Context, a enrichment.Association) error {
	delete(f.associations, a.ID())
	return nil
}

func (f *fakeAssociationRepo) FindByEnrichmentID(_ context.Context, enrichmentID int64) ([]enrichment.Association, error) {
	var result []enrichment.Association
	for _, a := range f.associations {
		if a.EnrichmentID() == enrichmentID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (f *fakeAssociationRepo) FindByEntityID(_ context.Context, entityID string) ([]enrichment.Association, error) {
	var result []enrichment.Association
	for _, a := range f.associations {
		if a.EntityID() == entityID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (f *fakeAssociationRepo) FindByEntityTypeAndID(_ context.Context, entityType enrichment.EntityTypeKey, entityID string) ([]enrichment.Association, error) {
	var result []enrichment.Association
	for _, a := range f.associations {
		if a.EntityType() == entityType && a.EntityID() == entityID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (f *fakeAssociationRepo) DeleteByEnrichmentID(_ context.Context, enrichmentID int64) error {
	for id, a := range f.associations {
		if a.EnrichmentID() == enrichmentID {
			delete(f.associations, id)
		}
	}
	return nil
}

func (f *fakeAssociationRepo) DeleteByEntityID(_ context.Context, entityID string) error {
	for id, a := range f.associations {
		if a.EntityID() == entityID {
			delete(f.associations, id)
		}
	}
	return nil
}

type fakeSnippetRepo struct {
	snippets map[string][]indexing.Snippet
}

func newFakeSnippetRepo() *fakeSnippetRepo {
	return &fakeSnippetRepo{
		snippets: make(map[string][]indexing.Snippet),
	}
}

func (f *fakeSnippetRepo) SnippetsForCommit(_ context.Context, commitSHA string) ([]indexing.Snippet, error) {
	return f.snippets[commitSHA], nil
}

func (f *fakeSnippetRepo) Save(_ context.Context, commitSHA string, snippets []indexing.Snippet) error {
	f.snippets[commitSHA] = snippets
	return nil
}

func (f *fakeSnippetRepo) DeleteForCommit(_ context.Context, commitSHA string) error {
	delete(f.snippets, commitSHA)
	return nil
}

func (f *fakeSnippetRepo) Search(_ context.Context, _ domain.MultiSearchRequest) ([]indexing.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) ByIDs(_ context.Context, _ []string) ([]indexing.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) BySHA(_ context.Context, sha string) (indexing.Snippet, error) {
	for _, snippets := range f.snippets {
		for _, snippet := range snippets {
			if snippet.SHA() == sha {
				return snippet, nil
			}
		}
	}
	return indexing.Snippet{}, nil
}

type fakeGitAdapter struct {
	diff  string
	files []git.FileInfo
	err   error
}

func (f *fakeGitAdapter) CloneRepository(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeGitAdapter) CheckoutCommit(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeGitAdapter) CheckoutBranch(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeGitAdapter) FetchRepository(_ context.Context, _ string) error {
	return nil
}

func (f *fakeGitAdapter) PullRepository(_ context.Context, _ string) error {
	return nil
}

func (f *fakeGitAdapter) AllBranches(_ context.Context, _ string) ([]git.BranchInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) BranchCommits(_ context.Context, _, _ string) ([]git.CommitInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) AllCommitsBulk(_ context.Context, _ string, _ *time.Time) (map[string]git.CommitInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) BranchCommitSHAs(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeGitAdapter) AllBranchHeadSHAs(_ context.Context, _ string, _ []string) (map[string]string, error) {
	return nil, nil
}

func (f *fakeGitAdapter) CommitFiles(_ context.Context, _, _ string) ([]git.FileInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.files, nil
}

func (f *fakeGitAdapter) RepositoryExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (f *fakeGitAdapter) CommitDetails(_ context.Context, _, _ string) (git.CommitInfo, error) {
	return git.CommitInfo{}, nil
}

func (f *fakeGitAdapter) EnsureRepository(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeGitAdapter) FileContent(_ context.Context, _, _, _ string) ([]byte, error) {
	return nil, nil
}

func (f *fakeGitAdapter) DefaultBranch(_ context.Context, _ string) (string, error) {
	return "main", nil
}

func (f *fakeGitAdapter) LatestCommitSHA(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) AllTags(_ context.Context, _ string) ([]git.TagInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) CommitDiff(_ context.Context, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.diff, nil
}

type fakeRepoRepo struct {
	repos map[int64]git.Repo
}

func newFakeRepoRepo() *fakeRepoRepo {
	return &fakeRepoRepo{
		repos: make(map[int64]git.Repo),
	}
}

func (f *fakeRepoRepo) Get(_ context.Context, id int64) (git.Repo, error) {
	r, ok := f.repos[id]
	if !ok {
		return git.Repo{}, errors.New("not found")
	}
	return r, nil
}

func (f *fakeRepoRepo) Find(_ context.Context, _ database.Query) ([]git.Repo, error) {
	return nil, nil
}

func (f *fakeRepoRepo) FindAll(_ context.Context) ([]git.Repo, error) {
	return nil, nil
}

func (f *fakeRepoRepo) Save(_ context.Context, r git.Repo) (git.Repo, error) {
	return r, nil
}

func (f *fakeRepoRepo) Delete(_ context.Context, _ git.Repo) error {
	return nil
}

func (f *fakeRepoRepo) GetByRemoteURL(_ context.Context, _ string) (git.Repo, error) {
	return git.Repo{}, nil
}

func (f *fakeRepoRepo) ExistsByRemoteURL(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func TestCommitDescriptionHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	repoRepo := newFakeRepoRepo()
	enrichmentRepo := newFakeEnrichmentRepo()
	associationRepo := newFakeAssociationRepo()
	queryService := enrichment.NewQueryService(enrichmentRepo, associationRepo)
	adapter := &fakeGitAdapter{diff: "diff --git a/file.go"}
	enricher := &fakeEnricher{}

	testRepo := git.ReconstructRepo(
		1, "https://github.com/test/repo",
		git.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo"),
		git.NewTrackingConfig("main", "", ""),
		time.Now(), time.Now(),
	)
	repoRepo.repos[1] = testRepo

	handler := NewCommitDescription(
		repoRepo,
		enrichmentRepo,
		associationRepo,
		queryService,
		adapter,
		enricher,
		&fakeTrackerFactory{},
		logger,
	)

	t.Run("creates commit description", func(t *testing.T) {
		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123def456",
		}

		err := handler.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Len(t, enrichmentRepo.enrichments, 1)
		assert.Len(t, associationRepo.associations, 1)

		for _, e := range enrichmentRepo.enrichments {
			assert.Equal(t, enrichment.TypeHistory, e.Type())
			assert.Equal(t, enrichment.SubtypeCommitDescription, e.Subtype())
		}
	})

	t.Run("skips when description exists", func(t *testing.T) {
		countBefore := len(enrichmentRepo.enrichments)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123def456",
		}

		err := handler.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Equal(t, countBefore, len(enrichmentRepo.enrichments))
	})
}

func TestCreateSummaryHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	snippetRepo := newFakeSnippetRepo()
	enrichmentRepo := newFakeEnrichmentRepo()
	associationRepo := newFakeAssociationRepo()
	queryService := enrichment.NewQueryService(enrichmentRepo, associationRepo)
	enricher := &fakeEnricher{}

	snippets := []indexing.Snippet{
		indexing.NewSnippet("func main() {}", ".go", nil),
		indexing.NewSnippet("def main():", ".py", nil),
	}
	snippetRepo.snippets["abc123"] = snippets

	handler := NewCreateSummary(
		snippetRepo,
		enrichmentRepo,
		associationRepo,
		queryService,
		enricher,
		&fakeTrackerFactory{},
		logger,
	)

	t.Run("creates summaries for snippets", func(t *testing.T) {
		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123",
		}

		err := handler.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Len(t, enrichmentRepo.enrichments, 2)

		for _, e := range enrichmentRepo.enrichments {
			assert.Equal(t, enrichment.TypeDevelopment, e.Type())
			assert.Equal(t, enrichment.SubtypeSnippetSummary, e.Subtype())
		}
	})

	t.Run("skips when no snippets", func(t *testing.T) {
		enrichmentRepo2 := newFakeEnrichmentRepo()
		associationRepo2 := newFakeAssociationRepo()
		queryService2 := enrichment.NewQueryService(enrichmentRepo2, associationRepo2)
		snippetRepo2 := newFakeSnippetRepo()

		handler2 := NewCreateSummary(
			snippetRepo2,
			enrichmentRepo2,
			associationRepo2,
			queryService2,
			enricher,
			&fakeTrackerFactory{},
			logger,
		)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "empty123",
		}

		err := handler2.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Len(t, enrichmentRepo2.enrichments, 0)
	})
}

func TestTruncateDiff(t *testing.T) {
	t.Run("returns short diff unchanged", func(t *testing.T) {
		diff := "short diff"
		result := TruncateDiff(diff, 100)
		assert.Equal(t, diff, result)
	})

	t.Run("truncates long diff", func(t *testing.T) {
		diff := make([]byte, 200)
		for i := range diff {
			diff[i] = 'x'
		}
		result := TruncateDiff(string(diff), 100)
		assert.True(t, len(result) <= 100)
		assert.Contains(t, result, "[diff truncated due to size]")
	})
}
