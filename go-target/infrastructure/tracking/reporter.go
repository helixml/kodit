package tracking

import (
	"context"

	"github.com/helixml/kodit/domain/task"
)

// Reporter defines the interface for progress reporting modules.
// Implementations receive notifications when task status changes.
type Reporter interface {
	// OnChange is called when a task status changes.
	OnChange(ctx context.Context, status task.Status) error
}
