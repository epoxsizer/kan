package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBulkMarkAndMoveCards(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	col0 := columns[0].ID
	cardA := model.cards[col0][0].ID
	cardB := model.cards[col0][1].ID

	model.Update(key(" "))
	require.True(t, model.marked[cardA], "space marks the selected card")
	model.Update(key("j"))
	model.Update(key(" "))
	require.True(t, model.marked[cardB])
	model.Update(key(" "))
	require.False(t, model.marked[cardB], "space toggles the mark off")
	model.Update(key(" "))

	_, command := model.executeCommand("card-move-marked")
	require.NotNil(t, model.movePicker)
	require.True(t, model.batchMove)
	_, command = model.Update(key("enter"))
	runCommands(model, command)

	require.Empty(t, model.marked, "marks clear after the batch move")
	require.Nil(t, model.movePicker)
	titles := columnTitles(t, repo, model.board.ID, columns[1].ID)
	require.Contains(t, titles, "A")
	require.Contains(t, titles, "B")
	require.Equal(t, []string{"C"}, columnTitles(t, repo, model.board.ID, col0))
}

func TestBulkArchiveMarkedCards(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	col0 := columns[0].ID
	model.Update(key(" "))
	model.Update(key("j"))
	model.Update(key(" "))

	_, command := model.executeCommand("card-archive-marked")
	require.NotNil(t, command)
	runCommands(model, command)
	require.Empty(t, model.marked)
	require.Equal(t, []string{"C"}, columnTitles(t, repo, model.board.ID, col0))
	require.Contains(t, model.notice, "2")
}

func TestMarkedCardShowsMarker(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, _, columns := dragBoardModel(t)
	card := model.cards[columns[0].ID][0]
	model.marked = map[string]bool{card.ID: true}
	require.Contains(t, model.cardLabel(card, 40, false), "•")
}

func TestBulkCommandsRequireMarks(t *testing.T) {
	model, _, _ := dragBoardModel(t)
	_, command := model.executeCommand("card-move-marked")
	require.Nil(t, command)
	require.Contains(t, model.notice, "Mark cards")
	_, command = model.executeCommand("card-archive-marked")
	require.Nil(t, command)
}
