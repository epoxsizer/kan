package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/epoxsizer/kan/internal/domain"
)

func (model *Model) View() string {
	view := model.view()
	if os.Getenv("NO_COLOR") != "" {
		return ansi.Strip(view)
	}
	return view
}

func (model *Model) view() string {
	width, height := model.dimensions()
	if width < 32 || height < 12 {
		return lipgloss.NewStyle().Width(width).Height(height).Render(truncate("Resize terminal to 32×12", width))
	}
	if model.discard != nil {
		return model.renderDiscard(width, height)
	}
	if model.movePicker != nil {
		return model.renderMovePicker(width, height)
	}
	if model.form != nil {
		return model.renderForm(width, height)
	}
	if model.confirm != nil {
		return model.renderConfirm(width, height)
	}
	if model.commandMode {
		return model.renderCommandPalette(width, height)
	}
	if model.detail != nil {
		return model.renderDetailPopup(width, height)
	}
	if model.help {
		return model.renderHelp(width, height)
	}
	return model.renderWorkspace(width, height)
}

func (model *Model) renderWorkspace(width, height int) string {
	header := model.renderHeader(width)
	contentHeight := max(height-4, 1)
	var content string
	hasContent := len(model.projects) > 0
	if model.screen == boardsScreen {
		hasContent = len(model.boards) > 0
	}
	if model.screen == boardScreen {
		hasContent = len(model.columns) > 0
	}
	if model.loading && !hasContent {
		content = model.styles.subtle.Render("Loading...")
	} else if model.err != nil && !hasContent {
		content = model.styles.error.Render("Error: " + model.err.Error())
	} else {
		switch model.screen {
		case projectsScreen:
			content = model.renderProjects(width)
		case boardsScreen:
			content = model.renderBoards(width)
		case boardScreen:
			content = model.renderBoard(width, contentHeight)
		}
	}
	content = model.styles.body.Copy().Width(width).Height(contentHeight).MaxHeight(contentHeight).Render(content)
	return header + "\n" + content + "\n" + model.renderStatus(width)
}

func (model *Model) dimensions() (int, int) {
	width, height := model.width, model.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}

func (model *Model) renderHeader(width int) string {
	title := model.styles.header.Render("kan")
	location := "Projects"
	if model.screen >= boardsScreen && model.project != nil {
		location = model.project.Name
	}
	if model.screen == boardScreen && model.board != nil {
		location += " / " + model.board.Name
	}
	status := "local"
	if model.mcpEnabled {
		status = "MCP enabled"
	}
	left := title + model.styles.subtle.Render("  /  "+truncate(location, max(width-lipgloss.Width(status)-12, 1)))
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(status), 1)
	return truncate(left+strings.Repeat(" ", gap)+model.styles.subtle.Render(status), width)
}

func (model *Model) renderProjects(width int) string {
	if len(model.projects) == 0 {
		return model.styles.subtle.Render("No projects. Press a to add one.")
	}
	rows := make([]tableRow, 0, len(model.projects))
	for index, project := range model.projects {
		rows = append(rows, tableRow{name: project.Name, comments: project.Description, items: model.projectCounts[project.ID], selected: index == model.projectIndex})
	}
	if model.listLayout == cardsLayout {
		return model.renderListCards("Projects", "Boards", rows, width)
	}
	return model.renderTable("Projects", "BOARDS", rows, width)
}

func (model *Model) renderBoards(width int) string {
	if len(model.boards) == 0 {
		return model.styles.subtle.Render("No boards in this project. Press a to add one.")
	}
	rows := make([]tableRow, 0, len(model.boards))
	for index, board := range model.boards {
		rows = append(rows, tableRow{
			name:     board.Name,
			comments: board.Description,
			status:   boardHealthLabel(model.boardHealth[board.ID], time.Now()),
			items:    model.boardCounts[board.ID],
			selected: index == model.boardIndex,
		})
	}
	if model.listLayout == cardsLayout {
		return model.renderListCards("Boards", "Cards", rows, width)
	}
	return model.renderTable("Boards", "CARDS", rows, width)
}

