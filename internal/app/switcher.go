package app

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
)

// switcherMatch is one jump target in the global quick-switcher.
type switcherMatch struct {
	kind        string
	label       string
	description string
	projectID   string
	boardID     string
	columnID    string
	cardID      string
}

type switcherState struct {
	query   string
	cursor  int
	matches []switcherMatch
	loading bool
}

type switcherLoadedMsg struct {
	matches []switcherMatch
	err     error
}

func (model *Model) openSwitcher() (tea.Model, tea.Cmd) {
	model.switcher = &switcherState{loading: true}
	return model, model.loadSwitcherCandidates()
}

// loadSwitcherCandidates gathers every project, board, and active card so the
// switcher can jump across the whole database.
func (model *Model) loadSwitcherCandidates() tea.Cmd {
	return func() tea.Msg {
		matches := []switcherMatch{}
		projects, err := model.repo.ListProjects(model.ctx)
		if err != nil {
			return switcherLoadedMsg{err: err}
		}
		for _, project := range projects {
			matches = append(matches, switcherMatch{kind: "project", label: project.Name, description: "Project", projectID: project.ID})
			boards, err := model.repo.ListBoards(model.ctx, project.ID)
			if err != nil {
				return switcherLoadedMsg{err: err}
			}
			for _, board := range boards {
				matches = append(matches, switcherMatch{kind: "board", label: board.Name, description: project.Name, projectID: project.ID, boardID: board.ID})
				cards, err := model.repo.ListCards(model.ctx, board.ID)
				if err != nil {
					return switcherLoadedMsg{err: err}
				}
				for _, card := range cards {
					matches = append(matches, switcherMatch{
						kind: "card", label: card.Title, description: project.Name + " / " + board.Name,
						projectID: project.ID, boardID: board.ID, columnID: card.ColumnID, cardID: card.ID,
					})
				}
			}
		}
		return switcherLoadedMsg{matches: matches}
	}
}

func (state *switcherState) visible() []switcherMatch {
	query := strings.ToLower(strings.TrimSpace(state.query))
	matches := state.matches
	if query != "" {
		matches = []switcherMatch{}
		for _, match := range state.matches {
			if fuzzyScore(query, strings.ToLower(match.label)) >= 0 {
				matches = append(matches, match)
			}
		}
		sort.SliceStable(matches, func(left, right int) bool {
			return fuzzyScore(query, strings.ToLower(matches[left].label)) > fuzzyScore(query, strings.ToLower(matches[right].label))
		})
	}
	if len(matches) > 50 {
		matches = matches[:50]
	}
	return matches
}

func (model *Model) handleSwitcherKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := model.switcher
	switch key.String() {
	case "esc", "ctrl+p":
		model.switcher = nil
		return model, nil
	case "up":
		state.cursor = clampIndex(state.cursor-1, len(state.visible()))
		return model, nil
	case "down":
		state.cursor = clampIndex(state.cursor+1, len(state.visible()))
		return model, nil
	case "enter":
		matches := state.visible()
		if len(matches) == 0 {
			return model, nil
		}
		match := matches[clampIndex(state.cursor, len(matches))]
		model.switcher = nil
		return model.jumpToMatch(match)
	case "backspace":
		runes := []rune(state.query)
		if len(runes) > 0 {
			state.query = string(runes[:len(runes)-1])
			state.cursor = 0
		}
		return model, nil
	}
	if len(key.Runes) > 0 {
		state.query += string(key.Runes)
		state.cursor = 0
	}
	return model, nil
}

func (model *Model) jumpToMatch(match switcherMatch) (tea.Model, tea.Cmd) {
	switch match.kind {
	case "project":
		for index, project := range model.projects {
			if project.ID == match.projectID {
				model.projectIndex = index
				return model.openSelectedProject()
			}
		}
	case "board", "card":
		var project *domain.Project
		for index := range model.projects {
			if model.projects[index].ID == match.projectID {
				project = &model.projects[index]
			}
		}
		if project == nil {
			break
		}
		if match.kind == "card" {
			model.pendingColumn, model.pendingCard = match.columnID, match.cardID
		}
		if model.project != nil && model.project.ID == match.projectID {
			for index, board := range model.boards {
				if board.ID == match.boardID {
					model.boardIndex = index
					return model.openSelectedBoard()
				}
			}
		}
		model.project = project
		model.pendingSwitcherBoard = match.boardID
		model.screen = boardsScreen
		model.loading = true
		return model, model.loadBoards(match.projectID)
	}
	model.notice = "Navigation target is no longer available"
	return model, nil
}

func (model *Model) renderSwitcher(width, height int) string {
	boxWidth := min(76, max(width-4, 24))
	innerWidth := max(boxWidth-6, 18)
	contentWidth := max(innerWidth-4, 14)
	state := model.switcher
	lines := []string{
		model.styles.header.Render("Jump to project, board, or card"),
		model.styles.command.Render(textViewport("> "+state.query, len([]rune(state.query))+2, max(contentWidth-1, 1))),
		model.styles.subtle.Render(truncate("type to filter · Enter jump · Esc close", contentWidth)),
		"",
	}
	if state.loading {
		lines = append(lines, "Loading…")
	}
	matches := state.visible()
	if len(matches) == 0 && !state.loading {
		lines = append(lines, model.styles.error.Render("No matches"))
	}
	maxRows := max(1, height-10)
	selected := clampIndex(state.cursor, len(matches))
	start := max(0, selected-maxRows+1)
	for index := start; index < min(start+maxRows, len(matches)); index++ {
		match := matches[index]
		row := padRight("["+match.kind+"]", 9) + " " + padRight(truncate(match.label, 24), 24) + "  " + truncate(match.description, max(contentWidth-36, 1))
		row = truncate(selectionText(row, index == selected), contentWidth)
		if index == selected {
			lines = append(lines, model.styles.selected.Copy().Padding(0).Width(contentWidth).Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	popup := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}
