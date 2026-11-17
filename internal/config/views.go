package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// SaveViews replaces the [[views]] blocks in a config file, preserving every
// other section and comment. An empty slice removes all saved views.
func SaveViews(path string, views []View) error {
	contents, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	lines := strings.Split(string(contents), "\n")
	kept := []string{}
	inViews := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inViews = trimmed == "[[views]]"
		}
		if !inViews {
			kept = append(kept, line)
		}
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	if len(views) > 0 {
		kept = append(kept, "")
		for _, view := range views {
			kept = append(kept, "[[views]]", fmt.Sprintf("name = %q", view.Name))
			if view.Filter != "" {
				kept = append(kept, fmt.Sprintf("filter = %q", view.Filter))
			}
			if view.Sort != "" {
				kept = append(kept, fmt.Sprintf("sort = %q", view.Sort))
			}
			if view.Group != "" {
				kept = append(kept, fmt.Sprintf("group = %q", view.Group))
			}
			if view.Planning != "" {
				kept = append(kept, fmt.Sprintf("planning = %q", view.Planning))
			}
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}
