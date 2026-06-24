package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
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
	Storage string   `toml:"storage"`
	S3      S3Backup `toml:"s3"`
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

type fileConfig struct {
	Database     string  `toml:"database"`
	LogFile      string  `toml:"log_file"`
	LogLevel     string  `toml:"log_level"`
	ShowCardTags *bool   `toml:"show_card_tags"`
	Theme        *Theme  `toml:"theme"`
	Backup       *Backup `toml:"backup"`
}

type Overrides struct {
	ConfigFile string
	Database   string
	LogFile    string
}

func Defaults() Config {
	return Config{
		ConfigFile:   filepath.Join(xdg.ConfigHome, "kan", "config.toml"),
		Database:     filepath.Join(xdg.DataHome, "kan", "kan.db"),
		LogFile:      filepath.Join(xdg.StateHome, "kan", "kan.log"),
		LogLevel:     "info",
		ShowCardTags: true,
		Theme: Theme{
			Primary: "#7D7AFF", Muted: "#909090", Text: "#C4C4D0", Background: "#24243A", SelectedForeground: "#FFFFFF", SelectedBackground: "#5A56E0", Danger: "#FF6B6B", Border: "rounded",
			SelectedColumnForeground: "#000000", SelectedColumnBackground: "#42C77A", SelectedColumnBorder: "#42C77A", SelectedCardForeground: "#000000", SelectedCardBackground: "#42C77A",
			PanelBorder: "#909090", FocusedPanelBorder: "#42C77A", StatusForeground: "#909090", StatusBackground: "#24243A", StatusAccentForeground: "#FFFFFF", StatusAccentBackground: "#7D7AFF",
			ShortcutKeyForeground: "#FFFFFF", ShortcutKeyBackground: "#5A56E0", ShortcutText: "#909090", HelpText: "#C4C4D0", HelpBorder: "#7D7AFF", Command: "#7D7AFF", ColumnDefault: "#4C8DFF",
		},
		Backup: Backup{Storage: "local", S3: S3Backup{Prefix: "kan/backups"}},
	}
}

func Load(overrides Overrides) (Config, error) {
	cfg := Defaults()
	configPath := firstNonEmpty(overrides.ConfigFile, os.Getenv(EnvConfig), cfg.ConfigFile)
	cfg.ConfigFile = configPath

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
	} else if overrides.ConfigFile != "" || os.Getenv(EnvConfig) != "" {
		return Config{}, fmt.Errorf("config file %q does not exist", configPath)
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
	return cfg, nil
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
	switch backup.Storage {
	case "", "local":
		return nil
	case "s3":
		return nil
	default:
		return errors.New("backup.storage must be local or s3")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
