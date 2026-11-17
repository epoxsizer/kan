// Package notify shows desktop notifications through the platform notifier.
// It is a no-op on platforms without a supported notifier.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// command is swapped in tests to observe invocations without notifying.
var command = exec.Command

// Send shows a desktop notification for the current platform.
func Send(title, body string) {
	sendFor(runtime.GOOS, title, body)
}

func sendFor(goos, title, body string) {
	switch goos {
	case "darwin":
		_ = command("osascript", "-e", appleScriptNotification(title, body)).Run()
	case "linux":
		_ = command("notify-send", title, body).Run()
	}
}

// appleScriptNotification builds a display notification script with safely
// quoted strings; newlines are folded so the script stays on one line.
func appleScriptNotification(title, body string) string {
	return fmt.Sprintf("display notification %s with title %s", appleScriptQuote(body), appleScriptQuote(title))
}

func appleScriptQuote(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
