package gitadapter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitpkg "github.com/helixml/kodit/internal/git"
)

// ErrBranchNotFound indicates the requested branch was not found.
var ErrBranchNotFound = errors.New("branch not found")

// GoGit implements gitpkg.Adapter using go-git library.
type GoGit struct {
	logger *slog.Logger
}

// NewGoGit creates a new GoGit adapter.
func NewGoGit(logger *slog.Logger) *GoGit {
	return &GoGit{logger: logger}
}

// CloneRepository clones a repository to local path.
func (g *GoGit) CloneRepository(ctx context.Context, remoteURI string, localPath string) error {
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

	_, err := gogit.PlainCloneContext(ctx, localPath, false, &gogit.CloneOptions{
		URL:      remoteURI,
		Progress: nil,
	})
	if err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}

	return nil
}

// CheckoutCommit checks out a specific commit.
func (g *GoGit) CheckoutCommit(ctx context.Context, localPath string, commitSHA string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Hash:  plumbing.NewHash(commitSHA),
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("checkout commit: %w", err)
	}

	return nil
}

// CheckoutBranch checks out a specific branch.
func (g *GoGit) CheckoutBranch(ctx context.Context, localPath string, branchName string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	// Try local branch first
	branchRef := plumbing.NewBranchReferenceName(branchName)
	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: branchRef,
		Force:  true,
	})
	if err != nil {
		// Try remote branch
		remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branchName)
		err = worktree.Checkout(&gogit.CheckoutOptions{
			Branch: remoteBranchRef,
			Force:  true,
		})
		if err != nil {
			return fmt.Errorf("checkout branch: %w", err)
		}
	}

	return nil
}

// FetchRepository fetches latest changes for existing repository.
func (g *GoGit) FetchRepository(ctx context.Context, localPath string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	err = repo.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch repository: %w", err)
	}

	return nil
}

// PullRepository pulls latest changes for existing repository.
func (g *GoGit) PullRepository(ctx context.Context, localPath string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	// Fetch first
	err = repo.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	err = worktree.PullContext(ctx, &gogit.PullOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		// Pull can fail in detached HEAD state - that's ok since fetch succeeded
		g.logger.Debug("pull failed (possibly detached HEAD)", slog.String("error", err.Error()))
	}

	return nil
}

// AllBranches returns all branches in repository.
func (g *GoGit) AllBranches(ctx context.Context, localPath string) ([]gitpkg.BranchInfo, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	var branches []gitpkg.BranchInfo

	// Get HEAD reference for detecting default branch
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	// Get local branches
	branchIter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("get branches: %w", err)
	}
	defer branchIter.Close()

	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, gitpkg.BranchInfo{
			Name:      ref.Name().Short(),
			HeadSHA:   ref.Hash().String(),
			IsDefault: ref.Hash() == headRef.Hash(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate branches: %w", err)
	}

	// Get remote branches
	remoteRefs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("get references: %w", err)
	}
	defer remoteRefs.Close()

	seenBranches := make(map[string]bool)
	for _, b := range branches {
		seenBranches[b.Name] = true
	}

	err = remoteRefs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() {
			name := ref.Name().Short()
			// Skip origin/HEAD
			if name == "origin/HEAD" {
				return nil
			}
			// Remove origin/ prefix
			if len(name) > 7 && name[:7] == "origin/" {
				name = name[7:]
			}
			if !seenBranches[name] {
				branches = append(branches, gitpkg.BranchInfo{
					Name:      name,
					HeadSHA:   ref.Hash().String(),
					IsDefault: false,
				})
				seenBranches[name] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate remote refs: %w", err)
	}

	return branches, nil
}

