package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
	storagesqlite "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

type readRepository struct {
	domain.Repository
	projects []domain.Project
	boards   []domain.Board
	columns  []domain.Column
	cards    []domain.Card
}

type searchRepository struct {
	readRepository
	results []domain.Card
	queries []string
}

func (repo *searchRepository) SearchCards(_ context.Context, _, query string) ([]domain.Card, error) {
	repo.queries = append(repo.queries, query)
	return repo.results, nil
}

func (repo readRepository) ListProjects(context.Context) ([]domain.Project, error) {
	return repo.projects, nil
}

func (repo readRepository) ListBoards(context.Context, string) ([]domain.Board, error) {
	return repo.boards, nil
}

func (repo readRepository) ListColumns(context.Context, string) ([]domain.Column, error) {
	return repo.columns, nil
}

func (repo readRepository) ListCards(context.Context, string) ([]domain.Card, error) {
	return repo.cards, nil
}

func (repo readRepository) ListCardsIncludingDeleted(context.Context, string) ([]domain.Card, error) {
	return repo.cards, nil
}

func (repo readRepository) ArchiveExpiredCards(context.Context, string) (int, error) {
	return 0, nil
}

func testModel(repo domain.Repository) *Model {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	model := New(context.Background(), repo, logger)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return model
}

func key(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+e":
		return tea.KeyMsg{Type: tea.KeyCtrlE}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "ctrl+w":
		return tea.KeyMsg{Type: tea.KeyCtrlW}
	case "ctrl+g":
		return tea.KeyMsg{Type: tea.KeyCtrlG}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
	}
}

func runCommands(model *Model, command tea.Cmd) {
	for command != nil {
		_, command = model.Update(command())
	}
}

