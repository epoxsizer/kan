package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	EnvConfig   = "KAN_CONFIG"
	EnvDB       = "KAN_DB"
	EnvLog      = "KAN_LOG"
	EnvMCPToken = "KAN_MCP_TOKEN"
)

type Config struct {
	ConfigFile   string `toml:"-"`
	Database     string `toml:"database"`
	LogFile      string `toml:"log_file"`
	LogLevel     string `toml:"log_level"`
	ShowCardTags bool   `toml:"show_card_tags"`

	ShowSelectedCardDetails bool `toml:"show_selected_card_details"`

	Theme    Theme    `toml:"theme"`
	MCP      MCP      `toml:"mcp"`
	Planning Planning `toml:"planning"`
}

type MCP struct {
	Enabled bool   `toml:"enabled"`
	Address string `toml:"address"`
	Token   string `toml:"token"`
}

type Planning struct {
	StaleAfterDays           int      `toml:"stale_after_days"`
	BlockedTags              []string `toml:"blocked_tags"`
	UntriagedWithoutPriority bool     `toml:"untriaged_without_priority"`
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

type legacyS3Backup struct {
	Bucket          string `toml:"bucket"`
	Prefix          string `toml:"prefix"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	ForcePathStyle  bool   `toml:"force_path_style"`
}

type fileBackup struct {
	Retention string         `toml:"retention"`
	Storage   string         `toml:"storage"`
	S3        legacyS3Backup `toml:"s3"`
}

type legacySync struct {
	Enabled   bool         `toml:"enabled"`
	Interval  string       `toml:"interval"`
	ObjectKey string       `toml:"object_key"`
	S3        legacyS3Sync `toml:"s3"`
}

type legacyS3Sync struct {
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	ForcePathStyle  bool   `toml:"force_path_style"`
}

type filePlanning struct {
	StaleAfterDays           *int     `toml:"stale_after_days"`
	BlockedTags              []string `toml:"blocked_tags"`
	UntriagedWithoutPriority *bool    `toml:"untriaged_without_priority"`
}

type fileConfig struct {
	Database     string `toml:"database"`
	LogFile      string `toml:"log_file"`
	LogLevel     string `toml:"log_level"`
	ShowCardTags *bool  `toml:"show_card_tags"`

	ShowSelectedCardDetails *bool `toml:"show_selected_card_details"`

	Theme    *Theme        `toml:"theme"`
	MCP      *MCP          `toml:"mcp"`
	Planning *filePlanning `toml:"planning"`
	Backup   *fileBackup   `toml:"backup"`
	Sync     *legacySync   `toml:"sync"`
}

type Overrides struct {
	ConfigFile string
	Database   string
	LogFile    string
}

var executablePath = os.Executable

func Defaults() Config {
	return Config{
		ConfigFile:   "config.toml",
		Database:     "kan.db",
		LogFile:      "kan.log",
		LogLevel:     "info",
		ShowCardTags: true,

		ShowSelectedCardDetails: false,
		Theme: Theme{
			Primary: "#7D7AFF", Muted: "#909090", Text: "#C4C4D0", Background: "#24243A", SelectedForeground: "#000000", SelectedBackground: "#4C8DFF", Danger: "#FF6B6B", Border: "rounded",
			SelectedColumnForeground: "#000000", SelectedColumnBackground: "#4C8DFF", SelectedColumnBorder: "#4C8DFF", SelectedCardForeground: "#000000", SelectedCardBackground: "#4C8DFF",
			PanelBorder: "#909090", FocusedPanelBorder: "#4C8DFF", StatusForeground: "#909090", StatusBackground: "#24243A", StatusAccentForeground: "#FFFFFF", StatusAccentBackground: "#7D7AFF",
			ShortcutKeyForeground: "#FFFFFF", ShortcutKeyBackground: "#5A56E0", ShortcutText: "#909090", HelpText: "#C4C4D0", HelpBorder: "#7D7AFF", Command: "#7D7AFF", ColumnDefault: "#4C8DFF",
		},
		MCP:      MCP{Address: "127.0.0.1:7337"},
		Planning: Planning{StaleAfterDays: 7, BlockedTags: []string{"blocked", "blocker"}, UntriagedWithoutPriority: true},
	}
}

func Load(overrides Overrides) (Config, error) {
	cfg := Defaults()
	explicitConfig := overrides.ConfigFile != "" || os.Getenv(EnvConfig) != ""
	configPath := firstNonEmpty(overrides.ConfigFile, os.Getenv(EnvConfig))
	if !explicitConfig {
		var err error
		configPath, err = defaultConfigPath()
		if err != nil {
			return Config{}, err
		}
	}
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
		if fileCfg.Backup != nil {
			switch strings.ToLower(strings.TrimSpace(fileCfg.Backup.Storage)) {
			case "", "local":
			case "s3":
				return Config{}, errors.New("S3 backup storage has been removed; backups are local-only")
			default:
				return Config{}, errors.New("backup.storage is no longer supported; backups are local-only")
			}
		}
		if fileCfg.Sync != nil && fileCfg.Sync.Enabled {
			return Config{}, errors.New("S3 sync has been removed")
		}
		merge(&cfg, fileCfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("stat config %q: %w", configPath, err)
	} else if explicitConfig {
		return Config{}, fmt.Errorf("config file %q does not exist", configPath)
	} else {
		if err = WriteDefaultFile(configPath, cfg); err != nil {
			return Config{}, err
		}
	}

	cfg.Database = firstNonEmpty(overrides.Database, os.Getenv(EnvDB), cfg.Database)
	cfg.LogFile = firstNonEmpty(overrides.LogFile, os.Getenv(EnvLog), cfg.LogFile)
	cfg.MCP.Token = firstNonEmpty(os.Getenv(EnvMCPToken), cfg.MCP.Token)
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Database == "" || cfg.LogFile == "" {
		return Config{}, errors.New("database and log paths must not be empty")
	}
	if err := validateTheme(cfg.Theme); err != nil {
		return Config{}, err
	}
	if err := validateMCP(cfg.MCP); err != nil {
		return Config{}, err
	}
	if err := validatePlanning(cfg.Planning); err != nil {
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
show_selected_card_details = %t

[mcp]
enabled = %t
address = %q
token = %q

[planning]
stale_after_days = %d
blocked_tags = [%s]
untriaged_without_priority = %t

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
		cfg.ShowSelectedCardDetails,
		cfg.MCP.Enabled,
		cfg.MCP.Address,
		cfg.MCP.Token,
		cfg.Planning.StaleAfterDays,
		quotedStringList(cfg.Planning.BlockedTags),
		cfg.Planning.UntriagedWithoutPriority,
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
	if src.ShowSelectedCardDetails != nil {
		dst.ShowSelectedCardDetails = *src.ShowSelectedCardDetails
	}
	if src.Theme != nil {
		mergeTheme(&dst.Theme, *src.Theme)
	}
	if src.MCP != nil {
		dst.MCP.Enabled = src.MCP.Enabled
		if src.MCP.Address != "" {
			dst.MCP.Address = src.MCP.Address
		}
		if src.MCP.Token != "" {
			dst.MCP.Token = src.MCP.Token
		}
	}
	if src.Planning != nil {
		if src.Planning.StaleAfterDays != nil {
			dst.Planning.StaleAfterDays = *src.Planning.StaleAfterDays
		}
		if src.Planning.BlockedTags != nil {
			dst.Planning.BlockedTags = append([]string(nil), src.Planning.BlockedTags...)
		}
		if src.Planning.UntriagedWithoutPriority != nil {
			dst.Planning.UntriagedWithoutPriority = *src.Planning.UntriagedWithoutPriority
		}
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

func validateMCP(value MCP) error {
	if !value.Enabled {
		return nil
	}
	host, portText, err := net.SplitHostPort(value.Address)
	if err != nil {
		return fmt.Errorf("mcp.address must use host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("mcp.address must use a literal loopback IP address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("mcp.address must use a port between 1 and 65535")
	}
	if len(strings.TrimSpace(value.Token)) < 32 {
		return errors.New("mcp.token or KAN_MCP_TOKEN must contain at least 32 characters when MCP is enabled")
	}
	return nil
}

func validatePlanning(value Planning) error {
	if value.StaleAfterDays < 1 {
		return errors.New("planning.stale_after_days must be positive")
	}
	for _, tag := range value.BlockedTags {
		if strings.TrimSpace(tag) == "" {
			return errors.New("planning.blocked_tags must not contain empty values")
		}
	}
	return nil
}

func defaultConfigPath() (string, error) {
	executable, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("resolve executable path for default config: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), "config.toml"), nil
}

func quotedStringList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
