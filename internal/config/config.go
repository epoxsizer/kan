package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	EnvConfig = "KAN_CONFIG"
	EnvDB     = "KAN_DB"
	EnvLog    = "KAN_LOG"
)

type Config struct {
	ConfigFile   string `toml:"-"`
	Database     string `toml:"database"`
	LogFile      string `toml:"log_file"`
	LogLevel     string `toml:"log_level"`
	ShowCardTags bool   `toml:"show_card_tags"`
	Theme        Theme  `toml:"theme"`
	Backup       Backup `toml:"backup"`
	Sync         Sync   `toml:"sync"`
}

type Theme struct {
	Primary                  string `toml:"primary"`
	Muted                    string `toml:"muted"`
	Text                     string `toml:"text"`
	Background               string `toml:"background"`
	SelectedForeground       string `toml:"selected_foreground"`
	SelectedBackground       string `toml:"selected_background"`
	Danger                   string `toml:"danger"`
	Border                   string `toml:"border"`
	SelectedColumnForeground string `toml:"selected_column_foreground"`
	SelectedColumnBackground string `toml:"selected_column_background"`
	SelectedColumnBorder     string `toml:"selected_column_border"`
	SelectedCardForeground   string `toml:"selected_card_foreground"`
	SelectedCardBackground   string `toml:"selected_card_background"`
	PanelBorder              string `toml:"panel_border"`
	FocusedPanelBorder       string `toml:"focused_panel_border"`
	StatusForeground         string `toml:"status_foreground"`
	StatusBackground         string `toml:"status_background"`
	StatusAccentForeground   string `toml:"status_accent_foreground"`
	StatusAccentBackground   string `toml:"status_accent_background"`
	ShortcutKeyForeground    string `toml:"shortcut_key_foreground"`
	ShortcutKeyBackground    string `toml:"shortcut_key_background"`
	ShortcutText             string `toml:"shortcut_text"`
	HelpText                 string `toml:"help_text"`
	HelpBorder               string `toml:"help_border"`
	Command                  string `toml:"command"`
	ColumnDefault            string `toml:"column_default"`
}

type Backup struct {
	Storage   string   `toml:"storage"`
	Retention string   `toml:"retention"`
	S3        S3Backup `toml:"s3"`
}

