package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestCardExportCommandPrintsAndWritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kan.db")
	logPath := filepath.Join(dir, "kan.log")
	configPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	ctx := context.Background()
	repo, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Doing", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Ship editor", Description: "Details here", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))
	require.NoError(t, repo.Close())

	root := New("test", "abc123", "today")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "card", "export", card.ID})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), "# Ship editor")
	require.Contains(t, output.String(), "Status: Doing")
	require.Contains(t, output.String(), "Details here")

	outPath := filepath.Join(dir, "card.md")
	root = New("test", "abc123", "today")
	output.Reset()
	root.SetOut(&output)
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "card", "export", card.ID, "--out", outPath})
	require.NoError(t, root.Execute())
	require.FileExists(t, outPath)
	contents, err := os.ReadFile(outPath)
	require.NoError(t, err)
	require.Contains(t, string(contents), "# Ship editor")
	require.Contains(t, output.String(), outPath)

	root = New("test", "abc123", "today")
	root.SetOut(&output)
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "card", "export", "missing"})
	require.Error(t, root.Execute())
}
