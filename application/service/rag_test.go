package service

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/task"
)

func newRAGTestDeps(t *testing.T) (*RAG, testStores) {
	t.Helper()
	stores := newTestStores(t)
	queue := NewQueue(stores.tasks, zerolog.Nop())
	prescribedOps := task.NewPrescribedOperations(true, true)
	repos := NewRepository(stores.repos, stores.commits, stores.branches, stores.tags, queue, prescribedOps, zerolog.Nop())
	// Search is not exercised in Convert/Index tests; pass nil.
	rag := NewRAG(repos, nil)
	return rag, stores
}

func TestRAG_Convert_IndexesDirectory(t *testing.T) {
	rag, _ := newRAGTestDeps(t)
	ctx := context.Background()

	// Use a file:// URI pointing at the current working directory.
	// The directory exists, so the repository is created immediately.
	repoID, err := rag.Convert(ctx, "/tmp")

	require.NoError(t, err)
	assert.Positive(t, repoID, "Convert must return a positive repository ID")
}

func TestRAG_Convert_Idempotent(t *testing.T) {
	rag, _ := newRAGTestDeps(t)
	ctx := context.Background()

	id1, err := rag.Convert(ctx, "/tmp")
	require.NoError(t, err)

	id2, err := rag.Convert(ctx, "/tmp")
	require.NoError(t, err)

	assert.Equal(t, id1, id2, "Convert must return the same ID for the same path")
}

func TestRAG_Index_NoOp(t *testing.T) {
	rag, _ := newRAGTestDeps(t)
	ctx := context.Background()

	err := rag.Index(ctx, 42)
	assert.NoError(t, err, "Index must be a no-op and return nil")
}
