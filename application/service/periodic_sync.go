package service

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/config"
)

// PeriodicSync enqueues SyncRepository tasks for all repositories on a timer.
type PeriodicSync struct {
	repositories  repository.RepositoryStore
	queue         *Queue
	prescribedOps task.PrescribedOperations
	logger        zerolog.Logger
	interval      time.Duration
	checkInterval time.Duration
	enabled       bool

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewPeriodicSync creates a new PeriodicSync from config and dependencies.
func NewPeriodicSync(
	cfg config.PeriodicSyncConfig,
	repositories repository.RepositoryStore,
	queue *Queue,
	prescribedOps task.PrescribedOperations,
	logger zerolog.Logger,
) *PeriodicSync {
	return &PeriodicSync{
		repositories:  repositories,
		queue:         queue,
		prescribedOps: prescribedOps,
		logger:        logger,
		interval:      cfg.Interval(),
		checkInterval: cfg.CheckInterval(),
		enabled:       cfg.Enabled(),
	}
}

// Start begins periodic sync in a background goroutine.
// If disabled, this is a no-op.
func (p *PeriodicSync) Start(ctx context.Context) {
	if !p.enabled {
		p.logger.Info().Msg("periodic sync disabled")
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, p.cancel = context.WithCancel(ctx)
	p.wg.Go(func() {
		p.run(ctx)
	})

	p.logger.Info().Dur("interval", p.interval).Msg("periodic sync started")
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
	p.logger.Info().Msg("periodic sync stopped")
}

func (p *PeriodicSync) run(ctx context.Context) {
	ticker := time.NewTicker(p.checkInterval)
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
	// Don't flood the queue — let workers drain existing tasks first.
	pending, err := p.queue.Count(ctx)
	if err != nil {
		if ctx.Err() == nil {
			p.logger.Error().Str("error", err.Error()).Msg("periodic sync failed to count pending tasks")
		}
		return
	}
	if pending > 0 {
		p.logger.Debug().Int64("pending", pending).Msg("periodic sync skipped, queue has pending tasks")
		return
	}

	repos, err := p.repositories.Find(ctx, repository.WithScanDueBefore(time.Now().Add(-p.interval)))
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		p.logger.Error().Str("error", err.Error()).Msg("periodic sync failed to find repositories")
		return
	}

	operations := p.prescribedOps.SyncRepository()

	for _, repo := range repos {
		payload := map[string]any{"repository_id": repo.ID()}
		if err := p.queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload); err != nil {
			p.logger.Error().Int64("repo_id", repo.ID()).Str("error", err.Error()).Msg("periodic sync failed to enqueue")
			if ctx.Err() != nil {
				return
			}
		}
		p.logger.Debug().Int64("repo_id", repo.ID()).Msg("periodic sync enqueued")
	}
}
