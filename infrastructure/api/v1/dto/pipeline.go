package dto

import (
	"time"

	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

// PipelineAttributes represents pipeline attributes in JSON:API format.
type PipelineAttributes struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PipelineLinks holds links for a pipeline resource.
type PipelineLinks struct {
	Self string `json:"self"`
}

// PipelineData represents pipeline data in JSON:API format.
type PipelineData struct {
	Type       string             `json:"type"`
	ID         int64              `json:"id"`
	Attributes PipelineAttributes `json:"attributes"`
	Links      PipelineLinks      `json:"links"`
}

// PipelineResponse represents a single pipeline response in JSON:API format.
type PipelineResponse struct {
	Data PipelineData `json:"data"`
}

// PipelineListResponse represents a list of pipelines in JSON:API format.
type PipelineListResponse struct {
	Data  []PipelineData `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
}

// PipelineDetailResponse represents a pipeline with its included steps.
type PipelineDetailResponse struct {
	Data     PipelineData `json:"data"`
	Included []StepData   `json:"included"`
}

// StepAttributes represents step attributes in JSON:API format.
type StepAttributes struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	DependsOn []int64 `json:"depends_on"`
	JoinType  string  `json:"join_type"`
}

// StepLinks holds links for a step resource.
type StepLinks struct {
	Self string `json:"self"`
}

// StepData represents step data in JSON:API format.
type StepData struct {
	Type       string         `json:"type"`
	ID         int64          `json:"id"`
	Attributes StepAttributes `json:"attributes"`
	Links      StepLinks      `json:"links"`
}

// StepResponse represents a single step response in JSON:API format.
type StepResponse struct {
	Data StepData `json:"data"`
}

// StepListResponse represents a list of steps in JSON:API format.
type StepListResponse struct {
	Data  []StepData     `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
}

// StepInput describes a step within a pipeline create/update request.
type StepInput struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	DependsOn []string `json:"depends_on,omitempty"`
	JoinType  string   `json:"join_type,omitempty"`
}

// PipelineCreateAttributes holds the attributes for creating a pipeline.
type PipelineCreateAttributes struct {
	Name  string      `json:"name"`
	Steps []StepInput `json:"steps"`
}

// PipelineCreateData holds the data for creating a pipeline.
type PipelineCreateData struct {
	Type       string                   `json:"type"`
	Attributes PipelineCreateAttributes `json:"attributes"`
}

// PipelineCreateRequest represents a JSON:API request to create a pipeline.
type PipelineCreateRequest struct {
	Data PipelineCreateData `json:"data"`
}

// PipelineUpdateAttributes holds the attributes for updating a pipeline.
type PipelineUpdateAttributes struct {
	Name  string      `json:"name"`
	Steps []StepInput `json:"steps"`
}

// PipelineUpdateData holds the data for updating a pipeline.
type PipelineUpdateData struct {
	Type       string                   `json:"type"`
	Attributes PipelineUpdateAttributes `json:"attributes"`
}

// PipelineUpdateRequest represents a JSON:API request to update a pipeline.
type PipelineUpdateRequest struct {
	Data PipelineUpdateData `json:"data"`
}

// AssignPipelineData holds the data for assigning a pipeline to a repository.
type AssignPipelineData struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
}

// AssignPipelineRequest represents a JSON:API request to assign a pipeline.
type AssignPipelineRequest struct {
	Data AssignPipelineData `json:"data"`
}
