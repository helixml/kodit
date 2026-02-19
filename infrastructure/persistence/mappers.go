package persistence

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// Tracking type constants.
const (
	TrackingTypeBranch = "branch"
	TrackingTypeTag    = "tag"
	TrackingTypeCommit = "commit"
)

// RepositoryMapper maps between domain Repository and persistence RepositoryModel.
type RepositoryMapper struct{}

// ToDomain converts a RepositoryModel to a domain Repository.
func (m RepositoryMapper) ToDomain(e RepositoryModel) repository.Repository {
	var wc repository.WorkingCopy
	if e.ClonedPath != nil {
		wc = repository.NewWorkingCopy(*e.ClonedPath, e.RemoteURI)
	}

	tc := trackingConfigFromDB(e.TrackingType, e.TrackingName)

	var lastSyncedAt time.Time
	if e.LastScannedAt != nil {
		lastSyncedAt = *e.LastScannedAt
	}

	return repository.ReconstructRepository(
		e.ID,
		e.RemoteURI,
		wc,
		tc,
		e.CreatedAt,
		e.UpdatedAt,
		lastSyncedAt,
	)
}

// ToModel converts a domain Repository to a RepositoryModel.
func (m RepositoryMapper) ToModel(r repository.Repository) RepositoryModel {
	var clonedPath *string
	if r.HasWorkingCopy() {
		path := r.WorkingCopy().Path()
		clonedPath = &path
	}

	trackingType, trackingName := trackingConfigToDB(r.TrackingConfig())

	var lastScannedAt *time.Time
	if !r.LastScannedAt().IsZero() {
		t := r.LastScannedAt()
		lastScannedAt = &t
	}

	return RepositoryModel{
		ID:                 r.ID(),
		SanitizedRemoteURI: sanitizeRemoteURI(r.RemoteURL()),
		RemoteURI:          r.RemoteURL(),
		ClonedPath:         clonedPath,
		LastScannedAt:      lastScannedAt,
		TrackingType:       trackingType,
		TrackingName:       trackingName,
		CreatedAt:          r.CreatedAt(),
		UpdatedAt:          r.UpdatedAt(),
	}
}

func trackingConfigFromDB(trackingType, trackingName string) repository.TrackingConfig {
	switch trackingType {
	case TrackingTypeBranch:
		return repository.NewTrackingConfigForBranch(trackingName)
	case TrackingTypeTag:
		return repository.NewTrackingConfigForTag(trackingName)
	case TrackingTypeCommit:
		return repository.NewTrackingConfigForCommit(trackingName)
	default:
		return repository.TrackingConfig{}
	}
}

func trackingConfigToDB(tc repository.TrackingConfig) (trackingType, trackingName string) {
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
	return uri
}

// CommitMapper maps between domain Commit and persistence CommitModel.
type CommitMapper struct{}

// ToDomain converts a CommitModel to a domain Commit.
func (m CommitMapper) ToDomain(e CommitModel) repository.Commit {
	author := parseAuthorString(e.Author)
	parentSHA := ""
	if e.ParentCommitSHA != nil {
		parentSHA = *e.ParentCommitSHA
	}
	return repository.ReconstructCommit(
		0,
		e.CommitSHA,
		e.RepoID,
		e.Message,
		author,
		author,
		e.Date,
		e.Date,
		e.CreatedAt,
		parentSHA,
	)
}

