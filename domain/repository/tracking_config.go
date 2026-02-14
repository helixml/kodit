package repository

// TrackingConfig represents the branch/tag/commit to monitor and keep indexed.
type TrackingConfig struct {
	branch string
	tag    string
	commit string
}

// NewTrackingConfig creates a new TrackingConfig.
func NewTrackingConfig(branch, tag, commit string) TrackingConfig {
	return TrackingConfig{
		branch: branch,
		tag:    tag,
		commit: commit,
	}
}

// NewTrackingConfigForBranch creates a TrackingConfig tracking a branch.
func NewTrackingConfigForBranch(branch string) TrackingConfig {
	return TrackingConfig{branch: branch}
}

// NewTrackingConfigForTag creates a TrackingConfig tracking a tag.
func NewTrackingConfigForTag(tag string) TrackingConfig {
	return TrackingConfig{tag: tag}
}

// NewTrackingConfigForCommit creates a TrackingConfig tracking a commit.
func NewTrackingConfigForCommit(commit string) TrackingConfig {
	return TrackingConfig{commit: commit}
}

// Branch returns the tracked branch name.
func (t TrackingConfig) Branch() string { return t.branch }

// Tag returns the tracked tag name.
func (t TrackingConfig) Tag() string { return t.tag }

// Commit returns the tracked commit SHA.
func (t TrackingConfig) Commit() string { return t.commit }

// IsBranch returns true if tracking a branch.
func (t TrackingConfig) IsBranch() bool { return t.branch != "" }

// IsTag returns true if tracking a tag.
func (t TrackingConfig) IsTag() bool { return t.tag != "" }

// IsCommit returns true if tracking a specific commit.
func (t TrackingConfig) IsCommit() bool { return t.commit != "" }

// IsEmpty returns true if no tracking is configured.
func (t TrackingConfig) IsEmpty() bool {
	return t.branch == "" && t.tag == "" && t.commit == ""
}

// Reference returns the tracking reference (branch, tag, or commit).
func (t TrackingConfig) Reference() string {
	if t.branch != "" {
		return t.branch
	}
	if t.tag != "" {
		return t.tag
	}
	return t.commit
}

// Equal returns true if two TrackingConfig values are equal.
func (t TrackingConfig) Equal(other TrackingConfig) bool {
	return t.branch == other.branch &&
		t.tag == other.tag &&
		t.commit == other.commit
}
