package git

import (
	"errors"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// ErrNotGitRepository is returned when a git operation is requested for a
// working copy kodit does not treat as a git repository.
var ErrNotGitRepository = errors.New("not a git repository")

// Layout decides which working copies kodit treats as git repositories.
//
// Git is never detected automatically in a directory kodit did not clone. A
// file:// repository is a directory someone else writes to, and git would
// honour whatever it finds there: a HEAD/config/objects/refs layout makes the
// directory a bare repository, objects/info/alternates lets it read another
// clone's objects, and its config can define commands (textconv, external
// diff, fsmonitor, hooks) that git runs. Such a directory is indexed as plain
// files unless the operator allows local git repositories, and even then only
// with a real .git directory.
type Layout struct {
	cloneDir      string
	allowLocalGit bool
}

// NewLayout creates a Layout for clones under cloneDir. allowLocalGit lets a
// directory outside cloneDir be read with git when it has a real .git
// directory.
func NewLayout(cloneDir string, allowLocalGit bool) Layout {
	return Layout{cloneDir: cloneDir, allowLocalGit: allowLocalGit}
}

// IsGitRepository reports whether kodit reads the working copy at path with
// git.
func (l Layout) IsGitRepository(path string) bool {
	if !l.allowLocalGit && !l.Managed(path) {
		return false
	}
	info, err := os.Lstat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

// Managed reports whether path is inside the directory kodit clones into.
func (l Layout) Managed(path string) bool {
	if l.cloneDir == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(l.cloneDir), filepath.Clean(path))
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

// hardened adds the configuration every git command kodit runs must carry,
// whatever the repository's own configuration says: no implicit bare
// repositories, no filesystem monitor, and no hooks.
func hardened(c *gitcmd.Command) *gitcmd.Command {
	return c.AddConfig("safe.bareRepository", "explicit").
		AddConfig("core.fsmonitor", "false").
		AddConfig("core.hooksPath", os.DevNull)
}
