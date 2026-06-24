package app

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type commandSpec struct {
	name        string
	description string
}

type paletteMatch struct {
	kind        string
	label       string
	description string
	score       int
	command     string
	item        *paletteItem
}

var commandCatalog = []commandSpec{
	{name: "projects", description: "Go to the project table"},
	{name: "boards", description: "Go to the current project's boards"},
	{name: "reload", description: "Reload the current screen"},
	{name: "help", description: "Open keyboard help"},
	{name: "quit", description: "Exit kan"},
	{name: "add", description: "Add an object on the current screen"},
	{name: "edit", description: "Edit the selected object"},
	{name: "delete", description: "Delete the selected object with confirmation"},
	{name: "add-column", description: "Add a column on the current board"},
	{name: "settings", description: "Open base kan settings"},
	{name: "sort", description: "Cycle card sorting on the current board"},
	{name: "group", description: "Cycle card grouping on the current board"},
	{name: "layout", description: "Toggle project and board list layout"},
	{name: "layout table", description: "Show projects and boards as tables"},
	{name: "layout cards", description: "Show projects and boards as card lists"},
}

func (model *Model) paletteMatches() []paletteMatch {
	query := strings.ToLower(strings.TrimSpace(model.command))
	matches := make([]paletteMatch, 0, len(commandCatalog)+len(model.paletteItems))
	for _, command := range commandCatalog {
		score := fuzzyScore(query, strings.ToLower(command.name+" "+command.description))
		if score < 0 {
			continue
		}
		if query == strings.ToLower(command.name) {
			score += 200
		} else if strings.HasPrefix(strings.ToLower(command.name), query) {
			score += 100
		}
		matches = append(matches, paletteMatch{kind: "command", label: command.name, description: command.description, score: score, command: command.name})
	}
	if query != "" {
		for index := range model.paletteItems {
			item := &model.paletteItems[index]
			score := fuzzyScore(query, strings.ToLower(item.searchText))
			if score < 0 {
				continue
			}
			if strings.HasPrefix(strings.ToLower(item.label), query) {
				score += 80
			}
			matches = append(matches, paletteMatch{kind: string(item.kind), label: item.label, description: item.description, score: score, item: item})
		}
	}
	sort.SliceStable(matches, func(left, right int) bool {
		return matches[left].score > matches[right].score
	})
	return matches
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

func (model *Model) renderCommandPalette(width, height int) string {
	boxWidth := min(76, max(width-4, 24))
	innerWidth := max(boxWidth-6, 18)
	contentWidth := max(innerWidth-4, 14)
	lines := []string{
		model.styles.header.Render("Search & Commands"),
		model.styles.command.Render(":" + model.command + "█"),
		model.styles.subtle.Render(truncate("fuzzy search commands and data · ↑/↓ select · Enter open · Esc close", contentWidth)),
		"",
	}
	matches := model.paletteMatches()
	if model.paletteLoading {
		lines = append(lines, model.styles.subtle.Render(truncate("Indexing projects, boards, columns, and cards...", contentWidth)))
	}
	if model.paletteErr != nil {
		lines = append(lines, model.styles.error.Render("Search index: "+model.paletteErr.Error()))
	}
	if len(matches) == 0 && !model.paletteLoading {
		lines = append(lines, model.styles.error.Render("No matching commands or data"))
	} else {
		selected := clampIndex(model.commandIndex, len(matches))
		maxRows := max(1, height-10)
		start := max(0, selected-maxRows+1)
		for index := start; index < min(start+maxRows, len(matches)); index++ {
			match := matches[index]
			kindWidth := 9
			nameWidth := min(22, max(contentWidth/3, 8))
			prefix := padRight("["+match.kind+"]", kindWidth)
			rowWidth := max(contentWidth-2, 1)
			row := prefix + " " + padRight(truncate(match.label, nameWidth), nameWidth) + "  " + truncate(match.description, max(rowWidth-kindWidth-nameWidth-3, 1))
			if index == selected {
				lines = append(lines, model.styles.selected.Copy().Padding(0).Width(contentWidth).Render("> "+row))
			} else {
				lines = append(lines, "  "+row)
			}
		}
	}
	palette := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, palette)
}
