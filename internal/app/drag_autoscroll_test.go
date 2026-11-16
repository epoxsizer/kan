package app

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

// overflowDragBoard adds enough cards to column 0 that the viewport must
// scroll to reach them.
func overflowDragBoard(t *testing.T) (*Model, []domain.Column) {
	t.Helper()
	model, repo, columns := dragBoardModel(t)
	ctx := context.Background()
	for index := range 15 {
		card := domain.Card{BoardID: model.board.ID, ColumnID: columns[0].ID, Title: fmt.Sprintf("Extra %02d", index), Position: float64(index+4) * 1024}
		require.NoError(t, repo.CreateCard(ctx, &card))
	}
	model.loading = true
	runCommands(model, loadBoard(ctx, repo, model.board.ID))
	return model, columns
}

func TestDragAutoScrollsAtBottomEdge(t *testing.T) {
	model, columns := overflowDragBoard(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, height := model.dimensions()

	_, command := model.Update(mouseMotion(x, height-4))
	require.NotNil(t, command, "holding the drag at the bottom edge schedules more scrolling")
	require.Equal(t, 1, model.cardIndexes[columns[0].ID])
	require.NotNil(t, model.drag)
}

func TestDragAutoScrollContinuesWhilePointerStaysAtEdge(t *testing.T) {
	model, columns := overflowDragBoard(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, height := model.dimensions()
	model.Update(mouseMotion(x, height-4))

	_, command := model.Update(dragAutoScrollMsg{})
	require.NotNil(t, command)
	require.Equal(t, 2, model.cardIndexes[columns[0].ID])

	model.Update(mouseMotion(x, 8))
	_, command = model.Update(dragAutoScrollMsg{})
	require.Nil(t, command, "scrolling stops once the pointer leaves the edge")
	require.Equal(t, 2, model.cardIndexes[columns[0].ID])
}

func TestDragAutoScrollsUpAtTopEdge(t *testing.T) {
	model, columns := overflowDragBoard(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, height := model.dimensions()
	model.Update(mouseMotion(x, height-4))
	// Scroll down until cards above the viewport are hidden.
	for range 15 {
		model.Update(dragAutoScrollMsg{})
	}
	require.Equal(t, 16, model.cardIndexes[columns[0].ID])

	_, command := model.Update(mouseMotion(x, 3))
	require.NotNil(t, command)
	require.Equal(t, 15, model.cardIndexes[columns[0].ID])
}

func TestDragDoesNotScrollWhenAllCardsFit(t *testing.T) {
	model, _, columns := dragBoardModel(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, height := model.dimensions()

	_, command := model.Update(mouseMotion(x, height-4))
	require.Nil(t, command, "no scrolling when nothing is hidden below")
	require.Zero(t, model.cardIndexes[columns[0].ID])
}

func TestDragAutoScrollStopsAtLastCard(t *testing.T) {
	model, columns := overflowDragBoard(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, height := model.dimensions()
	model.Update(mouseMotion(x, height-4))

	cards := model.visibleCards(columns[0].ID)
	var command tea.Cmd
	for range len(cards) + 2 {
		_, command = model.Update(dragAutoScrollMsg{})
		if command == nil {
			break
		}
	}
	require.Nil(t, command)
	require.Equal(t, len(cards)-1, model.cardIndexes[columns[0].ID])
}

func TestDragAutoScrollStopsAfterCancel(t *testing.T) {
	model, _ := overflowDragBoard(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, height := model.dimensions()
	model.Update(mouseMotion(x, height-4))
	model.Update(key("esc"))

	_, command := model.Update(dragAutoScrollMsg{})
	require.Nil(t, command)
}
