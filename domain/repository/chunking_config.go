package repository

import (
	"errors"
	"fmt"
)

// ChunkingConfig holds per-repository chunking parameters.
type ChunkingConfig struct {
	size    int
	overlap int
	minSize int
}

// DefaultChunkingConfig returns the system-wide default chunking parameters.
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{size: 1500, overlap: 200, minSize: 50}
}

// NewChunkingConfig creates a validated ChunkingConfig.
func NewChunkingConfig(size, overlap, minSize int) (ChunkingConfig, error) {
	cc := ChunkingConfig{size: size, overlap: overlap, minSize: minSize}
	if err := cc.Validate(); err != nil {
		return ChunkingConfig{}, err
	}
	return cc, nil
}

// ReconstructChunkingConfig rebuilds a ChunkingConfig from persistence without validation.
func ReconstructChunkingConfig(size, overlap, minSize int) ChunkingConfig {
	return ChunkingConfig{size: size, overlap: overlap, minSize: minSize}
}

// Size returns the chunk size in runes.
func (c ChunkingConfig) Size() int { return c.size }

// Overlap returns the overlap between consecutive chunks in runes.
func (c ChunkingConfig) Overlap() int { return c.overlap }

// MinSize returns the minimum chunk size in runes.
func (c ChunkingConfig) MinSize() int { return c.minSize }

// IsDefault returns true when the config matches DefaultChunkingConfig.
func (c ChunkingConfig) IsDefault() bool {
	d := DefaultChunkingConfig()
	return c.size == d.size && c.overlap == d.overlap && c.minSize == d.minSize
}

// Validate checks that the chunking parameters are consistent.
func (c ChunkingConfig) Validate() error {
	var errs []error
	if c.size <= 0 {
		errs = append(errs, fmt.Errorf("size must be positive, got %d", c.size))
	}
	if c.overlap < 0 {
		errs = append(errs, fmt.Errorf("overlap must be non-negative, got %d", c.overlap))
	}
	if c.minSize <= 0 {
		errs = append(errs, fmt.Errorf("min_size must be positive, got %d", c.minSize))
	}
	if c.size > 0 && c.overlap >= c.size {
		errs = append(errs, fmt.Errorf("overlap (%d) must be less than size (%d)", c.overlap, c.size))
	}
	if c.size > 0 && c.minSize > c.size {
		errs = append(errs, fmt.Errorf("min_size (%d) must not exceed size (%d)", c.minSize, c.size))
	}
	return errors.Join(errs...)
}
