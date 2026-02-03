package git

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"
)

// Scanner extracts data from Git repositories without mutation.
type Scanner struct {
	adapter Adapter
	logger  *slog.Logger
}

// NewScanner creates a new Scanner with the specified adapter.
func NewScanner(adapter Adapter, logger *slog.Logger) Scanner {
	return Scanner{
		adapter: adapter,
		logger:  logger,
	}
}

// ScanCommitResult holds the result of scanning a single commit.
type ScanCommitResult struct {
	commit Commit
	files  []File
}

// Commit returns the scanned commit.
func (r ScanCommitResult) Commit() Commit { return r.commit }

// Files returns the scanned files.
func (r ScanCommitResult) Files() []File { return r.files }

// ScanCommit scans a specific commit and returns commit with its files.
func (s Scanner) ScanCommit(ctx context.Context, clonedPath string, commitSHA string, repoID int64) (ScanCommitResult, error) {
	s.logger.Info("scanning commit",
		slog.String("sha", shortSHA(commitSHA)),
		slog.String("path", clonedPath),
	)

	commitInfo, err := s.adapter.CommitDetails(ctx, clonedPath, commitSHA)
	if err != nil {
		return ScanCommitResult{}, fmt.Errorf("get commit details: %w", err)
	}

	commit := s.commitFromInfo(commitInfo, repoID)

	filesInfo, err := s.adapter.CommitFiles(ctx, clonedPath, commitSHA)
	if err != nil {
		return ScanCommitResult{}, fmt.Errorf("get commit files: %w", err)
	}

	files := s.filesFromInfo(clonedPath, filesInfo, commitSHA)

	s.logger.Info("scanned commit",
		slog.String("sha", shortSHA(commitSHA)),
		slog.Int("files", len(files)),
	)

	return ScanCommitResult{commit: commit, files: files}, nil
}

// ScanBranch scans all commits on a branch.
func (s Scanner) ScanBranch(ctx context.Context, clonedPath string, branchName string, repoID int64) ([]Commit, error) {
	s.logger.Info("scanning branch",
		slog.String("branch", branchName),
		slog.String("path", clonedPath),
	)

	commitInfos, err := s.adapter.BranchCommits(ctx, clonedPath, branchName)
	if err != nil {
		return nil, fmt.Errorf("get branch commits: %w", err)
	}

	commits := make([]Commit, 0, len(commitInfos))
	for _, info := range commitInfos {
		commits = append(commits, s.commitFromInfo(info, repoID))
	}

	s.logger.Info("scanned branch",
		slog.String("branch", branchName),
		slog.Int("commits", len(commits)),
	)

	return commits, nil
}

// ScanAllBranches scans metadata for all branches.
func (s Scanner) ScanAllBranches(ctx context.Context, clonedPath string, repoID int64) ([]Branch, error) {
	s.logger.Info("scanning all branches",
		slog.String("path", clonedPath),
	)

	branchInfos, err := s.adapter.AllBranches(ctx, clonedPath)
	if err != nil {
		return nil, fmt.Errorf("get all branches: %w", err)
	}

	branches := make([]Branch, 0, len(branchInfos))
	for _, info := range branchInfos {
		branches = append(branches, s.branchFromInfo(info, repoID))
	}

	s.logger.Info("scanned all branches",
		slog.Int("branches", len(branches)),
	)

	return branches, nil
}

// ScanAllTags scans metadata for all tags.
func (s Scanner) ScanAllTags(ctx context.Context, clonedPath string, repoID int64) ([]Tag, error) {
	s.logger.Info("scanning all tags",
		slog.String("path", clonedPath),
	)

	tagInfos, err := s.adapter.AllTags(ctx, clonedPath)
	if err != nil {
		return nil, fmt.Errorf("get all tags: %w", err)
	}

	tags := make([]Tag, 0, len(tagInfos))
	for _, info := range tagInfos {
		tags = append(tags, s.tagFromInfo(info, repoID))
	}

	s.logger.Info("scanned all tags",
		slog.Int("tags", len(tags)),
	)

	return tags, nil
}

// FilesForCommitsBatch processes files for a batch of commits.
// Reuses adapter resources efficiently for large batches.
func (s Scanner) FilesForCommitsBatch(ctx context.Context, clonedPath string, commitSHAs []string) ([]File, error) {
	s.logger.Info("processing files for commit batch",
		slog.String("path", clonedPath),
		slog.Int("commits", len(commitSHAs)),
	)

	var files []File
	for _, sha := range commitSHAs {
		filesInfo, err := s.adapter.CommitFiles(ctx, clonedPath, sha)
		if err != nil {
			return nil, fmt.Errorf("get commit files for %s: %w", shortSHA(sha), err)
		}
		files = append(files, s.filesFromInfo(clonedPath, filesInfo, sha)...)
	}

	s.logger.Info("processed files for commit batch",
		slog.Int("commits", len(commitSHAs)),
		slog.Int("files", len(files)),
	)

	return files, nil
}

