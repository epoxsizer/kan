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
	binaryDirectory := t.TempDir()
	previousExecutablePath := executablePath
	executablePath = func() (string, error) {
		return filepath.Join(binaryDirectory, "kan"), nil
	}
	t.Cleanup(func() { executablePath = previousExecutablePath })
	workingDirectory := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	cfg, err := Load(Overrides{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(binaryDirectory, "config.toml"), cfg.ConfigFile)
	require.Equal(t, "kan.db", cfg.Database)
	require.Equal(t, "kan.log", cfg.LogFile)
	require.FileExists(t, cfg.ConfigFile)
	require.NoFileExists(t, filepath.Join(workingDirectory, "config.toml"))

	contents, err := os.ReadFile(cfg.ConfigFile)
	require.NoError(t, err)
	require.Contains(t, string(contents), `database = "kan.db"`)
	require.Contains(t, string(contents), `log_file = "kan.log"`)
	require.Contains(t, string(contents), `show_selected_card_details = false`)
	require.NotContains(t, string(contents), `[backup]`)
	require.NotContains(t, string(contents), `storage =`)
	require.NotContains(t, string(contents), `[backup.s3]`)
	require.NotContains(t, string(contents), `[sync]`)
	require.NotContains(t, string(contents), `[sync.s3]`)
	require.Contains(t, string(contents), `[theme]`)
	require.Contains(t, string(contents), `selected_card_background = "#4C8DFF"`)
	require.Contains(t, string(contents), `focused_panel_border = "#4C8DFF"`)
}

func TestDatabaseOverrideStillCreatesDefaultConfigBesideBinary(t *testing.T) {
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvDB, "")
	t.Setenv(EnvLog, "")
	binaryDirectory := t.TempDir()
	previousExecutablePath := executablePath
	executablePath = func() (string, error) {
		return filepath.Join(binaryDirectory, "kan"), nil
	}
	t.Cleanup(func() { executablePath = previousExecutablePath })
	workingDirectory := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	cfg, err := Load(Overrides{Database: filepath.Join(workingDirectory, "custom.db")})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(workingDirectory, "custom.db"), cfg.Database)
	require.FileExists(t, filepath.Join(binaryDirectory, "config.toml"))
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
	defaults := Defaults()
	require.True(t, defaults.ShowCardTags)
	require.False(t, defaults.ShowSelectedCardDetails)
	require.Equal(t, "#4C8DFF", defaults.Theme.SelectedBackground)
	require.Equal(t, "#4C8DFF", defaults.Theme.SelectedColumnBackground)
	require.Equal(t, "#4C8DFF", defaults.Theme.SelectedColumnBorder)
	require.Equal(t, "#4C8DFF", defaults.Theme.SelectedCardBackground)
	require.Equal(t, "#4C8DFF", defaults.Theme.FocusedPanelBorder)
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("show_card_tags = false\n"), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.False(t, cfg.ShowCardTags)

	require.NoError(t, os.WriteFile(path, []byte("show_selected_card_details = true\n"), 0o600))
	cfg, err = Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.True(t, cfg.ShowSelectedCardDetails)
}

func TestLegacySyncConfigCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `[sync]
enabled = false
interval = "15m"
object_key = "kan/sync.json"

[sync.s3]
bucket = "kan-sync"
region = "us-east-1"
endpoint = "https://s3.example.test"
access_key_id = "key"
secret_access_key = "secret"
force_path_style = true
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	_, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("[sync]\nenabled = true\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "S3 sync has been removed")
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

func TestLegacyBackupConfigCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `[backup]
storage = "local"
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
	_, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("[backup]\nstorage = 's3'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "S3 backup storage has been removed")

	require.NoError(t, os.WriteFile(path, []byte("[backup]\nstorage = 'ftp'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "backup.storage is no longer supported")
}
