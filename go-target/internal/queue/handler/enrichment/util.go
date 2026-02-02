package enrichment

import (
	"fmt"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/tracking"
)

// TrackerFactory creates trackers for progress reporting.
type TrackerFactory interface {
	ForOperation(operation queue.TaskOperation, trackableType domain.TrackableType, trackableID int64) *tracking.Tracker
}

func extractInt64(payload map[string]any, key string) (int64, error) {
	val, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("missing required field: %s", key)
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid type for %s: expected int64, got %T", key, val)
	}
}

func extractString(payload map[string]any, key string) (string, error) {
	val, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("missing required field: %s", key)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for %s: expected string, got %T", key, val)
	}

	return str, nil
}

// TruncateDiff truncates a diff to a reasonable length for LLM processing.
func TruncateDiff(diff string, maxLength int) string {
	if len(diff) <= maxLength {
		return diff
	}
	truncationNotice := "\n\n[diff truncated due to size]"
	return diff[:maxLength-len(truncationNotice)] + truncationNotice
}

// MaxDiffLength is the maximum characters for a commit diff (~25k tokens).
const MaxDiffLength = 100_000
