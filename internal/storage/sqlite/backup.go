package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/google/uuid"
)

func (repo *Repository) Backup(ctx context.Context, destination string) error {
	if err := config.EnsureParent(destination); err != nil {
		return err
	}
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("%w: backup %q already exists", domain.ErrConflict, destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat backup destination: %w", err)
	}

	temporary := destination + ".tmp-" + uuid.NewString()
	defer os.Remove(temporary)
	if _, err := repo.db.ExecContext(ctx, "VACUUM INTO ?", temporary); err != nil {
		return fmt.Errorf("create SQLite backup: %w", mapError(err))
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		return fmt.Errorf("set backup permissions: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}
	return nil
}

func BackupDirectory(workingDirectory string) string {
	return filepath.Join(workingDirectory, "backup")
}
