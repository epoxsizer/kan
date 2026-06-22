package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const automaticBackupInterval = 6 * time.Hour

type backupRepository interface {
	Backup(context.Context, string) error
}

func startAutomaticBackups(ctx context.Context, repo backupRepository, logger *slog.Logger, directory string) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if path, created, err := backupIfDue(ctx, repo, directory, automaticBackupInterval, time.Now()); err != nil {
			logger.Error("automatic database backup failed", "error", err)
		} else if created {
			logger.Info("automatic database backup created", "path", path)
		}

		ticker := time.NewTicker(automaticBackupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				path, err := createAutomaticBackup(ctx, repo, directory, now)
				if err != nil {
					logger.Error("automatic database backup failed", "error", err)
					continue
				}
				logger.Info("automatic database backup created", "path", path)
			}
		}
	}()
	return done
}

func backupIfDue(ctx context.Context, repo backupRepository, directory string, interval time.Duration, now time.Time) (string, bool, error) {
	files, err := filepath.Glob(filepath.Join(directory, "kan-auto-*.db"))
	if err != nil {
		return "", false, fmt.Errorf("list automatic backups: %w", err)
	}
	sort.Strings(files)
	if len(files) > 0 {
		latest := files[len(files)-1]
		info, statErr := os.Stat(latest)
		if statErr != nil {
			return "", false, fmt.Errorf("stat automatic backup: %w", statErr)
		}
		if now.Sub(info.ModTime()) < interval {
			return latest, false, nil
		}
	}
	path, err := createAutomaticBackup(ctx, repo, directory, now)
	return path, err == nil, err
}

func createAutomaticBackup(ctx context.Context, repo backupRepository, directory string, now time.Time) (string, error) {
	destination := filepath.Join(directory, fmt.Sprintf("kan-auto-%s.db", now.Format("20060102-150405")))
	if err := repo.Backup(ctx, destination); err != nil {
		return "", err
	}
	return destination, nil
}
