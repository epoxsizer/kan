package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func mousePress(x, y int, button tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: button}
}

func TestMouseSelectsThenOpensTableRows(t *testing.T) {
	repo := readRepository{
		projects: []domain.Project{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}},
		boards:   []domain.Board{{ID: "board", ProjectID: "two", Name: "Board"}},
	}
	model := testModel(repo)
	model.loading = false
	model.projects = repo.projects

	model.Update(mousePress(4, 6, tea.MouseButtonLeft))
	require.Equal(t, 1, model.projectIndex)
	require.Equal(t, projectsScreen, model.screen)

	_, command := model.Update(mousePress(4, 6, tea.MouseButtonLeft))
	require.Equal(t, boardsScreen, model.screen)
	require.NotNil(t, command)
}

func TestMouseSelectsAndOpensBoardCards(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "todo", BoardID: "board", Name: "Todo"}}
	model.cards["todo"] = []domain.Card{
		{ID: "one", ColumnID: "todo", Title: "One"},
		{ID: "two", ColumnID: "todo", Title: "Two"},
	}

	model.Update(mousePress(5, 5, tea.MouseButtonLeft))
	require.Equal(t, 1, model.cardIndexes["todo"])
	require.Nil(t, model.detail)

	model.Update(mousePress(5, 5, tea.MouseButtonLeft))
	require.NotNil(t, model.detail)
	require.Equal(t, "Two", model.detail.title)

	model.Update(mousePress(5, 5, tea.MouseButtonRight))
	require.Nil(t, model.detail)
}

func TestMouseWheelNavigatesPointedColumnAndDetails(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}}
	model.cards["two"] = []domain.Card{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}

	model.Update(mousePress(60, 5, tea.MouseButtonWheelDown))
	require.Equal(t, 1, model.columnIndex)
	require.Equal(t, 1, model.cardIndexes["two"])
	model.Update(mousePress(60, 5, tea.MouseButtonWheelLeft))
	require.Zero(t, model.columnIndex)

	model.detail = &detailPopup{kind: "card", title: "Long", lines: []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve"}}
	model.Update(mousePress(20, 10, tea.MouseButtonWheelDown))
	require.Greater(t, model.detail.offset, 0)
}

func TestMouseFocusesFormFieldsAndOpensControls(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}, {ID: "done", Name: "Done"}}
	_ = model.startCardForm(false)

	// At 80x24, the card form starts at row 4 and Status is the third field.
	model.Update(mousePress(20, 11, tea.MouseButtonLeft))
	require.Equal(t, 2, model.form.focus)
	require.Nil(t, model.form.control)
	model.Update(mousePress(20, 11, tea.MouseButtonLeft))
	require.NotNil(t, model.form.control)
	require.Equal(t, dropdownControl, model.form.control.kind)

	model.Update(mousePress(20, 11, tea.MouseButtonRight))
	require.Nil(t, model.form.control)
}

func TestModifiedAndReleaseMouseEventsAreIgnored(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{ID: "one"}, {ID: "two"}}

	model.Update(tea.MouseMsg{X: 2, Y: 6, Shift: true, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	model.Update(tea.MouseMsg{X: 2, Y: 6, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	require.Zero(t, model.projectIndex)
}
