package enrichment

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	infraGit "github.com/helixml/kodit/infrastructure/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTracker struct{}

func (f *fakeTracker) SetTotal(_ context.Context, _ int) error              { return nil }
func (f *fakeTracker) SetCurrent(_ context.Context, _ int, _ string) error  { return nil }
func (f *fakeTracker) Skip(_ context.Context, _ string) error               { return nil }
func (f *fakeTracker) Fail(_ context.Context, _ string) error               { return nil }
func (f *fakeTracker) Complete(_ context.Context) error                     { return nil }

type fakeTrackerFactory struct{}

func (f *fakeTrackerFactory) ForOperation(_ task.Operation, _ task.TrackableType, _ int64) handler.Tracker {
	return &fakeTracker{}
}

type fakeEnricher struct {
	responses []domainservice.EnrichmentResponse
	err       error
}

func (f *fakeEnricher) Enrich(_ context.Context, requests []domainservice.EnrichmentRequest) ([]domainservice.EnrichmentResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.responses != nil {
		return f.responses, nil
	}
	var responses []domainservice.EnrichmentResponse
	for _, r := range requests {
		responses = append(responses, domainservice.NewEnrichmentResponse(r.ID(), "enriched content for "+r.ID()))
	}
	return responses, nil
}

type fakeEnrichmentStore struct {
	enrichments  map[int64]enrichment.Enrichment
	nextID       int64
	associations *fakeAssociationStore
}

func newFakeEnrichmentStore() *fakeEnrichmentStore {
	return &fakeEnrichmentStore{
		enrichments: make(map[int64]enrichment.Enrichment),
		nextID:      1,
	}
}

func (f *fakeEnrichmentStore) Find(_ context.Context, options ...repository.Option) ([]enrichment.Enrichment, error) {
	q := repository.Build(options...)
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		match := true
		for _, c := range q.Conditions() {
			switch c.Field() {
			case "id":
				if id, ok := c.Value().(int64); ok && e.ID() != id {
					match = false
				}
			case "type":
				if t, ok := c.Value().(enrichment.Type); ok && e.Type() != t {
					match = false
				}
			case "subtype":
				if s, ok := c.Value().(enrichment.Subtype); ok && e.Subtype() != s {
					match = false
				}
			}
		}
		if match {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeEnrichmentStore) FindOne(ctx context.Context, options ...repository.Option) (enrichment.Enrichment, error) {
	results, err := f.Find(ctx, options...)
	if err != nil {
		return enrichment.Enrichment{}, err
	}
	if len(results) == 0 {
		return enrichment.Enrichment{}, errors.New("not found")
	}
	return results[0], nil
}

func (f *fakeEnrichmentStore) DeleteBy(_ context.Context, options ...repository.Option) error {
	q := repository.Build(options...)
	for id, e := range f.enrichments {
		match := true
		for _, c := range q.Conditions() {
			if c.Field() == "id" {
				if cid, ok := c.Value().(int64); ok && e.ID() != cid {
					match = false
				}
			}
		}
		if match {
			delete(f.enrichments, id)
		}
	}
	return nil
}

func (f *fakeEnrichmentStore) Save(_ context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	id := f.nextID
	f.nextID++
	saved := e.WithID(id)
	f.enrichments[id] = saved
	return saved, nil
}

func (f *fakeEnrichmentStore) Delete(_ context.Context, e enrichment.Enrichment) error {
	delete(f.enrichments, e.ID())
	return nil
}

func (f *fakeEnrichmentStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.enrichments)), nil
}

