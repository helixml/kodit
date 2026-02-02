package domain

import "errors"

// Domain errors.
var (
	// ErrEmptySource indicates a source with no content to process.
	ErrEmptySource = errors.New("source is empty")

	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrValidation indicates a validation error.
	ErrValidation = errors.New("validation error")

	// ErrConflict indicates a conflict with existing data.
	ErrConflict = errors.New("conflict")
)