// BranchCommits returns commit history for a specific branch.
func (g *GoGit) BranchCommits(ctx context.Context, localPath string, branchName string) ([]gitpkg.CommitInfo, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	// Find branch reference
	branchRef, err := g.findBranchRef(repo, branchName)
	if err != nil {
		return nil, err
	}

	// Get commit iterator
	commitIter, err := repo.Log(&gogit.LogOptions{
		From: branchRef.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}
	defer commitIter.Close()

	var commits []gitpkg.CommitInfo
	err = commitIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, commitToInfo(c))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate commits: %w", err)
	}

	return commits, nil
}

// AllCommitsBulk returns all commits from all branches in bulk.
func (g *GoGit) AllCommitsBulk(ctx context.Context, localPath string, since *time.Time) (map[string]gitpkg.CommitInfo, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	logOpts := &gogit.LogOptions{
		All: true,
	}
	if since != nil {
		logOpts.Since = since
	}

	commitIter, err := repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}
	defer commitIter.Close()

	commits := make(map[string]gitpkg.CommitInfo)
	err = commitIter.ForEach(func(c *object.Commit) error {
		commits[c.Hash.String()] = commitToInfo(c)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate commits: %w", err)
	}

	return commits, nil
}

// BranchCommitSHAs returns only commit SHAs for a branch.
func (g *GoGit) BranchCommitSHAs(ctx context.Context, localPath string, branchName string) ([]string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	branchRef, err := g.findBranchRef(repo, branchName)
	if err != nil {
		return nil, err
	}

	commitIter, err := repo.Log(&gogit.LogOptions{
		From: branchRef.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}
	defer commitIter.Close()

	var shas []string
	err = commitIter.ForEach(func(c *object.Commit) error {
		shas = append(shas, c.Hash.String())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate commits: %w", err)
	}

	return shas, nil
}

// AllBranchHeadSHAs returns head commit SHAs for all branches in one operation.
func (g *GoGit) AllBranchHeadSHAs(ctx context.Context, localPath string, branchNames []string) (map[string]string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	result := make(map[string]string)
	for _, name := range branchNames {
		ref, err := g.findBranchRef(repo, name)
		if err != nil {
			g.logger.Debug("branch not found", slog.String("branch", name))
			continue
		}
		result[name] = ref.Hash().String()
	}

	return result, nil
}

// CommitFiles returns all files in a specific commit from the git tree.
func (g *GoGit) CommitFiles(ctx context.Context, localPath string, commitSHA string) ([]gitpkg.FileInfo, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	var files []gitpkg.FileInfo
	err = tree.Files().ForEach(func(f *object.File) error {
		files = append(files, gitpkg.FileInfo{
			Path:     f.Name,
			BlobSHA:  f.Hash.String(),
			Size:     f.Size,
			MimeType: guessMimeType(f.Name),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate files: %w", err)
	}

	return files, nil
}

// RepositoryExists checks if repository exists at local path.
func (g *GoGit) RepositoryExists(ctx context.Context, localPath string) (bool, error) {
	_, err := gogit.PlainOpen(localPath)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return false, nil
		}
		return false, fmt.Errorf("check repository: %w", err)
	}
	return true, nil
}

// CommitDetails returns detailed information about a specific commit.
func (g *GoGit) CommitDetails(ctx context.Context, localPath string, commitSHA string) (gitpkg.CommitInfo, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return gitpkg.CommitInfo{}, fmt.Errorf("open repository: %w", err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return gitpkg.CommitInfo{}, fmt.Errorf("get commit: %w", err)
	}

	return commitToInfo(commit), nil
}

// EnsureRepository clones if doesn't exist, otherwise fetches latest changes.
func (g *GoGit) EnsureRepository(ctx context.Context, remoteURI string, localPath string) error {
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
func (g *GoGit) FileContent(ctx context.Context, localPath string, commitSHA string, filePath string) ([]byte, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	file, err := commit.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return []byte(content), nil
}

// DefaultBranch returns the default branch name with fallback strategies.
func (g *GoGit) DefaultBranch(ctx context.Context, localPath string) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}

	// Strategy 1: Try to get origin/HEAD reference
	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
	if err == nil && ref.Type() == plumbing.SymbolicReference {
		target := ref.Target().Short()
		if len(target) > 7 && target[:7] == "origin/" {
			return target[7:], nil
		}
		return target, nil
	}

	// Strategy 2: Look for common default branch names
	for _, candidate := range []string{"main", "master"} {
		_, err := g.findBranchRef(repo, candidate)
		if err == nil {
			return candidate, nil
		}
	}

	// Strategy 3: Use first available branch
	branchIter, err := repo.Branches()
	if err != nil {
		return "", fmt.Errorf("get branches: %w", err)
	}
	defer branchIter.Close()

	ref, err = branchIter.Next()
	if err != nil {
		return "", fmt.Errorf("no branches found")
	}

	return ref.Name().Short(), nil
}

// LatestCommitSHA returns the latest commit SHA for a branch.
func (g *GoGit) LatestCommitSHA(ctx context.Context, localPath string, branchName string) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}

	if branchName == "" || branchName == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return "", fmt.Errorf("get HEAD: %w", err)
		}
		return head.Hash().String(), nil
	}

	ref, err := g.findBranchRef(repo, branchName)
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

