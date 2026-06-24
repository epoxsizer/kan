package app

import (
	"context"
	"encoding/json"
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
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
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
	require.NotNil(t, cards[0].DueDate)
	require.Equal(t, time.Now().Format("2006-01-02"), cards[0].DueDate.Format("2006-01-02"))

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
	model.form = nil

	model.startCardForm(false)
	require.Equal(t, "Medium", model.form.fields[3].value)
	require.Equal(t, time.Now().Format("2006-01-02"), model.form.fields[4].value)
	require.Equal(t, []string{"Todo", "Done"}, model.form.fields[2].options)
	require.Equal(t, linksField, model.form.fields[6].kind)

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
	model.Update(key("down"))
	model.Update(key("enter"))
	require.Equal(t, "Done", model.form.fields[2].value)

	model.form.focus = 4
	model.Update(key("enter"))
	before := model.form.control.date
	model.Update(key("right"))
	model.Update(key("enter"))
	require.Equal(t, before.AddDate(0, 0, 1).Format("2006-01-02"), model.form.fields[4].value)

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
	require.Contains(t, view, "ACTIVE • Doing")
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
	for _, command := range []string{"projects", "boards", "reload", "help", "quit", "add", "settings"} {
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
	require.Contains(t, opened, "[ux] Review keyb")
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
}
