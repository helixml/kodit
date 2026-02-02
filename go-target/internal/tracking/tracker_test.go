package tracking

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReporter struct {
	mu     sync.Mutex
	calls  []queue.TaskStatus
	errFn  func() error
}

func newFakeReporter() *fakeReporter {
	return &fakeReporter{calls: make([]queue.TaskStatus, 0)}
}

func (r *fakeReporter) OnChange(_ context.Context, status queue.TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, status)
	if r.errFn != nil {
		return r.errFn()
	}
	return nil
}

func (r *fakeReporter) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *fakeReporter) lastCall() queue.TaskStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return queue.TaskStatus{}
	}
	return r.calls[len(r.calls)-1]
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestTracker_SetTotal(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	tracker.Subscribe(reporter)

	err := tracker.SetTotal(ctx, 100)
	require.NoError(t, err)

	assert.Equal(t, 1, reporter.callCount())
	assert.Equal(t, 100, reporter.lastCall().Total())
}

func TestTracker_SetCurrent(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	tracker.Subscribe(reporter)

	_ = tracker.SetTotal(ctx, 100)
	err := tracker.SetCurrent(ctx, 50, "halfway there")
	require.NoError(t, err)

	assert.Equal(t, 2, reporter.callCount())
	last := reporter.lastCall()
	assert.Equal(t, 50, last.Current())
	assert.Equal(t, "halfway there", last.Message())
	assert.Equal(t, domain.ReportingStateInProgress, last.State())
}

func TestTracker_Skip(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	tracker.Subscribe(reporter)

	err := tracker.Skip(ctx, "already exists")
	require.NoError(t, err)

	assert.Equal(t, domain.ReportingStateSkipped, tracker.Status().State())
	assert.Equal(t, domain.ReportingStateSkipped, reporter.lastCall().State())
}

func TestTracker_Fail(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	tracker.Subscribe(reporter)

	err := tracker.Fail(ctx, "network error")
	require.NoError(t, err)

	assert.Equal(t, domain.ReportingStateFailed, tracker.Status().State())
	assert.Equal(t, "network error", tracker.Status().Error())
}

func TestTracker_Complete(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	tracker.Subscribe(reporter)

	err := tracker.Complete(ctx)
	require.NoError(t, err)

	assert.Equal(t, domain.ReportingStateCompleted, tracker.Status().State())
}

func TestTracker_Child(t *testing.T) {
	ctx := context.Background()
	parent := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	parent.Subscribe(reporter)

	child := parent.Child(queue.OperationScanCommit)

	// Child should inherit trackable info
	assert.Equal(t, domain.TrackableTypeRepository, child.Status().TrackableType())
	assert.Equal(t, int64(42), child.Status().TrackableID())

	// Child should inherit subscribers
	err := child.SetTotal(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, reporter.callCount())

	// Child should have its own operation
	assert.Equal(t, queue.OperationScanCommit, child.Status().Operation())
}

func TestTracker_MultipleSubscribers(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter1 := newFakeReporter()
	reporter2 := newFakeReporter()
	tracker.Subscribe(reporter1)
	tracker.Subscribe(reporter2)

	err := tracker.SetTotal(ctx, 100)
	require.NoError(t, err)

	assert.Equal(t, 1, reporter1.callCount())
	assert.Equal(t, 1, reporter2.callCount())
}

func TestTracker_Notify(t *testing.T) {
	ctx := context.Background()
	tracker := TrackerForOperation(
		queue.OperationCloneRepository,
		testLogger(),
		domain.TrackableTypeRepository,
		42,
	)

	reporter := newFakeReporter()
	tracker.Subscribe(reporter)

	// No notifications yet
	assert.Equal(t, 0, reporter.callCount())

	// Explicit notify
	err := tracker.Notify(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, reporter.callCount())
}
