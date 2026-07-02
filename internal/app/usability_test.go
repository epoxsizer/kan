package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRelativeDueAndBoardHealthLabels(t *testing.T) {
	now := time.Date(2026, time.June, 27, 14, 0, 0, 0, time.Local)
	date := func(day int) *time.Time {
		value := time.Date(2026, time.June, day, 8, 0, 0, 0, time.Local)
		return &value
	}

	require.Equal(t, "overdue 2d", relativeDueLabel(date(25), now))
	require.Equal(t, "due today", relativeDueLabel(date(27), now))
	require.Equal(t, "due tomorrow", relativeDueLabel(date(28), now))
	require.Equal(t, "due in 7d", relativeDueLabel(timePtr(time.Date(2026, time.July, 4, 8, 0, 0, 0, time.Local)), now))
	require.Equal(t, "due 2026-07-05", relativeDueLabel(timePtr(time.Date(2026, time.July, 5, 8, 0, 0, 0, time.Local)), now))

	health := summarizeBoardHealth([]domain.Card{
		{ID: "late-one", DueDate: date(24)},
		{ID: "late-two", DueDate: date(26)},
		{ID: "next", DueDate: date(28)},
	}, now)
	require.Equal(t, 2, health.overdueCount)
	require.Equal(t, "2 overdue", boardHealthLabel(health, now))

	health = summarizeBoardHealth([]domain.Card{{ID: "next", DueDate: date(28)}}, now)
	require.Equal(t, "due tomorrow", boardHealthLabel(health, now))
	require.Equal(t, "no due dates", boardHealthLabel(summarizeBoardHealth(nil, now), now))
}

func TestSelectedCardExpandsWithinTerminal(t *testing.T) {
	priority := "High"
	due := time.Now().AddDate(0, 0, 1)
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board", Name: "Delivery"}
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "Doing"}}
	model.cards["doing"] = []domain.Card{{
		ID:             "card",
		ColumnID:       "doing",
		Title:          "Review keyboard shortcuts",
		Description:    "Make important actions discoverable without opening another popup.",
		Priority:       &priority,
		DueDate:        &due,
		Tags:           []string{"ux", "release"},
		RelatedCardIDs: []string{"other"},
		Checklist:      []domain.ChecklistItem{{Done: true}, {}},
	}}

	view := model.View()
	for _, value := range []string{"Review keyboard shortcuts", "HIGH", "due tomorrow", "✓1/2", "#ux +1", "↗1", "Make important actions"} {
		require.Contains(t, view, value)
	}
	require.LessOrEqual(t, lipgloss.Height(view), 24)
	for _, line := range splitLines(view) {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
}

func TestVisibleCardRangeAccountsForExpandedHeight(t *testing.T) {
	start, end := visibleCardRowRange([]int{1, 1, 4, 1}, 2, 5)
	require.Equal(t, 1, start)
	require.Equal(t, 3, end)
}

func TestBoardChooserShowsResponsiveDueHealth(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardsScreen
	model.project = &domain.Project{ID: "project", Name: "Platform"}
	model.boards = []domain.Board{{ID: "board", ProjectID: "project", Name: "Delivery", Description: "Release work"}}
	model.boardCounts["board"] = 4
	model.boardHealth["board"] = boardHealth{overdueCount: 2}

	view := model.View()
	for _, value := range []string{"Delivery", "Release work", "DUE", "2 overdue", "CARDS"} {
		require.Contains(t, view, value)
	}

	model.width = 50
	narrow := model.View()
	require.Contains(t, narrow, "Delivery")
	require.Contains(t, narrow, "DUE")
	require.Contains(t, narrow, "2 overdue")
	require.NotContains(t, narrow, "COMMENTS")

	model.width = 80
	model.openSelectedDetail()
	require.Contains(t, model.View(), "Due health: 2 overdue")
}

