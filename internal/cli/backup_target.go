package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
)

type backupRepository interface {
	Backup(context.Context, string) error
}

type s3BackupUploader interface {
	Upload(context.Context, config.S3Backup, string, string) error
}

type s3BackupRotator interface {
	Rotate(context.Context, config.S3Backup, time.Duration, time.Time) (int, error)
}

type backupResult struct {
	localPath     string
	localRelative string
	s3URI         string
}

func createConfiguredBackup(ctx context.Context, repo backupRepository, cfg config.Backup, workingDirectory, name string, now time.Time, uploader s3BackupUploader) (backupResult, error) {
	destination := filepath.Join(
		storage.BackupDirectory(workingDirectory),
		fmt.Sprintf("%s-%s.db", name, now.Format("20060102-150405")),
	)
	if err := repo.Backup(ctx, destination); err != nil {
		return backupResult{}, err
	}
	relative, relativeErr := filepath.Rel(workingDirectory, destination)
	if relativeErr != nil {
		relative = destination
	}
	result := backupResult{localPath: destination, localRelative: relative}
	if cfg.Storage != "s3" {
		return result, nil
	}
	key := s3ObjectKey(cfg.S3.Prefix, filepath.Base(destination))
	if uploader == nil {
		uploader = realS3Uploader{}
	}
	if err := uploader.Upload(ctx, cfg.S3, destination, key); err != nil {
		return result, fmt.Errorf("upload backup to s3: %w", err)
	}
	result.s3URI = "s3://" + cfg.S3.Bucket + "/" + key
	if rotator, ok := uploader.(s3BackupRotator); ok {
		retention, err := backupRetention(cfg)
		if err != nil {
			return result, err
		}
		if _, err = rotator.Rotate(ctx, cfg.S3, retention, now); err != nil {
			return result, fmt.Errorf("rotate s3 backups: %w", err)
		}
	}
	return result, nil
}

func backupConfigWithOverrides(base config.Backup, flags backupFlags) config.Backup {
	if flags.storage != "" {
		base.Storage = flags.storage
	}
	if flags.s3Bucket != "" {
		base.S3.Bucket = flags.s3Bucket
	}
	if flags.s3Prefix != "" {
		base.S3.Prefix = flags.s3Prefix
	}
	if flags.s3Region != "" {
		base.S3.Region = flags.s3Region
	}
	if flags.s3Endpoint != "" {
		base.S3.Endpoint = flags.s3Endpoint
	}
	if flags.s3AccessKeyID != "" {
		base.S3.AccessKeyID = flags.s3AccessKeyID
	}
	if flags.s3SecretAccessKey != "" {
		base.S3.SecretAccessKey = flags.s3SecretAccessKey
	}
	if flags.s3ForcePathStyle {
		base.S3.ForcePathStyle = true
	}
	return base
}

func s3ObjectKey(prefix, name string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func ensureBackupConfig(value config.Backup) error {
	switch value.Storage {
	case "", "local":
		return nil
	case "s3":
		if value.S3.Bucket == "" {
			return fmt.Errorf("backup.s3.bucket is required when backup storage is s3")
		}
		if value.S3.Region == "" {
			return fmt.Errorf("backup.s3.region is required when backup storage is s3")
		}
		if value.S3.AccessKeyID == "" {
			return fmt.Errorf("backup.s3.access_key_id is required when backup storage is s3")
		}
		if value.S3.SecretAccessKey == "" {
			return fmt.Errorf("backup.s3.secret_access_key is required when backup storage is s3")
		}
		return nil
	default:
		return fmt.Errorf("backup storage must be local or s3")
	}
}
