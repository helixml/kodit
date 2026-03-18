package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/service"
)

// RepositoryScanner extracts data from Git repositories without mutation.
// Implements domain/service.Scanner interface.
type RepositoryScanner struct {
	adapter Adapter
	logger  zerolog.Logger
}

// NewRepositoryScanner creates a new RepositoryScanner with the specified adapter.
func NewRepositoryScanner(adapter Adapter, logger zerolog.Logger) *RepositoryScanner {
	return &RepositoryScanner{
		adapter: adapter,
		logger:  logger,
	}
}

// ScanCommit scans a specific commit and returns commit with its files.
// For non-git local directories the commit SHA is a directory content hash;
// files are enumerated by walking the filesystem instead of asking git.
func (s *RepositoryScanner) ScanCommit(ctx context.Context, clonedPath string, commitSHA string, repoID int64) (service.ScanCommitResult, error) {
	s.logger.Info().Str("sha", shortSHA(commitSHA)).Str("path", clonedPath).Msg("scanning commit")

	if !isGitRepo(clonedPath) {
		now := time.Now()
		author := repository.NewAuthor("kodit", "kodit@local")
		commit := repository.NewCommit(commitSHA, repoID, "Directory snapshot", author, author, now, now)
		files, err := s.filesFromDir(clonedPath, commitSHA)
		if err != nil {
			return service.ScanCommitResult{}, fmt.Errorf("list directory files: %w", err)
		}
		s.logger.Info().Str("sha", shortSHA(commitSHA)).Int("files", len(files)).Msg("scanned local directory")
		return service.NewScanCommitResult(commit, files), nil
	}

	commitInfo, err := s.adapter.CommitDetails(ctx, clonedPath, commitSHA)
	if err != nil {
		return service.ScanCommitResult{}, fmt.Errorf("get commit details: %w", err)
	}

	commit := s.commitFromInfo(commitInfo, repoID)

	filesInfo, err := s.adapter.CommitFiles(ctx, clonedPath, commitSHA)
	if err != nil {
		return service.ScanCommitResult{}, fmt.Errorf("get commit files: %w", err)
	}

	files := s.filesFromInfo(filesInfo, commitSHA)

	s.logger.Info().Str("sha", shortSHA(commitSHA)).Int("files", len(files)).Msg("scanned commit")

	return service.NewScanCommitResult(commit, files), nil
}

// ScanBranch scans all commits on a branch.
func (s *RepositoryScanner) ScanBranch(ctx context.Context, clonedPath string, branchName string, repoID int64) ([]repository.Commit, error) {
	s.logger.Info().Str("branch", branchName).Str("path", clonedPath).Msg("scanning branch")

	commitInfos, err := s.adapter.BranchCommits(ctx, clonedPath, branchName)
	if err != nil {
		return nil, fmt.Errorf("get branch commits: %w", err)
	}

	commits := make([]repository.Commit, 0, len(commitInfos))
	for _, info := range commitInfos {
		commits = append(commits, s.commitFromInfo(info, repoID))
	}

	s.logger.Info().Str("branch", branchName).Int("commits", len(commits)).Msg("scanned branch")

	return commits, nil
}

// ScanAllBranches scans metadata for all branches.
// For non-git local directories a single synthetic branch is returned whose
// HEAD commit SHA is a hash of the directory's current contents.
func (s *RepositoryScanner) ScanAllBranches(ctx context.Context, clonedPath string, repoID int64) ([]repository.Branch, error) {
	s.logger.Info().Str("path", clonedPath).Msg("scanning all branches")

	if !isGitRepo(clonedPath) {
		hash, err := dirHash(clonedPath)
		if err != nil {
			return nil, fmt.Errorf("compute directory hash: %w", err)
		}
		branch := repository.NewBranch(repoID, "main", hash, true)
		s.logger.Info().Str("hash", shortSHA(hash)).Msg("non-git directory: created synthetic branch")
		return []repository.Branch{branch}, nil
	}

	branchInfos, err := s.adapter.AllBranches(ctx, clonedPath)
	if err != nil {
		return nil, fmt.Errorf("get all branches: %w", err)
	}

	branches := make([]repository.Branch, 0, len(branchInfos))
	for _, info := range branchInfos {
		branches = append(branches, s.branchFromInfo(info, repoID))
	}

	s.logger.Info().Int("branches", len(branches)).Msg("scanned all branches")

	return branches, nil
}

