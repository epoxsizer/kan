package app

import (
	"context"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
	storagesqlite "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func dragBoardModel(t *testing.T) (*Model, *storagesqlite.Repository, []domain.Column) {
	t.Helper()
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "drag.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	columns := []domain.Column{}
	for index, name := range []string{"Todo", "Doing", "Done"} {
		column := domain.Column{BoardID: board.ID, Name: name, Position: float64(index+1) * 1024}
		require.NoError(t, repo.CreateColumn(ctx, &column))
		columns = append(columns, column)
	}
	for index, title := range []string{"A", "B", "C"} {
		card := domain.Card{BoardID: board.ID, ColumnID: columns[0].ID, Title: title, Position: float64(index+1) * 1024}
		require.NoError(t, repo.CreateCard(ctx, &card))
	}
	doing := domain.Card{BoardID: board.ID, ColumnID: columns[1].ID, Title: "D", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &doing))

	model := testModel(repo)
	model.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	model.screen = boardScreen
	model.project = &project
	model.board = &board
	model.loading = true
	runCommands(model, loadBoard(ctx, repo, board.ID))
	require.Len(t, model.columns, 3)
	return model, repo, columns
}

// cardPoint finds a screen cell inside the card, using the board's own hit
// testing so the test follows the real layout.
func cardPoint(t *testing.T, model *Model, columnIndex, cardIndex int) (int, int) {
	t.Helper()
	width, height := model.dimensions()
	for x := range width {
		column, ok := model.mouseColumnAt(x)
		if !ok || column != columnIndex {
			continue
		}
		for y := range height {
			if index, found := model.mouseCardAt(columnIndex, y); found && index == cardIndex {
				return x + 3, y
			}
		}
	}
	require.FailNow(t, "card not on screen", "column %d card %d", columnIndex, cardIndex)
	return 0, 0
}

func columnX(t *testing.T, model *Model, columnIndex int) int {
	t.Helper()
	width, _ := model.dimensions()
	for x := range width {
		if column, ok := model.mouseColumnAt(x); ok && column == columnIndex {
			return x + 3
		}
	}
	require.FailNow(t, "column not on screen", "%d", columnIndex)
	return 0
}

func mouseMotion(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}
}

func mouseRelease(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}
}

func columnTitles(t *testing.T, repo *storagesqlite.Repository, boardID, columnID string) []string {
	t.Helper()
	cards, err := repo.ListCards(context.Background(), boardID)
	require.NoError(t, err)
	titles := []string{}
	for _, card := range cards {
		if card.ColumnID == columnID {
			titles = append(titles, card.Title)
		}
	}
	return titles
}

func TestDragCardToAnotherColumnMovesItAndCanBeUndone(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	require.NotNil(t, model.detail, "pressing the selected card still opens its detail")

	targetX := columnX(t, model, 1)
	_, height := model.dimensions()
	model.Update(mouseMotion(targetX, height-4))
	require.Contains(t, model.notice, "Doing")
	require.Equal(t, 1, model.columnIndex, "the target column is highlighted while dragging")
	require.Nil(t, model.detail, "dragging replaces the detail opened by the press")

	_, command := model.Update(mouseRelease(targetX, height-4))
	runCommands(model, command)
	require.Equal(t, []string{"B", "C"}, columnTitles(t, repo, model.board.ID, columns[0].ID))
	require.Equal(t, []string{"D", "A"}, columnTitles(t, repo, model.board.ID, columns[1].ID))
	require.Equal(t, "Card moved", model.notice)
	require.Equal(t, "A", model.selectedCard().Title)

	runCommands(model, func() tea.Cmd { _, next := model.Update(key("u")); return next }())
	require.Equal(t, []string{"A", "B", "C"}, columnTitles(t, repo, model.board.ID, columns[0].ID))
}

func TestDragCardOntoAnotherCardInsertsBeforeOrAfterIt(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	targetX, targetY := cardPoint(t, model, 1, 0)
	model.Update(mouseMotion(targetX, targetY))
	_, command := model.Update(mouseRelease(targetX, targetY))
	runCommands(model, command)
	require.Equal(t, []string{"A", "D"}, columnTitles(t, repo, model.board.ID, columns[1].ID), "dropping on a one-line card inserts before it")

	x, y = cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	targetX, targetY = cardPoint(t, model, 0, 1)
	model.Update(mouseMotion(targetX, targetY+1))
	_, command = model.Update(mouseRelease(targetX, targetY+1))
	runCommands(model, command)
	require.Equal(t, []string{"C", "B"}, columnTitles(t, repo, model.board.ID, columns[0].ID), "dropping below the last card reorders within the column")
}

func TestEscCancelsDragWithoutMoving(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	x, y := cardPoint(t, model, 0, 1)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	model.Update(mouseMotion(columnX(t, model, 2), y))
	require.Equal(t, 2, model.columnIndex)

	model.Update(key("esc"))
	require.Equal(t, 0, model.columnIndex)
	require.Equal(t, "Drag cancelled", model.notice)
	_, command := model.Update(mouseRelease(columnX(t, model, 2), y))
	require.Nil(t, command)
	require.Equal(t, []string{"A", "B", "C"}, columnTitles(t, repo, model.board.ID, columns[0].ID))
}

func TestClickWithoutMotionOnlySelects(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	x, y := cardPoint(t, model, 0, 2)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	_, command := model.Update(mouseRelease(x, y))
	require.Nil(t, command)
	require.Equal(t, 2, model.cardIndexes[columns[0].ID])
	require.Nil(t, model.detail)
	require.Equal(t, []string{"A", "B", "C"}, columnTitles(t, repo, model.board.ID, columns[0].ID))
}

func TestDragIsDisabledInDerivedBoardViews(t *testing.T) {
	model, repo, columns := dragBoardModel(t)
	model.sortMode = sortTitle
	x, y := cardPoint(t, model, 0, 0)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	model.Update(mouseMotion(columnX(t, model, 1), y))
	_, command := model.Update(mouseRelease(columnX(t, model, 1), y))
	require.Nil(t, command)
	require.Contains(t, model.notice, "Clear filter")
	require.Equal(t, []string{"A", "B", "C"}, columnTitles(t, repo, model.board.ID, columns[0].ID))
}
