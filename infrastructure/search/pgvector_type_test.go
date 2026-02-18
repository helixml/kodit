package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgVector_RoundTrip(t *testing.T) {
	original := NewPgVector([]float64{1.5, 2.25, -3.0, 0.0})

	val, err := original.Value()
	require.NoError(t, err)

	var restored PgVector
	err = restored.Scan(val)
	require.NoError(t, err)

	assert.Equal(t, original.Floats(), restored.Floats())
	assert.Equal(t, 4, restored.Dimension())
}

func TestPgVector_ScanFromString(t *testing.T) {
	var v PgVector
	err := v.Scan("[1.0,2.0,3.0]")
	require.NoError(t, err)
	assert.Equal(t, []float64{1.0, 2.0, 3.0}, v.Floats())
}

func TestPgVector_ScanFromBytes(t *testing.T) {
	var v PgVector
	err := v.Scan([]byte("[4.5,5.5]"))
	require.NoError(t, err)
	assert.Equal(t, []float64{4.5, 5.5}, v.Floats())
}

func TestPgVector_ScanNil(t *testing.T) {
	var v PgVector
	err := v.Scan(nil)
	require.NoError(t, err)
	assert.Nil(t, v.Floats())
}

func TestPgVector_EmptyVector(t *testing.T) {
	v := NewPgVector([]float64{})

	assert.Equal(t, 0, v.Dimension())
	assert.Equal(t, "[]", v.String())

	val, err := v.Value()
	require.NoError(t, err)

	var restored PgVector
	err = restored.Scan(val)
	require.NoError(t, err)
	assert.Equal(t, []float64{}, restored.Floats())
}

func TestPgVector_DefensiveCopy(t *testing.T) {
	input := []float64{1.0, 2.0, 3.0}
	v := NewPgVector(input)

	// Mutating the input should not affect the PgVector
	input[0] = 999.0
	assert.Equal(t, 1.0, v.Floats()[0])

	// Mutating the output of Floats() should not affect the PgVector
	output := v.Floats()
	output[1] = 999.0
	assert.Equal(t, 2.0, v.Floats()[1])
}

func TestPgVector_String(t *testing.T) {
	v := NewPgVector([]float64{1.0, 2.5, -3.14})
	assert.Equal(t, "[1,2.5,-3.14]", v.String())
}

func TestPgVector_ScanInvalidType(t *testing.T) {
	var v PgVector
	err := v.Scan(42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot scan int into PgVector")
}

func TestPgVector_ScanInvalidContent(t *testing.T) {
	var v PgVector
	err := v.Scan("[1.0,abc,3.0]")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse element 1")
}