func lineContainsAll(value string, parts ...string) bool {
	for _, line := range strings.Split(value, "\n") {
		matches := true
		for _, part := range parts {
			if !strings.Contains(line, part) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func TestPhaseTwoTUIFormsMovementAndPersistence(t *testing.T) {
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "phase2.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	model := testModel(repo)
	runCommands(model, model.Init())

	model.Update(key("a"))
	model.Update(key("Phase 2 Project"))
	model.Update(key("tab"))
	model.Update(key("enter"))
	model.Update(key("First line with spaces"))
	model.Update(key("enter"))
	model.Update(key("Second line"))
	model.Update(key("ctrl+s"))
	_, command := model.Update(key("ctrl+s"))
	runCommands(model, command)
	projects, err := repo.ListProjects(ctx)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, "First line with spaces\nSecond line", projects[0].Description)

	model.Update(key("e"))
	model.Update(key("ctrl+u"))
	model.Update(key("Renamed Project"))
	_, command = model.Update(key("ctrl+s"))
	runCommands(model, command)
	project, err := repo.GetProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed Project", project.Name)

	_, command = model.Update(key("enter"))
	runCommands(model, command)
	model.Update(key("a"))
	model.Update(key("Delivery"))
	_, command = model.Update(key("ctrl+s"))
	runCommands(model, command)
	boards, err := repo.ListBoards(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, boards, 1)

	_, command = model.Update(key("enter"))
	runCommands(model, command)
	for _, name := range []string{"Backlog", "Done"} {
		model.Update(key("c"))
		model.Update(key(name))
		_, command = model.Update(key("ctrl+s"))
		runCommands(model, command)
	}
	require.Len(t, model.columns, 2)
	for _, column := range model.columns {
		require.NotNil(t, column.WIPLimit)
		require.Equal(t, 10, *column.WIPLimit)
		require.NotNil(t, column.Color)
		require.Equal(t, "Blue", *column.Color)
	}

	model.Update(key("a"))
	model.Update(key("Ship release"))
	_, command = model.Update(key("ctrl+s"))
	runCommands(model, command)
	cards, err := repo.ListCards(ctx, boards[0].ID)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, model.columns[0].ID, cards[0].ColumnID)
	require.NotNil(t, cards[0].Priority)
	require.Equal(t, "Medium", *cards[0].Priority)
	require.Nil(t, cards[0].DueDate)

	_, command = model.Update(key("tab"))
	runCommands(model, command)
	moved, err := repo.GetCard(ctx, cards[0].ID)
	require.NoError(t, err)
	require.Equal(t, model.columns[1].ID, moved.ColumnID)
	require.Equal(t, 1, model.columnIndex)
	_, command = model.Update(key("shift+tab"))
	runCommands(model, command)
	moved, err = repo.GetCard(ctx, cards[0].ID)
	require.NoError(t, err)
	require.Equal(t, model.columns[0].ID, moved.ColumnID)
	_, command = model.Update(key("tab"))
	runCommands(model, command)

	model.Update(key("a"))
	model.Update(key("Publish artifacts"))
	_, command = model.Update(key("ctrl+s"))
	runCommands(model, command)
	_, command = model.Update(key("J"))
	runCommands(model, command)
	cards, err = repo.ListCards(ctx, boards[0].ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Publish artifacts", "Ship release"}, []string{cards[0].Title, cards[1].Title})

	model.Update(key("D"))
	require.NotNil(t, model.confirm)
	require.Equal(t, "Delete card?", model.confirm.title)
	_, command = model.Update(key("y"))
	runCommands(model, command)
	_, err = repo.GetCard(ctx, moved.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFormTitlesAcceptSpaceKey(t *testing.T) {
	tests := []struct {
		name  string
		start func(*Model)
	}{
		{name: "project", start: func(model *Model) { model.startProjectForm(false) }},
		{name: "board", start: func(model *Model) { model.startBoardForm(false) }},
		{name: "column", start: func(model *Model) { model.startColumnForm(false) }},
		{name: "card", start: func(model *Model) { _ = model.startCardForm(false) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := testModel(readRepository{})
			model.project = &domain.Project{ID: "project"}
			model.board = &domain.Board{ID: "board", ProjectID: "project"}
			model.columns = []domain.Column{{ID: "column", BoardID: "board", Name: "Backlog"}}
			test.start(model)

			model.Update(key("Release"))
			model.Update(tea.KeyMsg{Type: tea.KeySpace})
			model.Update(key("Planning"))

			require.Equal(t, "Release Planning", model.form.fields[0].value)
		})
	}
}

func TestFocusedFormTitleShowsTailForLongInput(t *testing.T) {
	model := testModel(readRepository{})
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.columns = []domain.Column{{ID: "column", BoardID: "board", Name: "Backlog"}}
	_ = model.startCardForm(false)

	model.form.fields[0].value = "this is a very long card title that keeps growing after some letter is still visible"
	model.form.fields[0].cursor = len([]rune(model.form.fields[0].value))
	view := model.View()

	require.Contains(t, view, "…")
	require.Contains(t, view, "after some letter is still visible")
	require.NotContains(t, view, "this is a very long card title that keeps growing")
}

func TestFormTextFieldsEditAtCursor(t *testing.T) {
	model := testModel(readRepository{})
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.columns = []domain.Column{{ID: "column", BoardID: "board", Name: "Backlog"}}
	_ = model.startCardForm(false)

	model.Update(key("Ship relese"))
	model.Update(key("left"))
	model.Update(key("left"))
	model.Update(key("a"))
	model.Update(key("home"))
	model.Update(key("[P1] "))
	model.Update(key("end"))
	model.Update(key("!"))

	require.Equal(t, "[P1] Ship release!", model.form.fields[0].value)
	require.Equal(t, len([]rune(model.form.fields[0].value)), model.form.fields[0].cursor)
}

func TestDiscardChangedFormAndCommentEditorRequiresConfirmation(t *testing.T) {
	model := testModel(readRepository{})
	model.projects = []domain.Project{{ID: "project", Name: "Project", Description: "old"}}
	model.startProjectForm(true)

	model.Update(key("!"))
	model.Update(key("esc"))
	require.NotNil(t, model.discard)
	require.Equal(t, discardForm, model.discard.kind)
	model.Update(key("n"))
	require.NotNil(t, model.form)
	model.Update(key("esc"))
	model.Update(key("y"))
	require.Nil(t, model.form)

	model.startProjectForm(true)
	model.form.focus = 1
	model.Update(key("enter"))
	model.Update(key(" changed"))
	model.Update(key("esc"))
	require.NotNil(t, model.discard)
	require.Equal(t, discardControl, model.discard.kind)
	model.Update(key("n"))
	require.NotNil(t, model.form.control)
	model.Update(key("esc"))
	model.Update(key("y"))
	require.Nil(t, model.form.control)
	require.NotNil(t, model.form)

	model.form = nil
	model.startProjectForm(true)
	model.Update(key("esc"))
	require.Nil(t, model.form)
	require.Nil(t, model.discard)
}

func TestTypedFormDefaultsSelectorsCalendarAndCommentEditor(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}, {ID: "done", Name: "Done"}}
	model.cards = map[string][]domain.Card{}

	model.startColumnForm(false)
	require.Equal(t, "10", model.form.fields[1].value)
	require.Equal(t, "Blue", model.form.fields[2].value)
	require.Equal(t, "Disabled", model.form.fields[3].value)
	require.Equal(t, "14", model.form.fields[4].value)
	model.form.focus = 3
	model.Update(key(" "))
	require.Equal(t, "Enabled", model.form.fields[3].value)
	model.form = nil

	model.startCardForm(false)
	require.Equal(t, "Medium", model.form.fields[3].value)
	require.Empty(t, model.form.fields[4].value)
	require.Equal(t, "No due date · Enter calendar", fieldDisplayValue(model.form.fields[4], nil))
	require.Equal(t, []string{"Todo", "Done"}, model.form.fields[2].options)
	require.Equal(t, linksField, model.form.fields[6].kind)
	require.NotContains(t, model.View(), "> Title")

	model.form.focus = 1
	model.Update(key("enter"))
	model.Update(key("alpha"))
	model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model.Update(key("beta"))
	model.Update(key("enter"))
	model.Update(key("gamma"))
	model.Update(key("tab"))
	model.Update(key("delta"))
	model.Update(key("ctrl+s"))
	require.Equal(t, "alpha beta\ngamma\tdelta", model.form.fields[1].value)

	model.form.focus = 2
	model.Update(key("enter"))
	require.NotContains(t, model.View(), "> Todo")
	model.Update(key("down"))
	model.Update(key("enter"))
	require.Equal(t, "Done", model.form.fields[2].value)

	model.form.focus = 4
	model.Update(key("enter"))
	before := model.form.control.date
	model.Update(key("right"))
	model.Update(key("enter"))
	require.Equal(t, before.AddDate(0, 0, 1).Format("2006-01-02"), model.form.fields[4].value)
	model.Update(key("enter"))
	model.Update(key("x"))
	require.Empty(t, model.form.fields[4].value)
	require.Nil(t, model.form.control)

	model.form.focus = 5
	model.Update(key("release,"))
	model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model.Update(key("urgent fix"))
	require.Equal(t, "release, urgent fix", model.form.fields[5].value)

	model.form.linkCandidates = []linkCandidate{{id: "related", label: "Other board / Related card"}}
	model.form.linksLoading = false
	model.form.focus = 6
	model.Update(key("enter"))
	model.Update(key("Other"))
	model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model.Update(key("board"))
	require.Equal(t, "Other board", model.form.control.query)
	model.Update(key("enter"))
	model.Update(key("ctrl+s"))
	require.Equal(t, "related", model.form.fields[6].value)

	model.form.focus = 7
	model.Update(key("enter"))
	model.Update(key("a"))
	model.Update(key("Verify"))
	model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	model.Update(key("deployment package"))
	model.Update(key("enter"))
	model.Update(key(" "))
	model.Update(key("a"))
	model.Update(key("Publish notes"))
	model.Update(key("enter"))
	model.Update(key("K"))
	model.Update(key("ctrl+s"))
	var checklist []domain.ChecklistItem
	require.NoError(t, json.Unmarshal([]byte(model.form.fields[7].value), &checklist))
	require.Len(t, checklist, 2)
	require.Equal(t, "Publish notes", checklist[0].Text)
	require.Equal(t, "Verify deployment package", checklist[1].Text)
	require.True(t, checklist[1].Done)
}

func TestCardTagPrefixesCanBeDisabled(t *testing.T) {
	card := domain.Card{Title: "Ship", Tags: []string{"release", "urgent"}}
	model := testModel(readRepository{})
	require.Equal(t, "[release][urgent] Ship", model.cardLabel(card, 80, true))
	model.showCardTags = false
	require.Equal(t, "Ship", model.cardLabel(card, 80, true))
	card.Checklist = []domain.ChecklistItem{{ID: "one", Text: "One", Done: true}, {ID: "two", Text: "Two"}}
	require.Equal(t, "Ship [1/2]", model.cardLabel(card, 80, true))
}

func TestColoredActiveColumnIsExplicit(t *testing.T) {
	color := "Red"
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "active", Name: "Doing", Color: &color}}
	model.cards["active"] = []domain.Card{{ID: "card", Title: "Selected"}}
	require.Equal(t, lipgloss.Color("#42C77A"), model.styles.selectedColumnBackground)
	view := model.View()
	require.Contains(t, view, "Doing")
	require.NotContains(t, view, "ACTIVE •")
	require.NotContains(t, view, "> Selected")
	require.Contains(t, view, "╔")
}