// ScanAllTags scans metadata for all tags.
func (s *RepositoryScanner) ScanAllTags(ctx context.Context, clonedPath string, repoID int64) ([]repository.Tag, error) {
	s.logger.Info().Str("path", clonedPath).Msg("scanning all tags")

	tagInfos, err := s.adapter.AllTags(ctx, clonedPath)
	if err != nil {
		return nil, fmt.Errorf("get all tags: %w", err)
	}

	tags := make([]repository.Tag, 0, len(tagInfos))
	for _, info := range tagInfos {
		tags = append(tags, s.tagFromInfo(info, repoID))
	}

	s.logger.Info().Int("tags", len(tags)).Msg("scanned all tags")

	return tags, nil
}

// FilesForCommitsBatch processes files for a batch of commits.
// Reuses adapter resources efficiently for large batches.
func (s *RepositoryScanner) FilesForCommitsBatch(ctx context.Context, clonedPath string, commitSHAs []string) ([]repository.File, error) {
	s.logger.Info().Str("path", clonedPath).Int("commits", len(commitSHAs)).Msg("processing files for commit batch")

	var files []repository.File
	for _, sha := range commitSHAs {
		filesInfo, err := s.adapter.CommitFiles(ctx, clonedPath, sha)
		if err != nil {
			return nil, fmt.Errorf("get commit files for %s: %w", shortSHA(sha), err)
		}
		files = append(files, s.filesFromInfo(filesInfo, sha)...)
	}

	s.logger.Info().Int("commits", len(commitSHAs)).Int("files", len(files)).Msg("processed files for commit batch")

	return files, nil
}

func (s *RepositoryScanner) commitFromInfo(info CommitInfo, repoID int64) repository.Commit {
	author := repository.NewAuthor(info.AuthorName, info.AuthorEmail)
	committer := repository.NewAuthor(info.CommitterName, info.CommitterEmail)

	return repository.NewCommit(
		info.SHA,
		repoID,
		info.Message,
		author,
		committer,
		info.AuthoredAt,
		info.CommittedAt,
	)
}

func (s *RepositoryScanner) branchFromInfo(info BranchInfo, repoID int64) repository.Branch {
	return repository.NewBranch(repoID, info.Name, info.HeadSHA, info.IsDefault)
}

func (s *RepositoryScanner) tagFromInfo(info TagInfo, repoID int64) repository.Tag {
	if info.Message != "" || info.TaggerName != "" {
		tagger := repository.NewAuthor(info.TaggerName, info.TaggerEmail)
		return repository.NewAnnotatedTag(repoID, info.Name, info.TargetCommitSHA, info.Message, tagger, info.TaggedAt)
	}
	return repository.NewTag(repoID, info.Name, info.TargetCommitSHA)
}

func (s *RepositoryScanner) filesFromInfo(infos []FileInfo, commitSHA string) []repository.File {
	now := time.Now()
	files := make([]repository.File, 0, len(infos))

	for _, info := range infos {
		language := languageFromPath(info.Path)
		extension := extensionFromPath(info.Path)
		mimeType := mimeTypeFromExtension(extension)

		file := repository.ReconstructFile(
			0, // ID assigned on save
			commitSHA,
			info.Path,
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

// dirHash computes a stable SHA-256 hash over the contents of a directory.
// Files are processed in sorted order so the hash is deterministic.
// Returns the first 40 hex characters (matching the length of a git SHA1).
func dirHash(path string) (string, error) {
	type entry struct {
		rel     string
		content []byte
	}

	var entries []entry
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		entries = append(entries, entry{rel: rel, content: content})
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })

	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.rel))
		h.Write(e.content)
	}
	full := hex.EncodeToString(h.Sum(nil))
	// Trim to 40 chars so it looks like a git SHA-1.
	return full[:40], nil
}

// filesFromDir walks a local directory and returns repository.File entries
// suitable for storing alongside a synthetic directory-hash commit.
func (s *RepositoryScanner) filesFromDir(dirPath, commitSHA string) ([]repository.File, error) {
	now := time.Now()
	var files []repository.File

	err := filepath.WalkDir(dirPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		blobSum := sha256.Sum256(content)
		blobSHA := hex.EncodeToString(blobSum[:])
		ext := extensionFromPath(rel)
		lang := languageFromPath(rel)
		mime := mimeTypeFromExtension(ext)

		files = append(files, repository.ReconstructFile(0, commitSHA, rel, blobSHA, mime, ext, lang, info.Size(), now))
		return nil
	})
	return files, err
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

// Ensure RepositoryScanner implements Scanner.
var _ service.Scanner = (*RepositoryScanner)(nil)
