package sourcelocation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	sl := New(42, 10, 25)

	assert.Equal(t, int64(0), sl.ID())
	assert.Equal(t, int64(42), sl.EnrichmentID())
	assert.Equal(t, 0, sl.Page())
	assert.Equal(t, 10, sl.StartLine())
	assert.Equal(t, 25, sl.EndLine())
}

func TestNewPage(t *testing.T) {
	sl := NewPage(42, 3)

	assert.Equal(t, int64(0), sl.ID())
	assert.Equal(t, int64(42), sl.EnrichmentID())
	assert.Equal(t, 3, sl.Page())
	assert.Equal(t, 0, sl.StartLine())
	assert.Equal(t, 0, sl.EndLine())
}

func TestReconstruct(t *testing.T) {
	sl := Reconstruct(7, 42, 0, 10, 25)

	assert.Equal(t, int64(7), sl.ID())
	assert.Equal(t, int64(42), sl.EnrichmentID())
	assert.Equal(t, 0, sl.Page())
	assert.Equal(t, 10, sl.StartLine())
	assert.Equal(t, 25, sl.EndLine())
}

func TestReconstruct_WithPage(t *testing.T) {
	sl := Reconstruct(7, 42, 3, 0, 0)

	assert.Equal(t, int64(7), sl.ID())
	assert.Equal(t, int64(42), sl.EnrichmentID())
	assert.Equal(t, 3, sl.Page())
	assert.Equal(t, 0, sl.StartLine())
	assert.Equal(t, 0, sl.EndLine())
}