func (f *fakeEnrichmentStore) FindByEntityKey(_ context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		if e.EntityTypeKey() == key {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeEnrichmentStore) FindByCommitSHA(_ context.Context, commitSHA string, options ...repository.Option) ([]enrichment.Enrichment, error) {
	if f.associations == nil {
		return nil, nil
	}
	q := repository.Build(options...)
	var result []enrichment.Enrichment
	for _, a := range f.associations.associations {
		if a.EntityID() == commitSHA && a.EntityType() == enrichment.EntityTypeCommit {
			if e, ok := f.enrichments[a.EnrichmentID()]; ok {
				match := true
				for _, c := range q.Conditions() {
					switch c.Field() {
					case "type":
						if t, ok := c.Value().(enrichment.Type); ok && e.Type() != t {
							match = false
						}
					case "subtype":
						if s, ok := c.Value().(enrichment.Subtype); ok && e.Subtype() != s {
							match = false
						}
					}
				}
				if match {
					result = append(result, e)
				}
			}
		}
	}
	return result, nil
}

func (f *fakeEnrichmentStore) CountByCommitSHA(_ context.Context, _ string, _ ...repository.Option) (int64, error) {
	return 0, nil
}

func (f *fakeEnrichmentStore) FindByCommitSHAs(_ context.Context, _ []string, _ ...repository.Option) ([]enrichment.Enrichment, error) {
	return nil, nil
}

func (f *fakeEnrichmentStore) CountByCommitSHAs(_ context.Context, _ []string, _ ...repository.Option) (int64, error) {
	return 0, nil
}

type fakeAssociationStore struct {
	associations map[int64]enrichment.Association
	nextID       int64
}

func newFakeAssociationStore() *fakeAssociationStore {
	return &fakeAssociationStore{
		associations: make(map[int64]enrichment.Association),
		nextID:       1,
	}
}

func (f *fakeAssociationStore) Find(_ context.Context, options ...repository.Option) ([]enrichment.Association, error) {
	q := repository.Build(options...)
	var result []enrichment.Association
	for _, a := range f.associations {
		match := true
		for _, c := range q.Conditions() {
			switch c.Field() {
			case "id":
				if id, ok := c.Value().(int64); ok && a.ID() != id {
					match = false
				}
			case "enrichment_id":
				if eid, ok := c.Value().(int64); ok && a.EnrichmentID() != eid {
					match = false
				}
			case "entity_id":
				if entityID, ok := c.Value().(string); ok && a.EntityID() != entityID {
					match = false
				}
			case "entity_type":
				if entityType, ok := c.Value().(enrichment.EntityTypeKey); ok && a.EntityType() != entityType {
					match = false
				}
			}
		}
		if match {
			result = append(result, a)
		}
	}
	return result, nil
}

func (f *fakeAssociationStore) FindOne(ctx context.Context, options ...repository.Option) (enrichment.Association, error) {
	results, err := f.Find(ctx, options...)
	if err != nil {
		return enrichment.Association{}, err
	}
	if len(results) == 0 {
		return enrichment.Association{}, errors.New("not found")
	}
	return results[0], nil
}

func (f *fakeAssociationStore) DeleteBy(_ context.Context, options ...repository.Option) error {
	q := repository.Build(options...)
	for id, a := range f.associations {
		match := true
		for _, c := range q.Conditions() {
			switch c.Field() {
			case "enrichment_id":
				if eid, ok := c.Value().(int64); ok && a.EnrichmentID() != eid {
					match = false
				}
			case "entity_id":
				if entityID, ok := c.Value().(string); ok && a.EntityID() != entityID {
					match = false
				}
			}
		}
		if match {
			delete(f.associations, id)
		}
	}
	return nil
}

func (f *fakeAssociationStore) Save(_ context.Context, a enrichment.Association) (enrichment.Association, error) {
	id := f.nextID
	f.nextID++
	saved := a.WithID(id)
	f.associations[id] = saved
	return saved, nil
}

func (f *fakeAssociationStore) Delete(_ context.Context, a enrichment.Association) error {
	delete(f.associations, a.ID())
	return nil
}

func (f *fakeAssociationStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.associations)), nil
}

type fakeSnippetStore struct {
	snippets map[string][]snippet.Snippet
}

func newFakeSnippetStore() *fakeSnippetStore {
	return &fakeSnippetStore{
		snippets: make(map[string][]snippet.Snippet),
	}
}

func (f *fakeSnippetStore) SnippetsForCommit(_ context.Context, commitSHA string, _ ...repository.Option) ([]snippet.Snippet, error) {
	return f.snippets[commitSHA], nil
}

func (f *fakeSnippetStore) CountForCommit(_ context.Context, commitSHA string) (int64, error) {
	return int64(len(f.snippets[commitSHA])), nil
}

func (f *fakeSnippetStore) Save(_ context.Context, commitSHA string, snippets []snippet.Snippet) error {
	f.snippets[commitSHA] = snippets
	return nil
}

func (f *fakeSnippetStore) DeleteForCommit(_ context.Context, commitSHA string) error {
	delete(f.snippets, commitSHA)
	return nil
}

func (f *fakeSnippetStore) ByIDs(_ context.Context, _ []string) ([]snippet.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetStore) BySHA(_ context.Context, sha string) (snippet.Snippet, error) {
	for _, snippets := range f.snippets {
		for _, s := range snippets {
			if s.SHA() == sha {
				return s, nil
			}
		}
	}
	return snippet.Snippet{}, errors.New("not found")
}

