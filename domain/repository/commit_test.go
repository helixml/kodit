package repository

import (
	"testing"
	"time"
)

func TestCommit_ShortSHA(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		want string
	}{
		{"normal SHA", "abc1234567890", "abc1234"},
		{"exactly 7 chars", "abc1234", "abc1234"},
		{"shorter than 7", "abc", "abc"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCommit(tt.sha, 1, "msg", NewAuthor("n", "e"), NewAuthor("n", "e"), time.Now(), time.Now())
			if got := c.ShortSHA(); got != tt.want {
				t.Errorf("ShortSHA() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommit_ShortMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"single line", "fix bug", "fix bug"},
		{"multi-line", "fix bug\n\nDetailed description", "fix bug"},
		{"empty", "", ""},
		{"only newline", "\n", ""},
		{"trailing newline", "fix bug\n", "fix bug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCommit("abc1234", 1, tt.message, NewAuthor("n", "e"), NewAuthor("n", "e"), time.Now(), time.Now())
			if got := c.ShortMessage(); got != tt.want {
				t.Errorf("ShortMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommit_Fields(t *testing.T) {
	authored := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	committed := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	author := NewAuthor("Alice", "alice@example.com")
	committer := NewAuthor("Bob", "bob@example.com")

	c := NewCommit("abc1234", 42, "fix: null pointer", author, committer, authored, committed)

	if c.SHA() != "abc1234" {
		t.Errorf("SHA() = %q", c.SHA())
	}
	if c.RepoID() != 42 {
		t.Errorf("RepoID() = %d", c.RepoID())
	}
	if c.Message() != "fix: null pointer" {
		t.Errorf("Message() = %q", c.Message())
	}
	if c.Author().Name() != "Alice" {
		t.Errorf("Author().Name() = %q", c.Author().Name())
	}
	if c.Committer().Name() != "Bob" {
		t.Errorf("Committer().Name() = %q", c.Committer().Name())
	}
	if !c.AuthoredAt().Equal(authored) {
		t.Errorf("AuthoredAt() = %v", c.AuthoredAt())
	}
	if !c.CommittedAt().Equal(committed) {
		t.Errorf("CommittedAt() = %v", c.CommittedAt())
	}
	if c.ParentCommitSHA() != "" {
		t.Errorf("ParentCommitSHA() = %q, want empty", c.ParentCommitSHA())
	}
	if c.ID() != 0 {
		t.Errorf("ID() = %d, want 0 for new commit", c.ID())
	}
}

func TestNewCommitWithParent(t *testing.T) {
	c := NewCommitWithParent(
		"abc1234", 1, "msg",
		NewAuthor("n", "e"), NewAuthor("n", "e"),
		time.Now(), time.Now(),
		"parent123",
	)

	if c.ParentCommitSHA() != "parent123" {
		t.Errorf("ParentCommitSHA() = %q, want %q", c.ParentCommitSHA(), "parent123")
	}
}

func TestCommit_WithID(t *testing.T) {
	c := NewCommit("abc1234", 1, "msg", NewAuthor("n", "e"), NewAuthor("n", "e"), time.Now(), time.Now())
	c2 := c.WithID(99)

	if c2.ID() != 99 {
		t.Errorf("WithID result ID() = %d, want 99", c2.ID())
	}
	if c.ID() != 0 {
		t.Errorf("original ID() = %d, want 0 (value type should be unchanged)", c.ID())
	}
}

func TestReconstructCommit(t *testing.T) {
	now := time.Now()
	c := ReconstructCommit(
		42, "sha256hash", 7, "message",
		NewAuthor("Alice", "a@b.com"), NewAuthor("Bob", "b@b.com"),
		now, now, now.Add(-time.Hour),
		"parentsha",
	)

	if c.ID() != 42 {
		t.Errorf("ID() = %d, want 42", c.ID())
	}
	if c.ParentCommitSHA() != "parentsha" {
		t.Errorf("ParentCommitSHA() = %q, want %q", c.ParentCommitSHA(), "parentsha")
	}
}

func TestAuthor_String(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"Alice", "alice@example.com", "Alice <alice@example.com>"},
		{"Bob", "", "Bob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAuthor(tt.name, tt.email)
			if got := a.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthor_IsEmpty(t *testing.T) {
	empty := NewAuthor("", "")
	if !empty.IsEmpty() {
		t.Error("IsEmpty() should be true for empty name")
	}

	notEmpty := NewAuthor("Alice", "")
	if notEmpty.IsEmpty() {
		t.Error("IsEmpty() should be false when name is set")
	}
}

func TestAuthor_Equal(t *testing.T) {
	a1 := NewAuthor("Alice", "alice@example.com")
	a2 := NewAuthor("Alice", "alice@example.com")
	a3 := NewAuthor("Bob", "alice@example.com")
	a4 := NewAuthor("Alice", "bob@example.com")

	if !a1.Equal(a2) {
		t.Error("identical authors should be equal")
	}
	if a1.Equal(a3) {
		t.Error("different names should not be equal")
	}
	if a1.Equal(a4) {
		t.Error("different emails should not be equal")
	}
}

func TestTag_IsAnnotated(t *testing.T) {
	lightweight := NewTag(1, "v1.0.0", "abc123")
	if lightweight.IsAnnotated() {
		t.Error("lightweight tag should not be annotated")
	}

	withMessage := NewAnnotatedTag(1, "v1.0.0", "abc123", "release", NewAuthor("", ""), time.Now())
	if !withMessage.IsAnnotated() {
		t.Error("tag with message should be annotated")
	}

	withTagger := NewAnnotatedTag(1, "v1.0.0", "abc123", "", NewAuthor("Alice", "a@b.com"), time.Now())
	if !withTagger.IsAnnotated() {
		t.Error("tag with non-empty tagger should be annotated")
	}
}

func TestTag_Fields(t *testing.T) {
	tag := NewTag(7, "v2.0.0", "def456")

	if tag.RepoID() != 7 {
		t.Errorf("RepoID() = %d, want 7", tag.RepoID())
	}
	if tag.Name() != "v2.0.0" {
		t.Errorf("Name() = %q, want %q", tag.Name(), "v2.0.0")
	}
	if tag.CommitSHA() != "def456" {
		t.Errorf("CommitSHA() = %q, want %q", tag.CommitSHA(), "def456")
	}
}

func TestTag_WithID(t *testing.T) {
	tag := NewTag(1, "v1.0.0", "abc123")
	tag2 := tag.WithID(55)

	if tag2.ID() != 55 {
		t.Errorf("WithID result ID() = %d, want 55", tag2.ID())
	}
	if tag.ID() != 0 {
		t.Errorf("original ID() = %d, want 0", tag.ID())
	}
}

func TestTrackingConfig_IsBranch(t *testing.T) {
	tc := NewTrackingConfigForBranch("main")
	if !tc.IsBranch() {
		t.Error("expected IsBranch() = true")
	}
	if tc.IsTag() {
		t.Error("expected IsTag() = false")
	}
	if tc.IsCommit() {
		t.Error("expected IsCommit() = false")
	}
	if tc.Reference() != "main" {
		t.Errorf("Reference() = %q, want %q", tc.Reference(), "main")
	}
}

func TestTrackingConfig_IsTag(t *testing.T) {
	tc := NewTrackingConfigForTag("v1.0.0")
	if tc.IsBranch() {
		t.Error("expected IsBranch() = false")
	}
	if !tc.IsTag() {
		t.Error("expected IsTag() = true")
	}
	if tc.Reference() != "v1.0.0" {
		t.Errorf("Reference() = %q, want %q", tc.Reference(), "v1.0.0")
	}
}

func TestTrackingConfig_IsCommit(t *testing.T) {
	tc := NewTrackingConfigForCommit("abc1234")
	if tc.IsBranch() {
		t.Error("expected IsBranch() = false")
	}
	if !tc.IsCommit() {
		t.Error("expected IsCommit() = true")
	}
	if tc.Reference() != "abc1234" {
		t.Errorf("Reference() = %q, want %q", tc.Reference(), "abc1234")
	}
}

func TestTrackingConfig_IsEmpty(t *testing.T) {
	tc := NewTrackingConfig("", "", "")
	if !tc.IsEmpty() {
		t.Error("expected IsEmpty() = true")
	}

	tcBranch := NewTrackingConfigForBranch("main")
	if tcBranch.IsEmpty() {
		t.Error("expected IsEmpty() = false")
	}
}

func TestTrackingConfig_Reference_Priority(t *testing.T) {
	// Branch takes priority over tag and commit
	tc := NewTrackingConfig("main", "v1.0.0", "abc123")
	if tc.Reference() != "main" {
		t.Errorf("Reference() = %q, want %q (branch should take priority)", tc.Reference(), "main")
	}

	// Tag takes priority over commit
	tc2 := NewTrackingConfig("", "v1.0.0", "abc123")
	if tc2.Reference() != "v1.0.0" {
		t.Errorf("Reference() = %q, want %q (tag should take priority over commit)", tc2.Reference(), "v1.0.0")
	}

	// Empty returns empty
	tc3 := NewTrackingConfig("", "", "")
	if tc3.Reference() != "" {
		t.Errorf("Reference() = %q, want empty", tc3.Reference())
	}
}

func TestTrackingConfig_Equal(t *testing.T) {
	tc1 := NewTrackingConfigForBranch("main")
	tc2 := NewTrackingConfigForBranch("main")
	tc3 := NewTrackingConfigForBranch("develop")

	if !tc1.Equal(tc2) {
		t.Error("identical configs should be equal")
	}
	if tc1.Equal(tc3) {
		t.Error("different configs should not be equal")
	}
}
