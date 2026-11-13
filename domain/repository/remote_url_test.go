package repository

import (
	"errors"
	"testing"
)

func TestValidateRemoteURL(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://github.com/org/repo.git", true},
		{"http://git.internal/repo", true},
		{"ssh://git@github.com/org/repo.git", true},
		{"git://git.example.com/repo.git", true},
		{"file:///srv/knowledge/folder", true},
		{"git@github.com:org/repo.git", true},
		{"github.com:org/repo.git", true},
		{"", false},
		{"/srv/repo", false},
		{"./repo", false},
		{"ext::sh -c true", false},
		{"fd::3", false},
		{"ftp://example.com/repo", false},
		{"-uhelp", false},
		{"dir/sub:path", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := ValidateRemoteURL(tt.url)
			if tt.ok && err != nil {
				t.Errorf("rejected %q: %v", tt.url, err)
			}
			if !tt.ok && err == nil {
				t.Errorf("accepted %q", tt.url)
			}
			if !tt.ok && err != nil && tt.url != "" && !errors.Is(err, ErrUnsupportedRemoteURL) {
				t.Errorf("error for %q is not ErrUnsupportedRemoteURL: %v", tt.url, err)
			}
		})
	}
}