func TestLiveBoardFTSFilterAndClear(t *testing.T) {
	target := domain.Card{ID: "target", BoardID: "board", ColumnID: "todo", Title: "Deploy release"}
	other := domain.Card{ID: "other", BoardID: "board", ColumnID: "todo", Title: "Write notes"}
	repo := &searchRepository{results: []domain.Card{target}}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}}
	model.cards["todo"] = []domain.Card{target, other}

	model.Update(key("/"))
	require.True(t, model.filterMode)
	_, command := model.Update(key("dep"))
	require.NotNil(t, command)
	model.Update(command())
	require.Equal(t, []string{`"dep"*`}, repo.queries)
	require.Contains(t, model.View(), "Deploy release")
	require.NotContains(t, model.View(), "Write notes")

	model.Update(key("enter"))
	require.False(t, model.filterMode)
	model.Update(key("/"))
	model.Update(key("ctrl+u"))
	require.False(t, model.filterActive())
	require.Contains(t, model.View(), "Write notes")
}

func TestFilterCommandUsesFuzzyCardMetadataMatching(t *testing.T) {
	priority := "Urgent"
	cards := []domain.Card{
		{ID: "release", BoardID: "board", ColumnID: "done", Title: "Publish release", Tags: []string{"shipping"}},
		{ID: "incident", BoardID: "board", ColumnID: "doing", Title: "Investigate outage", Priority: &priority},
	}
	columns := []domain.Column{{ID: "doing", Name: "In Progress"}, {ID: "done", Name: "Done"}}
	repo := &searchRepository{readRepository: readRepository{cards: cards, columns: columns}}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = columns
	model.cards["done"] = cards[:1]
	model.cards["doing"] = cards[1:]

	_, command := model.executeCommand("filter")
	require.Nil(t, command)
	require.True(t, model.filterMode)
	_, command = model.Update(key("relese"))
	require.NotNil(t, command)
	model.Update(command())
	require.Equal(t, []string{"release"}, []string{model.visibleCards("done")[0].ID})
	require.Empty(t, model.visibleCards("doing"))

	model.Update(key("ctrl+u"))
	_, command = model.Update(key("urgnt"))
	require.NotNil(t, command)
	model.Update(command())
	require.Equal(t, []string{"incident"}, []string{model.visibleCards("doing")[0].ID})
}

func TestRuneEditDistance(t *testing.T) {
	require.Equal(t, 1, runeEditDistance("relese", "release"))
	require.Equal(t, 1, runeEditDistance("urgnt", "urgent"))
	require.Equal(t, 0, runeEditDistance("карта", "карта"))
}

func TestBoardSortGroupAndMetadataLabels(t *testing.T) {
	urgent, low := "Urgent", "Low"
	due := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}}
	model.cards["todo"] = []domain.Card{
		{ID: "none", ColumnID: "todo", Title: "None", Position: 1},
		{ID: "low", ColumnID: "todo", Title: "Low", Position: 2, Priority: &low},
		{ID: "urgent", ColumnID: "todo", Title: "Urgent", Position: 3, Priority: &urgent, DueDate: &due, Tags: []string{"release"}},
	}

	model.Update(key("s"))
	ordered := model.visibleCards("todo")
	require.Equal(t, []string{"urgent", "low", "none"}, []string{ordered[0].ID, ordered[1].ID, ordered[2].ID})
	model.Update(key("v"))
	view := model.View()
	require.Contains(t, view, "URGENT")
	require.Contains(t, view, "LOW")
	label := model.cardLabel(model.cards["todo"][2], 80, false)
	require.Contains(t, label, "U @2026-07-01 [release] Urgent")
	require.Contains(t, label, "@2026-07-01")
}

