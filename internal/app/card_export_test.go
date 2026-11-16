package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCardExportCommandWritesMarkdownFile(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "todo", BoardID: "board", Name: "Todo"}}
	model.cards["todo"] = []domain.Card{{
		ID: "card-1", ColumnID: "todo", Title: "Ship editor", Description: "Details",
		Checklist: []domain.ChecklistItem{{ID: "i", Text: "Task", Position: 1024}},
	}}

	_, command := model.executeCommand("card-export")
	require.NotNil(t, command)
	finished, ok := command().(cardExportFinishedMsg)
	require.True(t, ok)
	require.NoError(t, finished.err)
	require.FileExists(t, finished.path)
	contents, err := os.ReadFile(finished.path)
	require.NoError(t, err)
	require.Contains(t, string(contents), "# Ship editor")
	require.Contains(t, string(contents), "Status: Todo")
	require.Contains(t, string(contents), "- [ ] Task")
	require.Contains(t, filepath.Base(finished.path), "ship-editor")

	model.Update(finished)
	require.Contains(t, model.notice, "Exported")
	require.Contains(t, model.notice, finished.path)
}

func TestCardExportRequiresSelectedCard(t *testing.T) {
	model := testModel(readRepository{})
	model.screen = projectsScreen
	_, command := model.executeCommand("card-export")
	require.Nil(t, command)
	require.Error(t, model.err)
}

func TestCardMenuOffersExport(t *testing.T) {
	model := testModel(readRepository{})
	model.commandMode = true
	model.commandMenu = "card"
	model.command = "export"
	matches := model.paletteMatches()
	require.NotEmpty(t, matches)
	require.Equal(t, "card-export", matches[0].command)
}
