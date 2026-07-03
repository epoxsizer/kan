package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type tableRow struct {
	name     string
	comments string
	status   string
	items    int
	selected bool
}

func (model *Model) renderTable(title, itemHeader string, rows []tableRow, width int) string {
	width = max(width, 24)
	hasStatus := false
	for _, row := range rows {
		if row.status != "" {
			hasStatus = true
			break
		}
	}
	if hasStatus {
		return model.renderStatusTable(title, itemHeader, rows, width)
	}
	innerWidth := max(width-4, 20)
	contentWidth := max(innerWidth-2, 12)
	available := max(contentWidth-tableSeparatorWidth(), 3)
	itemWidth := min(max(len(itemHeader), 5), max(1, available-4))
	remaining := max(available-itemWidth, 2)
	nameWidth := min(28, max(remaining/3, 2))
	commentWidth := max(remaining-nameWidth, 1)

	header := tableLine("NAME", "COMMENTS", itemHeader, nameWidth, commentWidth, itemWidth)
	lines := []string{
		model.styles.tableHeader.Render(header),
		model.styles.subtle.Render(strings.Repeat("─", lipgloss.Width(header))),
	}
	for _, row := range rows {
		comments := strings.Join(strings.Fields(row.comments), " ")
		line := tableLine(row.name, comments, fmt.Sprintf("%d", row.items), nameWidth, commentWidth, itemWidth)
		if row.selected {
			line = model.styles.selectedCard.Copy().Padding(0).Width(lipgloss.Width(header)).Render(line)
		}
		lines = append(lines, line)
	}
	table := model.styles.panel.Copy().Width(innerWidth).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{model.styles.header.Render(title), table}, "\n")
}

func (model *Model) renderStatusTable(title, itemHeader string, rows []tableRow, width int) string {
	innerWidth := max(width-4, 20)
	contentWidth := max(innerWidth-2, 12)
	if width < 64 {
		available := max(contentWidth-tableSeparatorWidth(), 3)
		itemWidth := min(max(len(itemHeader), 5), max(1, available-4))
		remaining := max(available-itemWidth, 2)
		nameWidth := min(28, max(remaining/2, 2))
		statusWidth := max(remaining-nameWidth, 1)
		header := tableLine("NAME", "DUE", itemHeader, nameWidth, statusWidth, itemWidth)
		lines := []string{model.styles.tableHeader.Render(header), model.styles.subtle.Render(strings.Repeat("─", lipgloss.Width(header)))}
		for _, row := range rows {
			line := tableLine(row.name, row.status, fmt.Sprintf("%d", row.items), nameWidth, statusWidth, itemWidth)
			if row.selected {
				line = model.styles.selectedCard.Copy().Padding(0).Width(lipgloss.Width(header)).Render(line)
			}
			lines = append(lines, line)
		}
		table := model.styles.panel.Copy().Width(innerWidth).Render(strings.Join(lines, "\n"))
		return strings.Join([]string{model.styles.header.Render(title), table}, "\n")
	}

	available := max(contentWidth-(3*lipgloss.Width("  "+tableSeparator()+"  ")), 4)
	itemWidth := min(max(len(itemHeader), 5), max(1, available-6))
	statusWidth := min(14, max(8, available/5))
	remaining := max(available-itemWidth-statusWidth, 2)
	nameWidth := min(24, max((remaining*2)/5, 16))
	nameWidth = min(nameWidth, max(remaining-1, 1))
	commentWidth := max(remaining-nameWidth, 1)
	header := statusTableLine("NAME", "COMMENTS", "DUE", itemHeader, nameWidth, commentWidth, statusWidth, itemWidth)
	lines := []string{model.styles.tableHeader.Render(header), model.styles.subtle.Render(strings.Repeat("─", lipgloss.Width(header)))}
	for _, row := range rows {
		comments := strings.Join(strings.Fields(row.comments), " ")
		line := statusTableLine(row.name, comments, row.status, fmt.Sprintf("%d", row.items), nameWidth, commentWidth, statusWidth, itemWidth)
		if row.selected {
			line = model.styles.selectedCard.Copy().Padding(0).Width(lipgloss.Width(header)).Render(line)
		}
		lines = append(lines, line)
	}
	table := model.styles.panel.Copy().Width(innerWidth).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{model.styles.header.Render(title), table}, "\n")
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
		Border(lipgloss.DoubleBorder()).
		BorderForeground(model.styles.selectedColumnBorder).
		Foreground(model.styles.selectedColumnForeground).
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
			titleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(model.styles.selectedColumnForeground).
				Background(model.styles.selectedColumnBackground).
				Width(contentWidth)
		}
		bodyLines := []string{
			titleStyle.Render(truncate(row.name, contentWidth)),
			model.styles.subtle.Render(truncate(comments, contentWidth)),
			fmt.Sprintf("%s: %d", itemLabel, row.items),
		}
		if row.status != "" {
			bodyLines = append(bodyLines, model.styles.subtle.Render("Due: "+truncate(row.status, max(contentWidth-5, 1))))
		}
		body := strings.Join(bodyLines, "\n")
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

func tableLine(name, comments, items string, nameWidth, commentWidth, itemWidth int) string {
	return padRight(truncate(name, nameWidth), nameWidth) +
		"  " + tableSeparator() + "  " + padRight(truncate(comments, commentWidth), commentWidth) +
		"  " + tableSeparator() + "  " + fmt.Sprintf("%*s", itemWidth, truncate(items, itemWidth))
}

func statusTableLine(name, comments, status, items string, nameWidth, commentWidth, statusWidth, itemWidth int) string {
	separator := "  " + tableSeparator() + "  "
	return padRight(truncate(name, nameWidth), nameWidth) +
		separator + padRight(truncate(comments, commentWidth), commentWidth) +
		separator + padRight(truncate(status, statusWidth), statusWidth) +
		separator + fmt.Sprintf("%*s", itemWidth, truncate(items, itemWidth))
}

func tableSeparatorWidth() int {
	return 2 * lipgloss.Width("  "+tableSeparator()+"  ")
}

func tableSeparator() string {
	return "│"
}
