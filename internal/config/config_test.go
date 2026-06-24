package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("show_card_tags = false\n"), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.False(t, cfg.ShowCardTags)
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
	require.Equal(t, "kan-backups", cfg.Backup.S3.Bucket)
	require.Equal(t, "daily", cfg.Backup.S3.Prefix)
	require.True(t, cfg.Backup.S3.ForcePathStyle)

	require.NoError(t, os.WriteFile(path, []byte("[backup]\nstorage = 'ftp'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "backup.storage")
}
