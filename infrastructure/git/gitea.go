package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	giteagit "code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
)

// ErrBranchNotFound indicates the requested branch was not found.
var ErrBranchNotFound = errors.New("branch not found")

// GiteaAdapter implements Adapter using Gitea's git module (native git binary).
type GiteaAdapter struct {
	logger *slog.Logger
}

var giteaInitOnce sync.Once
var giteaInitErr error

// NewGiteaAdapter creates a new GiteaAdapter. It initializes the Gitea git
// module once (verifying the git binary is available).
func NewGiteaAdapter(logger *slog.Logger) (*GiteaAdapter, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git is not installed or not in PATH: install git and try again")
	}

	giteaInitOnce.Do(func() {
		// Gitea's git module requires a HomePath for its git environment.
		// Use a temporary directory so git config is isolated.
		home, err := os.MkdirTemp("", "kodit-git-home-*")
		if err != nil {
			giteaInitErr = fmt.Errorf("create git home directory: %w", err)
			return
		}
		setting.Git.HomePath = home

		giteaInitErr = giteagit.InitSimple()
	})
	if giteaInitErr != nil {
		return nil, fmt.Errorf("init git: %w", giteaInitErr)
	}

	return &GiteaAdapter{logger: logger}, nil
}

// CloneRepository clones a repository to local path.
func (g *GiteaAdapter) CloneRepository(ctx context.Context, remoteURI string, localPath string) error {
	g.logger.Info("cloning repository",
		slog.String("uri", remoteURI),
		slog.String("path", localPath),
	)

	// Remove existing directory if it exists
	if _, err := os.Stat(localPath); err == nil {
		g.logger.Warn("removing existing directory", slog.String("path", localPath))
		if err := os.RemoveAll(localPath); err != nil {
			return fmt.Errorf("remove existing directory: %w", err)
		}
	}

	err := giteagit.Clone(ctx, remoteURI, localPath, giteagit.CloneRepoOptions{})
	if err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}

	return nil
}

// CheckoutCommit checks out a specific commit.
func (g *GiteaAdapter) CheckoutCommit(ctx context.Context, localPath string, commitSHA string) error {
	_, _, err := gitcmd.NewCommand("checkout", "--force").
		AddDynamicArguments(commitSHA).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return fmt.Errorf("checkout commit: %w", err)
	}
	return nil
}

// CheckoutBranch checks out a specific branch.
func (g *GiteaAdapter) CheckoutBranch(ctx context.Context, localPath string, branchName string) error {
	// Try local branch first
	_, _, err := gitcmd.NewCommand("checkout", "--force").
		AddDynamicArguments(branchName).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err == nil {
		return nil
	}

	// Try remote branch
	_, _, err = gitcmd.NewCommand("checkout", "--force", "-b").
		AddDynamicArguments(branchName).
		AddOptionValues("--track", "origin/"+branchName).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}
	return nil
}

// FetchRepository fetches latest changes for existing repository.
func (g *GiteaAdapter) FetchRepository(ctx context.Context, localPath string) error {
	_, _, err := gitcmd.NewCommand("fetch", "--force", "origin").
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return fmt.Errorf("fetch repository: %w", err)
	}
	return nil
}

// PullRepository pulls latest changes for existing repository.
func (g *GiteaAdapter) PullRepository(ctx context.Context, localPath string) error {
	// Fetch first
	if err := g.FetchRepository(ctx, localPath); err != nil {
		return err
	}

	// Pull - tolerate failure in detached HEAD state
	_, _, err := gitcmd.NewCommand("pull", "--force", "origin").
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		g.logger.Debug("pull failed (possibly detached HEAD)", slog.String("error", err.Error()))
	}

	return nil
}

