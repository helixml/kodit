package chunk

import "github.com/helixml/kodit/domain/repository"

// LineRangeStore defines persistence for chunk line ranges.
type LineRangeStore interface {
	repository.Store[LineRange]
}
