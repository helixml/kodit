package kodit

import "errors"

// Exported errors for library consumers.
var (
	// ErrEmptySource indicates a source with no content to process.
	ErrEmptySource = errors.New("kodit: source is empty")

	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("kodit: not found")

	// ErrValidation indicates a validation error.
	ErrValidation = errors.New("kodit: validation error")

	// ErrConflict indicates a conflict with existing data.
	ErrConflict = errors.New("kodit: conflict")

	// ErrNoStorage indicates no storage backend was configured.
	ErrNoStorage = errors.New("kodit: no storage backend configured")

	// ErrNoProvider indicates no AI provider was configured.
	ErrNoProvider = errors.New("kodit: no AI provider configured")

	// ErrProviderNotCapable indicates the provider lacks required capability.
	ErrProviderNotCapable = errors.New("kodit: provider does not support required capability")

	// ErrClientClosed indicates the client has been closed.
	ErrClientClosed = errors.New("kodit: client is closed")
)
