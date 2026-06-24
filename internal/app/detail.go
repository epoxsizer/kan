package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
)

type detailPopup struct {
	kind  string
	title string
	lines []string
}

func (model *Model) openSelectedDetail() {
	switch model.screen {
	case projectsScreen:
		if len(model.projects) == 0 {
			return
		}
		project := model.projects[clampIndex(model.projectIndex, len(model.projects))]
		model.detail = projectDetail(project, model.projectCounts[project.ID])
	case boardsScreen:
		if len(model.boards) == 0 {
			return
		}
		board := model.boards[clampIndex(model.boardIndex, len(model.boards))]
		model.detail = boardDetail(board, model.boardCounts[board.ID])
	case boardScreen:
		if len(model.columns) == 0 {
			if model.board != nil {
				model.detail = boardDetail(*model.board, model.cardCount())
			}
			return
		}
		column := model.columns[clampIndex(model.columnIndex, len(model.columns))]
		cards := model.visibleCards(column.ID)
		if len(cards) == 0 {
			model.detail = columnDetail(column, 0)
			return
		}
		card := cards[clampIndex(model.cardIndexes[column.ID], len(cards))]
		model.detail = cardDetail(card, column.Name)
	}
}

func projectDetail(project domain.Project, boardCount int) *detailPopup {
	return &detailPopup{kind: "project", title: project.Name, lines: []string{
		"ID: " + project.ID,
		"Comments: " + fallbackValue(project.Description),
		fmt.Sprintf("Boards: %d", boardCount),
		fmt.Sprintf("Position: %g", project.Position),
		"Updated: " + formatDetailTime(project.UpdatedAt),
	}}
}

func boardDetail(board domain.Board, cardCount int) *detailPopup {
	return &detailPopup{kind: "board", title: board.Name, lines: []string{
		"ID: " + board.ID,
		"Project ID: " + board.ProjectID,
		"Comments: " + fallbackValue(board.Description),
		fmt.Sprintf("Cards: %d", cardCount),
		fmt.Sprintf("Position: %g", board.Position),
		"Updated: " + formatDetailTime(board.UpdatedAt),
	}}
}

func columnDetail(column domain.Column, cardCount int) *detailPopup {
	wipLimit := "none"
	if column.WIPLimit != nil {
		wipLimit = fmt.Sprintf("%d", *column.WIPLimit)
	}
	color := "none"
	if column.Color != nil && *column.Color != "" {
		color = *column.Color
	}
	return &detailPopup{kind: "column", title: column.Name, lines: []string{
		"ID: " + column.ID,
		"Board ID: " + column.BoardID,
		fmt.Sprintf("Cards: %d", cardCount),
		"WIP limit: " + wipLimit,
		"Color: " + color,
		fmt.Sprintf("Position: %g", column.Position),
	}}
}

func cardDetail(card domain.Card, columnName string) *detailPopup {
	priority := "none"
	if card.Priority != nil && *card.Priority != "" {
		priority = *card.Priority
	}
	due := "none"
	if card.DueDate != nil {
		due = card.DueDate.Format("2006-01-02")
		if isOverdueDate(card.DueDate) {
			due += " (! overdue)"
		}
	}
	lines := []string{
		"ID: " + card.ID,
		"Status: " + columnName,
		"Comments: " + fallbackValue(card.Description),
		"Priority: " + priority,
		"Due: " + due,
		"Tags: " + fallbackValue(strings.Join(card.Tags, ", ")),
		"Related cards: " + fallbackValue(strings.Join(card.RelatedCardIDs, ", ")),
	}
	if len(card.Checklist) > 0 {
		lines = append(lines, "Checklist:")
		for _, item := range card.Checklist {
			mark := "[ ]"
			if item.Done {
				mark = "[x]"
			}
			lines = append(lines, "  "+mark+" "+item.Text)
		}
	}
	if len(card.Fields) > 0 {
		keys := make([]string, 0, len(card.Fields))
		for key := range card.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines = append(lines, "Fields:")
		for _, key := range keys {
			field := card.Fields[key]
			value, err := json.Marshal(field.Value)
			if err != nil {
				value = []byte("<invalid>")
			}
			lines = append(lines, fmt.Sprintf("  %s [%s]: %s", key, field.Type, value))
		}
	}
	lines = append(lines, "Updated: "+formatDetailTime(card.UpdatedAt))
	return &detailPopup{kind: "card", title: card.Title, lines: lines}
}

func (model *Model) renderDetailPopup(width, height int) string {
	boxWidth := min(70, max(width-4, 24))
	innerWidth := max(boxWidth-6, 18)
	contentWidth := max(innerWidth-4, 14)
	lines := []string{
		model.styles.header.Render(model.detail.title),
		model.styles.subtle.Render(strings.ToUpper(model.detail.kind)),
		"",
	}
	maxDetails := max(height-10, 1)
	for index, line := range model.detail.lines {
		if index >= maxDetails {
			lines = append(lines, model.styles.subtle.Render("…"))
			break
		}
		lines = append(lines, truncate(line, contentWidth))
	}
	hint := "Esc / d / Enter close"
	if model.detail.kind == "card" {
		hint += " · e edit"
	}
	lines = append(lines, "", model.styles.subtle.Render(hint))
	popup := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}

func fallbackValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func formatDetailTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Local().Format("2006-01-02 15:04")
}