func (model *Model) renderBoard(width, contentHeight int) string {
	if len(model.columns) == 0 {
		return model.styles.subtle.Render("This board has no columns. Press c to add one.")
	}
	layout := model.workspaceLayout(width, contentHeight+4)
	panels := []string{}
	if layout.navigationWidth > 0 {
		panels = append(panels, model.renderNavigation(layout.navigationWidth, contentHeight))
	}
	for _, column := range layout.columns {
		panels = append(panels, model.renderColumn(column.index, column.width, contentHeight))
	}
	if layout.inspectorWidth > 0 {
		panels = append(panels, model.renderInspector(layout.inspectorWidth, contentHeight))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, panels...)
}

func (model *Model) renderColumn(index, width, height int) string {
	column := model.columns[index]
	cards := model.visibleCards(column.ID)
	count := fmt.Sprintf("%d", len(cards))
	if model.filterActive() {
		count = fmt.Sprintf("%d/%d", len(cards), len(model.cards[column.ID]))
	}
	title := fmt.Sprintf("%s  %s", column.Name, count)
	if column.WIPLimit != nil {
		title += fmt.Sprintf("/%d", *column.WIPLimit)
	}
	innerWidth := max(width-4, 10)
	columnColor := model.styles.columnDefault
	if column.Color != nil {
		columnColor = colorForName(*column.Color)
	}
	headerStyle := model.styles.header.Copy().Foreground(columnColor)
	borderColor := columnColor
	if index == model.columnIndex {
		headerStyle = headerStyle.Foreground(model.styles.selectedColumnForeground).Background(model.styles.selectedColumnBackground).Padding(0, 1)
		borderColor = model.styles.selectedColumnBorder
	}
	lines := []string{headerStyle.Render(truncate(title, innerWidth))}
	if len(cards) == 0 {
		empty := "No cards"
		if model.filterActive() {
			empty = "No matches"
		}
		lines = append(lines, model.styles.subtle.Render(empty))
	} else {
		selected := clampIndex(model.cardIndexes[column.ID], len(cards))
		maxRows := max(height-5, 1)
		cardContentWidth := max(innerWidth-2, 1)
		rows, selectedRow := model.cardDisplayRows(cards, selected)
		renderedRows := make([]string, len(rows))
		rowHeights := make([]int, len(rows))
		for rowIndex, row := range rows {
			renderedRows[rowIndex] = model.renderCardDisplayRow(row, index == model.columnIndex && rowIndex == selectedRow, cardContentWidth, maxRows)
			rowHeights[rowIndex] = lipgloss.Height(renderedRows[rowIndex])
		}
		start, end := visibleCardRowRange(rowHeights, selectedRow, maxRows)
		lines = append(lines, renderedRows[start:end]...)
	}
	style := model.styles.panel
	if index == model.columnIndex {
		style = model.styles.focusedPanel.Copy().Border(lipgloss.DoubleBorder())
	}
	style = style.BorderForeground(borderColor)
	return style.Width(max(width-2, 1)).Height(max(height-2, 1)).Render(strings.Join(lines, "\n"))
}

func (model *Model) renderCardDisplayRow(row cardDisplayRow, selected bool, width, maxLines int) string {
	if row.group != "" {
		return model.styles.subtle.Copy().Bold(true).Render("─ " + truncate(row.group, max(width-2, 1)))
	}
	label := model.cardLabel(row.card, max(width-4, 1), true)
	content := "  " + label
	if selected {
		background := model.styles.selectedCard.GetBackground()
		label = model.cardLabelWithBackground(row.card, max(width-4, 1), true, background)
		content = "  " + label
		if model.showSelectedCardDetails {
			block := model.selectedCardBlock(row.card, max(width-4, 1), min(maxLines, 2))
			content = "  " + strings.ReplaceAll(block, "\n", "\n  ")
		}
		if os.Getenv("NO_COLOR") != "" {
			content = "> " + strings.TrimPrefix(content, "  ")
		}
		return model.styles.card.Copy().
			Background(background).
			Width(width).
			Render(content)
	}
	return model.styles.card.Width(width).Render(content)
}

func visibleCardRowRange(heights []int, selected, available int) (int, int) {
	if len(heights) == 0 || available <= 0 {
		return 0, 0
	}
	selected = clampIndex(selected, len(heights))
	start := selected
	used := min(max(heights[selected], 1), available)
	for start > 0 {
		height := max(heights[start-1], 1)
		if used+height > available {
			break
		}
		start--
		used += height
	}
	end := selected + 1
	for end < len(heights) {
		height := max(heights[end], 1)
		if used+height > available {
			break
		}
		end++
		used += height
	}
	return start, end
}