// AllBranches returns all branches in repository.
func (g *GiteaAdapter) AllBranches(ctx context.Context, localPath string) ([]BranchInfo, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Get HEAD for detecting default branch
	headSHA, err := repo.GetRefCommitID("HEAD")
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	// Get local branches
	names, _, err := repo.GetBranchNames(0, 0)
	if err != nil {
		return nil, fmt.Errorf("get branches: %w", err)
	}

	seen := make(map[string]bool)
	var branches []BranchInfo
	for _, name := range names {
		sha, err := repo.GetBranchCommitID(name)
		if err != nil {
			continue
		}
		branches = append(branches, BranchInfo{
			Name:      name,
			HeadSHA:   sha,
			IsDefault: sha == headSHA,
		})
		seen[name] = true
	}

	// Get remote branches
	stdout, _, err := gitcmd.NewCommand("branch", "-r", "--format=%(refname:short) %(objectname)").
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return nil, fmt.Errorf("get remote branches: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		refName := parts[0]
		sha := parts[1]

		// Skip origin/HEAD
		if refName == "origin/HEAD" {
			continue
		}

		// Remove origin/ prefix
		name := strings.TrimPrefix(refName, "origin/")
		if !seen[name] {
			branches = append(branches, BranchInfo{
				Name:      name,
				HeadSHA:   sha,
				IsDefault: false,
			})
			seen[name] = true
		}
	}

	return branches, nil
}

// BranchCommits returns commit history for a specific branch.
func (g *GiteaAdapter) BranchCommits(ctx context.Context, localPath string, branchName string) ([]CommitInfo, error) {
	ref, err := g.resolveBranch(ctx, localPath, branchName)
	if err != nil {
		return nil, err
	}

	stdout, _, err := gitcmd.NewCommand("log", commitLogFormat).
		AddDynamicArguments(ref).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}

	return parseCommitLog(stdout), nil
}

// AllCommitsBulk returns all commits from all branches in bulk.
func (g *GiteaAdapter) AllCommitsBulk(ctx context.Context, localPath string, since *time.Time) (map[string]CommitInfo, error) {
	cmd := gitcmd.NewCommand("log", "--all", commitLogFormat)
	if since != nil {
		cmd = cmd.AddOptionFormat("--since=%s", since.Format(time.RFC3339))
	}

	stdout, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}

	entries := parseCommitLog(stdout)
	commits := make(map[string]CommitInfo, len(entries))
	for _, c := range entries {
		commits[c.SHA] = c
	}
	return commits, nil
}

// BranchCommitSHAs returns only commit SHAs for a branch.
func (g *GiteaAdapter) BranchCommitSHAs(ctx context.Context, localPath string, branchName string) ([]string, error) {
	ref, err := g.resolveBranch(ctx, localPath, branchName)
	if err != nil {
		return nil, err
	}

	stdout, _, err := gitcmd.NewCommand("log", "--format=%H").
		AddDynamicArguments(ref).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var shas []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			shas = append(shas, line)
		}
	}
	return shas, nil
}

// AllBranchHeadSHAs returns head commit SHAs for all branches in one operation.
func (g *GiteaAdapter) AllBranchHeadSHAs(ctx context.Context, localPath string, branchNames []string) (map[string]string, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	result := make(map[string]string)
	for _, name := range branchNames {
		sha, err := repo.GetBranchCommitID(name)
		if err != nil {
			// Try remote ref
			sha, err = repo.GetRefCommitID("refs/remotes/origin/" + name)
			if err != nil {
				g.logger.Debug("branch not found", slog.String("branch", name))
				continue
			}
		}
		result[name] = sha
	}

	return result, nil
}

// CommitFiles returns all files in a specific commit from the git tree.
func (g *GiteaAdapter) CommitFiles(ctx context.Context, localPath string, commitSHA string) ([]FileInfo, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	commit, err := repo.GetCommit(commitSHA)
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	entries, err := commit.ListEntriesRecursiveWithSize()
	if err != nil {
		return nil, fmt.Errorf("list tree entries: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		if entry.IsDir() || entry.IsSubModule() {
			continue
		}
		files = append(files, FileInfo{
			Path:     entry.Name(),
			BlobSHA:  entry.ID.String(),
			Size:     entry.Size(),
			MimeType: guessMimeType(entry.Name()),
		})
	}

	return files, nil
}

