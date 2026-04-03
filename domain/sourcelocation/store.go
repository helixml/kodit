package sourcelocation

import "github.com/helixml/kodit/domain/repository"

// Store defines persistence for source locations.
type Store interface {
	repository.Store[SourceLocation]
}
