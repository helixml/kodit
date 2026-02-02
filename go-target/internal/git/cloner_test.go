package git

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeURIForPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://github.com/user/repo.git",
			expected: "github.com_user_repo.git",
		},
		{
			input:    "git@github.com:user/repo.git",
			expected: "git_github.com_user_repo.git",
		},
		{
			input:    "http://gitlab.com/org/project",
			expected: "gitlab.com_org_project",
		},
		{
			input:    "simple-name",
			expected: "simple-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeURIForPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloner_ClonePathFromURI(t *testing.T) {
	adapter := NewFakeAdapter() // From scanner_test.go
	logger := testLogger()
	cloner := NewCloner(adapter, "/tmp/clones", logger)

	path := cloner.ClonePathFromURI("https://github.com/user/repo.git")
	assert.Equal(t, "/tmp/clones/github.com_user_repo.git", path)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