type S3Backup struct {
	Bucket          string `toml:"bucket"`
	Prefix          string `toml:"prefix"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	ForcePathStyle  bool   `toml:"force_path_style"`
}

type Sync struct {
	Enabled   bool   `toml:"enabled"`
	Interval  string `toml:"interval"`
	ObjectKey string `toml:"object_key"`
	S3        S3Sync `toml:"s3"`
}

type S3Sync struct {
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	ForcePathStyle  bool   `toml:"force_path_style"`
}

type fileConfig struct {
	Database     string  `toml:"database"`
	LogFile      string  `toml:"log_file"`
	LogLevel     string  `toml:"log_level"`
	ShowCardTags *bool   `toml:"show_card_tags"`
	Theme        *Theme  `toml:"theme"`
	Backup       *Backup `toml:"backup"`
	Sync         *Sync   `toml:"sync"`
}

type Overrides struct {
	ConfigFile string
	Database   string
	LogFile    string
}

func Defaults() Config {
	return Config{
		ConfigFile:   "config.toml",
		Database:     "kan.db",
		LogFile:      "kan.log",
		LogLevel:     "info",
		ShowCardTags: true,
		Theme: Theme{
			Primary: "#7D7AFF", Muted: "#909090", Text: "#C4C4D0", Background: "#24243A", SelectedForeground: "#FFFFFF", SelectedBackground: "#5A56E0", Danger: "#FF6B6B", Border: "rounded",
			SelectedColumnForeground: "#000000", SelectedColumnBackground: "#42C77A", SelectedColumnBorder: "#42C77A", SelectedCardForeground: "#000000", SelectedCardBackground: "#42C77A",
			PanelBorder: "#909090", FocusedPanelBorder: "#42C77A", StatusForeground: "#909090", StatusBackground: "#24243A", StatusAccentForeground: "#FFFFFF", StatusAccentBackground: "#7D7AFF",
			ShortcutKeyForeground: "#FFFFFF", ShortcutKeyBackground: "#5A56E0", ShortcutText: "#909090", HelpText: "#C4C4D0", HelpBorder: "#7D7AFF", Command: "#7D7AFF", ColumnDefault: "#4C8DFF",
		},
		Backup: Backup{Storage: "local", Retention: "336h", S3: S3Backup{Prefix: "kan/backups"}},
		Sync:   Sync{Interval: "30m", ObjectKey: "kan/sync.json"},
	}
}

func Load(overrides Overrides) (Config, error) {
	cfg := Defaults()
	configPath := firstNonEmpty(overrides.ConfigFile, os.Getenv(EnvConfig), cfg.ConfigFile)
	cfg.ConfigFile = configPath
	explicitConfig := overrides.ConfigFile != "" || os.Getenv(EnvConfig) != ""
	defaultDatabase := overrides.Database == "" && os.Getenv(EnvDB) == ""

	if _, err := os.Stat(configPath); err == nil {
		var fileCfg fileConfig
		metadata, decodeErr := toml.DecodeFile(configPath, &fileCfg)
		if decodeErr != nil {
			return Config{}, fmt.Errorf("decode config %q: %w", configPath, decodeErr)
		}
		if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
			keys := make([]string, 0, len(undecoded))
			for _, key := range undecoded {
				keys = append(keys, key.String())
			}
			return Config{}, fmt.Errorf("unknown config keys: %s", strings.Join(keys, ", "))
		}
		merge(&cfg, fileCfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("stat config %q: %w", configPath, err)
	} else if explicitConfig {
		return Config{}, fmt.Errorf("config file %q does not exist", configPath)
	} else if defaultDatabase {
		if err = WriteDefaultFile(configPath, cfg); err != nil {
			return Config{}, err
		}
	} else {
		// When automation passes --db or KAN_DB, keep startup side-effect free.
		// Local first-run config is only created for the no-database-configured path.
	}

	cfg.Database = firstNonEmpty(overrides.Database, os.Getenv(EnvDB), cfg.Database)
	cfg.LogFile = firstNonEmpty(overrides.LogFile, os.Getenv(EnvLog), cfg.LogFile)
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Database == "" || cfg.LogFile == "" {
		return Config{}, errors.New("database and log paths must not be empty")
	}
	if err := validateTheme(cfg.Theme); err != nil {
		return Config{}, err
	}
	if err := validateBackup(cfg.Backup); err != nil {
		return Config{}, err
	}
	if err := validateSync(cfg.Sync); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func WriteDefaultFile(path string, cfg Config) error {
	if err := EnsureParent(path); err != nil {
		return err
	}
	contents := fmt.Sprintf(`database = %q
log_file = %q
log_level = %q
show_card_tags = %t

[backup]
storage = %q
retention = %q

[backup.s3]
prefix = %q

[sync]
enabled = %t
interval = %q
object_key = %q

[theme]
primary = %q
muted = %q
text = %q
background = %q
selected_foreground = %q
selected_background = %q
danger = %q
border = %q
selected_column_foreground = %q
selected_column_background = %q
selected_column_border = %q
selected_card_foreground = %q
selected_card_background = %q
panel_border = %q
focused_panel_border = %q
status_foreground = %q
status_background = %q
status_accent_foreground = %q
status_accent_background = %q
shortcut_key_foreground = %q
shortcut_key_background = %q
shortcut_text = %q
help_text = %q
help_border = %q
command = %q
column_default = %q
`,
		cfg.Database,
		cfg.LogFile,
		cfg.LogLevel,
		cfg.ShowCardTags,
		cfg.Backup.Storage,
		cfg.Backup.Retention,
		cfg.Backup.S3.Prefix,
		cfg.Sync.Enabled,
		cfg.Sync.Interval,
		cfg.Sync.ObjectKey,
		cfg.Theme.Primary,
		cfg.Theme.Muted,
		cfg.Theme.Text,
		cfg.Theme.Background,
		cfg.Theme.SelectedForeground,
		cfg.Theme.SelectedBackground,
		cfg.Theme.Danger,
		cfg.Theme.Border,
		cfg.Theme.SelectedColumnForeground,
		cfg.Theme.SelectedColumnBackground,
		cfg.Theme.SelectedColumnBorder,
		cfg.Theme.SelectedCardForeground,
		cfg.Theme.SelectedCardBackground,
		cfg.Theme.PanelBorder,
		cfg.Theme.FocusedPanelBorder,
		cfg.Theme.StatusForeground,
		cfg.Theme.StatusBackground,
		cfg.Theme.StatusAccentForeground,
		cfg.Theme.StatusAccentBackground,
		cfg.Theme.ShortcutKeyForeground,
		cfg.Theme.ShortcutKeyBackground,
		cfg.Theme.ShortcutText,
		cfg.Theme.HelpText,
		cfg.Theme.HelpBorder,
		cfg.Theme.Command,
		cfg.Theme.ColumnDefault,
	)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("write default config %q: %w", path, err)
	}
	return nil
}

func EnsureParent(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create directory for %q: %w", path, err)
	}
	return nil
}

func merge(dst *Config, src fileConfig) {
	if src.Database != "" {
		dst.Database = src.Database
	}
	if src.LogFile != "" {
		dst.LogFile = src.LogFile
	}
	if src.LogLevel != "" {
		dst.LogLevel = src.LogLevel
	}
	if src.ShowCardTags != nil {
		dst.ShowCardTags = *src.ShowCardTags
	}
	if src.Theme != nil {
		mergeTheme(&dst.Theme, *src.Theme)
	}
	if src.Backup != nil {
		mergeBackup(&dst.Backup, *src.Backup)
	}
	if src.Sync != nil {
		mergeSync(&dst.Sync, *src.Sync)
	}
}

func mergeTheme(dst *Theme, src Theme) {
	if src.Primary != "" {
		dst.Primary = src.Primary
	}
	if src.Muted != "" {
		dst.Muted = src.Muted
	}
	if src.Text != "" {
		dst.Text = src.Text
	}
	if src.Background != "" {
		dst.Background = src.Background
	}
	if src.SelectedForeground != "" {
		dst.SelectedForeground = src.SelectedForeground
	}
	if src.SelectedBackground != "" {
		dst.SelectedBackground = src.SelectedBackground
	}
	if src.Danger != "" {
		dst.Danger = src.Danger
	}
	if src.Border != "" {
		dst.Border = src.Border
	}
	if src.SelectedColumnForeground != "" {
		dst.SelectedColumnForeground = src.SelectedColumnForeground
	}
	if src.SelectedColumnBackground != "" {
		dst.SelectedColumnBackground = src.SelectedColumnBackground
	}
	if src.SelectedColumnBorder != "" {
		dst.SelectedColumnBorder = src.SelectedColumnBorder
	}
	if src.SelectedCardForeground != "" {
		dst.SelectedCardForeground = src.SelectedCardForeground
	}
	if src.SelectedCardBackground != "" {
		dst.SelectedCardBackground = src.SelectedCardBackground
	}
	if src.PanelBorder != "" {
		dst.PanelBorder = src.PanelBorder
	}
	if src.FocusedPanelBorder != "" {
		dst.FocusedPanelBorder = src.FocusedPanelBorder
	}
	if src.StatusForeground != "" {
		dst.StatusForeground = src.StatusForeground
	}
	if src.StatusBackground != "" {
		dst.StatusBackground = src.StatusBackground
	}
	if src.StatusAccentForeground != "" {
		dst.StatusAccentForeground = src.StatusAccentForeground
	}
	if src.StatusAccentBackground != "" {
		dst.StatusAccentBackground = src.StatusAccentBackground
	}
	if src.ShortcutKeyForeground != "" {
		dst.ShortcutKeyForeground = src.ShortcutKeyForeground
	}
	if src.ShortcutKeyBackground != "" {
		dst.ShortcutKeyBackground = src.ShortcutKeyBackground
	}
	if src.ShortcutText != "" {
		dst.ShortcutText = src.ShortcutText
	}
	if src.HelpText != "" {
		dst.HelpText = src.HelpText
	}
	if src.HelpBorder != "" {
		dst.HelpBorder = src.HelpBorder
	}
	if src.Command != "" {
		dst.Command = src.Command
	}
	if src.ColumnDefault != "" {
		dst.ColumnDefault = src.ColumnDefault
	}
}

func validateTheme(theme Theme) error {
	for name, value := range map[string]string{
		"primary": theme.Primary, "muted": theme.Muted, "text": theme.Text, "background": theme.Background, "selected_foreground": theme.SelectedForeground, "selected_background": theme.SelectedBackground, "danger": theme.Danger,
		"selected_column_foreground": theme.SelectedColumnForeground, "selected_column_background": theme.SelectedColumnBackground, "selected_column_border": theme.SelectedColumnBorder,
		"selected_card_foreground": theme.SelectedCardForeground, "selected_card_background": theme.SelectedCardBackground, "panel_border": theme.PanelBorder, "focused_panel_border": theme.FocusedPanelBorder,
		"status_foreground": theme.StatusForeground, "status_background": theme.StatusBackground, "status_accent_foreground": theme.StatusAccentForeground, "status_accent_background": theme.StatusAccentBackground,
		"shortcut_key_foreground": theme.ShortcutKeyForeground, "shortcut_key_background": theme.ShortcutKeyBackground, "shortcut_text": theme.ShortcutText, "help_text": theme.HelpText, "help_border": theme.HelpBorder,
		"command": theme.Command, "column_default": theme.ColumnDefault,
	} {
		if len(value) != 7 || value[0] != '#' {
			return fmt.Errorf("theme.%s must be a #RRGGBB color", name)
		}
		for _, character := range value[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
				return fmt.Errorf("theme.%s must be a #RRGGBB color", name)
			}
		}
	}
	switch theme.Border {
	case "rounded", "normal", "thick", "double":
		return nil
	default:
		return fmt.Errorf("theme.border must be rounded, normal, thick, or double")
	}
}

func mergeBackup(dst *Backup, src Backup) {
	if src.Storage != "" {
		dst.Storage = src.Storage
	}
	if src.Retention != "" {
		dst.Retention = src.Retention
	}
	if src.S3.Bucket != "" {
		dst.S3.Bucket = src.S3.Bucket
	}
	if src.S3.Prefix != "" {
		dst.S3.Prefix = src.S3.Prefix
	}
	if src.S3.Region != "" {
		dst.S3.Region = src.S3.Region
	}
	if src.S3.Endpoint != "" {
		dst.S3.Endpoint = src.S3.Endpoint
	}
	if src.S3.AccessKeyID != "" {
		dst.S3.AccessKeyID = src.S3.AccessKeyID
	}
	if src.S3.SecretAccessKey != "" {
		dst.S3.SecretAccessKey = src.S3.SecretAccessKey
	}
	if src.S3.ForcePathStyle {
		dst.S3.ForcePathStyle = true
	}
}

func validateBackup(backup Backup) error {
	retention, err := time.ParseDuration(backup.Retention)
	if err != nil {
		return fmt.Errorf("backup.retention must be a valid duration: %w", err)
	}
	if retention < 0 {
		return errors.New("backup.retention must not be negative")
	}
	switch backup.Storage {
	case "", "local":
		return nil
	case "s3":
		return nil
	default:
		return errors.New("backup.storage must be local or s3")
	}
}

func mergeSync(dst *Sync, src Sync) {
	dst.Enabled = src.Enabled
	if src.Interval != "" {
		dst.Interval = src.Interval
	}
	if src.ObjectKey != "" {
		dst.ObjectKey = src.ObjectKey
	}
	if src.S3.Bucket != "" {
		dst.S3.Bucket = src.S3.Bucket
	}
	if src.S3.Region != "" {
		dst.S3.Region = src.S3.Region
	}
	if src.S3.Endpoint != "" {
		dst.S3.Endpoint = src.S3.Endpoint
	}
	if src.S3.AccessKeyID != "" {
		dst.S3.AccessKeyID = src.S3.AccessKeyID
	}
	if src.S3.SecretAccessKey != "" {
		dst.S3.SecretAccessKey = src.S3.SecretAccessKey
	}
	if src.S3.ForcePathStyle {
		dst.S3.ForcePathStyle = true
	}
}

func validateSync(syncConfig Sync) error {
	interval, err := time.ParseDuration(syncConfig.Interval)
	if err != nil {
		return fmt.Errorf("sync.interval must be a valid duration: %w", err)
	}
	if interval < time.Minute {
		return errors.New("sync.interval must be at least 1m")
	}
	if strings.TrimSpace(syncConfig.ObjectKey) == "" {
		return errors.New("sync.object_key must not be empty")
	}
	if !syncConfig.Enabled {
		return nil
	}
	for name, value := range map[string]string{
		"bucket": syncConfig.S3.Bucket, "region": syncConfig.S3.Region,
		"access_key_id": syncConfig.S3.AccessKeyID, "secret_access_key": syncConfig.S3.SecretAccessKey,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("sync.s3.%s must not be empty when sync is enabled", name)
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
