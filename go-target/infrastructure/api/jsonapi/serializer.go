package jsonapi

import (
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/domain/tracking"
)

// RepositoryAttributes represents repository attributes in JSON:API format.
type RepositoryAttributes struct {
	RemoteURI      string     `json:"remote_uri"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	LastScannedAt  *time.Time `json:"last_scanned_at,omitempty"`
	ClonedPath     *string    `json:"cloned_path,omitempty"`
	TrackingBranch *string    `json:"tracking_branch,omitempty"`
	NumCommits     int        `json:"num_commits"`
	NumBranches    int        `json:"num_branches"`
	NumTags        int        `json:"num_tags"`
}

// BranchData represents branch data for repository details.
type BranchData struct {
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	CommitCount int    `json:"commit_count"`
}

// RecentCommitData represents commit data for repository details.
type RecentCommitData struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// RepositoryDetailsResponse represents repository details with branches and commits.
type RepositoryDetailsResponse struct {
	Data          *Resource          `json:"data"`
	Branches      []BranchData       `json:"branches"`
	RecentCommits []RecentCommitData `json:"recent_commits"`
}

// CommitAttributes represents commit attributes in JSON:API format.
type CommitAttributes struct {
	CommitSHA       string    `json:"commit_sha"`
	Date            time.Time `json:"date"`
	Message         string    `json:"message"`
	ParentCommitSHA string    `json:"parent_commit_sha"`
	Author          string    `json:"author"`
}

// FileAttributes represents file attributes in JSON:API format.
type FileAttributes struct {
	BlobSHA   string `json:"blob_sha"`
	Path      string `json:"path"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
}

// TagAttributes represents tag attributes in JSON:API format.
type TagAttributes struct {
	Name            string `json:"name"`
	TargetCommitSHA string `json:"target_commit_sha"`
	IsVersionTag    bool   `json:"is_version_tag"`
}

