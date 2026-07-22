package app

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type commandSpec struct {
	name        string
	description string
	command     string
}

type commandMenuSpec struct {
	name        string
	description string
	actions     []commandSpec
}

type paletteMatch struct {
	kind        string
	label       string
	description string
	score       int
	command     string
	menu        string
}

var commandMenus = []commandMenuSpec{
	{name: "card", description: "Card actions and templates", actions: []commandSpec{
		{name: "add", description: "Add a card to the selected column", command: "card-add"},
		{name: "edit", description: "Edit the selected card", command: "card-edit"},
		{name: "archive", description: "Archive the selected card with confirmation", command: "card-archive"},
		{name: "move", description: "Choose a destination column", command: "move"},
		{name: "undo", description: "Undo the last card move or reorder", command: "undo"},
		{name: "from-template", description: "Create a card from a board template", command: "template"},
		{name: "new-template", description: "Create a board-only card template", command: "new-template"},
		{name: "save-template", description: "Save the selected card as a template", command: "save-template"},
		{name: "templates", description: "List templates on the current board", command: "templates"},
	}},
	{name: "column", description: "Column actions and configuration", actions: []commandSpec{
		{name: "add", description: "Add a column to the current board", command: "add-column"},
		{name: "settings", description: "Configure name, WIP limit, and archiving", command: "column-settings"},
		{name: "delete", description: "Delete the selected column with confirmation", command: "delete-column"},
		{name: "archive", description: "Archive all active cards in the selected column", command: "archive"},
		{name: "move-left", description: "Move the selected column left", command: "move-column-left"},
		{name: "move-right", description: "Move the selected column right", command: "move-column-right"},
	}},
	{name: "board", description: "Board navigation and actions", actions: []commandSpec{
		{name: "list", description: "Go to the current project's boards", command: "boards"},
		{name: "add", description: "Add a board to the current project", command: "board-add"},
		{name: "edit", description: "Edit the selected board", command: "board-edit"},
		{name: "delete", description: "Delete the selected board with confirmation", command: "board-delete"},
	}},
	{name: "project", description: "Project navigation and actions", actions: []commandSpec{
		{name: "list", description: "Go to the project list", command: "projects"},
		{name: "add", description: "Add a project", command: "project-add"},
		{name: "edit", description: "Edit the selected project", command: "project-edit"},
		{name: "delete", description: "Delete the selected project with confirmation", command: "project-delete"},
	}},
	{name: "settings", description: "Application controls and settings", actions: []commandSpec{
		{name: "open", description: "Open base Kan settings", command: "settings"},
		{name: "reload", description: "Reload the current screen", command: "reload"},
		{name: "help", description: "Open keyboard help", command: "help"},
		{name: "quit", description: "Exit Kan", command: "quit"},
	}},
	{name: "view", description: "Filters, planning, sorting, and layout", actions: []commandSpec{
		{name: "today", description: "Show cards due today", command: "today"},
		{name: "overdue", description: "Show overdue cards", command: "overdue"},
		{name: "blocked", description: "Show blocked cards", command: "blocked"},
		{name: "stale", description: "Show stale cards", command: "stale"},
		{name: "untriaged", description: "Show cards without priority and due date", command: "untriaged"},
		{name: "archived", description: "Show archived cards on the current board", command: "archived"},
		{name: "clear", description: "Clear the current filter or planning view", command: "clear-filter"},
		{name: "sort", description: "Cycle card sorting", command: "sort"},
		{name: "group", description: "Cycle card grouping", command: "group"},
		{name: "table", description: "Show projects and boards as tables", command: "layout table"},
		{name: "cards", description: "Show projects and boards as card grids", command: "layout cards"},
	}},
}

func commandMenu(name string) *commandMenuSpec {
	for index := range commandMenus {
		if commandMenus[index].name == name {
			return &commandMenus[index]
		}
	}
	return nil
}

