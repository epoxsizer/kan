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
}

type Theme struct {
	Primary            string `toml:"primary"`
	Muted              string `toml:"muted"`
	Text               string `toml:"text"`
	Background         string `toml:"background"`
	SelectedForeground string `toml:"selected_foreground"`
	SelectedBackground string `toml:"selected_background"`
	Danger             string `toml:"danger"`
	Border             string `toml:"border"`
}

type fileConfig struct {
	Database     string `toml:"database"`
	LogFile      string `toml:"log_file"`
	LogLevel     string `toml:"log_level"`
	ShowCardTags *bool  `toml:"show_card_tags"`
	Theme        *Theme `toml:"theme"`
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
		Theme:        Theme{Primary: "#7D7AFF", Muted: "#909090", Text: "#C4C4D0", Background: "#24243A", SelectedForeground: "#FFFFFF", SelectedBackground: "#5A56E0", Danger: "#FF6B6B", Border: "rounded"},
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
}

func validateTheme(theme Theme) error {
	for name, value := range map[string]string{"primary": theme.Primary, "muted": theme.Muted, "text": theme.Text, "background": theme.Background, "selected_foreground": theme.SelectedForeground, "selected_background": theme.SelectedBackground, "danger": theme.Danger} {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
