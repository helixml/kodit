package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTask(t *testing.T) {
	payload := map[string]any{"repo_id": int64(123)}
	task := NewTask(OperationCloneRepository, 2000, payload)

	assert.Equal(t, OperationCloneRepository, task.Operation())
	assert.Equal(t, 2000, task.Priority())
	assert.Equal(t, "kodit.repository.clone:123", task.DedupKey())
	assert.Equal(t, payload, task.Payload())
	assert.Zero(t, task.ID())
	assert.True(t, task.CreatedAt().IsZero())
}

func TestTask_WithID(t *testing.T) {
	task := NewTask(OperationScanCommit, 1000, map[string]any{"commit_id": 1})
	taskWithID := task.WithID(42)

	assert.Equal(t, int64(42), taskWithID.ID())
	assert.Zero(t, task.ID()) // Original unchanged
}

func TestTask_WithTimestamps(t *testing.T) {
	task := NewTask(OperationScanCommit, 1000, map[string]any{"commit_id": 1})
	now := time.Now()
	taskWithTimestamps := task.WithTimestamps(now, now)

	assert.Equal(t, now, taskWithTimestamps.CreatedAt())
	assert.Equal(t, now, taskWithTimestamps.UpdatedAt())
	assert.True(t, task.CreatedAt().IsZero()) // Original unchanged
}

func TestTask_PayloadJSON(t *testing.T) {
	payload := map[string]any{
		"repo_id":    int64(123),
		"commit_sha": "abc123",
	}
	task := NewTask(OperationScanCommit, 1000, payload)

	jsonBytes, err := task.PayloadJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(jsonBytes), "repo_id")
	assert.Contains(t, string(jsonBytes), "commit_sha")
}

func TestTask_PayloadIsCopied(t *testing.T) {
	payload := map[string]any{"key": "original"}
	task := NewTask(OperationScanCommit, 1000, payload)

	// Modify the original payload
	payload["key"] = "modified"

	// Task should have the original value
	assert.Equal(t, "original", task.Payload()["key"])
}

func TestTask_PayloadReturnsCopy(t *testing.T) {
	task := NewTask(OperationScanCommit, 1000, map[string]any{"key": "value"})

	// Modify the returned payload
	returned := task.Payload()
	returned["key"] = "modified"

	// Task should still have the original value
	assert.Equal(t, "value", task.Payload()["key"])
}

func TestNewTaskWithID(t *testing.T) {
	now := time.Now()
	payload := map[string]any{"repo_id": int64(123)}

	task := NewTaskWithID(
		42,
		"custom-dedup-key",
		OperationCloneRepository,
		2000,
		payload,
		now,
		now.Add(time.Hour),
	)

	assert.Equal(t, int64(42), task.ID())
	assert.Equal(t, "custom-dedup-key", task.DedupKey())
	assert.Equal(t, OperationCloneRepository, task.Operation())
	assert.Equal(t, 2000, task.Priority())
	assert.Equal(t, payload, task.Payload())
	assert.Equal(t, now, task.CreatedAt())
	assert.Equal(t, now.Add(time.Hour), task.UpdatedAt())
}

func TestCreateDedupKey(t *testing.T) {
	tests := []struct {
		name      string
		operation TaskOperation
		payload   map[string]any
		want      string
	}{
		{
			name:      "with int64 value",
			operation: OperationCloneRepository,
			payload:   map[string]any{"repo_id": int64(123)},
			want:      "kodit.repository.clone:123",
		},
		{
			name:      "with string value",
			operation: OperationScanCommit,
			payload:   map[string]any{"commit_sha": "abc123"},
			want:      "kodit.commit.scan:abc123",
		},
		{
			name:      "empty payload",
			operation: OperationScanCommit,
			payload:   map[string]any{},
			want:      "kodit.commit.scan:<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask(tt.operation, 1000, tt.payload)
			assert.Equal(t, tt.want, task.DedupKey())
		})
	}
}
