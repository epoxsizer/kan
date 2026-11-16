package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestProjectViewportFollowsSelectionAndMouseAfterResize(t *testing.T) {
	for _, layout := range []listLayout{tableLayout, cardsLayout} {
		t.Run(layout.String(), func(t *testing.T) {
			model := testModel(readRepository{})
			model.loading = false
			model.listLayout = layout
			for index := 0; index < 40; index++ {
				model.projects = append(model.projects, domain.Project{ID: fmt.Sprint(index), Name: fmt.Sprintf("Project%03d", index+1)})
			}
			model.Update(key("G"))
			for _, size := range [][2]int{{80, 24}, {44, 16}, {120, 36}} {
				model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				view := ansi.Strip(model.View())
				require.Contains(t, view, "Project040")
				require.LessOrEqual(t, lipgloss.Height(view), size[1])
				for _, line := range strings.Split(view, "\n") {
					require.LessOrEqual(t, lipgloss.Width(line), size[0])
				}
			}
			if layout == tableLayout {
				index, ok := model.mouseListIndex(tea.MouseEvent{X: 8, Y: 5}, len(model.projects))
				require.True(t, ok)
				require.Greater(t, index, 0, "mouse hit testing must account for scroll offset")
			}
		})
	}
}

func TestCellAwareTruncationAndPadding(t *testing.T) {
	for _, value := range []string{"界面任务", "👩‍💻 ship", "e\u0301cole", "plain"} {
		for width := 1; width < 10; width++ {
			short := truncate(value, width)
			require.LessOrEqual(t, ansi.StringWidth(short), width)
			require.Equal(t, width, ansi.StringWidth(padRight(short, width)))
		}
	}
}

func TestOldBoardLoadCannotOverwriteNewNavigation(t *testing.T) {
	model := testModel(readRepository{columns: []domain.Column{{ID: "result"}}})
	model.project = &domain.Project{ID: "project"}
	model.boards = []domain.Board{{ID: "first"}, {ID: "second"}}
	_, old := model.openSelectedBoard()
	model.boardIndex = 1
	_, latest := model.openSelectedBoard()
	model.Update(old())
	require.Empty(t, model.columns)
	require.True(t, model.loading)
	runCommands(model, latest)
	require.Len(t, model.columns, 1)
	require.False(t, model.loading)
}

func TestExternalRefreshKeepsWorkspaceVisible(t *testing.T) {
	model, _, card := cardEditorModel(t)
	model.form = nil
	_, _ = model.Update(ExternalChangeMsg{Action: "card updated", BoardID: card.BoardID, CardID: card.ID})
	view := model.View()
	require.Contains(t, view, card.Title)
	require.NotContains(t, view, "Loading...")
}

func TestResponsiveWorkspaceInspectorNavigationAndFocus(t *testing.T) {
	model, _, card := cardEditorModel(t)
	model.form = nil
	model.projects = []domain.Project{*model.project}
	model.boards = []domain.Board{*model.board}
	model.Update(tea.WindowSizeMsg{Width: 160, Height: 36})
	view := ansi.Strip(model.View())
	require.Contains(t, strings.Split(view, "\n")[0], "Roadmap")
	require.Contains(t, view, "Boards")
	require.Contains(t, view, "Inspector")
	require.Contains(t, view, "Write the")
	_, command := model.Update(key("tab"))
	require.Nil(t, command, "Tab changes pane focus without moving cards")
	model.Update(key("j"))
	require.Equal(t, card.ID, model.selectedCard().ID)
	model.Update(key("i"))
	require.Zero(t, model.workspaceLayout(160, 36).inspectorWidth)
	model.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	view = ansi.Strip(model.View())
	require.Zero(t, model.workspaceLayout(60, 18).inspectorWidth)
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 60)
	}
}

func TestTinyTerminalShowsBoundedResizeMessage(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.Update(tea.WindowSizeMsg{Width: 24, Height: 8})
	view := model.View()
	require.Contains(t, view, "Resize")
	require.LessOrEqual(t, lipgloss.Height(view), 8)
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 24)
	}
}

func TestLateLinkCandidatesDoNotChangeAnotherForm(t *testing.T) {
	model, repo, card := cardEditorModel(t)
	other := domain.Card{BoardID: card.BoardID, ColumnID: card.ColumnID, Title: "Link candidate", Position: 2048}
	require.NoError(t, repo.CreateCard(context.Background(), &other))
	old := model.startCardForm(true)
	model.startProjectForm(false)
	model.Update(old())
	require.Empty(t, model.form.linkCandidates)
}

func TestLateSearchFromAnotherBoardIsIgnored(t *testing.T) {
	card := domain.Card{ID: "old-card", BoardID: "old-board", ColumnID: "column", Title: "needle"}
	repo := &searchRepository{readRepository: readRepository{cards: []domain.Card{card}}, results: []domain.Card{card}}
	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "old-board"}
	model.columns = []domain.Column{{ID: "column"}}
	model.filterMode = true
	_, old := model.Update(key("needle"))
	model.clearBoardFilter()
	model.board = &domain.Board{ID: "new-board"}
	model.filterMode = true
	model.Update(key("needle"))
	model.Update(old())
	require.Empty(t, model.filteredCards)
}

