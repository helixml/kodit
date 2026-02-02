package git

import (
	"testing"
	"time"
)

func TestWorkingCopy(t *testing.T) {
	wc := NewWorkingCopy("/path/to/clone", "https://github.com/user/repo")

	if wc.Path() != "/path/to/clone" {
		t.Errorf("Path() = %v, want /path/to/clone", wc.Path())
	}
	if wc.URI() != "https://github.com/user/repo" {
		t.Errorf("URI() = %v, want https://github.com/user/repo", wc.URI())
	}
	if wc.IsEmpty() {
		t.Error("IsEmpty() should be false")
	}

	empty := NewWorkingCopy("", "")
	if !empty.IsEmpty() {
		t.Error("IsEmpty() should be true for empty path")
	}

	other := NewWorkingCopy("/path/to/clone", "https://github.com/user/repo")
	if !wc.Equal(other) {
		t.Error("Equal() should return true for identical values")
	}
}

func TestTrackingConfig(t *testing.T) {
	branch := NewTrackingConfigForBranch("main")
	if !branch.IsBranch() {
		t.Error("IsBranch() should be true")
	}
	if branch.Reference() != "main" {
		t.Errorf("Reference() = %v, want main", branch.Reference())
	}

	tag := NewTrackingConfigForTag("v1.0.0")
	if !tag.IsTag() {
		t.Error("IsTag() should be true")
	}
	if tag.Reference() != "v1.0.0" {
		t.Errorf("Reference() = %v, want v1.0.0", tag.Reference())
	}

	commit := NewTrackingConfigForCommit("abc123")
	if !commit.IsCommit() {
		t.Error("IsCommit() should be true")
	}
	if commit.Reference() != "abc123" {
		t.Errorf("Reference() = %v, want abc123", commit.Reference())
	}

	empty := NewTrackingConfig("", "", "")
	if !empty.IsEmpty() {
		t.Error("IsEmpty() should be true")
	}
}

func TestAuthor(t *testing.T) {
	author := NewAuthor("John Doe", "john@example.com")

	if author.Name() != "John Doe" {
		t.Errorf("Name() = %v, want John Doe", author.Name())
	}
	if author.Email() != "john@example.com" {
		t.Errorf("Email() = %v, want john@example.com", author.Email())
	}
	if author.String() != "John Doe <john@example.com>" {
		t.Errorf("String() = %v, want John Doe <john@example.com>", author.String())
	}
	if author.IsEmpty() {
		t.Error("IsEmpty() should be false")
	}

	noEmail := NewAuthor("Jane", "")
	if noEmail.String() != "Jane" {
		t.Errorf("String() = %v, want Jane", noEmail.String())
	}

	empty := NewAuthor("", "")
	if !empty.IsEmpty() {
		t.Error("IsEmpty() should be true")
	}
}

func TestRepo(t *testing.T) {
	repo, err := NewRepo("https://github.com/user/repo")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}

	if repo.RemoteURL() != "https://github.com/user/repo" {
		t.Errorf("RemoteURL() = %v, want https://github.com/user/repo", repo.RemoteURL())
	}
	if repo.HasWorkingCopy() {
		t.Error("HasWorkingCopy() should be false initially")
	}
	if repo.HasTrackingConfig() {
		t.Error("HasTrackingConfig() should be false initially")
	}

	// Test empty URL error
	_, err = NewRepo("")
	if err != ErrEmptyRemoteURL {
		t.Errorf("NewRepo(\"\") error = %v, want ErrEmptyRemoteURL", err)
	}

	// Test with working copy
	wc := NewWorkingCopy("/path/to/clone", "https://github.com/user/repo")
	withWC := repo.WithWorkingCopy(wc)
	if !withWC.HasWorkingCopy() {
		t.Error("HasWorkingCopy() should be true after WithWorkingCopy")
	}

	// Test with tracking config
	tc := NewTrackingConfigForBranch("main")
	withTC := repo.WithTrackingConfig(tc)
	if !withTC.HasTrackingConfig() {
		t.Error("HasTrackingConfig() should be true after WithTrackingConfig")
	}
}

