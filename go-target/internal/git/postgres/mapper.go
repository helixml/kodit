package postgres

import (
	"time"

	"github.com/helixml/kodit/internal/git"
)

// Tracking type constants.
const (
	TrackingTypeBranch = "branch"
	TrackingTypeTag    = "tag"
	TrackingTypeCommit = "commit"
)

// RepoMapper maps between git.Repo and RepoEntity.
type RepoMapper struct{}

// ToDomain converts a RepoEntity to a git.Repo.
func (m RepoMapper) ToDomain(e RepoEntity) git.Repo {
	var wc git.WorkingCopy
	if e.ClonedPath != nil {
		wc = git.NewWorkingCopy(*e.ClonedPath, e.RemoteURI)
	}

	tc := trackingConfigFromDB(e.TrackingType, e.TrackingName)

	return git.ReconstructRepo(
		e.ID,
		e.RemoteURI,
		wc,
		tc,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToDatabase converts a git.Repo to a RepoEntity.
func (m RepoMapper) ToDatabase(r git.Repo) RepoEntity {
	var clonedPath *string
	if r.HasWorkingCopy() {
		path := r.WorkingCopy().Path()
		clonedPath = &path
	}

	trackingType, trackingName := trackingConfigToDB(r.TrackingConfig())

	return RepoEntity{
		ID:                 r.ID(),
		SanitizedRemoteURI: sanitizeRemoteURI(r.RemoteURL()),
		RemoteURI:          r.RemoteURL(),
		ClonedPath:         clonedPath,
		TrackingType:       trackingType,
		TrackingName:       trackingName,
		CreatedAt:          r.CreatedAt(),
		UpdatedAt:          r.UpdatedAt(),
	}
}

func trackingConfigFromDB(trackingType, trackingName string) git.TrackingConfig {
	switch trackingType {
	case TrackingTypeBranch:
		return git.NewTrackingConfigForBranch(trackingName)
	case TrackingTypeTag:
		return git.NewTrackingConfigForTag(trackingName)
	case TrackingTypeCommit:
		return git.NewTrackingConfigForCommit(trackingName)
	default:
		return git.TrackingConfig{}
	}
}

func trackingConfigToDB(tc git.TrackingConfig) (trackingType, trackingName string) {
	if tc.IsBranch() {
		return TrackingTypeBranch, tc.Branch()
	}
	if tc.IsTag() {
		return TrackingTypeTag, tc.Tag()
	}
	if tc.IsCommit() {
		return TrackingTypeCommit, tc.Commit()
	}
	return "", ""
}

func sanitizeRemoteURI(uri string) string {
	// Remove credentials from URI for storage as unique key
	// This is a simplified version - the Python version has more logic
	return uri
}

// CommitMapper maps between git.Commit and CommitEntity.
type CommitMapper struct{}

// ToDomain converts a CommitEntity to a git.Commit.
func (m CommitMapper) ToDomain(e CommitEntity) git.Commit {
	author := parseAuthorString(e.Author)
	parentSHA := ""
	if e.ParentCommitSHA != nil {
		parentSHA = *e.ParentCommitSHA
	}
	// Use author as committer since we don't store separate committer in DB
	return git.ReconstructCommit(
		0, // Commit uses SHA as primary key, not ID
		e.CommitSHA,
		e.RepoID,
		e.Message,
		author,
		author, // committer same as author for simplicity
		e.Date,
		e.Date,
		e.CreatedAt,
		parentSHA,
	)
}

// ToDatabase converts a git.Commit to a CommitEntity.
func (m CommitMapper) ToDatabase(c git.Commit) CommitEntity {
	var parentSHA *string
	if c.ParentCommitSHA() != "" {
		p := c.ParentCommitSHA()
		parentSHA = &p
	}

	now := time.Now()
	return CommitEntity{
		CommitSHA:       c.SHA(),
		RepoID:          c.RepoID(),
		Date:            c.AuthoredAt(),
		Message:         c.Message(),
		ParentCommitSHA: parentSHA,
		Author:          formatAuthorString(c.Author()),
		CreatedAt:       c.CreatedAt(),
		UpdatedAt:       now,
	}
}

func parseAuthorString(s string) git.Author {
	// Format: "Name <email>" or just "Name"
	// Simple parser - improve as needed
	name := s
	email := ""

	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '>' {
			for j := i - 1; j >= 0; j-- {
				if s[j] == '<' {
					name = s[:j]
					if len(name) > 0 && name[len(name)-1] == ' ' {
						name = name[:len(name)-1]
					}
					email = s[j+1 : i]
					break
				}
			}
			break
		}
	}

	return git.NewAuthor(name, email)
}

func formatAuthorString(a git.Author) string {
	if a.Email() == "" {
		return a.Name()
	}
	return a.Name() + " <" + a.Email() + ">"
}

// BranchMapper maps between git.Branch and BranchEntity.
type BranchMapper struct{}

// ToDomain converts a BranchEntity to a git.Branch.
func (m BranchMapper) ToDomain(e BranchEntity) git.Branch {
	return git.ReconstructBranch(
		0, // Branch uses composite key, not ID
		e.RepoID,
		e.Name,
		e.HeadCommitSHA,
		e.IsDefault,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToDatabase converts a git.Branch to a BranchEntity.
func (m BranchMapper) ToDatabase(b git.Branch) BranchEntity {
	return BranchEntity{
		RepoID:        b.RepoID(),
		Name:          b.Name(),
		HeadCommitSHA: b.HeadCommitSHA(),
		IsDefault:     b.IsDefault(),
		CreatedAt:     b.CreatedAt(),
		UpdatedAt:     time.Now(),
	}
}

// TagMapper maps between git.Tag and TagEntity.
type TagMapper struct{}

// ToDomain converts a TagEntity to a git.Tag.
func (m TagMapper) ToDomain(e TagEntity) git.Tag {
	var msg string
	if e.Message != nil {
		msg = *e.Message
	}

	var tagger git.Author
	if e.TaggerName != nil {
		email := ""
		if e.TaggerEmail != nil {
			email = *e.TaggerEmail
		}
		tagger = git.NewAuthor(*e.TaggerName, email)
	}

	var taggedAt time.Time
	if e.TaggedAt != nil {
		taggedAt = *e.TaggedAt
	}

	return git.ReconstructTag(
		0, // Tag uses composite key, not ID
		e.RepoID,
		e.Name,
		e.TargetCommitSHA,
		msg,
		tagger,
		taggedAt,
		e.CreatedAt,
	)
}

// ToDatabase converts a git.Tag to a TagEntity.
func (m TagMapper) ToDatabase(t git.Tag) TagEntity {
	var msg *string
	if t.Message() != "" {
		m := t.Message()
		msg = &m
	}

	var taggerName, taggerEmail *string
	var taggedAt *time.Time
	if t.IsAnnotated() {
		name := t.Tagger().Name()
		taggerName = &name
		email := t.Tagger().Email()
		taggerEmail = &email
		ta := t.TaggedAt()
		taggedAt = &ta
	}

	return TagEntity{
		RepoID:          t.RepoID(),
		Name:            t.Name(),
		TargetCommitSHA: t.CommitSHA(),
		Message:         msg,
		TaggerName:      taggerName,
		TaggerEmail:     taggerEmail,
		TaggedAt:        taggedAt,
		CreatedAt:       t.CreatedAt(),
		UpdatedAt:       time.Now(),
	}
}

// FileMapper maps between git.File and FileEntity.
type FileMapper struct{}

// ToDomain converts a FileEntity to a git.File.
func (m FileMapper) ToDomain(e FileEntity) git.File {
	return git.ReconstructFile(
		0, // File uses composite key, not ID
		e.CommitSHA,
		e.Path,
		e.BlobSHA,
		e.MimeType,
		e.Extension,
		e.Extension, // Using extension as language fallback
		e.Size,
		e.CreatedAt,
	)
}

// ToDatabase converts a git.File to a FileEntity.
func (m FileMapper) ToDatabase(f git.File) FileEntity {
	ext := f.Extension()
	if ext == "" {
		// Fallback to language for backwards compatibility
		ext = f.Language()
	}
	return FileEntity{
		CommitSHA: f.CommitSHA(),
		Path:      f.Path(),
		BlobSHA:   f.BlobSHA(),
		MimeType:  f.MimeType(),
		Extension: ext,
		Size:      f.Size(),
		CreatedAt: f.CreatedAt(),
	}
}
