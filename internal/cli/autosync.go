package cli

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

func startAutomaticSync(ctx context.Context, engine *syncEngine, logger *slog.Logger, notify func(string)) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		interval, err := time.ParseDuration(engine.config.Interval)
		if err != nil {
			logger.Error("automatic sync disabled", "error", err)
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				attemptContext, cancel := context.WithTimeout(ctx, 30*time.Second)
				err = engine.push(attemptContext, false)
				cancel()
				if err == nil {
					logger.Info("automatic JSON sync complete")
					continue
				}
				if errors.Is(err, errSyncConflict) {
					logger.Error("automatic JSON sync paused", "error", err)
					notify("S3 sync paused: remote changed; restart kan")
					return
				}
				logger.Error("automatic JSON sync failed", "error", err)
				notify("S3 sync failed; will retry")
			}
		}
	}()
	return done
}