func TestMouseUsesResponsivePaneBounds(t *testing.T) {
	model, _, card := cardEditorModel(t)
	model.form = nil
	card.Description = strings.Repeat("Long description line\n", 100)
	model.cards[card.ColumnID] = []domain.Card{card}
	model.boards = []domain.Board{*model.board, {ID: "another", Name: "Another board"}}
	model.Update(tea.WindowSizeMsg{Width: 160, Height: 36})
	layout := model.workspaceLayout(160, 36)
	model.Update(mousePress(layout.inspectorX+3, 8, tea.MouseButtonWheelDown))
	require.Positive(t, model.workspace.inspectorOffset)
	model.Update(mousePress(layout.inspectorX+3, 8, tea.MouseButtonLeft))
	require.Equal(t, inspectorFocus, model.workspace.focus)
	model.Update(mousePress(5, 7, tea.MouseButtonLeft))
	require.Equal(t, navigationFocus, model.workspace.focus)
	require.Equal(t, 1, model.workspace.navigationIndex)
	_, command := model.Update(mousePress(5, 7, tea.MouseButtonLeft))
	require.NotNil(t, command)
	require.Equal(t, "another", model.board.ID)
}

func TestMonochromeStillIdentifiesTheSelectedCard(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, _, _ := cardEditorModel(t)
	model.form = nil
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := model.View()
	require.NotContains(t, view, "\x1b[")
	require.Contains(t, view, "> H [tui] Ship editor")
}

func TestLegacyTabKeymapStillMovesCards(t *testing.T) {
	model, repo, card := cardEditorModel(t)
	model.form = nil
	model.legacyKeymap = true
	column := domain.Column{BoardID: card.BoardID, Name: "Done", Position: 2048}
	require.NoError(t, repo.CreateColumn(context.Background(), &column))
	model.columns = append(model.columns, column)
	_, command := model.Update(key("tab"))
	require.NotNil(t, command)
	runCommands(model, command)
	stored, err := repo.GetCard(context.Background(), card.ID)
	require.NoError(t, err)
	require.Equal(t, column.ID, stored.ColumnID)
}

func TestCompactEditorScrollsFocusedFieldIntoView(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	model.form.focus = 7
	view := ansi.Strip(model.View())
	require.Contains(t, view, "Checklist:")
	require.LessOrEqual(t, lipgloss.Height(view), 12)
}

func TestAgentBoardRenameRefreshesTheBreadcrumb(t *testing.T) {
	model, repo, card := cardEditorModel(t)
	model.form = nil
	board := *model.board
	board.Name = "Renamed board"
	require.NoError(t, repo.UpdateBoard(context.Background(), &board))
	_, command := model.Update(ExternalChangeMsg{Action: "board updated", BoardID: card.BoardID})
	runCommands(model, command)
	require.Contains(t, strings.Split(ansi.Strip(model.View()), "\n")[0], "Renamed board")
}

func TestProjectRefreshAndRecoverableErrorKeepTheListVisible(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.projects = []domain.Project{{ID: "project", Name: "Visible project"}}
	model.Update(ExternalChangeMsg{Action: "board created"})
	require.Contains(t, model.View(), "Visible project")
	model.loading = false
	model.err = fmt.Errorf("temporary read failure")
	view := model.View()
	require.Contains(t, view, "Visible project")
	require.Contains(t, view, "temporary read failure")
}

func TestLongDropdownAndRelatedCardListsFollowSelection(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model.form.focus = 2
	model.form.fields[2].options = nil
	for index := 1; index <= 40; index++ {
		model.form.fields[2].options = append(model.form.fields[2].options, fmt.Sprintf("Option%03d", index))
	}
	model.Update(key("enter"))
	model.form.control.selection = 39
	view := ansi.Strip(model.View())
	require.Contains(t, view, "Option040")
	require.LessOrEqual(t, lipgloss.Height(view), 24)
	model.form.control = nil
	model.form.focus = 6
	model.form.linksLoading = false
	for index := 1; index <= 40; index++ {
		model.form.linkCandidates = append(model.form.linkCandidates, linkCandidate{id: fmt.Sprint(index), label: fmt.Sprintf("Related%03d", index)})
	}
	model.Update(key("enter"))
	model.form.control.selection = 39
	view = ansi.Strip(model.View())
	require.Contains(t, view, "Related040")
	require.LessOrEqual(t, lipgloss.Height(view), 24)
}

func TestRenderedPanesMatchTheirMouseGeometry(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.form = nil
	require.Equal(t, 31, lipgloss.Width(model.renderColumn(0, 31, 24)))
	require.Equal(t, 24, lipgloss.Width(model.renderNavigation(24, 24)))
	require.Equal(t, 38, lipgloss.Width(model.renderInspector(38, 24)))
}

func TestNavigationPlanningViewsCanBeSelectedWithTheKeyboard(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.form = nil
	model.boards = []domain.Board{*model.board}
	model.Update(tea.WindowSizeMsg{Width: 160, Height: 36})
	model.workspace.focus = navigationFocus
	model.Update(key("G"))
	model.Update(key("enter"))
	require.Equal(t, planningBlocked, model.planningFilter)
}

func TestInspectorCanScrollToTheLastLineAndKeepsItsScrollHint(t *testing.T) {
	model, _, card := cardEditorModel(t)
	model.form = nil
	card.Description = strings.Repeat("A description line\n", 80)
	model.cards[card.ColumnID] = []domain.Card{card}
	model.Update(tea.WindowSizeMsg{Width: 160, Height: 36})
	model.workspace.focus = inspectorFocus
	model.Update(key("G"))
	view := ansi.Strip(model.renderInspector(38, 32))
	require.Contains(t, view, "Full details and IDs")
	require.Contains(t, view, "e edit · i hide")
}