// EnrichmentAttributes represents enrichment attributes in JSON:API format.
type EnrichmentAttributes struct {
	Type      string     `json:"type"`
	Subtype   *string    `json:"subtype"`
	Content   string     `json:"content"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// TaskAttributes represents task attributes in JSON:API format.
type TaskAttributes struct {
	Type      string     `json:"type"`
	Priority  int        `json:"priority"`
	Payload   any        `json:"payload"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// TaskStatusAttributes represents task status attributes in JSON:API format.
type TaskStatusAttributes struct {
	Step      string     `json:"step"`
	State     string     `json:"state"`
	Progress  float64    `json:"progress"`
	Total     int        `json:"total"`
	Current   int        `json:"current"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Error     string     `json:"error"`
	Message   string     `json:"message"`
}

// StatusSummaryAttributes represents status summary attributes in JSON:API format.
type StatusSummaryAttributes struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrackingConfigAttributes represents tracking config attributes in JSON:API format.
type TrackingConfigAttributes struct {
	Mode  string  `json:"mode"`
	Value *string `json:"value,omitempty"`
}

// SnippetContentSchema represents snippet content in search results.
type SnippetContentSchema struct {
	Value    string `json:"value"`
	Language string `json:"language"`
}

// GitFileSchema represents a git file reference in search results.
type GitFileSchema struct {
	BlobSHA  string `json:"blob_sha"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// EnrichmentSchema represents an enrichment in search results.
type EnrichmentSchema struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// SnippetAttributes represents snippet attributes in search results.
type SnippetAttributes struct {
	CreatedAt      *time.Time           `json:"created_at,omitempty"`
	UpdatedAt      *time.Time           `json:"updated_at,omitempty"`
	DerivesFrom    []GitFileSchema      `json:"derives_from"`
	Content        SnippetContentSchema `json:"content"`
	Enrichments    []EnrichmentSchema   `json:"enrichments"`
	OriginalScores []float64            `json:"original_scores"`
}

// SearchFilters represents search filters in JSON:API format.
type SearchFilters struct {
	Languages          []string   `json:"languages,omitempty"`
	Authors            []string   `json:"authors,omitempty"`
	StartDate          *time.Time `json:"start_date,omitempty"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	Sources            []string   `json:"sources,omitempty"`
	FilePatterns       []string   `json:"file_patterns,omitempty"`
	EnrichmentTypes    []string   `json:"enrichment_types,omitempty"`
	EnrichmentSubtypes []string   `json:"enrichment_subtypes,omitempty"`
	CommitSHA          []string   `json:"commit_sha,omitempty"`
}

// SearchAttributes represents search request attributes in JSON:API format.
type SearchAttributes struct {
	Keywords []string       `json:"keywords,omitempty"`
	Code     *string        `json:"code,omitempty"`
	Text     *string        `json:"text,omitempty"`
	Limit    *int           `json:"limit,omitempty"`
	Filters  *SearchFilters `json:"filters,omitempty"`
}

// SearchData represents search request data in JSON:API format.
type SearchData struct {
	Type       string           `json:"type"`
	Attributes SearchAttributes `json:"attributes"`
}

// SearchRequest represents a JSON:API search request.
type SearchRequest struct {
	Data SearchData `json:"data"`
}

// Serializer converts domain objects to JSON:API resources.
type Serializer struct{}

// NewSerializer creates a new Serializer.
func NewSerializer() *Serializer {
	return &Serializer{}
}

// RepositoryResource converts a repository source to a JSON:API resource.
func (s *Serializer) RepositoryResource(source repository.Source) *Resource {
	repo := source.Repo()
	createdAt := repo.CreatedAt()
	updatedAt := repo.UpdatedAt()
	clonedPath := repo.WorkingCopy().Path()

	attrs := &RepositoryAttributes{
		RemoteURI:     repo.RemoteURL(),
		CreatedAt:     &createdAt,
		UpdatedAt:     &updatedAt,
		ClonedPath:    &clonedPath,
		NumCommits:    0, // These would need separate queries
		NumBranches:   0,
		NumTags:       0,
	}

	tc := repo.TrackingConfig()
	if tc.Branch() != "" {
		attrs.TrackingBranch = &[]string{tc.Branch()}[0]
	}

	return NewResource("repository", fmt.Sprintf("%d", repo.ID()), attrs)
}

// RepositoryResources converts multiple sources to JSON:API resources.
func (s *Serializer) RepositoryResources(sources []repository.Source) []*Resource {
	resources := make([]*Resource, len(sources))
	for i, source := range sources {
		resources[i] = s.RepositoryResource(source)
	}
	return resources
}

// CommitResource converts a commit to a JSON:API resource.
func (s *Serializer) CommitResource(commit repository.Commit) *Resource {
	attrs := &CommitAttributes{
		CommitSHA:       commit.SHA(),
		Date:            commit.CommittedAt(),
		Message:         commit.Message(),
		ParentCommitSHA: commit.ParentCommitSHA(),
		Author:          commit.Author().Name(),
	}
	return NewResource("commit", commit.SHA(), attrs)
}

// CommitResources converts multiple commits to JSON:API resources.
func (s *Serializer) CommitResources(commits []repository.Commit) []*Resource {
	resources := make([]*Resource, len(commits))
	for i, commit := range commits {
		resources[i] = s.CommitResource(commit)
	}
	return resources
}

// FileResource converts a file to a JSON:API resource.
func (s *Serializer) FileResource(file repository.File) *Resource {
	attrs := &FileAttributes{
		BlobSHA:   file.BlobSHA(),
		Path:      file.Path(),
		MimeType:  file.MimeType(),
		Size:      file.Size(),
		Extension: file.Extension(),
	}
	return NewResource("file", file.BlobSHA(), attrs)
}

// FileResources converts multiple files to JSON:API resources.
func (s *Serializer) FileResources(files []repository.File) []*Resource {
	resources := make([]*Resource, len(files))
	for i, file := range files {
		resources[i] = s.FileResource(file)
	}
	return resources
}

// TagResource converts a tag to a JSON:API resource.
func (s *Serializer) TagResource(tag repository.Tag) *Resource {
	attrs := &TagAttributes{
		Name:            tag.Name(),
		TargetCommitSHA: tag.CommitSHA(),
		IsVersionTag:    isVersionTag(tag.Name()),
	}
	return NewResource("tag", fmt.Sprintf("%d", tag.ID()), attrs)
}

// TagResources converts multiple tags to JSON:API resources.
func (s *Serializer) TagResources(tags []repository.Tag) []*Resource {
	resources := make([]*Resource, len(tags))
	for i, tag := range tags {
		resources[i] = s.TagResource(tag)
	}
	return resources
}

// EnrichmentResource converts an enrichment to a JSON:API resource.
func (s *Serializer) EnrichmentResource(e enrichment.Enrichment) *Resource {
	subtype := string(e.Subtype())
	createdAt := e.CreatedAt()
	updatedAt := e.UpdatedAt()

	attrs := &EnrichmentAttributes{
		Type:      string(e.Type()),
		Subtype:   &subtype,
		Content:   e.Content(),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	return NewResource("enrichment", fmt.Sprintf("%d", e.ID()), attrs)
}

// EnrichmentResources converts multiple enrichments to JSON:API resources.
func (s *Serializer) EnrichmentResources(enrichments []enrichment.Enrichment) []*Resource {
	resources := make([]*Resource, len(enrichments))
	for i, e := range enrichments {
		resources[i] = s.EnrichmentResource(e)
	}
	return resources
}

// TaskResource converts a task to a JSON:API resource.
func (s *Serializer) TaskResource(t task.Task) *Resource {
	createdAt := t.CreatedAt()
	updatedAt := t.UpdatedAt()

	attrs := &TaskAttributes{
		Type:      string(t.Operation()),
		Priority:  t.Priority(),
		Payload:   t.Payload(),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	return NewResource("task", fmt.Sprintf("%d", t.ID()), attrs)
}

// TaskResources converts multiple tasks to JSON:API resources.
func (s *Serializer) TaskResources(tasks []task.Task) []*Resource {
	resources := make([]*Resource, len(tasks))
	for i, t := range tasks {
		resources[i] = s.TaskResource(t)
	}
	return resources
}

// TaskStatusResource converts a task status to a JSON:API resource.
func (s *Serializer) TaskStatusResource(status task.Status) *Resource {
	createdAt := status.CreatedAt()
	updatedAt := status.UpdatedAt()

	attrs := &TaskStatusAttributes{
		Step:      string(status.Operation()),
		State:     string(status.State()),
		Progress:  status.CompletionPercent(),
		Total:     status.Total(),
		Current:   status.Current(),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
		Error:     status.Error(),
		Message:   status.Message(),
	}
	return NewResource("task_status", status.ID(), attrs)
}

// TaskStatusResources converts multiple statuses to JSON:API resources.
func (s *Serializer) TaskStatusResources(statuses []task.Status) []*Resource {
	resources := make([]*Resource, len(statuses))
	for i, status := range statuses {
		resources[i] = s.TaskStatusResource(status)
	}
	return resources
}

// StatusSummaryResource converts a status summary to a JSON:API resource.
func (s *Serializer) StatusSummaryResource(repoID int64, summary tracking.RepositoryStatusSummary) *Resource {
	attrs := &StatusSummaryAttributes{
		Status:    string(summary.Status()),
		Message:   summary.Message(),
		UpdatedAt: summary.UpdatedAt(),
	}
	return NewResource("repository_status_summary", fmt.Sprintf("%d", repoID), attrs)
}

// TrackingConfigResource converts a tracking config to a JSON:API resource.
func (s *Serializer) TrackingConfigResource(repoID int64, tc repository.TrackingConfig) *Resource {
	mode := "branch"
	var value *string

	if tc.Tag() != "" {
		mode = "tag"
		v := tc.Tag()
		value = &v
	} else if tc.Branch() != "" {
		v := tc.Branch()
		value = &v
	}

	attrs := &TrackingConfigAttributes{
		Mode:  mode,
		Value: value,
	}
	return NewResource("tracking-config", fmt.Sprintf("%d", repoID), attrs)
}

// SnippetResource converts a snippet with enrichments to a JSON:API resource for search results.
func (s *Serializer) SnippetResource(
	snip snippet.Snippet,
	enrichments []enrichment.Enrichment,
	files []repository.File,
	scores []float64,
) *Resource {
	createdAt := snip.CreatedAt()
	updatedAt := snip.UpdatedAt()

	derivesFrom := make([]GitFileSchema, len(files))
	for i, f := range files {
		derivesFrom[i] = GitFileSchema{
			BlobSHA:  f.BlobSHA(),
			Path:     f.Path(),
			MimeType: f.MimeType(),
			Size:     f.Size(),
		}
	}

	enrichmentSchemas := make([]EnrichmentSchema, len(enrichments))
	for i, e := range enrichments {
		enrichmentSchemas[i] = EnrichmentSchema{
			Type:    string(e.Type()),
			Content: e.Content(),
		}
	}

	attrs := &SnippetAttributes{
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
		DerivesFrom: derivesFrom,
		Content: SnippetContentSchema{
			Value:    snip.Content(),
			Language: snip.Extension(),
		},
		Enrichments:    enrichmentSchemas,
		OriginalScores: scores,
	}
	return NewResource("snippet", snip.SHA(), attrs)
}

// isVersionTag checks if a tag name looks like a version tag.
func isVersionTag(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Simple heuristic: starts with 'v' followed by digit, or starts with digit
	if name[0] == 'v' && len(name) > 1 && name[1] >= '0' && name[1] <= '9' {
		return true
	}
	if name[0] >= '0' && name[0] <= '9' {
		return true
	}
	return false
}
