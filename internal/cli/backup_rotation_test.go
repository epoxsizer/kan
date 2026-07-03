package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackupRotationRemovesOnlyExpiredGeneratedBackups(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.Local)
	old := now.Add(-15 * 24 * time.Hour)
	recent := now.Add(-13 * 24 * time.Hour)
	for _, name := range []string{"kan-auto-20260616-120000.db", "release-20260616-120000.db", "kan-pre-import-20260616-120000.db"} {
		path := filepath.Join(directory, name)
		require.NoError(t, os.WriteFile(path, []byte("backup"), 0o600))
		require.NoError(t, os.Chtimes(path, old, old))
	}
	recentPath := filepath.Join(directory, "kan-auto-20260618-120000.db")
	require.NoError(t, os.WriteFile(recentPath, []byte("backup"), 0o600))
	require.NoError(t, os.Chtimes(recentPath, recent, recent))
	unrelated := filepath.Join(directory, "notes.db")
	require.NoError(t, os.WriteFile(unrelated, []byte("keep"), 0o600))
	require.NoError(t, os.Chtimes(unrelated, old, old))

	removed, err := rotateBackups(directory, now)
	require.NoError(t, err)
	require.Equal(t, 3, removed)
	require.FileExists(t, recentPath)
	require.FileExists(t, unrelated)
}
