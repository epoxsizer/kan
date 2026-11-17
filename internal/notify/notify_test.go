package notify

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendUsesPlatformNotifier(t *testing.T) {
	type call struct {
		name string
		args []string
	}
	var calls []call
	previous := command
	command = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: args})
		// Return a command that succeeds without running anything.
		return exec.Command("true")
	}
	t.Cleanup(func() { command = previous })

	sendFor("darwin", "Kan: agent finished", "Card \"Fix login\" is done")
	require.Len(t, calls, 1)
	require.Equal(t, "osascript", calls[0].name)
	script := calls[0].args[1]
	require.Contains(t, script, `display notification "Card \"Fix login\" is done"`)
	require.Contains(t, script, `with title "Kan: agent finished"`)

	calls = nil
	sendFor("linux", "Kan: new card", "Buy milk")
	require.Len(t, calls, 1)
	require.Equal(t, "notify-send", calls[0].name)
	require.Equal(t, []string{"Kan: new card", "Buy milk"}, calls[0].args)

	calls = nil
	sendFor("windows", "Kan", "ignored")
	require.Empty(t, calls, "unsupported platforms are a no-op")
}

func TestAppleScriptEscaping(t *testing.T) {
	script := appleScriptNotification(`a"b\c`, "line1\nline2")
	require.False(t, strings.Contains(script, "\n"), "newlines must not break the script")
	require.Contains(t, script, `a\"b\\c`)
}
