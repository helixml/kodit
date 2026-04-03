package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// testStores bundles real persistence stores backed by a migrated in-memory database.
// Tests should prefer these over fakes wherever possible.
type testStores struct {
	repos        persistence.RepositoryStore
	pipelines    persistence.PipelineStore
	commits      persistence.CommitStore
	branches     persistence.BranchStore
	tags         persistence.TagStore
	tasks        persistence.TaskStore
	enrichments  persistence.EnrichmentStore
	associations persistence.AssociationStore
	lineRanges   persistence.SourceLocationStore
}

func newTestStores(t *testing.T) testStores {
	t.Helper()
	db := testdb.New(t)
	return testStores{
		repos:        persistence.NewRepositoryStore(db),
		pipelines:    persistence.NewPipelineStore(db),
		commits:      persistence.NewCommitStore(db),
		branches:     persistence.NewBranchStore(db),
		tags:         persistence.NewTagStore(db),
		tasks:        persistence.NewTaskStore(db),
		enrichments:  persistence.NewEnrichmentStore(db),
		associations: persistence.NewAssociationStore(db),
		lineRanges:   persistence.NewSourceLocationStore(db),
	}
}

// fakeGitAdapter provides a configurable in-memory git.Adapter for tests.
// Only git operations genuinely need faking — they interact with the filesystem and git CLI.
type fakeGitAdapter struct {
	content     map[string][]byte // "commitSHA:filePath" → content
	commitFiles []git.FileInfo
	matches     []git.GrepMatch
	grepErr     error
	cloneFn     func(remoteURI, localPath string) error
	cloneCalled bool
}

func (f *fakeGitAdapter) FileContent(_ context.Context, _, commitSHA, filePath string) ([]byte, error) {
	key := commitSHA + ":" + filePath
	content, ok := f.content[key]
	if !ok {
		return nil, fmt.Errorf("get file: %w", git.ErrFileNotFound)
	}
	return content, nil
}

func (f *fakeGitAdapter) CloneRepository(_ context.Context, remoteURI, localPath string) error {
	f.cloneCalled = true
	if f.cloneFn != nil {
		return f.cloneFn(remoteURI, localPath)
	}
	return nil
}

func (f *fakeGitAdapter) RepositoryExists(_ context.Context, localPath string) (bool, error) {
	_, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (f *fakeGitAdapter) CommitFiles(_ context.Context, _, _ string) ([]git.FileInfo, error) {
	return f.commitFiles, nil
}

func (f *fakeGitAdapter) Grep(_ context.Context, _, _, _, _ string, _ int) ([]git.GrepMatch, error) {
	return f.matches, f.grepErr
}

func (f *fakeGitAdapter) CheckoutCommit(context.Context, string, string) error   { return nil }
func (f *fakeGitAdapter) CheckoutBranch(context.Context, string, string) error   { return nil }
func (f *fakeGitAdapter) FetchRepository(context.Context, string) error          { return nil }
func (f *fakeGitAdapter) PullRepository(context.Context, string) error           { return nil }
func (f *fakeGitAdapter) EnsureRepository(context.Context, string, string) error { return nil }

func (f *fakeGitAdapter) AllBranches(context.Context, string) ([]git.BranchInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) BranchCommits(context.Context, string, string) ([]git.CommitInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) AllCommitsBulk(context.Context, string, *time.Time) (map[string]git.CommitInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) BranchCommitSHAs(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (f *fakeGitAdapter) AllBranchHeadSHAs(context.Context, string, []string) (map[string]string, error) {
	return nil, nil
}

func (f *fakeGitAdapter) CommitDetails(context.Context, string, string) (git.CommitInfo, error) {
	return git.CommitInfo{}, nil
}

func (f *fakeGitAdapter) DefaultBranch(context.Context, string) (string, error) {
	return "main", nil
}

func (f *fakeGitAdapter) LatestCommitSHA(context.Context, string, string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) AllTags(context.Context, string) ([]git.TagInfo, error) {
	return nil, nil
}

func (f *fakeGitAdapter) CommitDiff(context.Context, string, string) (string, error) {
	return "", nil
}