// RepositoryExists checks if repository exists at local path.
func (g *GiteaAdapter) RepositoryExists(ctx context.Context, localPath string) (bool, error) {
	_, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		// Check if the path exists at all
		_, statErr := os.Stat(localPath)
		if os.IsNotExist(statErr) {
			return false, nil
		}
		// Check for .git directory
		_, statErr = os.Stat(filepath.Join(localPath, ".git"))
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, fmt.Errorf("check repository: %w", err)
	}
	return true, nil
}

// CommitDetails returns detailed information about a specific commit.
func (g *GiteaAdapter) CommitDetails(ctx context.Context, localPath string, commitSHA string) (CommitInfo, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return CommitInfo{}, fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	commit, err := repo.GetCommit(commitSHA)
	if err != nil {
		return CommitInfo{}, fmt.Errorf("get commit: %w", err)
	}

	return giteaCommitToInfo(commit), nil
}

// EnsureRepository clones if doesn't exist, otherwise fetches latest changes.
func (g *GiteaAdapter) EnsureRepository(ctx context.Context, remoteURI string, localPath string) error {
	exists, err := g.RepositoryExists(ctx, localPath)
	if err != nil {
		return err
	}

	if exists {
		return g.PullRepository(ctx, localPath)
	}
	return g.CloneRepository(ctx, remoteURI, localPath)
}

// FileContent returns file content at specific commit.
func (g *GiteaAdapter) FileContent(ctx context.Context, localPath string, commitSHA string, filePath string) ([]byte, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	commit, err := repo.GetCommit(commitSHA)
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	content, err := commit.GetFileContent(filePath, 0)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	return []byte(content), nil
}

// DefaultBranch returns the default branch name with fallback strategies.
func (g *GiteaAdapter) DefaultBranch(ctx context.Context, localPath string) (string, error) {
	// Strategy 1: Try symbolic-ref HEAD (origin/HEAD)
	stdout, _, runErr := gitcmd.NewCommand("symbolic-ref", "refs/remotes/origin/HEAD").
		RunStdString(ctx, &gitcmd.RunOpts{Dir: localPath})
	if runErr == nil {
		ref := strings.TrimSpace(stdout)
		ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
		if ref != "" {
			return ref, nil
		}
	}

	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Strategy 2: Look for common default branch names
	for _, candidate := range []string{"main", "master"} {
		if repo.IsBranchExist(candidate) {
			return candidate, nil
		}
	}

	// Strategy 3: Use first available branch
	names, _, err := repo.GetBranchNames(0, 1)
	if err != nil {
		return "", fmt.Errorf("get branches: %w", err)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no branches found")
	}
	return names[0], nil
}

// LatestCommitSHA returns the latest commit SHA for a branch.
func (g *GiteaAdapter) LatestCommitSHA(ctx context.Context, localPath string, branchName string) (string, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	if branchName == "" || branchName == "HEAD" {
		sha, err := repo.GetRefCommitID("HEAD")
		if err != nil {
			return "", fmt.Errorf("get HEAD: %w", err)
		}
		return sha, nil
	}

	sha, err := repo.GetBranchCommitID(branchName)
	if err != nil {
		// Try remote ref
		sha, err = repo.GetRefCommitID("refs/remotes/origin/" + branchName)
		if err != nil {
			return "", fmt.Errorf("%w: %s", ErrBranchNotFound, branchName)
		}
	}
	return sha, nil
}

