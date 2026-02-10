package kodit

import (
	"errors"

	"github.com/helixml/kodit/application/service"
)

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

	// ErrNoDatabase indicates no database was configured.
	ErrNoDatabase = errors.New("kodit: no database configured")

	// ErrNoProvider indicates no AI provider was configured.
	ErrNoProvider = errors.New("kodit: no AI provider configured")

	// ErrProviderNotCapable indicates the provider lacks required capability.
	ErrProviderNotCapable = errors.New("kodit: provider does not support required capability")

	// ErrClientClosed is the canonical error for a closed client.
	// It references the service-level error so errors.Is works across packages.
	ErrClientClosed = service.ErrClientClosed
)
