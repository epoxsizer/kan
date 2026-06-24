package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type tableRow struct {
	name     string
	comments string
	items    int
	selected bool
}

func (model *Model) renderTable(title, itemHeader string, rows []tableRow, width int) string {
	width = max(width, 20)
	itemWidth := max(len(itemHeader), 5)
	nameWidth := min(28, max(width/3, 8))
	commentWidth := width - 2 - nameWidth - itemWidth - 4
	if commentWidth < 4 {
		commentWidth = 4
		nameWidth = max(width-2-itemWidth-commentWidth-4, 4)
	}

	header := padRight("NAME", nameWidth) + "  " + padRight("COMMENTS", commentWidth) + "  " + fmt.Sprintf("%*s", itemWidth, itemHeader)
	lines := []string{
		model.styles.header.Render(title),
		model.styles.tableHeader.Render("  " + header),
	}
	for _, row := range rows {
		comments := strings.Join(strings.Fields(row.comments), " ")
		body := padRight(truncate(row.name, nameWidth), nameWidth) + "  " +
			padRight(truncate(comments, commentWidth), commentWidth) + "  " +
			fmt.Sprintf("%*d", itemWidth, row.items)
		line := "  " + body
		if row.selected {
			line = model.styles.selected.Copy().Padding(0).Width(width).Render("> " + body)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (model *Model) renderListCards(title, itemLabel string, rows []tableRow, width int) string {
	width = max(width, 20)
	lines := []string{model.styles.header.Render(title)}
	columns := cardLayoutColumns(width)
	cardWidth := cardLayoutWidth(width)
	cardHeight := 7
	gap := 2
	if width < 44 {
		gap = 1
	}
	contentWidth := max(cardWidth-4, 12)
	normalStyle := model.styles.panel.Copy().Width(cardWidth - 2).Height(cardHeight - 2).MarginRight(gap)
	selectedStyle := model.styles.focusedPanel.Copy().
		BorderForeground(namedColors["green"]).
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(cardWidth - 2).
		Height(cardHeight - 2).
		MarginRight(gap)
	rowCards := []string{}
	for _, row := range rows {
		comments := strings.Join(strings.Fields(row.comments), " ")
		if comments == "" {
			comments = "No comments"
		}
		style := normalStyle
		titleStyle := model.styles.header
		if row.selected {
			style = selectedStyle
			titleStyle = lipgloss.NewStyle().Bold(true).Foreground(namedColors["green"])
		}
		body := strings.Join([]string{
			titleStyle.Render(truncate(row.name, contentWidth)),
			model.styles.subtle.Render(truncate(comments, contentWidth)),
			fmt.Sprintf("%s: %d", itemLabel, row.items),
		}, "\n")
		rowCards = append(rowCards, style.Render(body))
		if len(rowCards) == columns {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
			rowCards = nil
		}
	}
	if len(rowCards) > 0 {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
	}
	return strings.Join(lines, "\n")
}

func cardLayoutColumns(width int) int {
	width = max(width, 20)
	gap := 2
	if width < 44 {
		gap = 1
	}
	cardWidth := cardLayoutWidth(width)
	return max(1, (width+gap)/(cardWidth+gap))
}

func cardLayoutWidth(width int) int {
	width = max(width, 20)
	gap := 2
	if width < 44 {
		gap = 1
	}
	return min(24, max(18, width-gap))
}

func padRight(value string, width int) string {
	runes := []rune(value)
	if len(runes) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(runes))
}