// AllTags returns all tags in repository.
func (g *GiteaAdapter) AllTags(ctx context.Context, localPath string) ([]TagInfo, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	giteaTags, _, err := repo.GetTagInfos(0, 0)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}

	var tags []TagInfo
	for _, t := range giteaTags {
		tag := TagInfo{
			Name: t.Name,
		}

		if t.Object != nil {
			tag.TargetCommitSHA = t.Object.String()
		} else if t.ID != nil {
			tag.TargetCommitSHA = t.ID.String()
		}

		if t.Tagger != nil {
			tag.TaggerName = t.Tagger.Name
			tag.TaggerEmail = t.Tagger.Email
			tag.TaggedAt = t.Tagger.When
			tag.Message = t.Message
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

// CommitDiff returns the diff for a specific commit.
func (g *GiteaAdapter) CommitDiff(ctx context.Context, localPath string, commitSHA string) (string, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	var buf bytes.Buffer
	err = giteagit.GetRawDiff(repo, commitSHA, giteagit.RawDiffNormal, &buf)
	if err != nil {
		return "", fmt.Errorf("get diff: %w", err)
	}

	return buf.String(), nil
}

// resolveBranch resolves a branch name to a ref that git log can use.
// It checks local branches first, then remote branches.
func (g *GiteaAdapter) resolveBranch(ctx context.Context, localPath string, branchName string) (string, error) {
	repo, err := giteagit.OpenRepository(ctx, localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Try local branch
	_, err = repo.GetBranchCommitID(branchName)
	if err == nil {
		return branchName, nil
	}

	// Try remote branch
	_, err = repo.GetRefCommitID("refs/remotes/origin/" + branchName)
	if err == nil {
		return "refs/remotes/origin/" + branchName, nil
	}

	return "", fmt.Errorf("%w: %s", ErrBranchNotFound, branchName)
}

// commitLogFormat is the git log format string for parsing commits.
// Fields are separated by \x00, records by \x01.
const commitLogFormat = "--format=%x01%H%x00%B%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%P"

// parseCommitLog parses the output of git log with commitLogFormat.
func parseCommitLog(stdout string) []CommitInfo {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil
	}

	records := strings.Split(stdout, "\x01")
	var commits []CommitInfo

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.SplitN(record, "\x00", 9)
		if len(fields) < 9 {
			continue
		}

		authoredAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(fields[4]))
		committedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(fields[7]))

		info := CommitInfo{
			SHA:            strings.TrimSpace(fields[0]),
			Message:        strings.TrimSpace(fields[1]),
			AuthorName:     strings.TrimSpace(fields[2]),
			AuthorEmail:    strings.TrimSpace(fields[3]),
			AuthoredAt:     authoredAt,
			CommitterName:  strings.TrimSpace(fields[5]),
			CommitterEmail: strings.TrimSpace(fields[6]),
			CommittedAt:    committedAt,
		}

		parents := strings.TrimSpace(fields[8])
		if parents != "" {
			// Take only the first parent
			parentParts := strings.Fields(parents)
			if len(parentParts) > 0 {
				info.ParentSHA = parentParts[0]
			}
		}

		commits = append(commits, info)
	}

	return commits
}

// giteaCommitToInfo converts a gitea Commit to our CommitInfo type.
func giteaCommitToInfo(c *giteagit.Commit) CommitInfo {
	info := CommitInfo{
		SHA:            c.ID.String(),
		Message:        c.CommitMessage,
		AuthorName:     c.Author.Name,
		AuthorEmail:    c.Author.Email,
		AuthoredAt:     c.Author.When,
		CommitterName:  c.Committer.Name,
		CommitterEmail: c.Committer.Email,
		CommittedAt:    c.Committer.When,
	}

	if len(c.Parents) > 0 {
		info.ParentSHA = c.Parents[0].String()
	}

	return info
}

func guessMimeType(_ string) string {
	return "application/octet-stream"
}

// Ensure GiteaAdapter implements Adapter.
var _ Adapter = (*GiteaAdapter)(nil)
