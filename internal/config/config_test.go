package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultLoadCreatesLocalConfigAndUsesLocalPaths(t *testing.T) {
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvDB, "")
	t.Setenv(EnvLog, "")
	workingDirectory := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	require.Equal(t, "config.toml", cfg.ConfigFile)
	require.Equal(t, "kan.db", cfg.Database)
	require.Equal(t, "kan.log", cfg.LogFile)
	require.FileExists(t, filepath.Join(workingDirectory, "config.toml"))

	contents, err := os.ReadFile(filepath.Join(workingDirectory, "config.toml"))
	require.NoError(t, err)
	require.Contains(t, string(contents), `database = "kan.db"`)
	require.Contains(t, string(contents), `log_file = "kan.log"`)
	require.Contains(t, string(contents), `[sync]`)
	require.Contains(t, string(contents), `interval = "30m"`)
	require.Contains(t, string(contents), `[theme]`)
}

func TestDatabaseOverrideDoesNotCreateDefaultConfig(t *testing.T) {
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvDB, "")
	t.Setenv(EnvLog, "")
	workingDirectory := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	cfg, err := Load(Overrides{Database: filepath.Join(workingDirectory, "custom.db")})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(workingDirectory, "custom.db"), cfg.Database)
	require.NoFileExists(t, filepath.Join(workingDirectory, "config.toml"))
}

func TestLoadPrecedenceAndStrictKeys(t *testing.T) {
	t.Setenv(EnvDB, "/env/database.db")
	t.Setenv(EnvLog, "/env/kan.log")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("database = '/file/database.db'\nlog_file = '/file/kan.log'\nlog_level = 'debug'\n"), 0o600))

	cfg, err := Load(Overrides{ConfigFile: path, Database: "/flag/database.db"})
	require.NoError(t, err)
	require.Equal(t, "/flag/database.db", cfg.Database)
	require.Equal(t, "/env/kan.log", cfg.LogFile)
	require.Equal(t, "debug", cfg.LogLevel)

	require.NoError(t, os.WriteFile(path, []byte("unknown = true\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "unknown config keys")
}

func TestExplicitMissingConfigFails(t *testing.T) {
	t.Setenv(EnvConfig, "")
	_, err := Load(Overrides{ConfigFile: filepath.Join(t.TempDir(), "missing.toml")})
	require.ErrorContains(t, err, "does not exist")
}

func TestCardTagDisplayDefaultsOnAndCanBeDisabled(t *testing.T) {
	require.True(t, Defaults().ShowCardTags)
	require.Equal(t, "local", Defaults().Backup.Storage)
	require.Equal(t, "kan/backups", Defaults().Backup.S3.Prefix)
	require.False(t, Defaults().Sync.Enabled)
	require.Equal(t, "30m", Defaults().Sync.Interval)
	require.Equal(t, "kan/sync.json", Defaults().Sync.ObjectKey)
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("show_card_tags = false\n"), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.False(t, cfg.ShowCardTags)
}

func TestSyncConfigOverridesAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `[sync]
enabled = true
interval = "45m"
object_key = "team/kan.json"

[sync.s3]
bucket = "kan-sync"
region = "us-east-1"
endpoint = "https://s3.example.test"
access_key_id = "key"
secret_access_key = "secret"
force_path_style = true
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.True(t, cfg.Sync.Enabled)
	require.Equal(t, "45m", cfg.Sync.Interval)
	require.Equal(t, "team/kan.json", cfg.Sync.ObjectKey)
	require.Equal(t, "kan-sync", cfg.Sync.S3.Bucket)
	require.True(t, cfg.Sync.S3.ForcePathStyle)

	require.NoError(t, os.WriteFile(path, []byte("[sync]\ninterval = '30s'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "at least 1m")

	require.NoError(t, os.WriteFile(path, []byte("[sync]\nenabled = true\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "sync.s3.")
}

func TestThemeOverridesAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `[theme]
primary = "#112233"
selected_background = "#445566"
selected_column_background = "#42C77A"
focused_panel_border = "#223344"
border = "double"
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Equal(t, "#112233", cfg.Theme.Primary)
	require.Equal(t, "#445566", cfg.Theme.SelectedBackground)
	require.Equal(t, "#42C77A", cfg.Theme.SelectedColumnBackground)
	require.Equal(t, "#223344", cfg.Theme.FocusedPanelBorder)
	require.Equal(t, "double", cfg.Theme.Border)
	require.Equal(t, Defaults().Theme.Muted, cfg.Theme.Muted)

	require.NoError(t, os.WriteFile(path, []byte("[theme]\nprimary = 'red'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "theme.primary")
}

func TestBackupConfigOverridesAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `[backup]
storage = "s3"
retention = "720h"

[backup.s3]
bucket = "kan-backups"
prefix = "daily"
region = "us-east-1"
endpoint = "https://s3.example.test"
access_key_id = "key"
secret_access_key = "secret"
force_path_style = true
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Equal(t, "s3", cfg.Backup.Storage)
	require.Equal(t, "720h", cfg.Backup.Retention)
	require.Equal(t, "kan-backups", cfg.Backup.S3.Bucket)
	require.Equal(t, "daily", cfg.Backup.S3.Prefix)
	require.True(t, cfg.Backup.S3.ForcePathStyle)

	require.NoError(t, os.WriteFile(path, []byte("[backup]\nstorage = 'ftp'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "backup.storage")
}
