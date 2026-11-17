package app

import (
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBackForwardNavigation(t *testing.T) {
	model, _, _ := dragBoardModel(t)
	model.projects = []domain.Project{*model.project}
	model.boards = []domain.Board{*model.board}
	boardID := model.board.ID
	require.Equal(t, boardScreen, model.screen)

	model.Update(key("esc"))
	require.Equal(t, boardsScreen, model.screen)

	_, command := model.Update(key("ctrl+o"))
	runCommands(model, command)
	require.Equal(t, boardScreen, model.screen)
	require.Equal(t, boardID, model.board.ID, "ctrl+o returns to the previous board")

	_, command = model.Update(key("ctrl+i"))
	runCommands(model, command)
	require.Equal(t, boardsScreen, model.screen, "ctrl+i moves forward again")

	model.Update(key("esc"))
	require.Equal(t, projectsScreen, model.screen)
	_, command = model.Update(key("ctrl+i"))
	require.Nil(t, command, "a new transition drops the forward tail")
	require.Equal(t, projectsScreen, model.screen)
}
