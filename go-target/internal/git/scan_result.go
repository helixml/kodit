package git

import "github.com/helixml/kodit/domain/repository"

// ScanResult contains the results of scanning a Git repository.
type ScanResult struct {
	branches []repository.Branch
	commits  []repository.Commit
	files    []repository.File
	tags     []repository.Tag
}

// NewScanResult creates a new ScanResult.
func NewScanResult(branches []repository.Branch, commits []repository.Commit, files []repository.File, tags []repository.Tag) ScanResult {
	return ScanResult{
		branches: copyBranches(branches),
		commits:  copyCommits(commits),
		files:    copyFiles(files),
		tags:     copyTags(tags),
	}
}

// Branches returns the discovered branches.
func (s ScanResult) Branches() []repository.Branch {
	return copyBranches(s.branches)
}

// Commits returns the discovered commits.
func (s ScanResult) Commits() []repository.Commit {
	return copyCommits(s.commits)
}

// Files returns the discovered files.
func (s ScanResult) Files() []repository.File {
	return copyFiles(s.files)
}

// Tags returns the discovered tags.
func (s ScanResult) Tags() []repository.Tag {
	return copyTags(s.tags)
}

// IsEmpty returns true if no results were found.
func (s ScanResult) IsEmpty() bool {
	return len(s.branches) == 0 &&
		len(s.commits) == 0 &&
		len(s.files) == 0 &&
		len(s.tags) == 0
}

func copyBranches(branches []repository.Branch) []repository.Branch {
	if branches == nil {
		return nil
	}
	result := make([]repository.Branch, len(branches))
	copy(result, branches)
	return result
}

func copyCommits(commits []repository.Commit) []repository.Commit {
	if commits == nil {
		return nil
	}
	result := make([]repository.Commit, len(commits))
	copy(result, commits)
	return result
}

func copyFiles(files []repository.File) []repository.File {
	if files == nil {
		return nil
	}
	result := make([]repository.File, len(files))
	copy(result, files)
	return result
}

func copyTags(tags []repository.Tag) []repository.Tag {
	if tags == nil {
		return nil
	}
	result := make([]repository.Tag, len(tags))
	copy(result, tags)
	return result
}
