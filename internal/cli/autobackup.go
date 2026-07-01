package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/epoxsizer/kan/internal/config"
)

const automaticBackupInterval = 6 * time.Hour

func startAutomaticBackups(ctx context.Context, repo backupRepository, logger *slog.Logger, directory string, backupConfig config.Backup) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if path, created, err := backupIfDue(ctx, repo, directory, backupConfig, automaticBackupInterval, time.Now()); err != nil {
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
				path, err := createAutomaticBackup(ctx, repo, directory, backupConfig, now)
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

func backupIfDue(ctx context.Context, repo backupRepository, directory string, backupConfig config.Backup, interval time.Duration, now time.Time) (string, bool, error) {
	if _, err := rotateBackups(directory, backupConfig, now); err != nil {
		return "", false, err
	}
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
	path, err := createAutomaticBackup(ctx, repo, directory, backupConfig, now)
	return path, err == nil, err
}

func createAutomaticBackup(ctx context.Context, repo backupRepository, directory string, backupConfig config.Backup, now time.Time) (string, error) {
	destination := filepath.Join(directory, fmt.Sprintf("kan-auto-%s.db", now.Format("20060102-150405")))
	if err := repo.Backup(ctx, destination); err != nil {
		return "", err
	}
	if backupConfig.Storage == "s3" {
		if err := ensureBackupConfig(backupConfig); err != nil {
			return destination, err
		}
		key := s3ObjectKey(backupConfig.S3.Prefix, filepath.Base(destination))
		uploader := realS3Uploader{}
		if err := uploader.Upload(ctx, backupConfig.S3, destination, key); err != nil {
			return destination, fmt.Errorf("upload automatic backup to s3: %w", err)
		}
		retention, err := backupRetention(backupConfig)
		if err != nil {
			return destination, err
		}
		if _, err = uploader.Rotate(ctx, backupConfig.S3, retention, now); err != nil {
			return destination, fmt.Errorf("rotate s3 backups: %w", err)
		}
	}
	if _, err := rotateBackups(directory, backupConfig, now); err != nil {
		return destination, err
	}
	return destination, nil
}
