package repository

import "time"

// WithSHA filters by the "commit_sha" column.
func WithSHA(sha string) Option {
	return WithCondition("commit_sha", sha)
}

// WithName filters by the "name" column.
func WithName(name string) Option {
	return WithCondition("name", name)
}

// WithRemoteURL filters by the "sanitized_remote_uri" column.
func WithRemoteURL(url string) Option {
	return WithCondition("sanitized_remote_uri", url)
}

// WithDefault filters for the default branch (is_default = true).
func WithDefault() Option {
	return WithCondition("is_default", true)
}

// WithCommitSHA filters by the "commit_sha" column.
func WithCommitSHA(sha string) Option {
	return WithCondition("commit_sha", sha)
}

// WithCommitSHAIn filters by the "commit_sha" column using IN.
func WithCommitSHAIn(shas []string) Option {
	return WithConditionIn("commit_sha", shas)
}

// WithBlobSHA filters by the "blob_sha" column.
func WithBlobSHA(sha string) Option {
	return WithCondition("blob_sha", sha)
}

// WithPath filters by the "path" column.
func WithPath(path string) Option {
	return WithCondition("path", path)
}

// WithScanDueBefore filters repositories whose last scan was before the given time (or never scanned).
func WithScanDueBefore(t time.Time) Option {
	return WithWhere("last_scanned_at IS NULL OR last_scanned_at < ?", t)
}
