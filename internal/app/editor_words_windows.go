//go:build windows

package app

import "golang.org/x/sys/windows"

func parseEditorCommand(value string) ([]string, error) {
	return windows.DecomposeCommandLine(value)
}
