package upgrade

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
)

const CheckInterval = 24 * time.Hour

type cacheRecord struct {
	CheckedAt  time.Time `json:"checked_at"`
	Current    string    `json:"current"`
	Latest     string    `json:"latest"`
	ReleaseURL string    `json:"release_url,omitempty"`
}

func DefaultCachePath() (string, error) {
	directory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(directory, "kan", "version-check.json"), nil
}

func ReadFreshCache(path, current string, now time.Time) (Result, bool, error) {
	currentVersion, err := parseCurrentVersion(current)
	if err != nil {
		return Result{}, false, err
	}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, fmt.Errorf("read update cache: %w", err)
	}
	var record cacheRecord
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&record); err != nil {
		return Result{}, false, fmt.Errorf("decode update cache: %w", err)
	}
	if err = ensureCacheEOF(decoder); err != nil {
		return Result{}, false, err
	}
	if record.CheckedAt.IsZero() || now.Sub(record.CheckedAt) < 0 || now.Sub(record.CheckedAt) >= CheckInterval || record.Current != currentVersion.String() {
		return Result{}, false, nil
	}
	latestVersion, err := semver.NewVersion(record.Latest)
	if err != nil {
		return Result{}, false, fmt.Errorf("decode cached latest version: %w", err)
	}
	return Result{
		Current: currentVersion.String(), Latest: latestVersion.String(),
		Available: latestVersion.GreaterThan(currentVersion), ReleaseURL: record.ReleaseURL,
	}, true, nil
}

func ensureCacheEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("decode update cache: multiple JSON documents are not allowed")
		}
		return fmt.Errorf("decode update cache: %w", err)
	}
	return nil
}

func WriteCache(path string, result Result, now time.Time) error {
	record := cacheRecord{
		CheckedAt: now.UTC(), Current: result.Current, Latest: result.Latest, ReleaseURL: result.ReleaseURL,
	}
	contents, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode update cache: %w", err)
	}
	contents = append(contents, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create update cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".version-check-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary update cache: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(contents)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close update cache: %w", closeErr)
	}
	if err = replaceCacheFile(temporaryPath, path); err != nil {
		return fmt.Errorf("publish update cache: %w", err)
	}
	return nil
}
