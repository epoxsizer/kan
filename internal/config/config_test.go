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
	require.Contains(t, string(contents), `[mcp]`)
	require.Contains(t, string(contents), `address = "127.0.0.1:7337"`)
	require.Contains(t, string(contents), `[planning]`)
	require.Contains(t, string(contents), `stale_after_days = 7`)
	require.Contains(t, string(contents), `blocked_tags = ["blocked", "blocker"]`)
	require.Contains(t, string(contents), `untriaged_without_priority = true`)
	require.NotContains(t, string(contents), `[backup]`)
	require.NotContains(t, string(contents), `storage =`)
	require.NotContains(t, string(contents), `[backup.s3]`)
	require.Contains(t, string(contents), `[sync]`)
	require.Contains(t, string(contents), `enabled = false`)
	require.Contains(t, string(contents), `interval = "10m"`)
	require.Contains(t, string(contents), `[sync.s3]`)
	require.Contains(t, string(contents), `[theme]`)
	require.Contains(t, string(contents), `selected_card_background = "#4C8DFF"`)
	require.Contains(t, string(contents), `focused_panel_border = "#4C8DFF"`)
}

func TestPlanningConfiguration(t *testing.T) {
	defaults := Defaults()
	require.Equal(t, 7, defaults.Planning.StaleAfterDays)
	require.Equal(t, []string{"blocked", "blocker"}, defaults.Planning.BlockedTags)
	require.True(t, defaults.Planning.UntriagedWithoutPriority)

	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("[planning]\nstale_after_days = 14\nblocked_tags = ['waiting']\nuntriaged_without_priority = false\n"), 0o600))
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Equal(t, 14, cfg.Planning.StaleAfterDays)
	require.Equal(t, []string{"waiting"}, cfg.Planning.BlockedTags)
	require.False(t, cfg.Planning.UntriagedWithoutPriority)

	require.NoError(t, os.WriteFile(path, []byte("[planning]\nstale_after_days = 3\n"), 0o600))
	cfg, err = Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Equal(t, 3, cfg.Planning.StaleAfterDays)
	require.Equal(t, []string{"blocked", "blocker"}, cfg.Planning.BlockedTags)
	require.True(t, cfg.Planning.UntriagedWithoutPriority)

	require.NoError(t, os.WriteFile(path, []byte("[planning]\nstale_after_days = 0\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "planning.stale_after_days")
}

func TestMCPConfigurationAndTokenOverride(t *testing.T) {
	t.Setenv(EnvMCPToken, "")
	path := filepath.Join(t.TempDir(), "config.toml")
	token := "12345678901234567890123456789012"
	require.NoError(t, os.WriteFile(path, []byte("[mcp]\nenabled = true\naddress = '127.0.0.1:7447'\ntoken = '"+token+"'\n"), 0o600))

	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.True(t, cfg.MCP.Enabled)
	require.Equal(t, "127.0.0.1:7447", cfg.MCP.Address)
	require.Equal(t, token, cfg.MCP.Token)

	override := "abcdefghijklmnopqrstuvwxyzABCDEF"
	t.Setenv(EnvMCPToken, override)
	cfg, err = Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Equal(t, override, cfg.MCP.Token)
}

func TestMCPConfigurationRejectsUnsafeSettings(t *testing.T) {
	t.Setenv(EnvMCPToken, "")
	path := filepath.Join(t.TempDir(), "config.toml")
	for name, contents := range map[string]string{
		"short token":     "[mcp]\nenabled=true\naddress='127.0.0.1:7337'\ntoken='short'\n",
		"non-loopback":    "[mcp]\nenabled=true\naddress='0.0.0.0:7337'\ntoken='12345678901234567890123456789012'\n",
		"hostname":        "[mcp]\nenabled=true\naddress='localhost:7337'\ntoken='12345678901234567890123456789012'\n",
		"invalid port":    "[mcp]\nenabled=true\naddress='127.0.0.1:0'\ntoken='12345678901234567890123456789012'\n",
		"missing address": "[mcp]\nenabled=true\naddress='bad'\ntoken='12345678901234567890123456789012'\n",
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
			_, err := Load(Overrides{ConfigFile: path})
			require.Error(t, err)
		})
	}

	require.NoError(t, os.WriteFile(path, []byte("[mcp]\nenabled=true\naddress='[::1]:7337'\ntoken='12345678901234567890123456789012'\n"), 0o600))
	_, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
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

func TestSyncConfiguration(t *testing.T) {
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
	cfg, err := Load(Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.False(t, cfg.Sync.Enabled)
	require.Equal(t, "15m", cfg.Sync.Interval)
	require.Equal(t, "kan-sync", cfg.Sync.S3.Bucket)
	require.True(t, cfg.Sync.S3.ForcePathStyle)

	require.NoError(t, os.WriteFile(path, []byte("[sync]\nenabled = true\ninterval = '30s'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "sync.interval must be at least 1m")

	require.NoError(t, os.WriteFile(path, []byte("[sync]\nenabled = true\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "sync.s3.")

	require.NoError(t, os.WriteFile(path, []byte("[sync]\nenabled = true\n\n[sync.s3]\nbucket = 'bucket'\nregion = 'region'\naccess_key_id = 'key'\nsecret_access_key = 'secret'\nendpoint = 'not-a-url'\n"), 0o600))
	_, err = Load(Overrides{ConfigFile: path})
	require.ErrorContains(t, err, "sync.s3.endpoint")
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
