package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
	storagesqlite "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

const (
	editorTestWidth  = 120
	editorTestHeight = 36
)

func cardEditorModel(t *testing.T) (*Model, *storagesqlite.Repository, domain.Card) {
	t.Helper()
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "editor.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Roadmap", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Doing", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	priority := "High"
	card := domain.Card{
		BoardID: board.ID, ColumnID: column.ID, Title: "Ship editor", Position: 1024, Priority: &priority,
		Description: "# Plan\n\nWrite the **tests** first.",
		Tags:        []string{"tui"},
		Checklist:   []domain.ChecklistItem{{ID: "red", Text: "Red test", Done: true, Position: 1024}, {ID: "green", Text: "Green code", Position: 2048}},
	}
	require.NoError(t, repo.CreateCard(ctx, &card))

	model := testModel(repo)
	model.Update(tea.WindowSizeMsg{Width: editorTestWidth, Height: editorTestHeight})
	model.loading = false
	model.screen = boardScreen
	model.project = &project
	model.board = &board
	model.columns = []domain.Column{column}
	model.cards = map[string][]domain.Card{column.ID: {card}}
	runCommands(model, model.startCardForm(true))
	return model, repo, card
}

func TestCardEditorUsesFullScreenLayoutOnWideTerminals(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	view := model.View()
	lines := strings.Split(view, "\n")

	require.Len(t, lines, editorTestHeight)
	for _, line := range lines {
		require.LessOrEqual(t, lipgloss.Width(line), editorTestWidth)
	}
	require.Contains(t, lines[0], "Edit card")
	require.Contains(t, lines[0], "Roadmap › Doing")
	require.True(t, lineContainsAll(view, "Title", "Ship editor"))
	require.True(t, lineContainsAll(view, "Description"), "description pane header")
	require.True(t, lineContainsAll(view, "Plan"), "rendered Markdown heading")
	require.False(t, lineContainsAll(view, "# Plan"), "description is rendered, not shown as source")
	require.True(t, lineContainsAll(view, "Checklist 1/2"))
	require.True(t, lineContainsAll(view, "[x]", "Red test"))
	require.True(t, lineContainsAll(view, "[ ]", "Green code"))
}

func TestCardEditorKeepsCompactFormOnSmallTerminals(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := model.View()
	require.False(t, lineContainsAll(view, "Checklist 1/2"))
	require.True(t, lineContainsAll(view, "Edit card"))
}

func TestCardEditorEditsChecklistInlineAndSaves(t *testing.T) {
	model, repo, card := cardEditorModel(t)
	checklistIndex := len(model.form.fields) - 1
	for model.form.focus < checklistIndex {
		model.Update(key("tab"))
	}
	require.NotNil(t, model.form.control, "focusing the checklist starts inline editing")
	require.True(t, model.form.control.inline)

	model.Update(key("j"))
	model.Update(key(" "))
	model.Update(key("a"))
	model.Update(key("Refactor"))
	model.Update(key("enter"))
	view := model.View()
	require.True(t, lineContainsAll(view, "Checklist 2/3"), "inline edits render inside the editor, not in a popup")
	require.True(t, lineContainsAll(view, "Title", "Ship editor"))

	_, command := model.Update(key("ctrl+s"))
	runCommands(model, command)
	require.Nil(t, model.form)
	stored, err := repo.GetCard(context.Background(), card.ID)
	require.NoError(t, err)
	require.Len(t, stored.Checklist, 3)
	require.True(t, stored.Checklist[1].Done)
	require.Equal(t, "Refactor", stored.Checklist[2].Text)
}

func TestCardEditorLeavesInlineChecklistWithTabAndUpArrow(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	checklistIndex := len(model.form.fields) - 1
	model.form.focus = checklistIndex - 1
	model.Update(key("tab"))
	require.Equal(t, checklistIndex, model.form.focus)
	model.Update(key("D"))

	model.Update(key("shift+tab"))
	require.Equal(t, checklistIndex-1, model.form.focus)
	require.Nil(t, model.form.control)
	var checklist []domain.ChecklistItem
	require.NoError(t, json.Unmarshal([]byte(model.form.fields[checklistIndex].value), &checklist))
	require.Len(t, checklist, 1, "leaving the checklist keeps its edits in the form")

	model.Update(key("tab"))
	model.Update(key("up"))
	require.Equal(t, checklistIndex-1, model.form.focus, "up on the first item moves to the previous field")
}

func TestCardEditorEscWithInlineChecklistChangesAsksToDiscard(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.form.focus = len(model.form.fields) - 2
	model.Update(key("tab"))
	model.Update(key(" "))
	model.Update(key("esc"))
	require.NotNil(t, model.discard)
	require.Equal(t, discardForm, model.discard.kind)
	model.Update(key("y"))
	require.Nil(t, model.form)
}

func TestCardEditorMouseFocusesDescriptionAndOpensEditor(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	layout := cardEditorLayoutForSize(editorTestWidth, editorTestHeight)
	x, y := layout.previewLeft+4, layout.bodyTop+3

	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	require.Equal(t, 1, model.form.focus)
	require.Nil(t, model.form.control)
	model.Update(mousePress(x, y, tea.MouseButtonLeft))
	require.NotNil(t, model.form.control)
	require.Equal(t, commentControl, model.form.control.kind)
}

func TestCardEditorMouseFocusesFieldsAndTogglesChecklistItems(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	layout := cardEditorLayoutForSize(editorTestWidth, editorTestHeight)

	statusRow := layout.bodyTop + 1 + cardEditorFieldRow(model.form, 2)
	model.Update(mousePress(layout.fieldsLeft+4, statusRow, tea.MouseButtonLeft))
	require.Equal(t, 2, model.form.focus)

	secondItemRow := layout.bodyTop + 1 + cardEditorChecklistRow(model.form, 1)
	model.Update(mousePress(layout.fieldsLeft+4, secondItemRow, tea.MouseButtonLeft))
	require.Equal(t, len(model.form.fields)-1, model.form.focus)
	require.NotNil(t, model.form.control)
	require.Equal(t, 1, model.form.control.selection)
	model.Update(mousePress(layout.fieldsLeft+4, secondItemRow, tea.MouseButtonLeft))
	require.True(t, model.form.control.checklist[1].Done)
}
