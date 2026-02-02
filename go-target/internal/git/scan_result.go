package git

// ScanResult contains the results of scanning a Git repository.
type ScanResult struct {
	branches []Branch
	commits  []Commit
	files    []File
	tags     []Tag
}

// NewScanResult creates a new ScanResult.
func NewScanResult(branches []Branch, commits []Commit, files []File, tags []Tag) ScanResult {
	return ScanResult{
		branches: copyBranches(branches),
		commits:  copyCommits(commits),
		files:    copyFiles(files),
		tags:     copyTags(tags),
	}
}

// Branches returns the discovered branches.
func (s ScanResult) Branches() []Branch {
	return copyBranches(s.branches)
}

// Commits returns the discovered commits.
func (s ScanResult) Commits() []Commit {
	return copyCommits(s.commits)
}

// Files returns the discovered files.
func (s ScanResult) Files() []File {
	return copyFiles(s.files)
}

// Tags returns the discovered tags.
func (s ScanResult) Tags() []Tag {
	return copyTags(s.tags)
}

// IsEmpty returns true if no results were found.
func (s ScanResult) IsEmpty() bool {
	return len(s.branches) == 0 &&
		len(s.commits) == 0 &&
		len(s.files) == 0 &&
		len(s.tags) == 0
}

func copyBranches(branches []Branch) []Branch {
	if branches == nil {
		return nil
	}
	result := make([]Branch, len(branches))
	copy(result, branches)
	return result
}

func copyCommits(commits []Commit) []Commit {
	if commits == nil {
		return nil
	}
	result := make([]Commit, len(commits))
	copy(result, commits)
	return result
}

func copyFiles(files []File) []File {
	if files == nil {
		return nil
	}
	result := make([]File, len(files))
	copy(result, files)
	return result
}

func copyTags(tags []Tag) []Tag {
	if tags == nil {
		return nil
	}
	result := make([]Tag, len(tags))
	copy(result, tags)
	return result
}