type fakeGitAdapter struct {
	diff  string
	files []infraGit.FileInfo
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

func (f *fakeGitAdapter) AllBranches(_ context.Context, _ string) ([]infraGit.BranchInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) BranchCommits(_ context.Context, _, _ string) ([]infraGit.CommitInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) AllCommitsBulk(_ context.Context, _ string, _ *time.Time) (map[string]infraGit.CommitInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) BranchCommitSHAs(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeGitAdapter) AllBranchHeadSHAs(_ context.Context, _ string, _ []string) (map[string]string, error) {
	return nil, nil
}

func (f *fakeGitAdapter) CommitFiles(_ context.Context, _, _ string) ([]infraGit.FileInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.files, nil
}

func (f *fakeGitAdapter) RepositoryExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (f *fakeGitAdapter) CommitDetails(_ context.Context, _, _ string) (infraGit.CommitInfo, error) {
	return infraGit.CommitInfo{}, nil
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

func (f *fakeGitAdapter) AllTags(_ context.Context, _ string) ([]infraGit.TagInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) CommitDiff(_ context.Context, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.diff, nil
}

type fakeRepoStore struct {
	repos map[int64]repository.Repository
}

func newFakeRepoStore() *fakeRepoStore {
	return &fakeRepoStore{
		repos: make(map[int64]repository.Repository),
	}
}

func (f *fakeRepoStore) Find(_ context.Context, options ...repository.Option) ([]repository.Repository, error) {
	q := repository.Build(options...)
	var result []repository.Repository
	for _, r := range f.repos {
		match := true
		for _, c := range q.Conditions() {
			if c.Field() == "id" {
				if id, ok := c.Value().(int64); ok && r.ID() != id {
					match = false
				}
			}
		}
		if match {
			result = append(result, r)
		}
	}
	return result, nil
}

func (f *fakeRepoStore) FindOne(ctx context.Context, options ...repository.Option) (repository.Repository, error) {
	results, err := f.Find(ctx, options...)
	if err != nil {
		return repository.Repository{}, err
	}
	if len(results) == 0 {
		return repository.Repository{}, errors.New("not found")
	}
	return results[0], nil
}

func (f *fakeRepoStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	results, err := f.Find(ctx, options...)
	if err != nil {
		return false, err
	}
	return len(results) > 0, nil
}

func (f *fakeRepoStore) Save(_ context.Context, r repository.Repository) (repository.Repository, error) {
	return r, nil
}

func (f *fakeRepoStore) Delete(_ context.Context, _ repository.Repository) error {
	return nil
}

func (f *fakeRepoStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.repos)), nil
}

func newFakeEnrichmentContext(
	enrichmentStore *fakeEnrichmentStore,
	associationStore *fakeAssociationStore,
	enricher domainservice.Enricher,
	logger *slog.Logger,
) handler.EnrichmentContext {
	enrichmentStore.associations = associationStore
	return handler.EnrichmentContext{
		Enrichments:  enrichmentStore,
		Associations: associationStore,
		Query:        service.NewEnrichment(enrichmentStore, associationStore),
		Enricher:     enricher,
		Tracker:      &fakeTrackerFactory{},
		Logger:       logger,
	}
}

func TestCommitDescriptionHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	repoStore := newFakeRepoStore()
	enrichmentStore := newFakeEnrichmentStore()
	associationStore := newFakeAssociationStore()
	adapter := &fakeGitAdapter{diff: "diff --git a/file.go"}
	enricher := &fakeEnricher{}

	enrichCtx := newFakeEnrichmentContext(enrichmentStore, associationStore, enricher, logger)

	testRepo := repository.ReconstructRepository(
		1, "https://github.com/test/repo",
		repository.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo"),
		repository.NewTrackingConfig("main", "", ""),
		time.Now(), time.Now(),
	)
	repoStore.repos[1] = testRepo

	h := NewCommitDescription(
		repoStore,
		enrichCtx,
		adapter,
	)

	t.Run("creates commit description", func(t *testing.T) {
		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123def456",
		}

		err := h.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Len(t, enrichmentStore.enrichments, 1)
		assert.Len(t, associationStore.associations, 1)

		for _, e := range enrichmentStore.enrichments {
			assert.Equal(t, enrichment.TypeHistory, e.Type())
			assert.Equal(t, enrichment.SubtypeCommitDescription, e.Subtype())
		}
	})

	t.Run("skips when description exists", func(t *testing.T) {
		countBefore := len(enrichmentStore.enrichments)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123def456",
		}

		err := h.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Equal(t, countBefore, len(enrichmentStore.enrichments))
	})
}

func TestCreateSummaryHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	snippetStore := newFakeSnippetStore()
	enrichmentStore := newFakeEnrichmentStore()
	associationStore := newFakeAssociationStore()
	enricher := &fakeEnricher{}

	enrichCtx := newFakeEnrichmentContext(enrichmentStore, associationStore, enricher, logger)

	snippets := []snippet.Snippet{
		snippet.NewSnippet("func main() {}", ".go", nil),
		snippet.NewSnippet("def main():", ".py", nil),
	}
	snippetStore.snippets["abc123"] = snippets

	h := NewCreateSummary(
		snippetStore,
		enrichCtx,
	)

	t.Run("creates summaries for snippets", func(t *testing.T) {
		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123",
		}

		err := h.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Len(t, enrichmentStore.enrichments, 2)

		for _, e := range enrichmentStore.enrichments {
			assert.Equal(t, enrichment.TypeDevelopment, e.Type())
			assert.Equal(t, enrichment.SubtypeSnippetSummary, e.Subtype())
		}
	})

	t.Run("skips when no snippets", func(t *testing.T) {
		enrichmentStore2 := newFakeEnrichmentStore()
		associationStore2 := newFakeAssociationStore()
		snippetStore2 := newFakeSnippetStore()

		enrichCtx2 := newFakeEnrichmentContext(enrichmentStore2, associationStore2, enricher, logger)

		handler2 := NewCreateSummary(
			snippetStore2,
			enrichCtx2,
		)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "empty123",
		}

		err := handler2.Execute(ctx, payload)
		require.NoError(t, err)

		assert.Len(t, enrichmentStore2.enrichments, 0)
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
