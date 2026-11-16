package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

// doubleClickBoardModel builds a board with two cards and a controllable
// clock so double-click timing is deterministic.
func doubleClickBoardModel(t *testing.T) (*Model, *time.Time) {
	t.Helper()
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.columns = []domain.Column{{ID: "todo", BoardID: "board", Name: "Todo"}}
	model.cards["todo"] = []domain.Card{
		{ID: "one", ColumnID: "todo", Title: "One"},
		{ID: "two", ColumnID: "todo", Title: "Two"},
	}
	clock := time.Now()
	model.now = func() time.Time { return clock }
	return model, &clock
}

func TestDoubleClickOpensCardEditor(t *testing.T) {
	model, clock := doubleClickBoardModel(t)

	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	require.Equal(t, 1, model.cardIndexes["todo"])
	require.Nil(t, model.form)

	*clock = clock.Add(100 * time.Millisecond)
	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	require.NotNil(t, model.form, "a quick second press opens the editor")
	require.Equal(t, editCardForm, model.form.kind)
	require.Equal(t, "two", model.form.entityID)
	require.Nil(t, model.drag, "the pending drag must not survive the double-click")
	require.Nil(t, model.detail)
}

func TestDoubleClickOnSelectedCardOpensEditor(t *testing.T) {
	model, clock := doubleClickBoardModel(t)
	model.cardIndexes["todo"] = 1

	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	require.NotNil(t, model.detail, "the first press on the selected card opens its detail")

	*clock = clock.Add(100 * time.Millisecond)
	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	require.NotNil(t, model.form, "the quick second press converts the detail into the editor")
	require.Equal(t, editCardForm, model.form.kind)
	require.Equal(t, "two", model.form.entityID)
	require.Nil(t, model.detail)
}

func TestSlowSecondClickStillOpensDetail(t *testing.T) {
	model, clock := doubleClickBoardModel(t)

	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	*clock = clock.Add(doubleClickWindow + time.Millisecond)
	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	require.NotNil(t, model.detail)
	require.Equal(t, "Two", model.detail.title)
	require.Nil(t, model.form)
}

func TestDragBetweenPressesPreventsDoubleClick(t *testing.T) {
	model, clock := doubleClickBoardModel(t)

	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	model.Update(mouseMotion(5, 3))
	require.NotNil(t, model.drag)
	model.Update(key("esc"))
	require.Nil(t, model.drag)

	*clock = clock.Add(50 * time.Millisecond)
	model.Update(mousePress(5, 4, tea.MouseButtonLeft))
	require.Nil(t, model.form, "a drag between presses breaks the double-click")
	require.NotNil(t, model.detail)
}
