package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recordingBackupRepository struct {
	paths []string
}

func (repo *recordingBackupRepository) Backup(_ context.Context, destination string) error {
	repo.paths = append(repo.paths, destination)
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	return os.WriteFile(destination, []byte("backup"), 0o600)
}

func TestAutomaticBackupRunsAtSixHourCadence(t *testing.T) {
	repo := &recordingBackupRepository{}
	directory := filepath.Join(t.TempDir(), "backup")
	start := time.Date(2026, 6, 22, 6, 0, 0, 0, time.Local)

	first, created, err := backupIfDue(context.Background(), repo, directory, 6*time.Hour, start)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, "kan-auto-20260622-060000.db", filepath.Base(first))
	require.NoError(t, os.Chtimes(first, start, start))

	path, created, err := backupIfDue(context.Background(), repo, directory, 6*time.Hour, start.Add(5*time.Hour+59*time.Minute))
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first, path)
	require.Len(t, repo.paths, 1)

	second, created, err := backupIfDue(context.Background(), repo, directory, 6*time.Hour, start.Add(6*time.Hour))
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, "kan-auto-20260622-120000.db", filepath.Base(second))
	require.Len(t, repo.paths, 2)
}
