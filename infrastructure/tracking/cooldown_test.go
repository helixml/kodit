package tracking_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/tracking"
)

// fakeReporter records all statuses delivered to it.
type fakeReporter struct {
	mu       sync.Mutex
	statuses []task.Status
}

func (f *fakeReporter) OnChange(_ context.Context, status task.Status) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeReporter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.statuses)
}

func (f *fakeReporter) last() task.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statuses[len(f.statuses)-1]
}

func TestCooldown_FirstUpdatePassesThrough(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, time.Second)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)
	status = status.SetTotal(10)

	if err := cooldown.OnChange(ctx, status); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.count() != 1 {
		t.Fatalf("expected 1 delivery, got %d", fake.count())
	}
}

func TestCooldown_ThrottlesRapidUpdates(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, 500*time.Millisecond)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	// First update passes through immediately.
	status = status.SetCurrent(1, "step 1")
	_ = cooldown.OnChange(ctx, status)

	// Rapid subsequent updates should be throttled.
	for i := 2; i <= 20; i++ {
		status = status.SetCurrent(i, "step")
		_ = cooldown.OnChange(ctx, status)
	}

	// Only the first update should have been delivered so far.
	if fake.count() != 1 {
		t.Fatalf("expected 1 delivery during throttle window, got %d", fake.count())
	}

	// Wait for the cooldown timer to flush the pending status.
	time.Sleep(700 * time.Millisecond)

	if fake.count() != 2 {
		t.Fatalf("expected 2 deliveries after cooldown, got %d", fake.count())
	}

	// The flushed status should carry the latest progress.
	if fake.last().Current() != 20 {
		t.Fatalf("expected pending flush to have current=20, got %d", fake.last().Current())
	}
}

func TestCooldown_TerminalStateAlwaysFlushes(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, time.Hour) // very long interval
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	// First update passes through.
	status = status.SetCurrent(1, "step 1")
	_ = cooldown.OnChange(ctx, status)

	// This would normally be throttled, but terminal states bypass.
	status = status.Complete()
	_ = cooldown.OnChange(ctx, status)

	if fake.count() != 2 {
		t.Fatalf("expected 2 deliveries (initial + terminal), got %d", fake.count())
	}

	if fake.last().State() != task.ReportingStateCompleted {
		t.Fatalf("expected completed state, got %s", fake.last().State())
	}
}

func TestCooldown_FailedStateFlushesImmediately(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, time.Hour)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	status = status.SetCurrent(1, "step 1")
	_ = cooldown.OnChange(ctx, status)

	status = status.Fail("something broke")
	_ = cooldown.OnChange(ctx, status)

	if fake.count() != 2 {
		t.Fatalf("expected 2 deliveries, got %d", fake.count())
	}

	if fake.last().State() != task.ReportingStateFailed {
		t.Fatalf("expected failed state, got %s", fake.last().State())
	}
}

func TestCooldown_SkippedStateFlushesImmediately(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, time.Hour)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	status = status.SetCurrent(1, "step 1")
	_ = cooldown.OnChange(ctx, status)

	status = status.Skip("not needed")
	_ = cooldown.OnChange(ctx, status)

	if fake.count() != 2 {
		t.Fatalf("expected 2 deliveries, got %d", fake.count())
	}

	if fake.last().State() != task.ReportingStateSkipped {
		t.Fatalf("expected skipped state, got %s", fake.last().State())
	}
}

func TestCooldown_IndependentStatusIDsNotAffected(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, time.Hour)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()

	// Two different status IDs (different trackable IDs).
	status1 := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)
	status2 := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 2)

	// Both first updates should pass through.
	_ = cooldown.OnChange(ctx, status1.SetCurrent(1, "repo 1"))
	_ = cooldown.OnChange(ctx, status2.SetCurrent(1, "repo 2"))

	if fake.count() != 2 {
		t.Fatalf("expected 2 deliveries for independent IDs, got %d", fake.count())
	}
}

func TestCooldown_ConcurrentUpdates(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, 200*time.Millisecond)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s := status.SetCurrent(n, "concurrent")
			_ = cooldown.OnChange(ctx, s)
		}(i)
	}
	wg.Wait()

	// Complete to flush everything.
	_ = cooldown.OnChange(ctx, status.Complete())

	// Should have far fewer than 50 deliveries due to throttling,
	// plus the terminal delivery.
	if fake.count() >= 50 {
		t.Fatalf("expected throttling to reduce deliveries, got %d", fake.count())
	}

	// The last delivery should be the terminal state.
	if fake.last().State() != task.ReportingStateCompleted {
		t.Fatalf("expected completed state last, got %s", fake.last().State())
	}
}

func TestCooldown_CloseFlushesPending(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, time.Hour) // long interval

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	// First passes through.
	_ = cooldown.OnChange(ctx, status.SetCurrent(1, "step 1"))

	// This is throttled (pending).
	_ = cooldown.OnChange(ctx, status.SetCurrent(5, "step 5"))

	if fake.count() != 1 {
		t.Fatalf("expected 1 delivery before close, got %d", fake.count())
	}

	// Close should flush the pending status.
	_ = cooldown.Close()

	if fake.count() != 2 {
		t.Fatalf("expected 2 deliveries after close, got %d", fake.count())
	}

	if fake.last().Current() != 5 {
		t.Fatalf("expected flushed status current=5, got %d", fake.last().Current())
	}
}

func TestCooldown_AllowsUpdateAfterIntervalPasses(t *testing.T) {
	fake := &fakeReporter{}
	cooldown := tracking.NewCooldown(fake, 100*time.Millisecond)
	defer func() { _ = cooldown.Close() }()

	ctx := context.Background()
	status := task.NewStatus(task.OperationRunIndex, nil, task.TrackableTypeRepository, 1)

	_ = cooldown.OnChange(ctx, status.SetCurrent(1, "first"))
	if fake.count() != 1 {
		t.Fatalf("expected 1, got %d", fake.count())
	}

	// Wait for interval to pass.
	time.Sleep(150 * time.Millisecond)

	_ = cooldown.OnChange(ctx, status.SetCurrent(2, "second"))
	if fake.count() != 2 {
		t.Fatalf("expected 2 after interval passed, got %d", fake.count())
	}
}
