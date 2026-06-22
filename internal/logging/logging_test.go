package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenWritesOnlyToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "kan.log")
	logger, closer, err := Open(path, "debug")
	require.NoError(t, err)
	logger.Info("opened", "database", "test.db")
	require.NoError(t, closer.Close())
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(contents), "msg=opened")
	require.Contains(t, string(contents), "database=test.db")
}
