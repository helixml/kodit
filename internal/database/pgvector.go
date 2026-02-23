package database

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// PgVector wraps a float64 slice for use as a PostgreSQL VECTOR column value.
// It implements sql.Scanner and driver.Valuer to convert between Go and the
// PostgreSQL text format "[1.0,2.0,3.0]".
type PgVector struct {
	floats []float64
}

// NewPgVector creates a PgVector from a float64 slice. The input is
// defensively copied so later mutations of the source slice have no effect.
func NewPgVector(floats []float64) PgVector {
	cp := make([]float64, len(floats))
	copy(cp, floats)
	return PgVector{floats: cp}
}

// Floats returns a defensive copy of the underlying float64 slice.
// Returns nil if the vector was never initialized (e.g. scanned from nil).
func (v PgVector) Floats() []float64 {
	if v.floats == nil {
		return nil
	}
	cp := make([]float64, len(v.floats))
	copy(cp, v.floats)
	return cp
}

// Dimension returns the number of elements in the vector.
func (v PgVector) Dimension() int {
	return len(v.floats)
}

// Scan implements sql.Scanner. It parses the PostgreSQL vector text format
// "[1.0,2.0,3.0]" from either a string or []byte value.
func (v *PgVector) Scan(value any) error {
	if value == nil {
		v.floats = nil
		return nil
	}

	var raw string
	switch val := value.(type) {
	case string:
		raw = val
	case []byte:
		raw = string(val)
	default:
		return fmt.Errorf("cannot scan %T into PgVector", value)
	}

	raw = strings.TrimSpace(raw)
	if raw == "[]" || raw == "" {
		v.floats = []float64{}
		return nil
	}

	// Strip surrounding brackets
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	parts := strings.Split(raw, ",")
	floats := make([]float64, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return fmt.Errorf("parse element %d: %w", i, err)
		}
		floats[i] = f
	}

	v.floats = floats
	return nil
}

// Value implements driver.Valuer. It serializes the vector to the PostgreSQL
// text format "[1.0,2.0,3.0]".
func (v PgVector) Value() (driver.Value, error) {
	return v.String(), nil
}

// String returns the PostgreSQL vector literal "[1.0,2.0,3.0]".
func (v PgVector) String() string {
	// Pre-allocate: ~12 bytes per float (digits + comma) plus brackets.
	var b strings.Builder
	b.Grow(len(v.floats)*12 + 2)
	b.WriteByte('[')
	for i, f := range v.floats {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(f, 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String()
}
