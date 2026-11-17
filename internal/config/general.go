package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// GeneralSettings are the top-level behavior switches editable from the TUI
// settings menu.
type GeneralSettings struct {
	ShowCardTags            bool
	ShowSelectedCardDetails bool
	LegacyKeymap            bool
	Mouse                   bool
	WrapCardTitles          bool
}

// SaveGeneral updates only the general keys in a config file, preserving
// sections, comments, and every other setting. Missing keys are inserted at
// the end of the top-level block, before the first section.
func SaveGeneral(path string, values GeneralSettings) error {
	contents, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	updates := []struct {
		key   string
		value bool
	}{
		{"show_card_tags", values.ShowCardTags},
		{"show_selected_card_details", values.ShowSelectedCardDetails},
		{"legacy_keymap", values.LegacyKeymap},
		{"mouse", values.Mouse},
		{"wrap_card_titles", values.WrapCardTitles},
	}
	lines := strings.Split(string(contents), "\n")
	sectionStart := len(lines)
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			sectionStart = index
			break
		}
	}
	seen := make(map[string]bool, len(updates))
	for index := 0; index < sectionStart; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		separator := strings.Index(trimmed, "=")
		if separator <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:separator])
		for _, update := range updates {
			if update.key == key {
				lines[index] = fmt.Sprintf("%s = %t", key, update.value)
				seen[key] = true
			}
		}
	}
	missing := []string{}
	for _, update := range updates {
		if !seen[update.key] {
			missing = append(missing, fmt.Sprintf("%s = %t", update.key, update.value))
		}
	}
	if len(missing) > 0 {
		if sectionStart < len(lines) {
			missing = append(missing, "")
		}
		merged := append([]string{}, lines[:sectionStart]...)
		merged = append(merged, missing...)
		lines = append(merged, lines[sectionStart:]...)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}
