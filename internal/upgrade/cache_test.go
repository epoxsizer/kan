package upgrade

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVersionCheckCacheFreshnessAndCurrentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "version-check.json")
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	result := Result{Current: "1.0.0", Latest: "1.1.0", Available: true, ReleaseURL: "https://example.test/v1.1.0"}
	require.NoError(t, WriteCache(path, result, now))

	cached, fresh, err := ReadFreshCache(path, "v1.0.0", now.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, fresh)
	require.Equal(t, result, cached)

	_, fresh, err = ReadFreshCache(path, "1.0.0", now.Add(CheckInterval))
	require.NoError(t, err)
	require.False(t, fresh)

	_, fresh, err = ReadFreshCache(path, "1.0.1", now.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, fresh)
}

func TestVersionCheckCacheHandlesMissingAndInvalidFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-check.json")
	_, fresh, err := ReadFreshCache(path, "1.0.0", time.Now())
	require.NoError(t, err)
	require.False(t, fresh)

	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0o600))
	_, _, err = ReadFreshCache(path, "1.0.0", time.Now())
	require.ErrorContains(t, err, "decode update cache")
}

func TestVersionCheckCacheFileIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-check.json")
	require.NoError(t, WriteCache(path, Result{Current: "1.0.0", Latest: "1.0.0"}, time.Now()))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
