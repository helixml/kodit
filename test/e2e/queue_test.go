package e2e_test

import (
	"net/http"
	"testing"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

func TestQueue_ListTasks_Empty(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/queue")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.TaskListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestQueue_ListTasks_WithData(t *testing.T) {
	ts := NewTestServer(t)

	// Create a task
	ts.CreateTask(task.OperationCloneRepository, map[string]any{
		"repo_id":    1,
		"remote_url": "https://github.com/test/repo.git",
	})

	resp := ts.GET("/api/v1/queue")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.TaskListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(result.Data))
	}
	if result.Data[0].Type != "task" {
		t.Errorf("type = %q, want %q", result.Data[0].Type, "task")
	}
	if result.Data[0].Attributes.Type != string(task.OperationCloneRepository) {
		t.Errorf("attributes.type = %q, want %q", result.Data[0].Attributes.Type, task.OperationCloneRepository)
	}
}

func TestQueue_ListTasks_WithFilter(t *testing.T) {
	ts := NewTestServer(t)

	// Create tasks with different operations
	ts.CreateTask(task.OperationCloneRepository, map[string]any{"repo_id": 1})
	ts.CreateTask(task.OperationSyncRepository, map[string]any{"repo_id": 2})

	// Filter by task_type
	resp := ts.GET("/api/v1/queue?task_type=kodit.repository.clone")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.TaskListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(result.Data))
	}
	if result.Data[0].Attributes.Type != string(task.OperationCloneRepository) {
		t.Errorf("attributes.type = %q, want %q", result.Data[0].Attributes.Type, task.OperationCloneRepository)
	}
}
