package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestMCPCommandServesStdioWithoutHoldingTheDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kan.db")
	configPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, nil, 0o600))
	args := []string{"--config", configPath, "--db", dbPath, "--log", filepath.Join(dir, "kan.log")}
	seedRoot := New("test", "abc123", "today")
	seedRoot.SetOut(io.Discard)
	seedRoot.SetArgs(append(args, "seed"))
	require.NoError(t, seedRoot.Execute())

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	root := New("test", "abc123", "today")
	root.SetIn(stdinReader)
	root.SetOut(stdoutWriter)
	root.SetArgs(append(args, "mcp"))
	done := make(chan error, 1)
	go func() {
		err := root.Execute()
		// Unblock the test's pipe reads and writes if the command exits early.
		stdinReader.Close()
		stdoutWriter.Close()
		done <- err
	}()

	responses := bufio.NewScanner(stdoutReader)
	responses.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	send := func(message string) {
		t.Helper()
		if _, err := io.WriteString(stdinWriter, message+"\n"); err != nil {
			require.FailNow(t, "kan mcp stopped reading stdin", "%v", <-done)
		}
	}
	receive := func() map[string]any {
		t.Helper()
		if !responses.Scan() {
			select {
			case err := <-done:
				require.FailNow(t, "kan mcp exited before responding", "%v", err)
			default:
				require.FailNow(t, "expected a JSON-RPC response")
			}
		}
		var response map[string]any
		require.NoError(t, json.Unmarshal(responses.Bytes(), &response))
		return response
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	initialized := receive()
	require.Equal(t, "kan", initialized["result"].(map[string]any)["serverInfo"].(map[string]any)["name"])
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"kan_list_projects","arguments":{}}}`)
	called := receive()
	result := called["result"].(map[string]any)
	require.NotEqual(t, true, result["isError"], "%v", result)
	projects := result["structuredContent"].(map[string]any)["data"].(map[string]any)["projects"].([]any)
	require.NotEmpty(t, projects, "seeded projects must be visible")

	lock, err := storage.AcquireLock(dbPath)
	require.NoError(t, err, "kan mcp must release the database between tool calls so the TUI can start")
	require.NoError(t, lock.Close())

	require.NoError(t, stdinWriter.Close())
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("kan mcp did not stop after stdin closed")
	}
}

func TestMCPCommandTreatsEndOfInputAsCleanExit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, nil, 0o600))
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"kan_list_projects","arguments":{}}}`,
	}, "\n") + "\n"
	for range 5 {
		root := New("test", "abc123", "today")
		root.SetIn(strings.NewReader(input))
		root.SetOut(io.Discard)
		root.SetArgs([]string{"--config", configPath, "--db", filepath.Join(dir, "kan.db"), "--log", filepath.Join(dir, "kan.log"), "mcp"})
		require.NoError(t, root.Execute())
	}
}
