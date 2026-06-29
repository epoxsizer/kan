//go:build !windows

package app

import "github.com/mattn/go-shellwords"

func parseEditorCommand(value string) ([]string, error) {
	return shellwords.Parse(value)
}
