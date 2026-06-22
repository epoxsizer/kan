package app

import (
	"fmt"
	"strings"
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

func padRight(value string, width int) string {
	runes := []rune(value)
	if len(runes) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(runes))
}