// ToModel converts a domain Commit to a CommitModel.
func (m CommitMapper) ToModel(c repository.Commit) CommitModel {
	var parentSHA *string
	if c.ParentCommitSHA() != "" {
		p := c.ParentCommitSHA()
		parentSHA = &p
	}

	now := time.Now()
	return CommitModel{
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

func parseAuthorString(s string) repository.Author {
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

	return repository.NewAuthor(name, email)
}

func formatAuthorString(a repository.Author) string {
	if a.Email() == "" {
		return a.Name()
	}
	return a.Name() + " <" + a.Email() + ">"
}

// BranchMapper maps between domain Branch and persistence BranchModel.
type BranchMapper struct{}

// ToDomain converts a BranchModel to a domain Branch.
func (m BranchMapper) ToDomain(e BranchModel) repository.Branch {
	return repository.ReconstructBranch(
		0,
		e.RepoID,
		e.Name,
		e.HeadCommitSHA,
		e.IsDefault,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain Branch to a BranchModel.
func (m BranchMapper) ToModel(b repository.Branch) BranchModel {
	return BranchModel{
		RepoID:        b.RepoID(),
		Name:          b.Name(),
		HeadCommitSHA: b.HeadCommitSHA(),
		IsDefault:     b.IsDefault(),
		CreatedAt:     b.CreatedAt(),
		UpdatedAt:     time.Now(),
	}
}

// TagMapper maps between domain Tag and persistence TagModel.
type TagMapper struct{}

// ToDomain converts a TagModel to a domain Tag.
func (m TagMapper) ToDomain(e TagModel) repository.Tag {
	var msg string
	if e.Message != nil {
		msg = *e.Message
	}

	var tagger repository.Author
	if e.TaggerName != nil {
		email := ""
		if e.TaggerEmail != nil {
			email = *e.TaggerEmail
		}
		tagger = repository.NewAuthor(*e.TaggerName, email)
	}

	var taggedAt time.Time
	if e.TaggedAt != nil {
		taggedAt = *e.TaggedAt
	}

	return repository.ReconstructTag(
		0,
		e.RepoID,
		e.Name,
		e.TargetCommitSHA,
		msg,
		tagger,
		taggedAt,
		e.CreatedAt,
	)
}

// ToModel converts a domain Tag to a TagModel.
func (m TagMapper) ToModel(t repository.Tag) TagModel {
	var msg *string
	if t.Message() != "" {
		message := t.Message()
		msg = &message
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

	return TagModel{
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

// FileMapper maps between domain File and persistence FileModel.
type FileMapper struct{}

// ToDomain converts a FileModel to a domain File.
func (m FileMapper) ToDomain(e FileModel) repository.File {
	return repository.ReconstructFile(
		e.ID,
		e.CommitSHA,
		e.Path,
		e.BlobSHA,
		e.MimeType,
		e.Extension,
		e.Extension,
		e.Size,
		e.CreatedAt,
	)
}

// ToModel converts a domain File to a FileModel.
func (m FileMapper) ToModel(f repository.File) FileModel {
	ext := f.Extension()
	if ext == "" {
		ext = f.Language()
	}
	return FileModel{
		ID:        f.ID(),
		CommitSHA: f.CommitSHA(),
		Path:      f.Path(),
		BlobSHA:   f.BlobSHA(),
		MimeType:  f.MimeType(),
		Extension: ext,
		Size:      f.Size(),
		CreatedAt: f.CreatedAt(),
	}
}

// CommitIndexMapper maps between domain CommitIndex and persistence CommitIndexModel.
type CommitIndexMapper struct{}

// ToDomain converts a CommitIndexModel to a domain CommitIndex.
// Note: snippets are loaded separately as they come from different tables.
func (m CommitIndexMapper) ToDomain(e CommitIndexModel) snippet.CommitIndex {
	var indexedAt time.Time
	if e.IndexedAt.Valid {
		indexedAt = e.IndexedAt.Time
	}
	var errorMessage string
	if e.ErrorMessage.Valid {
		errorMessage = e.ErrorMessage.String
	}
	return snippet.ReconstructCommitIndex(
		e.CommitSHA,
		nil, // snippets - loaded separately via joins
		snippet.IndexStatus(e.Status),
		indexedAt,
		errorMessage,
		e.FilesProcessed,
		e.ProcessingTimeSeconds,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain CommitIndex to a CommitIndexModel.
func (m CommitIndexMapper) ToModel(ci snippet.CommitIndex) CommitIndexModel {
	model := CommitIndexModel{
		CommitSHA:             ci.CommitSHA(),
		Status:                string(ci.Status()),
		FilesProcessed:        ci.FilesProcessed(),
		ProcessingTimeSeconds: ci.ProcessingTimeSeconds(),
		CreatedAt:             ci.CreatedAt(),
		UpdatedAt:             ci.UpdatedAt(),
	}
	if !ci.IndexedAt().IsZero() {
		model.IndexedAt.Valid = true
		model.IndexedAt.Time = ci.IndexedAt()
	}
	if ci.ErrorMessage() != "" {
		model.ErrorMessage.Valid = true
		model.ErrorMessage.String = ci.ErrorMessage()
	}
	return model
}

// EnrichmentMapper maps between domain Enrichment and persistence EnrichmentModel.
type EnrichmentMapper struct{}

// ToDomain converts an EnrichmentModel to a domain Enrichment.
func (m EnrichmentMapper) ToDomain(e EnrichmentModel) enrichment.Enrichment {
	return enrichment.ReconstructEnrichment(
		e.ID,
		enrichment.Type(e.Type),
		enrichment.Subtype(e.Subtype),
		enrichment.EntityTypeCommit, // Default - actual entity type comes from associations
		e.Content,
		e.Language,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain Enrichment to an EnrichmentModel.
func (m EnrichmentMapper) ToModel(e enrichment.Enrichment) EnrichmentModel {
	return EnrichmentModel{
		ID:        e.ID(),
		Type:      string(e.Type()),
		Subtype:   string(e.Subtype()),
		Content:   e.Content(),
		Language:  e.Language(),
		CreatedAt: e.CreatedAt(),
		UpdatedAt: e.UpdatedAt(),
	}
}

// AssociationMapper maps between domain Association and persistence EnrichmentAssociationModel.
type AssociationMapper struct{}

// ToDomain converts an EnrichmentAssociationModel to a domain Association.
func (m AssociationMapper) ToDomain(e EnrichmentAssociationModel) enrichment.Association {
	return enrichment.ReconstructAssociation(
		e.ID,
		e.EnrichmentID,
		e.EntityID,
		enrichment.EntityTypeKey(e.EntityType),
	)
}

// ToModel converts a domain Association to an EnrichmentAssociationModel.
func (m AssociationMapper) ToModel(a enrichment.Association) EnrichmentAssociationModel {
	now := time.Now()
	return EnrichmentAssociationModel{
		ID:           a.ID(),
		EnrichmentID: a.EnrichmentID(),
		EntityType:   string(a.EntityType()),
		EntityID:     a.EntityID(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// TaskMapper maps between domain Task and persistence TaskModel.
type TaskMapper struct{}

// ToDomain converts a TaskModel to a domain Task.
func (m TaskMapper) ToDomain(e TaskModel) (task.Task, error) {
	var payload map[string]any
	if len(e.Payload) > 0 {
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			return task.Task{}, fmt.Errorf("failed to unmarshal task payload: %w", err)
		}
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	return task.NewTaskWithID(
		e.ID,
		e.DedupKey,
		task.Operation(e.Type),
		e.Priority,
		payload,
		e.CreatedAt,
		e.UpdatedAt,
	), nil
}

// ToModel converts a domain Task to a TaskModel.
func (m TaskMapper) ToModel(t task.Task) (TaskModel, error) {
	payloadJSON, err := json.Marshal(t.Payload())
	if err != nil {
		return TaskModel{}, fmt.Errorf("failed to marshal task payload: %w", err)
	}

	return TaskModel{
		ID:        t.ID(),
		DedupKey:  t.DedupKey(),
		Type:      string(t.Operation()),
		Payload:   payloadJSON,
		Priority:  t.Priority(),
		CreatedAt: t.CreatedAt(),
		UpdatedAt: t.UpdatedAt(),
	}, nil
}

// TaskStatusMapper maps between domain Status and persistence TaskStatusModel.
type TaskStatusMapper struct{}

// ToDomain converts a TaskStatusModel to a domain Status.
func (m TaskStatusMapper) ToDomain(e TaskStatusModel) task.Status {
	var trackableID int64
	var trackableType task.TrackableType

	if e.TrackableID != nil {
		trackableID = *e.TrackableID
	}
	if e.TrackableType != nil {
		trackableType = task.TrackableType(*e.TrackableType)
	}

	return task.NewStatusFull(
		e.ID,
		task.ReportingState(e.State),
		task.Operation(e.Operation),
		e.Message,
		e.CreatedAt,
		e.UpdatedAt,
		e.Total,
		e.Current,
		e.Error,
		nil,
		trackableID,
		trackableType,
	)
}

// ToModel converts a domain Status to a TaskStatusModel.
func (m TaskStatusMapper) ToModel(s task.Status) TaskStatusModel {
	model := TaskStatusModel{
		ID:        s.ID(),
		CreatedAt: s.CreatedAt(),
		UpdatedAt: s.UpdatedAt(),
		Operation: string(s.Operation()),
		Message:   s.Message(),
		State:     string(s.State()),
		Error:     s.Error(),
		Total:     s.Total(),
		Current:   s.Current(),
	}

	if s.TrackableID() != 0 {
		id := s.TrackableID()
		model.TrackableID = &id
	}

	if s.TrackableType() != "" {
		t := string(s.TrackableType())
		model.TrackableType = &t
	}

	if s.Parent() != nil {
		parentID := s.Parent().ID()
		model.ParentID = &parentID
	}

	return model
}