func TestOverdueCardsShowDeadlineMarker(t *testing.T) {
	pastDue := time.Now().AddDate(0, 0, -1)
	futureDue := time.Now().AddDate(0, 0, 1)
	model := testModel(readRepository{})
	model.loading = false

	overdue := domain.Card{ID: "late", Title: "Late card", DueDate: &pastDue}
	current := domain.Card{ID: "next", Title: "Next card", DueDate: &futureDue}

	lateLabel := model.cardLabel(overdue, 80, false)
	require.Contains(t, lateLabel, "!@"+pastDue.Format("2006-01-02"))
	require.NotContains(t, model.cardLabel(current, 80, false), "!@")

	detail := cardDetail(overdue, "Backlog")
	require.Contains(t, strings.Join(detail.lines, "\n"), "Due: "+pastDue.Format("2006-01-02")+" (! overdue)")
}

func TestTransientTextInputsEditAtCursorWithoutUnnecessarySearch(t *testing.T) {
	repo := &searchRepository{}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}}

	model.Update(key("/"))
	_, command := model.Update(key("relese"))
	require.NotNil(t, command)
	runCommands(model, command)
	model.Update(key("left"))
	model.Update(key("left"))
	_, command = model.Update(key("a"))
	require.NotNil(t, command)
	runCommands(model, command)
	require.Equal(t, "release", model.filterText)
	queryCount := len(repo.queries)
	_, command = model.Update(key("left"))
	require.Nil(t, command)
	require.Len(t, repo.queries, queryCount)

	model.Update(key("esc"))
	model.Update(key(":"))
	model.Update(key("helo"))
	model.Update(key("left"))
	model.Update(key("l"))
	require.Equal(t, "hello", model.command)
	require.Equal(t, 4, model.commandCursor)
}

func TestRelatedCardQueryAndChecklistTextEditAtCursor(t *testing.T) {
	model := testModel(readRepository{})
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}}
	_ = model.startCardForm(false)

	model.form.focus = 6
	model.Update(key("enter"))
	model.Update(key("Other crd"))
	model.Update(key("left"))
	model.Update(key("left"))
	model.Update(key("a"))
	require.Equal(t, "Other card", model.form.control.query)

	model.Update(key("esc"))
	model.form.focus = 7
	model.Update(key("enter"))
	model.Update(key("a"))
	model.Update(key("Check iem"))
	model.Update(key("left"))
	model.Update(key("left"))
	model.Update(key("t"))
	require.Equal(t, "Check item", model.form.control.input)

	model.Update(key("esc"))
	require.NotNil(t, model.discard)
	require.Equal(t, discardChecklistInput, model.discard.kind)
	model.Update(key("n"))
	require.True(t, model.form.control.inputMode)
}

func TestBuildFTSQueryUsesSafePrefixTerms(t *testing.T) {
	require.Equal(t, `"ship"* AND "release"*`, buildFTSQuery(" ship  release "))
	require.Equal(t, `"say""hello"*`, buildFTSQuery(`say"hello`))
}

func TestDirectCommentShortcutAndLongEditorViewport(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{ID: "project", Name: "Project", Description: "old"}}
	model.Update(key("m"))
	require.NotNil(t, model.form)
	require.NotNil(t, model.form.control)
	require.True(t, model.form.control.standalone)
	require.Equal(t, commentControl, model.form.control.kind)

	viewport := editorViewport(strings.Repeat("line content\n", 30), len([]rune(strings.Repeat("line content\n", 30))), 20, 5)
	require.Contains(t, viewport, "█")
	require.LessOrEqual(t, len(strings.Split(viewport, "\n")), 5)
}

func TestCommentEditorUsesFullScreenViewportForHugeText(t *testing.T) {
	commentLines := make([]string, 0, 80)
	for index := 0; index < 80; index++ {
		commentLines = append(commentLines, fmt.Sprintf("editor-comment-line-%02d with enough text to wrap safely", index))
	}
	model := testModel(readRepository{})
	model.loading = false
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "In Progress"}}
	model.cards["doing"] = []domain.Card{{ID: "card-id", BoardID: "board", ColumnID: "doing", Title: "Huge comment", Description: strings.Join(commentLines, "\n")}}

	model.Update(key("e"))
	model.form.focus = 1
	model.Update(key("enter"))
	view := model.View()

	require.Contains(t, view, "Comments editor")
	require.Contains(t, view, "editor-comment-line-")
	require.LessOrEqual(t, lipgloss.Height(view), 24)
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
}

func TestNavigateProjectsBoardsAndCards(t *testing.T) {
	repo := readRepository{
		projects: []domain.Project{{ID: "project", Name: "Demo Project"}},
		boards:   []domain.Board{{ID: "board", ProjectID: "project", Name: "Product Board"}},
		columns: []domain.Column{
			{ID: "backlog", BoardID: "board", Name: "Backlog"},
			{ID: "doing", BoardID: "board", Name: "In Progress"},
		},
		cards: []domain.Card{
			{ID: "one", BoardID: "board", ColumnID: "backlog", Title: "First card"},
			{ID: "two", BoardID: "board", ColumnID: "doing", Title: "Second card"},
			{ID: "three", BoardID: "board", ColumnID: "doing", Title: "Third card"},
		},
	}
	model := testModel(repo)

	model.Update(model.Init()())
	require.Contains(t, model.View(), "Demo Project")
	require.Contains(t, model.View(), "BOARDS")
	require.Equal(t, 1, model.projectCounts["project"])

	_, command := model.Update(key("enter"))
	require.NotNil(t, command)
	model.Update(command())
	require.Equal(t, boardsScreen, model.screen)
	require.Contains(t, model.View(), "Product Board")
	require.Contains(t, model.View(), "CARDS")
	require.Equal(t, 3, model.boardCounts["board"])

	_, command = model.Update(key("enter"))
	require.NotNil(t, command)
	model.Update(command())
	require.Equal(t, boardScreen, model.screen)
	view := model.View()
	for _, value := range []string{"Backlog", "In Progress", "First card", "Second card", "2 columns", "3 cards"} {
		require.Contains(t, view, value)
	}

	model.Update(key("l"))
	require.Equal(t, 1, model.columnIndex)
	model.Update(key("j"))
	require.Equal(t, 1, model.cardIndexes["doing"])
	model.Update(key("g"))
	require.Equal(t, 0, model.cardIndexes["doing"])
	model.Update(key("G"))
	require.Equal(t, 1, model.cardIndexes["doing"])
	model.Update(key("esc"))
	require.Equal(t, boardsScreen, model.screen)
	model.Update(key("esc"))
	require.Equal(t, projectsScreen, model.screen)
}

