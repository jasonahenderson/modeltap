package storage

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Pruner periodically deletes records older than a configured retention period.
// If retentionDays is <= 0, the pruner does nothing (all data is kept forever).
type Pruner struct {
	store         Store
	retentionDays int
	interval      time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPruner creates a new Pruner. The interval parameter controls how
// frequently the pruner checks for expired records. If retentionDays <= 0,
// the pruner will not delete any records.
func NewPruner(store Store, retentionDays int, interval time.Duration) *Pruner {
	return &Pruner{
		store:         store,
		retentionDays: retentionDays,
		interval:      interval,
	}
}

// Start launches a background goroutine that periodically prunes old records.
// It returns immediately. Use Stop to shut down the goroutine.
func (p *Pruner) Start(ctx context.Context) {
	if p.retentionDays <= 0 {
		slog.Info("retention pruning disabled (retentionDays <= 0)")
		return
	}

	ctx, p.cancel = context.WithCancel(ctx)
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		slog.Info("retention pruner started",
			"retention_days", p.retentionDays,
			"interval", p.interval,
		)

		// Run once immediately at startup.
		p.prune(ctx)

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("retention pruner stopped")
				return
			case <-ticker.C:
				p.prune(ctx)
			}
		}
	}()
}

// Stop cancels the background goroutine and waits for it to finish.
func (p *Pruner) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

// prune deletes records older than the retention period.
func (p *Pruner) prune(ctx context.Context) {
	cutoff := time.Now().UTC().AddDate(0, 0, -p.retentionDays)
	deleted, err := p.store.DeleteBefore(ctx, cutoff)
	if err != nil {
		slog.Error("retention pruning failed", "error", err)
		return
	}
	slog.Info("retention pruning complete", "deleted", deleted, "cutoff", cutoff)
}
