//go:build !windows

package upgrade

import "os"

func replaceCacheFile(source, destination string) error {
	return os.Rename(source, destination)
}
