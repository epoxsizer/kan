package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUndoStackReversesMultipleMoves(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	require.Equal(t, "A", model.selectedCard().Title)

	_, command := model.moveSelectedCard(1)
	runCommands(model, command)
	_, command = model.moveSelectedCard(1)
	runCommands(model, command)
	require.Equal(t, columns[2].ID, model.selectedCard().ColumnID)
	require.Len(t, model.undoStack, 2)

	_, command = model.Update(key("u"))
	runCommands(model, command)
	require.Equal(t, columns[1].ID, model.selectedCard().ColumnID, "first undo reverses the last move")

	_, command = model.Update(key("u"))
	runCommands(model, command)
	require.Equal(t, columns[0].ID, model.selectedCard().ColumnID, "second undo reverses the earlier move")
	require.Empty(t, model.undoStack)

	model.Update(key("u"))
	require.Equal(t, "No card move to undo", model.notice)

	cards, err := repo.ListCards(model.ctx, model.board.ID)
	require.NoError(t, err)
	for _, card := range cards {
		if card.Title == "A" {
			require.Equal(t, columns[0].ID, card.ColumnID, "the database matches the undone state")
		}
	}
}
