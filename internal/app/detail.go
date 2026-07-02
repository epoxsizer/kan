package app

import (
	"encoding/json"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/epoxsizer/kan/internal/domain"
)

type detailPopup struct {
	kind          string
	title         string
	lines         []string
	markdown      string
	markdownIndex int
	offset        int
	expanded      bool
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
		model.detail = boardDetail(board, model.boardCounts[board.ID], model.boardHealth[board.ID])
	case boardScreen:
		if len(model.columns) == 0 {
			if model.board != nil {
				model.detail = boardDetail(*model.board, model.cardCount(), summarizeBoardHealth(nil, time.Now()))
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

func boardDetail(board domain.Board, cardCount int, health boardHealth) *detailPopup {
	return &detailPopup{kind: "board", title: board.Name, lines: []string{
		"ID: " + board.ID,
		"Project ID: " + board.ProjectID,
		"Comments: " + fallbackValue(board.Description),
		fmt.Sprintf("Cards: %d", cardCount),
		"Due health: " + boardHealthLabel(health, time.Now()),
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
	archiving := "disabled"
	if column.AutoArchive {
		archiving = fmt.Sprintf("after %d days", column.ArchiveAfterDays)
	}
	return &detailPopup{kind: "column", title: column.Name, lines: []string{
		"ID: " + column.ID,
		"Board ID: " + column.BoardID,
		fmt.Sprintf("Cards: %d", cardCount),
		"WIP limit: " + wipLimit,
		"Color: " + color,
		"Auto archive: " + archiving,
		fmt.Sprintf("Position: %g", column.Position),
	}}
}

func cardDetail(card domain.Card, columnName string) *detailPopup {
	priority := "none"
	if card.Priority != nil && *card.Priority != "" {
		priority = *card.Priority
	}
	due := "No due date"
	if card.DueDate != nil {
		due = card.DueDate.Format("2006-01-02")
		if isOverdueDate(card.DueDate) {
			due += " (! overdue)"
		}
	}
	lines := []string{
		"ID: " + card.ID,
		"Status: " + columnName,
		"Priority: " + priority,
		"Due: " + due,
		"Tags: " + fallbackValue(strings.Join(card.Tags, ", ")),
		"Related cards: " + fallbackValue(strings.Join(card.RelatedCardIDs, ", ")),
	}
	markdownIndex := len(lines)
	if strings.TrimSpace(card.Description) != "" {
		lines = append(lines, "Description:")
		markdownIndex = len(lines)
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
	return &detailPopup{kind: "card", title: card.Title, lines: lines, markdown: card.Description, markdownIndex: markdownIndex}
}

func archivedCardsDetail(cards []domain.Card, columns []domain.Column) *detailPopup {
	columnNames := make(map[string]string, len(columns))
	for _, column := range columns {
		columnNames[column.ID] = column.Name
	}
	sort.SliceStable(cards, func(left, right int) bool {
		return cards[left].DeletedAt.After(*cards[right].DeletedAt)
	})
	lines := []string{fmt.Sprintf("Archived cards: %d", len(cards))}
	if len(cards) == 0 {
		lines = append(lines, "", "No archived cards on this board.")
	}
	for _, card := range cards {
		column := fallbackValue(columnNames[card.ColumnID])
		lines = append(lines, "", card.Title, fmt.Sprintf("  Column: %s · Archived: %s", column, card.DeletedAt.Local().Format("2006-01-02 15:04")), "  ID: "+card.ID)
	}
	return &detailPopup{kind: "archive", title: "Archived cards", lines: lines}
}

func (model *Model) renderDetailPopup(width, height int) string {
	layout := detailLayoutForSize(width, height, model.detail.expanded)
	header := []string{
		model.styles.header.Render(model.detail.title),
		model.styles.subtle.Render(strings.ToUpper(model.detail.kind)),
	}
	if layout.headerLines == 3 {
		header = append(header, "")
	}
	detailLines := model.detailLines(layout.contentWidth)
	model.detail.offset = clampDetailOffset(model.detail.offset, len(detailLines), layout.viewportHeight)
	start := model.detail.offset
	end := min(start+layout.viewportHeight, len(detailLines))
	lines := append([]string{}, header...)
	lines = append(lines, detailLines[start:end]...)
	for len(lines) < layout.insideHeight-1 {
		lines = append(lines, "")
	}
	hint := "Esc / d / Enter close"
	if model.detail.kind == "card" {
		hint += " · e edit"
	}
	if model.detail.expanded {
		hint += " · Shift+E compact"
	} else {
		hint += " · Shift+E expand"
	}
	hint += fmt.Sprintf(" · j/k scroll · PgUp/PgDn page · %d-%d/%d", min(start+1, len(detailLines)), end, len(detailLines))
	lines = append(lines, model.styles.subtle.Render(truncate(hint, layout.contentWidth)))
	if len(lines) > layout.insideHeight {
		lines = lines[:layout.insideHeight]
	}
	popup := model.styles.help.Width(layout.boxWidth).Render(strings.Join(lines, "\n"))
	if model.detail.expanded {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
	}
	return overlayCentered(model.renderWorkspace(width, height), popup, width, height)
}

func (model *Model) detailLines(width int) []string {
	lines := wrappedDetailLines(model.detail.lines, width)
	if strings.TrimSpace(model.detail.markdown) == "" {
		return lines
	}
	index := min(model.detail.markdownIndex, len(model.detail.lines))
	before := wrappedDetailLines(model.detail.lines[:index], width)
	after := wrappedDetailLines(model.detail.lines[index:], width)
	rendered, err := renderMarkdown(model.detail.markdown, width)
	if err != nil {
		rendered = wrappedDetailLines([]string{model.detail.markdown}, width)
	}
	return append(append(before, rendered...), after...)
}

type detailLayout struct {
	boxWidth       int
	contentWidth   int
	insideHeight   int
	headerLines    int
	viewportHeight int
}

func detailLayoutForSize(width, height int, expanded bool) detailLayout {
	width = max(width, 20)
	height = max(height, 6)
	insideHeight := max(height-4, 1)
	boxWidth := max(width-2, 14)
	if !expanded {
		insideHeight = min(insideHeight, 14)
		boxWidth = min(max(width-4, 14), 72)
	}
	headerLines := 2
	if insideHeight >= 6 {
		headerLines = 3
	}
	viewportHeight := max(insideHeight-headerLines-1, 0)
	return detailLayout{
		boxWidth:       boxWidth,
		contentWidth:   max(boxWidth-4, 10),
		insideHeight:   insideHeight,
		headerLines:    headerLines,
		viewportHeight: viewportHeight,
	}
}

func overlayCentered(base, popup string, width, height int) string {
	buffer := cellbuf.NewBuffer(width, height)
	cellbuf.SetContent(buffer, base)
	popupWidth := min(lipgloss.Width(popup), width)
	popupHeight := min(lipgloss.Height(popup), height)
	left := max((width-popupWidth)/2, 0)
	top := max((height-popupHeight)/2, 0)
	cellbuf.SetContentRect(buffer, popup, image.Rect(left, top, left+popupWidth, top+popupHeight))
	lines := make([]string, height)
	for row := range height {
		_, lines[row] = cellbuf.RenderLine(buffer, row)
	}
	return strings.Join(lines, "\n")
}

func clampDetailOffset(offset, totalLines, viewportHeight int) int {
	return min(max(offset, 0), max(totalLines-viewportHeight, 0))
}

func fallbackValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func wrappedDetailLines(values []string, width int) []string {
	lines := []string{}
	for _, value := range values {
		parts := strings.Split(value, "\n")
		for index, part := range parts {
			prefix := ""
			if index > 0 {
				prefix = "  "
			}
			lines = append(lines, wrapDetailLine(prefix+part, width)...)
		}
	}
	return lines
}

func wrapDetailLine(value string, width int) []string {
	width = max(width, 1)
	runes := []rune(value)
	if len(runes) == 0 {
		return []string{""}
	}
	lines := []string{}
	for len(runes) > width {
		breakAt := width
		consume := width
		for index := width; index > 0; index-- {
			if runes[index-1] == ' ' || runes[index-1] == '\t' {
				breakAt = index - 1
				consume = index
				break
			}
		}
		if breakAt <= 0 {
			breakAt = width
			consume = width
		}
		lines = append(lines, strings.TrimRight(string(runes[:breakAt]), " \t"))
		runes = runes[min(consume, len(runes)):]
	}
	lines = append(lines, string(runes))
	return lines
}

func (model *Model) scrollDetail(delta int) {
	if model.detail == nil {
		return
	}
	width, height := model.dimensions()
	layout := detailLayoutForSize(width, height, model.detail.expanded)
	total := len(model.detailLines(layout.contentWidth))
	model.detail.offset = clampDetailOffset(model.detail.offset+delta, total, layout.viewportHeight)
}

func (model *Model) clampDetailForCurrentLayout() {
	if model.detail == nil {
		return
	}
	width, height := model.dimensions()
	layout := detailLayoutForSize(width, height, model.detail.expanded)
	total := len(model.detailLines(layout.contentWidth))
	model.detail.offset = clampDetailOffset(model.detail.offset, total, layout.viewportHeight)
}

func (model *Model) scrollDetailToEnd() {
	if model.detail == nil {
		return
	}
	width, height := model.dimensions()
	layout := detailLayoutForSize(width, height, model.detail.expanded)
	total := len(model.detailLines(layout.contentWidth))
	model.detail.offset = clampDetailOffset(total, total, layout.viewportHeight)
}

func formatDetailTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Local().Format("2006-01-02 15:04")
}
