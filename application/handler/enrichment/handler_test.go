package enrichment

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	infraGit "github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTracker struct{}

func (f *fakeTracker) SetTotal(_ context.Context, _ int)             {}
func (f *fakeTracker) SetCurrent(_ context.Context, _ int, _ string) {}
func (f *fakeTracker) Skip(_ context.Context, _ string)              {}
func (f *fakeTracker) Fail(_ context.Context, _ string)              {}
func (f *fakeTracker) Complete(_ context.Context)                    {}

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

func openTestDB(t *testing.T) database.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///"+dbPath)
	require.NoError(t, err)
	require.NoError(t, persistence.AutoMigrate(db))
	t.Cleanup(func() { _ = db.Close() })
	return db
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

func newEnrichmentContext(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	enricher domainservice.Enricher,
	logger *slog.Logger,
) handler.EnrichmentContext {
	return handler.EnrichmentContext{
		Enrichments:  enrichmentStore,
		Associations: associationStore,
		Enricher:     enricher,
		Tracker:      &fakeTrackerFactory{},
		Logger:       logger,
	}
}

func TestCommitDescriptionHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db := openTestDB(t)
	repoStore := persistence.NewRepositoryStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	adapter := &fakeGitAdapter{diff: "diff --git a/file.go"}
	enricher := &fakeEnricher{}

	enrichCtx := newEnrichmentContext(enrichmentStore, associationStore, enricher, logger)

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	h, err := NewCommitDescription(
		repoStore,
		enrichCtx,
		adapter,
	)
	require.NoError(t, err)

	t.Run("creates commit description", func(t *testing.T) {
		payload := map[string]any{
			"repository_id": savedRepo.ID(),
			"commit_sha":    "abc123def456",
		}

		err := h.Execute(ctx, payload)
		require.NoError(t, err)

		descriptions, err := enrichmentStore.FindByCommitSHA(ctx, "abc123def456",
			enrichment.WithType(enrichment.TypeHistory),
			enrichment.WithSubtype(enrichment.SubtypeCommitDescription))
		require.NoError(t, err)
		assert.Len(t, descriptions, 1)
		assert.Equal(t, enrichment.TypeHistory, descriptions[0].Type())
		assert.Equal(t, enrichment.SubtypeCommitDescription, descriptions[0].Subtype())
	})

	t.Run("skips when description exists", func(t *testing.T) {
		countBefore, err := enrichmentStore.Count(ctx)
		require.NoError(t, err)

		payload := map[string]any{
			"repository_id": savedRepo.ID(),
			"commit_sha":    "abc123def456",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		countAfter, err := enrichmentStore.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, countBefore, countAfter)
	})
}

func TestCreateSummaryHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	enricher := &fakeEnricher{}

	t.Run("creates summaries for snippets", func(t *testing.T) {
		db := openTestDB(t)
		enrichmentStore := persistence.NewEnrichmentStore(db)
		associationStore := persistence.NewAssociationStore(db)

		enrichCtx := newEnrichmentContext(enrichmentStore, associationStore, enricher, logger)

		// Seed snippet enrichments for commit "abc123"
		snip1 := enrichment.NewSnippetEnrichmentWithLanguage("func main() {}", "go")
		saved1, err := enrichmentStore.Save(ctx, snip1)
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved1.ID(), "abc123"))
		require.NoError(t, err)

		snip2 := enrichment.NewSnippetEnrichmentWithLanguage("def main():", "py")
		saved2, err := enrichmentStore.Save(ctx, snip2)
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved2.ID(), "abc123"))
		require.NoError(t, err)

		h, err := NewCreateSummary(enrichCtx)
		require.NoError(t, err)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "abc123",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		summaries, err := enrichmentStore.FindByCommitSHA(ctx, "abc123",
			enrichment.WithSubtype(enrichment.SubtypeSnippetSummary))
		require.NoError(t, err)
		assert.Len(t, summaries, 2)
		for _, s := range summaries {
			assert.Equal(t, enrichment.TypeDevelopment, s.Type())
		}
	})

	t.Run("skips when no snippets", func(t *testing.T) {
		db := openTestDB(t)
		enrichmentStore := persistence.NewEnrichmentStore(db)
		associationStore := persistence.NewAssociationStore(db)

		enrichCtx := newEnrichmentContext(enrichmentStore, associationStore, enricher, logger)

		h, err := NewCreateSummary(enrichCtx)
		require.NoError(t, err)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "empty123",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		count, err := enrichmentStore.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
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
