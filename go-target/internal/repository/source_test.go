package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/helixml/kodit/internal/git"
	"github.com/stretchr/testify/assert"
)

func TestNewSource(t *testing.T) {
	tests := []struct {
		name           string
		repo           git.Repo
		expectedStatus SourceStatus
	}{
		{
			name:           "new repo without working copy has pending status",
			repo:           mustNewRepo("https://github.com/test/repo.git"),
			expectedStatus: StatusPending,
		},
		{
			name:           "repo with working copy has cloned status",
			repo:           repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo"),
			expectedStatus: StatusCloned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewSource(tt.repo)

			assert.Equal(t, tt.expectedStatus, source.Status())
			assert.Equal(t, tt.repo.RemoteURL(), source.RemoteURL())
		})
	}
}

func TestReconstructSource(t *testing.T) {
	repo := mustNewRepo("https://github.com/test/repo.git")
	source := ReconstructSource(repo, StatusSyncing, "")

	assert.Equal(t, StatusSyncing, source.Status())
	assert.Equal(t, repo.RemoteURL(), source.RemoteURL())
	assert.Empty(t, source.LastError())
}

func TestSource_WithStatus(t *testing.T) {
	repo := mustNewRepo("https://github.com/test/repo.git")
	source := NewSource(repo)

	updated := source.WithStatus(StatusCloning)

	assert.Equal(t, StatusCloning, updated.Status())
	assert.Equal(t, StatusPending, source.Status()) // original unchanged
}

func TestSource_WithError(t *testing.T) {
	repo := mustNewRepo("https://github.com/test/repo.git")
	source := NewSource(repo)

	err := errors.New("clone failed: network error")
	updated := source.WithError(err)

	assert.Equal(t, StatusFailed, updated.Status())
	assert.Equal(t, "clone failed: network error", updated.LastError())
}

func TestSource_WithWorkingCopy(t *testing.T) {
	repo := mustNewRepo("https://github.com/test/repo.git")
	source := NewSource(repo)
	wc := git.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo.git")

	updated := source.WithWorkingCopy(wc)

	assert.Equal(t, StatusCloned, updated.Status())
	assert.True(t, updated.IsCloned())
	assert.Equal(t, "/tmp/repo", updated.ClonedPath())
	assert.Empty(t, updated.LastError())
}

func TestSource_WithTrackingConfig(t *testing.T) {
	repo := mustNewRepo("https://github.com/test/repo.git")
	source := NewSource(repo)
	tc := git.NewTrackingConfigForBranch("main")

	updated := source.WithTrackingConfig(tc)

	assert.Equal(t, "main", updated.TrackingConfig().Branch())
}

func TestSource_IsCloned(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		expected bool
	}{
		{
			name:     "new repo is not cloned",
			source:   NewSource(mustNewRepo("https://github.com/test/repo.git")),
			expected: false,
		},
		{
			name:     "repo with working copy is cloned",
			source:   NewSource(repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo")),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.source.IsCloned())
		})
	}
}

func TestSource_ClonedPath(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		expected string
	}{
		{
			name:     "uncloned repo returns empty path",
			source:   NewSource(mustNewRepo("https://github.com/test/repo.git")),
			expected: "",
		},
		{
			name:     "cloned repo returns path",
			source:   NewSource(repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo")),
			expected: "/tmp/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.source.ClonedPath())
		})
	}
}

func TestSource_CanSync(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		expected bool
	}{
		{
			name:     "uncloned repo cannot sync",
			source:   NewSource(mustNewRepo("https://github.com/test/repo.git")),
			expected: false,
		},
		{
			name:     "cloned repo can sync",
			source:   NewSource(repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo")),
			expected: true,
		},
		{
			name:     "syncing repo cannot sync again",
			source:   NewSource(repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo")).WithStatus(StatusSyncing),
			expected: false,
		},
		{
			name:     "deleting repo cannot sync",
			source:   NewSource(repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo")).WithStatus(StatusDeleting),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.source.CanSync())
		})
	}
}

func TestSource_CanDelete(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		expected bool
	}{
		{
			name:     "pending repo can be deleted",
			source:   NewSource(mustNewRepo("https://github.com/test/repo.git")),
			expected: true,
		},
		{
			name:     "cloned repo can be deleted",
			source:   NewSource(repoWithWorkingCopy("https://github.com/test/repo.git", "/tmp/repo")),
			expected: true,
		},
		{
			name:     "deleting repo cannot be deleted again",
			source:   NewSource(mustNewRepo("https://github.com/test/repo.git")).WithStatus(StatusDeleting),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.source.CanDelete())
		})
	}
}

func TestSourceStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   SourceStatus
		terminal bool
	}{
		{StatusPending, false},
		{StatusCloning, false},
		{StatusCloned, true},
		{StatusSyncing, false},
		{StatusFailed, true},
		{StatusDeleting, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.terminal, tt.status.IsTerminal())
		})
	}
}

func TestSource_Accessors(t *testing.T) {
	now := time.Now()
	repo := git.ReconstructRepo(
		42,
		"https://github.com/test/repo.git",
		git.NewWorkingCopy("/tmp/repo", "https://github.com/test/repo.git"),
		git.NewTrackingConfigForBranch("main"),
		now,
		now,
	)
	source := NewSource(repo)

	assert.Equal(t, int64(42), source.ID())
	assert.Equal(t, "https://github.com/test/repo.git", source.RemoteURL())
	assert.Equal(t, "/tmp/repo", source.WorkingCopy().Path())
	assert.Equal(t, "main", source.TrackingConfig().Branch())
	assert.Equal(t, now, source.CreatedAt())
	assert.Equal(t, now, source.UpdatedAt())
	assert.Equal(t, repo, source.Repo())
}

func mustNewRepo(url string) git.Repo {
	repo, err := git.NewRepo(url)
	if err != nil {
		panic(err)
	}
	return repo
}

func repoWithWorkingCopy(url, path string) git.Repo {
	repo := mustNewRepo(url)
	wc := git.NewWorkingCopy(path, url)
	return repo.WithWorkingCopy(wc)
}
