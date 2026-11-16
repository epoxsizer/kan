package sqlite

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// DatabaseIdentity identifies a canonical database path without publishing the
// user's filesystem layout over MCP. Symlinks resolve to the same identity.
func DatabaseIdentity(path string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalDatabasePath(path))))
}

func canonicalDatabasePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	} else if parent, err := filepath.EvalSymlinks(filepath.Dir(absolute)); err == nil {
		absolute = filepath.Join(parent, filepath.Base(absolute))
	}
	return absolute
}

func (repo *Repository) DatabaseIdentity() string { return repo.identity }
