package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	appupgrade "github.com/epoxsizer/kan/internal/upgrade"
)

func startVersionCheck(ctx context.Context, currentVersion string, service versionUpgradeService, logger *slog.Logger, notify func(string)) <-chan struct{} {
	cachePath, cachePathErr := appupgrade.DefaultCachePath()
	return startVersionCheckWithOptions(ctx, currentVersion, service, logger, notify, versionCheckOptions{
		cachePath: cachePath, cachePathErr: cachePathErr, now: time.Now().UTC(),
	})
}

type versionCheckOptions struct {
	cachePath    string
	cachePathErr error
	now          time.Time
}

func startVersionCheckWithOptions(ctx context.Context, currentVersion string, service versionUpgradeService, logger *slog.Logger, notify func(string), options versionCheckOptions) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if service == nil {
			return
		}
		if options.cachePathErr != nil {
			logVersionCheckError(logger, "version check cache unavailable", options.cachePathErr)
		} else {
			result, fresh, err := appupgrade.ReadFreshCache(options.cachePath, currentVersion, options.now)
			if err != nil {
				logVersionCheckError(logger, "version check cache ignored", err)
			} else if fresh {
				notifyUpgrade(result, notify)
				return
			}
		}

		checkContext, cancel := context.WithTimeout(ctx, versionCheckTimeout)
		result, err := service.Check(checkContext, currentVersion)
		cancel()
		if err != nil {
			if !errors.Is(err, appupgrade.ErrDevelopmentBuild) && !errors.Is(err, context.Canceled) {
				logVersionCheckError(logger, "background version check failed", err)
			}
			return
		}
		if options.cachePathErr == nil {
			if err = appupgrade.WriteCache(options.cachePath, result, options.now); err != nil {
				logVersionCheckError(logger, "version check cache write failed", err)
			}
		}
		notifyUpgrade(result, notify)
	}()
	return done
}

func notifyUpgrade(result appupgrade.Result, notify func(string)) {
	if result.Available && notify != nil {
		notify(fmt.Sprintf("kan v%s available — run: kan upgrade", result.Latest))
	}
}

func logVersionCheckError(logger *slog.Logger, message string, err error) {
	if logger != nil {
		logger.Info(message, "error", err)
	}
}
