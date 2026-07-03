package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
)

type backupRepository interface {
	Backup(context.Context, string) error
}

type backupResult struct {
	localPath     string
	localRelative string
}

func createLocalBackup(ctx context.Context, repo backupRepository, workingDirectory, name string, now time.Time) (backupResult, error) {
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
	return backupResult{localPath: destination, localRelative: relative}, nil
}