type cardDisplayRow struct {
	card      domain.Card
	cardIndex int
	group     string
}

func (model *Model) cardDisplayRows(cards []domain.Card, selected int) ([]cardDisplayRow, int) {
	rows := []cardDisplayRow{}
	selectedRow := 0
	lastGroup := ""
	for index, card := range cards {
		if model.groupMode != groupNone {
			group := model.cardGroupLabel(card)
			if group != lastGroup {
				rows = append(rows, cardDisplayRow{group: group})
				lastGroup = group
			}
		}
		if index == selected {
			selectedRow = len(rows)
		}
		rows = append(rows, cardDisplayRow{card: card, cardIndex: index})
	}
	return rows, selectedRow
}

func (model *Model) cardLabel(card domain.Card, width int, colorPriority bool) string {
	return model.cardLabelWithBackground(card, width, colorPriority, nil)
}

func (model *Model) cardLabelWithBackground(card domain.Card, width int, colorPriority bool, background lipgloss.TerminalColor) string {
	prefix := ""
	if model.showCardTags {
		for _, tag := range card.Tags {
			prefix += "[" + tag + "]"
		}
		if prefix != "" {
			prefix += " "
		}
	}
	suffix := ""
	if len(card.Checklist) > 0 {
		done := 0
		for _, item := range card.Checklist {
			if item.Done {
				done++
			}
		}
		suffix = fmt.Sprintf(" [%d/%d]", done, len(card.Checklist))
	}
	duePrefix := ""
	if card.DueDate != nil {
		marker := "@"
		if isOverdueDate(card.DueDate) {
			marker = "!@"
		}
		duePrefix = marker + card.DueDate.Format("2006-01-02") + " "
	}
	content := duePrefix + prefix + card.Title + suffix
	label := truncate(content, width)
	if card.Priority == nil || *card.Priority == "" || width < 3 {
		return label
	}
	priority := strings.ToUpper((*card.Priority)[:1])
	if !colorPriority {
		return priority + " " + truncate(content, max(width-2, 1))
	}
	markerStyle := lipgloss.NewStyle().Bold(true).Foreground(priorityColor(*card.Priority))
	textStyle := lipgloss.NewStyle()
	if background != nil {
		markerStyle = markerStyle.Background(background)
		textStyle = textStyle.Background(background)
	}
	marker := markerStyle.Render(priority)
	return marker + textStyle.Render(" "+truncate(content, max(width-2, 1)))
}

func isOverdueDate(value *time.Time) bool {
	if value == nil {
		return false
	}
	return calendarDayDelta(*value, time.Now()) < 0
}

func (model *Model) renderStatus(width int) string {
	if model.filterMode {
		hint := "  fuzzy card filter · Enter/Esc close · Ctrl-U delete left"
		if model.filterLoading {
			hint = "  searching…"
		} else if model.filterErr != nil {
			hint = "  error: " + model.filterErr.Error()
		}
		inputWidth := max(width-lipgloss.Width(hint)-1, 4)
		info := lipgloss.NewStyle().Width(width).Render(model.styles.command.Render("/"+textViewport(model.filterText, model.filterCursor, inputWidth)) + model.styles.subtle.Render(hint))
		return info + "\n" + model.renderShortcutBar(width)
	}
	if model.commandMode {
		line := model.styles.command.Render(":" + textViewport(model.command, model.commandCursor, max(width-1, 1)))
		return lipgloss.NewStyle().Width(width).Render(line)
	}
	breadcrumb := "Projects"
	count := fmt.Sprintf("%d projects", len(model.projects))
	if model.screen >= boardsScreen && model.project != nil {
		breadcrumb += " › " + model.project.Name
		count = fmt.Sprintf("%d boards · layout:%s", len(model.boards), model.listLayout.String())
	} else {
		count += " · layout:" + model.listLayout.String()
	}
	if model.screen == boardScreen && model.board != nil {
		breadcrumb += " › " + model.board.Name
		count = fmt.Sprintf("%d columns · %d cards", len(model.columns), model.cardCount())
		if model.filterActive() {
			if model.planningFilter != planningNone {
				count = fmt.Sprintf("%d matches · planning:%s", model.visibleCardCount(), model.planningFilter)
			} else {
				count = fmt.Sprintf("%d matches · /%s", model.visibleCardCount(), model.filterText)
			}
			if model.filterErr != nil {
				count = "filter error: " + model.filterErr.Error()
			}
		}
		if health := model.boardPlanningHealth().Summary(); health != "" && model.notice == "" {
			count += " · " + health
		}
		count += " · sort:" + model.sortMode.String() + " · group:" + model.groupMode.String()
	}
	left := model.styles.statusAccent.Render(breadcrumb)
	if lipgloss.Width(left) > width/2 {
		breadcrumb = truncate(breadcrumb, max(width/2-2, 1))
		left = model.styles.statusAccent.Render(breadcrumb)
	}
	if model.notice != "" {
		count = model.notice
	}
	if model.loading {
		count = "Refreshing…"
	}
	if model.err != nil {
		count = "Error: " + model.err.Error()
	}
	right := model.styles.status.Render(truncate(count, max(width-lipgloss.Width(left)-2, 1)))
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	info := left + strings.Repeat(" ", gap) + right
	return info + "\n" + model.renderShortcutBar(width) + "\n"
}

