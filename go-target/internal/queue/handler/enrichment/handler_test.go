package enrichment

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	domainenrichment "github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
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
	enrichments map[int64]domainenrichment.Enrichment
	nextID      int64
}

func newFakeEnrichmentRepo() *fakeEnrichmentRepo {
	return &fakeEnrichmentRepo{
		enrichments: make(map[int64]domainenrichment.Enrichment),
		nextID:      1,
	}
}

func (f *fakeEnrichmentRepo) Find(_ context.Context, options ...repository.Option) ([]domainenrichment.Enrichment, error) {
	q := repository.Build(options...)
	var result []domainenrichment.Enrichment
	for _, e := range f.enrichments {
		if matchesEnrichment(e, q) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeEnrichmentRepo) FindOne(_ context.Context, options ...repository.Option) (domainenrichment.Enrichment, error) {
	q := repository.Build(options...)
	for _, e := range f.enrichments {
		if matchesEnrichment(e, q) {
			return e, nil
		}
	}
	return domainenrichment.Enrichment{}, errors.New("not found")
}

func (f *fakeEnrichmentRepo) DeleteBy(_ context.Context, options ...repository.Option) error {
	q := repository.Build(options...)
	for id, e := range f.enrichments {
		if matchesEnrichment(e, q) {
			delete(f.enrichments, id)
		}
	}
	return nil
}

func (f *fakeEnrichmentRepo) Save(_ context.Context, e domainenrichment.Enrichment) (domainenrichment.Enrichment, error) {
	id := f.nextID
	f.nextID++
	saved := e.WithID(id)
	f.enrichments[id] = saved
	return saved, nil
}

func (f *fakeEnrichmentRepo) Delete(_ context.Context, e domainenrichment.Enrichment) error {
	delete(f.enrichments, e.ID())
	return nil
}

func (f *fakeEnrichmentRepo) FindByEntityKey(_ context.Context, key domainenrichment.EntityTypeKey) ([]domainenrichment.Enrichment, error) {
	var result []domainenrichment.Enrichment
	for _, e := range f.enrichments {
		if e.EntityTypeKey() == key {
			result = append(result, e)
		}
	}
	return result, nil
}

func matchesEnrichment(e domainenrichment.Enrichment, q repository.Query) bool {
	for _, c := range q.Conditions() {
		switch c.Field() {
		case "id":
			if id, ok := c.Value().(int64); ok && e.ID() != id {
				return false
			}
		case "type":
			if t, ok := c.Value().(string); ok && string(e.Type()) != t {
				return false
			}
		case "subtype":
			if s, ok := c.Value().(string); ok && string(e.Subtype()) != s {
				return false
			}
		}
	}
	return true
}

type fakeAssociationRepo struct {
	associations map[int64]domainenrichment.Association
	nextID       int64
}

func newFakeAssociationRepo() *fakeAssociationRepo {
	return &fakeAssociationRepo{
		associations: make(map[int64]domainenrichment.Association),
		nextID:       1,
	}
}

func (f *fakeAssociationRepo) Find(_ context.Context, options ...repository.Option) ([]domainenrichment.Association, error) {
	q := repository.Build(options...)
	var result []domainenrichment.Association
	for _, a := range f.associations {
		if matchesAssociation(a, q) {
			result = append(result, a)
		}
	}
	return result, nil
}

func (f *fakeAssociationRepo) FindOne(_ context.Context, options ...repository.Option) (domainenrichment.Association, error) {
	q := repository.Build(options...)
	for _, a := range f.associations {
		if matchesAssociation(a, q) {
			return a, nil
		}
	}
	return domainenrichment.Association{}, errors.New("not found")
}

func (f *fakeAssociationRepo) DeleteBy(_ context.Context, options ...repository.Option) error {
	q := repository.Build(options...)
	for id, a := range f.associations {
		if matchesAssociation(a, q) {
			delete(f.associations, id)
		}
	}
	return nil
}

func (f *fakeAssociationRepo) Save(_ context.Context, a domainenrichment.Association) (domainenrichment.Association, error) {
	id := f.nextID
	f.nextID++
	saved := a.WithID(id)
	f.associations[id] = saved
	return saved, nil
}

func (f *fakeAssociationRepo) Delete(_ context.Context, a domainenrichment.Association) error {
	delete(f.associations, a.ID())
	return nil
}

func matchesAssociation(a domainenrichment.Association, q repository.Query) bool {
	for _, c := range q.Conditions() {
		switch c.Field() {
		case "id":
			if id, ok := c.Value().(int64); ok && a.ID() != id {
				return false
			}
		case "enrichment_id":
			if id, ok := c.Value().(int64); ok && a.EnrichmentID() != id {
				return false
			}
		case "entity_id":
			if eid, ok := c.Value().(string); ok && a.EntityID() != eid {
				return false
			}
		case "entity_type":
			if et, ok := c.Value().(string); ok && string(a.EntityType()) != et {
				return false
			}
		}
	}
	return true
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
	repos map[int64]repository.Repository
}

func newFakeRepoRepo() *fakeRepoRepo {
	return &fakeRepoRepo{
		repos: make(map[int64]repository.Repository),
	}
}

func (f *fakeRepoRepo) Find(_ context.Context, options ...repository.Option) ([]repository.Repository, error) {
	q := repository.Build(options...)
	var result []repository.Repository
	for _, r := range f.repos {
		if matchesRepo(r, q) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (f *fakeRepoRepo) FindOne(_ context.Context, options ...repository.Option) (repository.Repository, error) {
	q := repository.Build(options...)
	for _, r := range f.repos {
		if matchesRepo(r, q) {
			return r, nil
		}
	}
	return repository.Repository{}, errors.New("not found")
}

func (f *fakeRepoRepo) Exists(_ context.Context, options ...repository.Option) (bool, error) {
	q := repository.Build(options...)
	for _, r := range f.repos {
		if matchesRepo(r, q) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRepoRepo) Save(_ context.Context, r repository.Repository) (repository.Repository, error) {
	return r, nil
}

func (f *fakeRepoRepo) Delete(_ context.Context, _ repository.Repository) error {
	return nil
}

func matchesRepo(r repository.Repository, q repository.Query) bool {
	for _, c := range q.Conditions() {
		switch c.Field() {
		case "id":
			if id, ok := c.Value().(int64); ok && r.ID() != id {
				return false
			}
		}
	}
	return true
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

	testRepo := repository.ReconstructRepository(
		1, "https://github.com/test/repo",
		repository.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo"),
		repository.NewTrackingConfig("main", "", ""),
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
			assert.Equal(t, domainenrichment.TypeHistory, e.Type())
			assert.Equal(t, domainenrichment.SubtypeCommitDescription, e.Subtype())
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
			assert.Equal(t, domainenrichment.TypeDevelopment, e.Type())
			assert.Equal(t, domainenrichment.SubtypeSnippetSummary, e.Subtype())
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
