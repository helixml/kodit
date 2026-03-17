package snippet

// IndexStatus represents the status of commit indexing.
type IndexStatus string

// IndexStatus values.
const (
	IndexStatusPending             IndexStatus = "pending"
	IndexStatusInProgress          IndexStatus = "in_progress"
	IndexStatusCompleted           IndexStatus = "completed"
	IndexStatusCompletedWithErrors IndexStatus = "completed_with_errors"
	IndexStatusFailed              IndexStatus = "failed"
)
