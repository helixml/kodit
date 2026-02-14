package repository

// RepositorySummary provides a summary view of a repository.
type RepositorySummary struct {
	source        Source
	branchCount   int
	tagCount      int
	commitCount   int
	defaultBranch string
}

// NewRepositorySummary creates a new RepositorySummary.
func NewRepositorySummary(
	source Source,
	branchCount, tagCount, commitCount int,
	defaultBranch string,
) RepositorySummary {
	return RepositorySummary{
		source:        source,
		branchCount:   branchCount,
		tagCount:      tagCount,
		commitCount:   commitCount,
		defaultBranch: defaultBranch,
	}
}

// Source returns the repository source.
func (s RepositorySummary) Source() Source { return s.source }

// BranchCount returns the number of branches.
func (s RepositorySummary) BranchCount() int { return s.branchCount }

// TagCount returns the number of tags.
func (s RepositorySummary) TagCount() int { return s.tagCount }

// CommitCount returns the number of indexed commits.
func (s RepositorySummary) CommitCount() int { return s.commitCount }

// DefaultBranch returns the default branch name.
func (s RepositorySummary) DefaultBranch() string { return s.defaultBranch }
