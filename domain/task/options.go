package task

import "github.com/helixml/kodit/domain/repository"

// WithPriorityOrder returns options that order by priority DESC, created_at ASC.
func WithPriorityOrder() []repository.Option {
	return []repository.Option{
		repository.WithOrderDesc("priority"),
		repository.WithOrderAsc("created_at"),
	}
}

// WithTrackable filters by trackable_type and trackable_id.
func WithTrackable(trackableType TrackableType, trackableID int64) []repository.Option {
	return []repository.Option{
		repository.WithCondition("trackable_type", string(trackableType)),
		repository.WithCondition("trackable_id", trackableID),
	}
}

// WithActiveState filters for statuses in started or in_progress state.
func WithActiveState() repository.Option {
	return repository.WithConditionIn("state", []string{
		string(ReportingStateStarted),
		string(ReportingStateInProgress),
	})
}