func TestHelpCommandBarAndResize(t *testing.T) {
	model := testModel(readRepository{})
	model.Update(model.Init()())
	model.Update(key("?"))
	require.True(t, model.help)
	help := model.View()
	require.Contains(t, help, "EDITING")
	require.LessOrEqual(t, lipgloss.Height(help), 24)
	for _, line := range strings.Split(help, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
	model.Update(key("esc"))
	require.False(t, model.help)

	model.Update(key(":"))
	palette := model.View()
	for _, command := range []string{"projects", "boards", "reload", "help", "quit", "add", "column-settings", "settings"} {
		require.Contains(t, palette, command)
	}
	require.NotContains(t, palette, "new")
	for _, character := range "help" {
		model.Update(key(string(character)))
	}
	require.Contains(t, model.View(), ":help")
	model.Update(key("enter"))
	require.True(t, model.help)

	model.Update(tea.WindowSizeMsg{Width: 24, Height: 8})
	require.NotPanics(t, func() { _ = model.View() })
}

func TestSettingsCommandAppliesBaseParameters(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.showCardTags = true
	model.sortMode = sortPosition
	model.groupMode = groupNone

	_, indexCommand := model.Update(key(":"))
	model.Update(indexCommand())
	for _, character := range "settings" {
		model.Update(key(string(character)))
	}
	model.Update(key("enter"))

	require.NotNil(t, model.form)
	require.Equal(t, settingsForm, model.form.kind)
	require.Equal(t, "Settings", model.form.title)

	model.form.fields[0].value = "Cards"
	model.form.fields[1].value = "Disabled"
	model.form.fields[2].value = "Due date"
	model.form.fields[3].value = "Priority"
	model.Update(key("ctrl+s"))

	require.Nil(t, model.form)
	require.Equal(t, cardsLayout, model.listLayout)
	require.False(t, model.showCardTags)
	require.Equal(t, sortDue, model.sortMode)
	require.Equal(t, groupPriority, model.groupMode)
	require.Equal(t, "Settings applied", model.notice)
}

func TestCommandPaletteUsesFuzzyMatching(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.project = &domain.Project{Name: "Project"}
	model.board = &domain.Board{Name: "Board"}
	_, indexCommand := model.Update(key(":"))
	model.Update(indexCommand())
	for _, character := range "prj" {
		model.Update(key(string(character)))
	}
	view := model.View()
	require.Contains(t, view, "projects")
	require.NotContains(t, view, "> [")
	model.Update(key("enter"))
	require.Equal(t, projectsScreen, model.screen)

	_, indexCommand = model.Update(key(":"))
	model.Update(indexCommand())
	for _, character := range "zzzz" {
		model.Update(key(string(character)))
	}
	require.Contains(t, model.View(), "No matching commands")
}

func TestCommandPaletteArrowSelection(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.project = &domain.Project{Name: "Project"}
	_, indexCommand := model.Update(key(":"))
	model.Update(indexCommand())
	model.Update(key("down"))
	require.Equal(t, 1, model.commandIndex)
	model.Update(key("enter"))
	require.Equal(t, boardsScreen, model.screen)
}

func TestLayoutCommandSwitchesProjectsAndBoardsBetweenTableAndCards(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{
		{ID: "project", Name: "Platform", Description: "Internal tooling"},
		{ID: "console", Name: "Console", Description: "Operator UI"},
	}
	model.projectCounts["project"] = 2
	model.projectCounts["console"] = 1

	tableView := model.View()
	require.Contains(t, tableView, "NAME")
	require.Contains(t, tableView, "BOARDS")
	require.Contains(t, tableView, "│")
	require.Contains(t, tableView, "╭")
	require.Contains(t, tableView, "layout:table")

	model.executeCommand("layout cards")
	cardsView := model.View()
	require.NotContains(t, cardsView, "COMMENTS")
	require.Contains(t, cardsView, "Platform")
	require.Contains(t, cardsView, "Console")
	require.Contains(t, cardsView, "Boards: 2")
	require.True(t, lineContainsAll(cardsView, "Platform", "Console"))
	require.Contains(t, cardsView, "Layout: cards")

	model.screen = boardsScreen
	model.project = &domain.Project{ID: "project", Name: "Platform"}
	model.boards = []domain.Board{{ID: "board", ProjectID: "project", Name: "Delivery", Description: "Release work"}}
	model.boardCounts["board"] = 7
	boardCardsView := model.View()
	require.Contains(t, boardCardsView, "Delivery")
	require.Contains(t, boardCardsView, "Cards: 7")

	model.executeCommand("layout table")
	boardTableView := model.View()
	require.Contains(t, boardTableView, "NAME")
	require.Contains(t, boardTableView, "CARDS")
	require.Contains(t, boardTableView, "│")
	require.Contains(t, boardTableView, "Layout: table")
}

func TestCommandPaletteAcceptsLayoutCommandWithSpace(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	_, indexCommand := model.Update(key(":"))
	model.Update(indexCommand())
	for _, character := range "layout" {
		model.Update(key(string(character)))
	}
	model.Update(tea.KeyMsg{Type: tea.KeySpace})
	for _, character := range "cards" {
		model.Update(key(string(character)))
	}
	require.Contains(t, model.View(), ":layout cards")

	model.Update(key("enter"))
	require.Equal(t, cardsLayout, model.listLayout)
	require.Equal(t, "Layout: cards", model.notice)
}

func TestCardLayoutNavigationUsesGridDirections(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.listLayout = cardsLayout
	model.width = 80
	model.projects = []domain.Project{
		{ID: "one", Name: "One"},
		{ID: "two", Name: "Two"},
		{ID: "three", Name: "Three"},
		{ID: "four", Name: "Four"},
	}

	model.Update(key("right"))
	require.Equal(t, 1, model.projectIndex)
	model.Update(key("left"))
	require.Equal(t, 0, model.projectIndex)
	model.Update(key("down"))
	require.Equal(t, 3, model.projectIndex)
	model.Update(key("up"))
	require.Equal(t, 0, model.projectIndex)

	_, command := model.Update(key("right"))
	require.Nil(t, command)
	require.Equal(t, projectsScreen, model.screen)
	_, command = model.Update(key("enter"))
	require.NotNil(t, command)
	require.Equal(t, boardsScreen, model.screen)
	require.Equal(t, "Two", model.project.Name)

	model.loading = false
	model.listLayout = cardsLayout
	model.boards = []domain.Board{
		{ID: "alpha", ProjectID: "one", Name: "Alpha"},
		{ID: "beta", ProjectID: "one", Name: "Beta"},
		{ID: "gamma", ProjectID: "one", Name: "Gamma"},
		{ID: "delta", ProjectID: "one", Name: "Delta"},
	}
	model.boardIndex = 0
	_, command = model.Update(key("right"))
	require.Nil(t, command)
	require.Equal(t, 1, model.boardIndex)
	model.Update(key("down"))
	require.Equal(t, 3, model.boardIndex)
	model.Update(key("left"))
	require.Equal(t, 2, model.boardIndex)
	_, command = model.Update(key("enter"))
	require.NotNil(t, command)
	require.Equal(t, boardScreen, model.screen)
	require.Equal(t, "Gamma", model.board.Name)
}

func TestCommandPaletteSearchesAndOpensCards(t *testing.T) {
	repo := readRepository{
		projects: []domain.Project{{ID: "project", Name: "Platform"}},
		boards:   []domain.Board{{ID: "board", ProjectID: "project", Name: "Delivery"}},
		columns: []domain.Column{
			{ID: "backlog", BoardID: "board", Name: "Backlog"},
			{ID: "doing", BoardID: "board", Name: "In Progress"},
			{ID: "done", BoardID: "board", Name: "Done"},
		},
		cards: []domain.Card{
			{ID: "other", BoardID: "board", ColumnID: "doing", Title: "Other task"},
			{ID: "target", BoardID: "board", ColumnID: "doing", Title: "Review keyboard shortcuts", Description: "Discoverable help", Tags: []string{"ux"}, Fields: map[string]domain.FieldValue{"owner": {Type: domain.FieldText, Value: "Ada"}}},
		},
	}
	model := testModel(repo)
	model.loading = false
	_, indexCommand := model.Update(key(":"))
	require.NotNil(t, indexCommand)
	model.Update(indexCommand())
	for _, character := range "keybrd" {
		model.Update(key(string(character)))
	}
	view := model.View()
	require.Contains(t, view, "[card]")
	require.Contains(t, view, "Review keyboard")
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}

	_, openCommand := model.Update(key("enter"))
	require.NotNil(t, openCommand)
	require.Equal(t, boardScreen, model.screen)
	model.Update(openCommand())
	require.Equal(t, 1, model.columnIndex)
	require.Equal(t, 1, model.cardIndexes["doing"])
	opened := model.View()
	require.Contains(t, opened, "Review keyboard")
	require.Contains(t, opened, "#ux")
}