func TestCommit(t *testing.T) {
	author := NewAuthor("John Doe", "john@example.com")
	committer := NewAuthor("Jane Doe", "jane@example.com")
	now := time.Now()

	commit := NewCommit("abc123def456789", 1, "Initial commit\n\nDetailed description", author, committer, now, now)

	if commit.SHA() != "abc123def456789" {
		t.Errorf("SHA() = %v, want abc123def456789", commit.SHA())
	}
	if commit.ShortSHA() != "abc123d" {
		t.Errorf("ShortSHA() = %v, want abc123d", commit.ShortSHA())
	}
	if commit.RepoID() != 1 {
		t.Errorf("RepoID() = %v, want 1", commit.RepoID())
	}
	if commit.ShortMessage() != "Initial commit" {
		t.Errorf("ShortMessage() = %v, want Initial commit", commit.ShortMessage())
	}
	if !commit.Author().Equal(author) {
		t.Error("Author() should match")
	}
	if !commit.Committer().Equal(committer) {
		t.Error("Committer() should match")
	}

	// Test short SHA for short hashes
	shortCommit := NewCommit("abc", 1, "test", author, committer, now, now)
	if shortCommit.ShortSHA() != "abc" {
		t.Errorf("ShortSHA() = %v, want abc", shortCommit.ShortSHA())
	}

	// Test short message for single line
	singleLine := NewCommit("def", 1, "No newline here", author, committer, now, now)
	if singleLine.ShortMessage() != "No newline here" {
		t.Errorf("ShortMessage() = %v, want No newline here", singleLine.ShortMessage())
	}
}

func TestBranch(t *testing.T) {
	branch := NewBranch(1, "main", "abc123", true)

	if branch.RepoID() != 1 {
		t.Errorf("RepoID() = %v, want 1", branch.RepoID())
	}
	if branch.Name() != "main" {
		t.Errorf("Name() = %v, want main", branch.Name())
	}
	if branch.HeadCommitSHA() != "abc123" {
		t.Errorf("HeadCommitSHA() = %v, want abc123", branch.HeadCommitSHA())
	}
	if !branch.IsDefault() {
		t.Error("IsDefault() should be true")
	}

	updated := branch.WithHeadCommitSHA("def456")
	if updated.HeadCommitSHA() != "def456" {
		t.Errorf("HeadCommitSHA() = %v, want def456", updated.HeadCommitSHA())
	}
}

func TestTag(t *testing.T) {
	tag := NewTag(1, "v1.0.0", "abc123")

	if tag.RepoID() != 1 {
		t.Errorf("RepoID() = %v, want 1", tag.RepoID())
	}
	if tag.Name() != "v1.0.0" {
		t.Errorf("Name() = %v, want v1.0.0", tag.Name())
	}
	if tag.CommitSHA() != "abc123" {
		t.Errorf("CommitSHA() = %v, want abc123", tag.CommitSHA())
	}
	if tag.IsAnnotated() {
		t.Error("IsAnnotated() should be false for lightweight tag")
	}

	tagger := NewAuthor("John Doe", "john@example.com")
	annotated := NewAnnotatedTag(1, "v2.0.0", "def456", "Release notes", tagger, time.Now())
	if !annotated.IsAnnotated() {
		t.Error("IsAnnotated() should be true for annotated tag")
	}
	if annotated.Message() != "Release notes" {
		t.Errorf("Message() = %v, want Release notes", annotated.Message())
	}
}

func TestFile(t *testing.T) {
	file := NewFile("abc123", "src/main.go", "go", 1024)

	if file.CommitSHA() != "abc123" {
		t.Errorf("CommitSHA() = %v, want abc123", file.CommitSHA())
	}
	if file.Path() != "src/main.go" {
		t.Errorf("Path() = %v, want src/main.go", file.Path())
	}
	if file.Language() != "go" {
		t.Errorf("Language() = %v, want go", file.Language())
	}
	if file.Size() != 1024 {
		t.Errorf("Size() = %v, want 1024", file.Size())
	}
}

func TestScanResult(t *testing.T) {
	branches := []Branch{NewBranch(1, "main", "abc123", true)}
	commits := []Commit{NewCommit("abc123", 1, "test", NewAuthor("", ""), NewAuthor("", ""), time.Now(), time.Now())}
	files := []File{NewFile("abc123", "main.go", "go", 100)}
	tags := []Tag{NewTag(1, "v1.0.0", "abc123")}

	result := NewScanResult(branches, commits, files, tags)

	if len(result.Branches()) != 1 {
		t.Errorf("len(Branches()) = %v, want 1", len(result.Branches()))
	}
	if len(result.Commits()) != 1 {
		t.Errorf("len(Commits()) = %v, want 1", len(result.Commits()))
	}
	if len(result.Files()) != 1 {
		t.Errorf("len(Files()) = %v, want 1", len(result.Files()))
	}
	if len(result.Tags()) != 1 {
		t.Errorf("len(Tags()) = %v, want 1", len(result.Tags()))
	}
	if result.IsEmpty() {
		t.Error("IsEmpty() should be false")
	}

	empty := NewScanResult(nil, nil, nil, nil)
	if !empty.IsEmpty() {
		t.Error("IsEmpty() should be true for empty result")
	}
}