type shortcut struct {
	key   string
	label string
}

func (model *Model) shortcuts() []shortcut {
	shortcuts := []shortcut{{"?", "Help"}, {":", "Cmd"}, {"q", "Quit"}}
	switch model.screen {
	case projectsScreen:
		shortcuts = append(shortcuts, shortcut{"j/k", "Navigate"}, shortcut{"Enter", "Open"}, shortcut{"a", "Add"}, shortcut{"e", "Edit"}, shortcut{"D", "Delete"}, shortcut{"d", "Describe"}, shortcut{"m", "Comment"})
	case boardsScreen:
		shortcuts = append(shortcuts, shortcut{"j/k", "Navigate"}, shortcut{"Enter", "Open"}, shortcut{"a", "Add"}, shortcut{"e", "Edit"}, shortcut{"D", "Delete"}, shortcut{"d", "Describe"})
	case boardScreen:
		shortcuts = append(shortcuts, shortcut{"j/k", "Card"}, shortcut{"h/l", "Column"}, shortcut{"Tab", "Focus"})
		if model.lastMove != nil {
			shortcuts = append(shortcuts, shortcut{"u", "Undo"})
		}
		shortcuts = append(shortcuts, shortcut{"a", "Add"}, shortcut{"e", "Edit"}, shortcut{"M", "Move"}, shortcut{"/", "Filter"})
	}
	if model.screen == boardScreen {
		for _, action := range workspaceActions {
			shortcuts = append(shortcuts, shortcut{action.key, action.label})
		}
		if model.workspace.focus == inspectorFocus {
			shortcuts = []shortcut{{"?", "Help"}, {"Tab", "Focus"}, {"j/k", "Scroll"}, {"e", "Edit"}, {"i", "Hide"}, {"q", "Quit"}}
		}
		if model.workspace.focus == navigationFocus {
			shortcuts = []shortcut{{"?", "Help"}, {"Tab", "Focus"}, {"j/k", "Board"}, {"Enter", "Open"}, {"p", "Projects"}, {"b", "Hide"}, {"q", "Quit"}}
		}
	}
	if model.filterMode {
		shortcuts = []shortcut{{"Enter", "Keep"}, {"Esc", "Close"}, {"Ctrl-U", "Clear"}, {"Ctrl-C", "Quit"}}
	}
	return shortcuts
}

type shortcutRegion struct {
	item       shortcut
	text       string
	start, end int
}

func (model *Model) shortcutRegions(width int) []shortcutRegion {
	regions := []shortcutRegion{}
	x := 0
	for _, item := range model.shortcuts() {
		token := model.styles.shortcutKey.Render("<"+item.key+">") + model.styles.shortcutText.Render(" "+item.label) + "   "
		size := lipgloss.Width(token)
		if x+size > width {
			break
		}
		regions = append(regions, shortcutRegion{item: item, text: token, start: x, end: x + size})
		x += size
	}
	return regions
}

func (model *Model) renderShortcutBar(width int) string {
	line := ""
	for _, region := range model.shortcutRegions(width) {
		line += region.text
	}
	return model.styles.shortcutText.Copy().Width(width).MaxWidth(width).Render(line)
}

func (model *Model) cardCount() int {
	count := 0
	for _, cards := range model.cards {
		count += len(cards)
	}
	return count
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	return min(max(index, 0), length-1)
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}