func TestCommandPaletteSearchesCardMetadata(t *testing.T) {
	repo := readRepository{
		projects: []domain.Project{{ID: "project", Name: "Platform"}},
		boards:   []domain.Board{{ID: "board", ProjectID: "project", Name: "Delivery"}},
		cards:    []domain.Card{{ID: "target", BoardID: "board", Title: "Metadata card", Tags: []string{"urgent"}, Fields: map[string]domain.FieldValue{"owner": {Type: domain.FieldText, Value: "Ada"}}}},
	}
	model := testModel(repo)
	model.loading = false
	_, indexCommand := model.Update(key(":"))
	model.Update(indexCommand())
	for _, character := range "urgent" {
		model.Update(key(string(character)))
	}
	require.Contains(t, model.View(), "Metadata card")
	model.command = "Ada"
	require.Contains(t, model.View(), "Metadata card")
}

func TestProjectAndBoardTablesShowCommentsAndCounts(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{ID: "p", Name: "Platform", Description: "Internal tooling"}}
	model.projectCounts["p"] = 4
	view := model.View()
	for _, value := range []string{"NAME", "COMMENTS", "BOARDS", "Platform", "Internal tooling", "4"} {
		require.Contains(t, view, value)
	}
	require.True(t, lineContainsAll(view, "NAME", "COMMENTS", "BOARDS"))
	require.True(t, lineContainsAll(view, "Platform", "Internal tooling", "4"))
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}

	model.screen = boardsScreen
	model.project = &model.projects[0]
	model.boards = []domain.Board{{ID: "b", Name: "Delivery", Description: "Release work"}}
	model.boardCounts["b"] = 12
	view = model.View()
	for _, value := range []string{"NAME", "COMMENTS", "CARDS", "Delivery", "Release work", "12"} {
		require.Contains(t, view, value)
	}
	require.True(t, lineContainsAll(view, "NAME", "COMMENTS", "CARDS"))
	require.True(t, lineContainsAll(view, "Delivery", "Release work", "12"))
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
}

