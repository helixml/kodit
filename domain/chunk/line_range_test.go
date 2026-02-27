package chunk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLineRange(t *testing.T) {
	lr := NewLineRange(42, 10, 25)

	assert.Equal(t, int64(0), lr.ID())
	assert.Equal(t, int64(42), lr.EnrichmentID())
	assert.Equal(t, 10, lr.StartLine())
	assert.Equal(t, 25, lr.EndLine())
}

func TestReconstructLineRange(t *testing.T) {
	lr := ReconstructLineRange(7, 42, 10, 25)

	assert.Equal(t, int64(7), lr.ID())
	assert.Equal(t, int64(42), lr.EnrichmentID())
	assert.Equal(t, 10, lr.StartLine())
	assert.Equal(t, 25, lr.EndLine())
}
