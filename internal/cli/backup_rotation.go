package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const defaultBackupRetention = 14 * 24 * time.Hour

var timestampedBackupPattern = regexp.MustCompile(`^.+-\d{8}-\d{6}\.db$`)

func rotateBackups(directory string, now time.Time) (int, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read backup directory: %w", err)
	}
	cutoff := now.Add(-defaultBackupRetention)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !timestampedBackupPattern.MatchString(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return removed, fmt.Errorf("stat backup %q: %w", entry.Name(), infoErr)
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if removeErr := os.Remove(filepath.Join(directory, entry.Name())); removeErr != nil {
			return removed, fmt.Errorf("remove expired backup %q: %w", entry.Name(), removeErr)
		}
		removed++
	}
	return removed, nil
}