func TestEmptyHierarchyStates(t *testing.T) {
	model := testModel(readRepository{})
	model.Update(model.Init()())
	require.Contains(t, model.View(), "No projects")

	model.loading = false
	model.screen = boardsScreen
	model.project = &domain.Project{Name: "Empty Project"}
	require.Contains(t, model.View(), "No boards")

	model.screen = boardScreen
	model.board = &domain.Board{Name: "Empty Board"}
	require.Contains(t, model.View(), "no columns")
}

func TestContextualShortcutHelpStaysAboveBottomPadding(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{ID: "project", Name: "Project"}}
	view := model.View()
	lines := strings.Split(view, "\n")
	require.Equal(t, 24, lipgloss.Height(view))
	require.Empty(t, lines[len(lines)-1])
	shortcutLine := lines[len(lines)-2]
	for _, value := range []string{"<?> Help", "<:> Cmd", "<q> Quit", "<j/k> Navigate", "<Enter> Open"} {
		require.Contains(t, shortcutLine, value)
	}

	model.screen = boardScreen
	model.board = &domain.Board{ID: "board", Name: "Board"}
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}}
	view = model.View()
	lines = strings.Split(view, "\n")
	require.Empty(t, lines[len(lines)-1])
	shortcutLine = lines[len(lines)-2]
	require.Contains(t, shortcutLine, "<j/k> Card")
	require.Contains(t, shortcutLine, "<h/l> Column")
	require.Equal(t, 24, lipgloss.Height(view))

	model.filterMode = true
	view = model.View()
	lines = strings.Split(view, "\n")
	shortcutLine = lines[len(lines)-1]
	require.Contains(t, shortcutLine, "<Ctrl-U> Clear")
}

func TestWideBoardHorizontallyFollowsFocus(t *testing.T) {
	columns := make([]domain.Column, 5)
	for index := range columns {
		columns[index] = domain.Column{ID: string(rune('a' + index)), Name: "Column " + string(rune('A'+index))}
	}
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = columns
	model.columnIndex = 4
	model.width = 60
	view := model.View()
	require.Contains(t, view, "Column E")
	require.NotContains(t, view, "Column A")

	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 60)
	}
}

func TestColumnsExpandToTerminalWidth(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = []domain.Column{
		{ID: "a", Name: "Backlog"},
		{ID: "b", Name: "In Progress"},
		{ID: "c", Name: "Done"},
	}
	model.width = 80
	view := model.View()
	for _, name := range []string{"Backlog", "In Progress", "Done"} {
		require.Contains(t, view, name)
	}
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}

	model.width = 120
	view = model.View()
	borderLine := strings.Split(view, "\n")[1]
	require.Equal(t, 120, lipgloss.Width(borderLine))
}

func TestMutationKeysOpenFormsAndConfirmations(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.Update(key("a"))
	require.Contains(t, model.View(), "Add project")
	model.Update(key("esc"))

	model.projects = []domain.Project{{ID: "p", Name: "Project"}}
	model.Update(key("e"))
	require.Contains(t, model.View(), "Edit project")
	model.Update(key("esc"))
	model.Update(key("D"))
	require.Contains(t, model.View(), "Delete project?")
}

func TestArchiveColumnCommandRequiresConfirmation(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "done", Name: "Done", ArchiveAfterDays: 14}}
	model.cards["done"] = []domain.Card{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}}

	_, command := model.executeCommand("archive")
	require.Nil(t, command)
	require.NotNil(t, model.confirm)
	require.Equal(t, archiveColumnCards, model.confirm.kind)
	require.Equal(t, "done", model.confirm.id)
	require.Contains(t, model.confirm.message, "2 active cards")
}

func TestColumnSettingsCommandOpensSelectedColumn(t *testing.T) {
	limit := 4
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{
		{ID: "todo", BoardID: "board", Name: "Todo", ArchiveAfterDays: 14},
		{ID: "doing", BoardID: "board", Name: "Doing", WIPLimit: &limit, AutoArchive: true, ArchiveAfterDays: 21},
	}
	model.columnIndex = 1

	_, command := model.executeCommand("column-settings")
	require.Nil(t, command)
	require.NotNil(t, model.form)
	require.Equal(t, editColumnForm, model.form.kind)
	require.Equal(t, "Column settings", model.form.title)
	require.Equal(t, "Doing", model.form.fields[0].value)
	require.Equal(t, "4", model.form.fields[1].value)
	require.Equal(t, "Enabled", model.form.fields[3].value)
	require.Equal(t, "21", model.form.fields[4].value)
}

func TestColumnSettingsCommandRequiresBoardColumn(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false

	_, command := model.executeCommand("column-settings")
	require.Nil(t, command)
	require.ErrorContains(t, model.err, "open a board column first")
	require.Nil(t, model.form)
}

func TestArchivedCommandShowsBoardCards(t *testing.T) {
	archivedAt := time.Date(2026, 7, 1, 9, 30, 0, 0, time.Local)
	repo := readRepository{
		columns: []domain.Column{{ID: "done", BoardID: "board", Name: "Done", ArchiveAfterDays: 14}},
		cards: []domain.Card{
			{ID: "active", BoardID: "board", ColumnID: "done", Title: "Still active"},
			{ID: "archived", BoardID: "board", ColumnID: "done", Title: "Shipped release", DeletedAt: &archivedAt},
		},
	}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}

	_, command := model.executeCommand("archived")
	require.NotNil(t, command)
	require.Contains(t, model.View(), "Loading archived cards")
	model.Update(command())
	view := model.View()
	require.Contains(t, view, "Shipped release")
	require.Contains(t, view, "Column: Done")
	require.Contains(t, view, "2026-07-01 09:30")
	require.NotContains(t, view, "Still active")
}

