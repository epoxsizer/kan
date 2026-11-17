package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func addTestDatabase(t *testing.T) (string, string, domain.Board, domain.Column, domain.Column) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kan.db")
	logPath := filepath.Join(dir, "kan.log")
	ctx := context.Background()
	repo, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Inbox", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	other := domain.Column{BoardID: board.ID, Name: "Later", Position: 2048}
	require.NoError(t, repo.CreateColumn(ctx, &other))
	require.NoError(t, repo.Close())
	return dbPath, logPath, board, column, other
}

func TestAddCommandCapturesToInbox(t *testing.T) {
	dbPath, logPath, board, column, other := addTestDatabase(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[inbox]\nboard = \""+board.ID+"\"\ncolumn = \""+column.ID+"\"\n"), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	root := New("test", "abc123", "today")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "add", "Buy milk", "--tags", "home, errands", "--priority", "low"})
	require.NoError(t, root.Execute())
	var card domain.Card
	require.NoError(t, json.Unmarshal(output.Bytes(), &card))
	require.Equal(t, "Buy milk", card.Title)
	require.Equal(t, column.ID, card.ColumnID)
	require.Equal(t, []string{"home", "errands"}, card.Tags)
	require.Equal(t, "low", *card.Priority)

	output.Reset()
	root = New("test", "abc123", "today")
	root.SetOut(&output)
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "add", "Other", "--board", board.ID, "--column", other.ID})
	require.NoError(t, root.Execute())
	card = domain.Card{}
	require.NoError(t, json.Unmarshal(output.Bytes(), &card))
	require.Equal(t, other.ID, card.ColumnID)
}

func TestAddCommandRequiresInboxOrFlags(t *testing.T) {
	dbPath, logPath, _, _, _ := addTestDatabase(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	root := New("test", "abc123", "today")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "add", "No home"})
	require.ErrorContains(t, root.Execute(), "inbox")
}