func TestExternalEditorPreparationAndBufferImport(t *testing.T) {
	t.Setenv("KAN_EDITOR_HELPER", "1")
	t.Setenv("VISUAL", fmt.Sprintf("%q -test.run=TestExternalEditorHelper --", os.Args[0]))
	t.Setenv("EDITOR", "ignored-editor")

	prepared := prepareExternalEditor("original")().(externalEditorPreparedMsg)
	require.NoError(t, prepared.err)
	require.NotNil(t, prepared.command)
	require.NoError(t, prepared.command.Run())
	finished := finishExternalEditor(prepared.path, nil).(externalEditorFinishedMsg)
	require.True(t, finished.apply)
	require.NoError(t, finished.err)
	require.Equal(t, "edited externally\n", finished.content)
	_, err := os.Stat(prepared.path)
	require.ErrorIs(t, err, os.ErrNotExist)

	model := testModel(readRepository{})
	model.projects = []domain.Project{{ID: "project", Name: "Project", Description: "original"}}
	model.startProjectForm(true)
	model.form.focus = 1
	model.Update(key("enter"))
	model.Update(finished)
	require.Equal(t, "original", model.form.fields[1].value)
	require.Equal(t, "edited externally\n", model.form.control.value)
	model.Update(key("ctrl+s"))
	require.Equal(t, "edited externally\n", model.form.fields[1].value)
}