func TestDetailPopupForProjectAndBoard(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{ID: "project-id", Name: "Platform", Description: "Internal tooling", Position: 1024}}
	model.projectCounts["project-id"] = 3
	model.Update(key("d"))
	require.NotNil(t, model.detail)
	view := model.View()
	for _, value := range []string{"Platform", "PROJECT", "project-id", "Internal tooling", "Boards: 3"} {
		require.Contains(t, view, value)
	}
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
	model.Update(key("d"))
	require.Nil(t, model.detail)

	model.screen = boardsScreen
	model.boards = []domain.Board{{ID: "board-id", ProjectID: "project-id", Name: "Delivery", Description: "Release work", Position: 1024}}
	model.boardCounts["board-id"] = 7
	model.Update(key("d"))
	view = model.View()
	for _, value := range []string{"Delivery", "BOARD", "Release work", "Cards: 7"} {
		require.Contains(t, view, value)
	}
	model.Update(key("esc"))
	require.Nil(t, model.detail)
}

func TestDetailPopupForCardAndEmptyColumn(t *testing.T) {
	due := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	priority := "high"
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "In Progress"}, {ID: "empty", BoardID: "board", Name: "Done"}}
	model.cards["doing"] = []domain.Card{{
		ID: "card-id", Title: "Ship release", Description: "Agent-owned task", Priority: &priority, DueDate: &due,
		Tags: []string{"release", "urgent"}, Fields: map[string]domain.FieldValue{"owner": {Type: domain.FieldText, Value: "Ada"}},
	}}
	model.Update(key("d"))
	view := model.View()
	for _, value := range []string{"Ship release", "CARD", "Status: In Progress", "Agent-owned task", "Priority: high", "2026-07-01", "release, urgent", "owner [text]: \"Ada\""} {
		require.Contains(t, view, value)
	}
	require.Contains(t, view, "e edit")
	model.Update(key("enter"))
	require.Nil(t, model.detail)

	model.columnIndex = 1
	model.Update(key("d"))
	view = model.View()
	for _, value := range []string{"Done", "COLUMN", "Cards: 0", "WIP limit: none"} {
		require.Contains(t, view, value)
	}
}

func TestDetailPopupShowsFullMultilineComments(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{
		ID:          "project-id",
		Name:        "Platform",
		Description: "This is a long comment that should wrap across multiple lines without being replaced by an ellipsis.\nSecond line keeps exact text visible.",
	}}
	model.Update(key("d"))
	view := model.View()

	for _, value := range []string{"This is a long comment", "lines", "without being replaced", "by an ellipsis.", "Second line keeps exact text visible"} {
		require.Contains(t, view, value)
	}
	require.NotContains(t, view, "This is a long comment that should wrap across multiple lines without being replaced by an ellipsis.…")
}

func TestDetailPopupIsCompactAndCanExpandForHugeComments(t *testing.T) {
	commentLines := make([]string, 0, 80)
	for index := 0; index < 80; index++ {
		commentLines = append(commentLines, fmt.Sprintf("comment-line-%02d with enough text to exercise wrapping safely", index))
	}
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "In Progress"}}
	model.cards["doing"] = []domain.Card{{ID: "card-id", BoardID: "board", ColumnID: "doing", Title: "Huge comment", Description: strings.Join(commentLines, "\n")}}

	model.Update(key("d"))
	view := model.View()
	require.Contains(t, view, "comment-line-00")
	require.NotContains(t, view, "comment-line-79")
	require.Contains(t, view, "Shift+E expand")
	require.Contains(t, view, "<?> Help")
	require.LessOrEqual(t, lipgloss.Height(view), 24)
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}

	model.Update(key("pgdown"))
	scrolled := model.View()
	require.NotEqual(t, view, scrolled)
	require.Contains(t, scrolled, "comment-line-")
	require.LessOrEqual(t, lipgloss.Height(scrolled), 24)
	for _, line := range strings.Split(scrolled, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}

	model.Update(key("E"))
	require.True(t, model.detail.expanded)
	expanded := model.View()
	require.Contains(t, expanded, "Shift+E compact")
	require.NotContains(t, expanded, "<?> Help")
	model.Update(key("E"))
	require.False(t, model.detail.expanded)
	require.Contains(t, model.View(), "<?> Help")

	model.Update(key("G"))
	require.Contains(t, model.View(), "comment-line-79")
	model.Update(key("g"))
	require.Contains(t, model.View(), "comment-line-00")
}

func TestDetailPopupWrapsLongUnbrokenTextWithinScreen(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{
		ID:          "project-id",
		Name:        "Platform",
		Description: strings.Repeat("x", 400),
	}}
	model.Update(key("d"))
	view := model.View()

	require.LessOrEqual(t, lipgloss.Height(view), 24)
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
}

func TestCardDetailCanOpenEditForm(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.project = &domain.Project{ID: "project"}
	model.board = &domain.Board{ID: "board", ProjectID: "project"}
	model.screen = boardScreen
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "In Progress"}}
	model.cards["doing"] = []domain.Card{{ID: "card-id", BoardID: "board", ColumnID: "doing", Title: "Ship release", Description: "Edit from detail"}}

	model.Update(key("enter"))
	require.NotNil(t, model.detail)
	require.Equal(t, "card", model.detail.kind)

	_, command := model.Update(key("e"))
	require.Nil(t, model.detail)
	require.NotNil(t, model.form)
	require.Equal(t, editCardForm, model.form.kind)
	require.Equal(t, "Ship release", model.form.fields[0].value)
	require.Equal(t, "Edit from detail", model.form.fields[1].value)
	require.NotNil(t, command)
	require.Contains(t, model.View(), "<?> Help")
}
