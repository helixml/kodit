package domain

import "errors"

// Domain errors.
var (
	// ErrEmptySource indicates a source with no content to process.
	ErrEmptySource = errors.New("source is empty")
)