func (s Scanner) commitFromInfo(info CommitInfo, repoID int64) Commit {
	author := NewAuthor(info.AuthorName, info.AuthorEmail)
	committer := NewAuthor(info.CommitterName, info.CommitterEmail)

	return NewCommit(
		info.SHA,
		repoID,
		info.Message,
		author,
		committer,
		info.AuthoredAt,
		info.CommittedAt,
	)
}

func (s Scanner) branchFromInfo(info BranchInfo, repoID int64) Branch {
	return NewBranch(repoID, info.Name, info.HeadSHA, info.IsDefault)
}

func (s Scanner) tagFromInfo(info TagInfo, repoID int64) Tag {
	if info.Message != "" || info.TaggerName != "" {
		tagger := NewAuthor(info.TaggerName, info.TaggerEmail)
		return NewAnnotatedTag(repoID, info.Name, info.TargetCommitSHA, info.Message, tagger, info.TaggedAt)
	}
	return NewTag(repoID, info.Name, info.TargetCommitSHA)
}

func (s Scanner) filesFromInfo(clonedPath string, infos []FileInfo, commitSHA string) []File {
	now := time.Now()
	files := make([]File, 0, len(infos))

	for _, info := range infos {
		fullPath := filepath.Join(clonedPath, info.Path)
		language := languageFromPath(info.Path)
		extension := extensionFromPath(info.Path)
		mimeType := mimeTypeFromExtension(extension)

		file := ReconstructFile(
			0, // ID assigned on save
			commitSHA,
			fullPath,
			info.BlobSHA,
			mimeType,
			extension,
			language,
			info.Size,
			now,
		)
		files = append(files, file)
	}

	return files
}

func shortSHA(sha string) string {
	if len(sha) >= 8 {
		return sha[:8]
	}
	return sha
}

func languageFromPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	// Remove leading dot
	ext = ext[1:]

	// Common language mappings
	switch ext {
	case "go":
		return "go"
	case "py":
		return "python"
	case "js":
		return "javascript"
	case "ts":
		return "typescript"
	case "tsx":
		return "typescript"
	case "jsx":
		return "javascript"
	case "rb":
		return "ruby"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "c":
		return "c"
	case "cpp", "cc", "cxx":
		return "cpp"
	case "h", "hpp":
		return "c"
	case "cs":
		return "csharp"
	case "php":
		return "php"
	case "swift":
		return "swift"
	case "kt", "kts":
		return "kotlin"
	case "scala":
		return "scala"
	case "sh", "bash":
		return "shell"
	case "sql":
		return "sql"
	case "md", "markdown":
		return "markdown"
	case "json":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "xml":
		return "xml"
	case "html", "htm":
		return "html"
	case "css":
		return "css"
	case "scss", "sass":
		return "scss"
	case "vue":
		return "vue"
	case "svelte":
		return "svelte"
	default:
		return ext
	}
}

func extensionFromPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	// Remove leading dot
	return ext[1:]
}

func mimeTypeFromExtension(ext string) string {
	switch ext {
	case "go":
		return "text/x-go"
	case "py":
		return "text/x-python"
	case "js":
		return "text/javascript"
	case "ts", "tsx":
		return "text/typescript"
	case "jsx":
		return "text/javascript"
	case "java":
		return "text/x-java-source"
	case "c":
		return "text/x-c"
	case "cpp", "cc", "cxx":
		return "text/x-c++"
	case "h", "hpp":
		return "text/x-c"
	case "cs":
		return "text/x-csharp"
	case "rs":
		return "text/x-rust"
	case "rb":
		return "text/x-ruby"
	case "php":
		return "text/x-php"
	case "swift":
		return "text/x-swift"
	case "kt", "kts":
		return "text/x-kotlin"
	case "scala":
		return "text/x-scala"
	case "sh", "bash":
		return "text/x-shellscript"
	case "sql":
		return "text/x-sql"
	case "md", "markdown":
		return "text/markdown"
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "text/yaml"
	case "xml":
		return "application/xml"
	case "html", "htm":
		return "text/html"
	case "css":
		return "text/css"
	case "scss", "sass":
		return "text/scss"
	default:
		return "text/plain"
	}
}