// AllTags returns all tags in repository.
func (g *GoGit) AllTags(ctx context.Context, localPath string) ([]gitpkg.TagInfo, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	tagIter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	defer tagIter.Close()

	var tags []gitpkg.TagInfo
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		tag := gitpkg.TagInfo{
			Name: ref.Name().Short(),
		}

		// Try to get annotated tag object
		tagObj, err := repo.TagObject(ref.Hash())
		if err == nil {
			// Annotated tag
			tag.TargetCommitSHA = tagObj.Target.String()
			tag.Message = tagObj.Message
			tag.TaggerName = tagObj.Tagger.Name
			tag.TaggerEmail = tagObj.Tagger.Email
			tag.TaggedAt = tagObj.Tagger.When
		} else {
			// Lightweight tag - points directly to commit
			tag.TargetCommitSHA = ref.Hash().String()
		}

		tags = append(tags, tag)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

// CommitDiff returns the diff for a specific commit.
func (g *GoGit) CommitDiff(ctx context.Context, localPath string, commitSHA string) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("open repository: %w", err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return "", fmt.Errorf("get commit: %w", err)
	}

	parentTree := &object.Tree{}
	if len(commit.ParentHashes) > 0 {
		parent, err := repo.CommitObject(commit.ParentHashes[0])
		if err != nil {
			return "", fmt.Errorf("get parent commit: %w", err)
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return "", fmt.Errorf("get parent tree: %w", err)
		}
	}

	commitTree, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("get commit tree: %w", err)
	}

	changes, err := parentTree.Diff(commitTree)
	if err != nil {
		return "", fmt.Errorf("compute diff: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("get patch: %w", err)
	}

	return patch.String(), nil
}

func (g *GoGit) findBranchRef(repo *gogit.Repository, branchName string) (*plumbing.Reference, error) {
	// Try local branch
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	if err == nil {
		return ref, nil
	}

	// Try remote branch
	ref, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", branchName), true)
	if err == nil {
		return ref, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrBranchNotFound, branchName)
}

func commitToInfo(c *object.Commit) gitpkg.CommitInfo {
	info := gitpkg.CommitInfo{
		SHA:            c.Hash.String(),
		Message:        c.Message,
		AuthorName:     c.Author.Name,
		AuthorEmail:    c.Author.Email,
		CommitterName:  c.Committer.Name,
		CommitterEmail: c.Committer.Email,
		AuthoredAt:     c.Author.When,
		CommittedAt:    c.Committer.When,
	}

	if len(c.ParentHashes) > 0 {
		info.ParentSHA = c.ParentHashes[0].String()
	}

	return info
}

func guessMimeType(_ string) string {
	// Simple mime type detection based on extension
	// A full implementation would use the mime package
	return "application/octet-stream"
}
