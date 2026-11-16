package sqlite

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDatabaseAliasesShareIdentityAndProcessLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires optional Windows privileges")
	}
	root := t.TempDir()
	path := filepath.Join(root, "kan.db")
	alias := filepath.Join(root, "alias.db")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	require.NoError(t, os.Symlink(path, alias))
	require.Equal(t, DatabaseIdentity(path), DatabaseIdentity(alias))
	first, err := AcquireLock(path)
	require.NoError(t, err)
	defer first.Close()
	second, err := AcquireLock(alias)
	if second != nil {
		second.Close()
	}
	require.ErrorIs(t, err, domain.ErrLocked)
}
