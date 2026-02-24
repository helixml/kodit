package tracking

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/helixml/kodit/domain/task"
)

// Ensure Cooldown implements both Reporter and io.Closer.
var (
	_ Reporter  = (*Cooldown)(nil)
	_ io.Closer = (*Cooldown)(nil)
)

// Cooldown wraps a Reporter and limits how frequently updates are delivered
// for each status ID. Terminal states (completed, failed, skipped) are always
// delivered immediately. Non-terminal updates are delivered at most once per
// the configured interval; the latest pending status is flushed when the
// interval elapses or when a terminal state arrives.
type Cooldown struct {
	inner    Reporter
	interval time.Duration
	mu       sync.Mutex
	entries  map[string]*cooldownEntry
}

type cooldownEntry struct {
	lastFlush time.Time
	pending   *task.Status
	timer     *time.Timer
}

// NewCooldown creates a Cooldown wrapping the given reporter with the
// specified minimum interval between deliveries per status ID.
func NewCooldown(inner Reporter, interval time.Duration) *Cooldown {
	return &Cooldown{
		inner:    inner,
		interval: interval,
		entries:  make(map[string]*cooldownEntry),
	}
}

// OnChange receives a status update. Terminal states flush immediately.
// Non-terminal states are throttled to at most one delivery per interval.
func (c *Cooldown) OnChange(ctx context.Context, status task.Status) error {
	id := status.ID()

	c.mu.Lock()

	if status.State().IsTerminal() {
		entry := c.entries[id]
		if entry != nil {
			if entry.timer != nil {
				entry.timer.Stop()
			}
			delete(c.entries, id)
		}
		c.mu.Unlock()
		return c.inner.OnChange(ctx, status)
	}

	entry, exists := c.entries[id]
	if !exists {
		entry = &cooldownEntry{}
		c.entries[id] = entry
	}

	elapsed := time.Since(entry.lastFlush)
	if elapsed >= c.interval {
		if entry.timer != nil {
			entry.timer.Stop()
			entry.timer = nil
		}
		entry.pending = nil
		entry.lastFlush = time.Now()
		c.mu.Unlock()
		return c.inner.OnChange(ctx, status)
	}

	// Throttled: store as pending, schedule flush if no timer is running.
	statusCopy := status
	entry.pending = &statusCopy

	if entry.timer == nil {
		remaining := c.interval - elapsed
		entry.timer = time.AfterFunc(remaining, func() {
			c.flushPending(id)
		})
	}

	c.mu.Unlock()
	return nil
}

// Close flushes all pending statuses and stops all timers.
func (c *Cooldown) Close() error {
	c.mu.Lock()
	entries := make(map[string]*cooldownEntry, len(c.entries))
	for k, v := range c.entries {
		entries[k] = v
	}
	c.entries = make(map[string]*cooldownEntry)
	c.mu.Unlock()

	for _, entry := range entries {
		if entry.timer != nil {
			entry.timer.Stop()
		}
		if entry.pending != nil {
			_ = c.inner.OnChange(context.Background(), *entry.pending)
		}
	}
	return nil
}

func (c *Cooldown) flushPending(id string) {
	c.mu.Lock()
	entry, exists := c.entries[id]
	if !exists || entry.pending == nil {
		if exists {
			entry.timer = nil
		}
		c.mu.Unlock()
		return
	}

	status := *entry.pending
	entry.pending = nil
	entry.lastFlush = time.Now()
	entry.timer = nil
	c.mu.Unlock()

	_ = c.inner.OnChange(context.Background(), status)
}
