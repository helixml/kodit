package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/config"
)

// PeriodicSync enqueues SyncRepository tasks for all repositories on a timer.
type PeriodicSync struct {
	repositories repository.RepositoryStore
	queue        *Queue
	logger       *slog.Logger
	interval     time.Duration
	enabled      bool

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewPeriodicSync creates a new PeriodicSync from config and dependencies.
func NewPeriodicSync(
	cfg config.PeriodicSyncConfig,
	repositories repository.RepositoryStore,
	queue *Queue,
	logger *slog.Logger,
) *PeriodicSync {
	return &PeriodicSync{
		repositories: repositories,
		queue:        queue,
		logger:       logger,
		interval:     cfg.Interval(),
		enabled:      cfg.Enabled(),
	}
}

// Start begins periodic sync in a background goroutine.
// If disabled, this is a no-op.
func (p *PeriodicSync) Start(ctx context.Context) {
	if !p.enabled {
		p.logger.Info("periodic sync disabled")
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, p.cancel = context.WithCancel(ctx)
	p.wg.Go(func() {
		p.run(ctx)
	})

	p.logger.Info("periodic sync started", slog.Duration("interval", p.interval))
}

// Stop cancels the background goroutine and waits for it to finish.
func (p *PeriodicSync) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	p.wg.Wait()
	p.logger.Info("periodic sync stopped")
}

func (p *PeriodicSync) run(ctx context.Context) {
	// Sync immediately on startup
	p.sync(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.sync(ctx)
		}
	}
}

func (p *PeriodicSync) sync(ctx context.Context) {
	repos, err := p.repositories.Find(ctx, repository.WithScanDueBefore(time.Now().Add(-p.interval)))
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		p.logger.Error("periodic sync failed to find repositories",
			slog.String("error", err.Error()),
		)
		return
	}

	operations := task.PrescribedOperations{}.SyncRepository()

	for _, repo := range repos {
		payload := map[string]any{"repository_id": repo.ID()}
		if err := p.queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload); err != nil {
			if ctx.Err() != nil {
				return
			}
			p.logger.Warn("periodic sync failed to enqueue",
				slog.Int64("repo_id", repo.ID()),
				slog.String("error", err.Error()),
			)
		}
	}

	p.logger.Debug("periodic sync enqueued", slog.Int("count", len(repos)))
}