func (model *Model) paletteContext() (*commandMenuSpec, string) {
	if menu := commandMenu(model.commandMenu); menu != nil {
		return menu, strings.ToLower(strings.TrimSpace(model.command))
	}
	input := strings.TrimLeft(model.command, " ")
	if separator := strings.IndexRune(input, ' '); separator >= 0 {
		name := strings.ToLower(strings.TrimSpace(input[:separator]))
		if menu := commandMenu(name); menu != nil {
			return menu, strings.ToLower(strings.TrimSpace(input[separator+1:]))
		}
	}
	return nil, strings.ToLower(strings.TrimSpace(model.command))
}

func (model *Model) paletteMatches() []paletteMatch {
	menu, query := model.paletteContext()
	if menu == nil {
		matches := make([]paletteMatch, 0, len(commandMenus))
		for _, candidate := range commandMenus {
			score := fuzzyScore(query, candidate.name)
			if score < 0 {
				continue
			}
			if query == candidate.name {
				score += 200
			} else if strings.HasPrefix(candidate.name, query) {
				score += 100
			}
			matches = append(matches, paletteMatch{kind: "menu", label: candidate.name, description: candidate.description, score: score, menu: candidate.name})
		}
		sortPaletteMatches(matches)
		return matches
	}

	matches := make([]paletteMatch, 0, len(menu.actions))
	for _, action := range menu.actions {
		score := fuzzyScore(query, action.name)
		if score < 0 {
			continue
		}
		if query == action.name {
			score += 200
		} else if strings.HasPrefix(action.name, query) {
			score += 100
		}
		matches = append(matches, paletteMatch{kind: "action", label: action.name, description: action.description, score: score, command: action.command})
	}
	sortPaletteMatches(matches)
	return matches
}

func sortPaletteMatches(matches []paletteMatch) {
	sort.SliceStable(matches, func(left, right int) bool {
		return matches[left].score > matches[right].score
	})
}

func fuzzyScore(query, candidate string) int {
	if query == "" {
		return 0
	}
	score := 0
	position := 0
	previous := -2
	for _, character := range query {
		found := strings.IndexRune(candidate[position:], character)
		if found < 0 {
			return -1
		}
		absolute := position + found
		score += 10 - min(found, 9)
		if absolute == previous+1 {
			score += 8
		}
		previous = absolute
		position = absolute + 1
	}
	return score
}

func (model *Model) commandPrompt() string {
	if model.commandMenu == "" {
		return ":" + model.command
	}
	return ":" + model.commandMenu + " " + model.command
}

func (model *Model) renderCommandPalette(width, height int) string {
	boxWidth := min(76, max(width-4, 24))
	innerWidth := max(boxWidth-6, 18)
	contentWidth := max(innerWidth-4, 14)
	menu, _ := model.paletteContext()
	title := "Command menus"
	hint := "type to filter menus · Enter open · Esc close"
	if menu != nil {
		title = strings.ToUpper(menu.name[:1]) + menu.name[1:] + " actions"
		hint = "type to filter actions · Enter run · Esc back"
	}
	lines := []string{
		model.styles.header.Render(title),
		model.styles.command.Render(textViewport(model.commandPrompt(), model.commandCursor+len([]rune(model.commandPrompt()))-len([]rune(model.command)), max(contentWidth-1, 1))),
		model.styles.subtle.Render(truncate(hint, contentWidth)),
		"",
	}
	matches := model.paletteMatches()
	if len(matches) == 0 {
		lines = append(lines, model.styles.error.Render("No matching menu action"))
	} else {
		selected := clampIndex(model.commandIndex, len(matches))
		maxRows := max(1, height-10)
		start := max(0, selected-maxRows+1)
		for index := start; index < min(start+maxRows, len(matches)); index++ {
			match := matches[index]
			kindWidth := 9
			nameWidth := min(22, max(contentWidth/3, 8))
			prefix := padRight("["+match.kind+"]", kindWidth)
			rowWidth := max(contentWidth, 1)
			row := prefix + " " + padRight(truncate(match.label, nameWidth), nameWidth) + "  " + truncate(match.description, max(rowWidth-kindWidth-nameWidth-3, 1))
			if index == selected {
				lines = append(lines, model.styles.selected.Copy().Padding(0).Width(contentWidth).Render(row))
			} else {
				lines = append(lines, row)
			}
		}
	}
	palette := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, palette)
}