func TestExternalEditorMissingAndFailedCommand(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	prepared := prepareExternalEditor("original")().(externalEditorPreparedMsg)
	require.ErrorContains(t, prepared.err, "set VISUAL or EDITOR")

	file, err := os.CreateTemp("", "kan-editor-failure-*.md")
	require.NoError(t, err)
	path := file.Name()
	require.NoError(t, file.Close())
	finished := finishExternalEditor(path, errors.New("exit status 1")).(externalEditorFinishedMsg)
	require.False(t, finished.apply)
	require.ErrorContains(t, finished.err, "exit status 1")
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestExternalEditorHelper(t *testing.T) {
	if os.Getenv("KAN_EDITOR_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	require.NoError(t, os.WriteFile(path, []byte("edited externally\n"), 0o600))
}

type recordedMove struct {
	cardID   string
	columnID string
	index    int
}

type moveRepository struct {
	readRepository
	moves   []recordedMove
	moveErr error
}

type columnMoveRepository struct {
	readRepository
	moves   []string
	moveErr error
}

func (repo *columnMoveRepository) MoveColumn(_ context.Context, columnID string, target int) error {
	repo.moves = append(repo.moves, fmt.Sprintf("%s:%d", columnID, target))
	if repo.moveErr != nil {
		return repo.moveErr
	}
	current := -1
	for index := range repo.columns {
		if repo.columns[index].ID == columnID {
			current = index
			break
		}
	}
	column := repo.columns[current]
	repo.columns = append(repo.columns[:current], repo.columns[current+1:]...)
	repo.columns = append(repo.columns, domain.Column{})
	copy(repo.columns[target+1:], repo.columns[target:])
	repo.columns[target] = column
	return nil
}

func TestMoveSelectedColumnLeftAndRight(t *testing.T) {
	repo := &columnMoveRepository{readRepository: readRepository{
		columns: []domain.Column{
			{ID: "backlog", BoardID: "board", Name: "Backlog", Position: 1024},
			{ID: "doing", BoardID: "board", Name: "Doing", Position: 2048},
			{ID: "done", BoardID: "board", Name: "Done", Position: 3072},
		},
	}}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = append([]domain.Column{}, repo.columns...)
	model.columnIndex = 1

	_, command := model.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	require.NotNil(t, command)
	runCommands(model, command)
	require.Equal(t, []string{"doing:0"}, repo.moves)
	require.Equal(t, []string{"Doing", "Backlog", "Done"}, []string{model.columns[0].Name, model.columns[1].Name, model.columns[2].Name})
	require.Zero(t, model.columnIndex)
	require.Equal(t, "Column reordered", model.notice)

	_, command = model.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	require.NotNil(t, command)
	runCommands(model, command)
	require.Equal(t, []string{"doing:0", "doing:1"}, repo.moves)
	require.Equal(t, 1, model.columnIndex)

	model.columnIndex = len(model.columns) - 1
	_, command = model.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	require.Nil(t, command)
	require.Equal(t, "Column is already last", model.notice)

	repo.moveErr = errors.New("write failed")
	model.columnIndex = 1
	_, command = model.executeCommand("move-column-left")
	require.NotNil(t, command)
	model.Update(command())
	require.ErrorContains(t, model.err, "write failed")
	require.False(t, model.loading)
}

func (repo *moveRepository) MoveCard(_ context.Context, cardID, columnID string, index int) error {
	repo.moves = append(repo.moves, recordedMove{cardID: cardID, columnID: columnID, index: index})
	return repo.moveErr
}

func TestMovePickerSkipsFullColumnsAndUndoRestoresSource(t *testing.T) {
	limit := 1
	columns := []domain.Column{
		{ID: "todo", BoardID: "board", Name: "Todo"},
		{ID: "full", BoardID: "board", Name: "Full", WIPLimit: &limit},
		{ID: "done", BoardID: "board", Name: "Done"},
	}
	cards := []domain.Card{
		{ID: "moving", BoardID: "board", ColumnID: "todo", Title: "Move me"},
		{ID: "occupied", BoardID: "board", ColumnID: "full", Title: "Occupied"},
	}
	repo := &moveRepository{readRepository: readRepository{columns: columns, cards: cards}}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.columns = columns
	model.cards["todo"] = cards[:1]
	model.cards["full"] = cards[1:]

	model.Update(key("M"))
	require.NotNil(t, model.movePicker)
	require.Equal(t, 2, model.movePicker.targetColumnIndex)
	picker := model.View()
	require.Contains(t, picker, "FULL")
	require.Contains(t, picker, "CURRENT")
	require.NotContains(t, picker, "> Done")

	_, command := model.Update(key("enter"))
	require.NotNil(t, command)
	runCommands(model, command)
	require.Equal(t, recordedMove{cardID: "moving", columnID: "done", index: 0}, repo.moves[0])
	require.NotNil(t, model.lastMove)

	_, command = model.Update(key("u"))
	require.NotNil(t, command)
	runCommands(model, command)
	require.Equal(t, recordedMove{cardID: "moving", columnID: "todo", index: 0}, repo.moves[1])
	require.Nil(t, model.lastMove)
}

func TestMovePickerAndUndoRespectDerivedViewsAndFailures(t *testing.T) {
	repo := &moveRepository{moveErr: errors.New("move failed")}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}}
	model.cards["one"] = []domain.Card{{ID: "card", ColumnID: "one", Title: "Card"}}
	model.filterText = "active"

	model.Update(key("M"))
	require.Nil(t, model.movePicker)
	require.Contains(t, model.notice, "Clear filter")

	model.clearBoardFilter()
	model.Update(key("M"))
	_, command := model.Update(key("enter"))
	runCommands(model, command)
	require.ErrorContains(t, model.err, "move failed")
	require.Nil(t, model.lastMove)

	record := cardMoveRecord{boardID: "board", cardID: "card", fromColumn: "one", toColumn: "two"}
	model.lastMove = &record
	_, command = model.Update(key("u"))
	runCommands(model, command)
	require.NotNil(t, model.lastMove)
}

func TestReorderCanBeUndone(t *testing.T) {
	columns := []domain.Column{{ID: "todo", BoardID: "board", Name: "Todo"}}
	cards := []domain.Card{
		{ID: "first", BoardID: "board", ColumnID: "todo", Title: "First"},
		{ID: "second", BoardID: "board", ColumnID: "todo", Title: "Second"},
	}
	repo := &moveRepository{readRepository: readRepository{columns: columns, cards: cards}}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = columns
	model.cards["todo"] = cards

	_, command := model.Update(key("J"))
	runCommands(model, command)
	require.Equal(t, recordedMove{cardID: "first", columnID: "todo", index: 1}, repo.moves[0])
	require.NotNil(t, model.lastMove)

	_, command = model.Update(key("u"))
	runCommands(model, command)
	require.Equal(t, recordedMove{cardID: "first", columnID: "todo", index: 0}, repo.moves[1])
	require.Nil(t, model.lastMove)
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func splitLines(value string) []string {
	lines := []string{}
	current := ""
	for _, character := range value {
		if character == '\n' {
			lines = append(lines, current)
			current = ""
			continue
		}
		current += string(character)
	}
	return append(lines, current)
}
