package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (model *Model) handleMouse(message tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(message)
	if event.Shift || event.Alt || event.Ctrl || event.Action != tea.MouseActionPress {
		return model, nil
	}
	if event.Button == tea.MouseButtonRight {
		return model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	}
	if event.IsWheel() {
		return model.handleMouseWheel(event)
	}
	if event.Button != tea.MouseButtonLeft {
		return model, nil
	}
	return model.handleMouseClick(event)
}

func (model *Model) handleMouseWheel(event tea.MouseEvent) (tea.Model, tea.Cmd) {
	delta := 1
	if event.Button == tea.MouseButtonWheelUp || event.Button == tea.MouseButtonWheelLeft {
		delta = -1
	}
	if model.detail != nil {
		model.scrollDetail(delta * 3)
		return model, nil
	}
	if model.movePicker != nil {
		model.movePicker.targetColumnIndex = model.nextMoveTarget(delta)
		return model, nil
	}
	if model.form != nil {
		return model.handleFormMouseWheel(delta)
	}
	if model.commandMode {
		model.commandIndex = clampIndex(model.commandIndex+delta, len(model.paletteMatches()))
		return model, nil
	}
	if model.help || model.loading {
		return model, nil
	}
	if model.screen == boardScreen && (event.Button == tea.MouseButtonWheelLeft || event.Button == tea.MouseButtonWheelRight) {
		model.columnIndex = clampIndex(model.columnIndex+delta, len(model.columns))
		return model, nil
	}
	switch model.screen {
	case projectsScreen:
		model.projectIndex = model.moveListSelection(model.projectIndex, len(model.projects), delta, 0)
	case boardsScreen:
		model.boardIndex = model.moveListSelection(model.boardIndex, len(model.boards), delta, 0)
	case boardScreen:
		columnIndex, ok := model.mouseColumnAt(event.X)
		if ok {
			model.columnIndex = columnIndex
			column := model.columns[columnIndex]
			cards := model.visibleCards(column.ID)
			model.cardIndexes[column.ID] = clampIndex(model.cardIndexes[column.ID]+delta, len(cards))
		}
	}
	return model, nil
}

func (model *Model) handleFormMouseWheel(delta int) (tea.Model, tea.Cmd) {
	if model.form.control == nil {
		model.form.focus = clampIndex(model.form.focus+delta, len(model.form.fields))
		return model, nil
	}
	control := model.form.control
	switch control.kind {
	case calendarControl:
		control.date = control.date.AddDate(0, delta, 0)
	case dropdownControl:
		control.selection = clampIndex(control.selection+delta, len(model.form.fields[control.field].options))
	case linksControl:
		control.selection = clampIndex(control.selection+delta, len(model.linkMatches(control.query)))
	case checklistControl:
		control.selection = clampIndex(control.selection+delta, len(control.checklist))
	case commentControl:
		key := "down"
		if delta < 0 {
			key = "up"
		}
		return model.handleCommentKey(mouseKey(key))
	}
	return model, nil
}

func (model *Model) handleMouseClick(event tea.MouseEvent) (tea.Model, tea.Cmd) {
	width, height := model.dimensions()
	switch {
	case model.discard != nil:
		return model.clickDecision(event, true)
	case model.confirm != nil:
		return model.clickDecision(event, false)
	case model.movePicker != nil:
		return model.clickMovePicker(event, height)
	case model.form != nil:
		return model.clickForm(event, width, height)
	case model.commandMode:
		return model.clickPalette(event, height)
	case model.detail != nil:
		return model.clickDetail(event, width, height)
	case model.help:
		return model, nil
	}
	if event.Y >= height-1 {
		switch {
		case event.X < 11:
			model.help = true
		case event.X < 22:
			model.commandMode = true
			model.command = ""
			model.commandCursor = 0
			model.commandIndex = 0
			model.paletteLoading = true
			return model, loadPaletteIndex(model.ctx, model.repo)
		case event.X < 33:
			return model, tea.Quit
		}
		return model, nil
	}
	if model.loading || event.Y < 1 {
		return model, nil
	}
	switch model.screen {
	case projectsScreen:
		index, ok := model.mouseListIndex(event, len(model.projects))
		if !ok {
			return model, nil
		}
		if index == model.projectIndex {
			return model.openSelectedProject()
		}
		model.projectIndex = index
	case boardsScreen:
		index, ok := model.mouseListIndex(event, len(model.boards))
		if !ok {
			return model, nil
		}
		if index == model.boardIndex {
			return model.openSelectedBoard()
		}
		model.boardIndex = index
	case boardScreen:
		return model.clickBoard(event)
	}
	return model, nil
}

func (model *Model) mouseListIndex(event tea.MouseEvent, total int) (int, bool) {
	if total == 0 {
		return 0, false
	}
	if model.listLayout == tableLayout {
		index := event.Y - 5
		return index, index >= 0 && index < total
	}
	columns := cardLayoutColumns(model.width)
	cardWidth := cardLayoutWidth(model.width)
	gap := 2
	if model.width < 44 {
		gap = 1
	}
	if event.Y < 2 {
		return 0, false
	}
	column := event.X / (cardWidth + gap)
	row := (event.Y - 2) / 7
	index := row*columns + column
	return index, column < columns && index >= 0 && index < total
}

func (model *Model) mouseColumnAt(x int) (int, bool) {
	if len(model.columns) == 0 || x < 0 {
		return 0, false
	}
	const minimumColumnWidth = 22
	width, _ := model.dimensions()
	visible := min(len(model.columns), max(1, width/minimumColumnWidth))
	start := max(0, model.columnIndex-visible+1)
	start = min(start, len(model.columns)-visible)
	baseWidth, remainder := width/visible, width%visible
	left := 0
	for offset, index := 0, start; index < start+visible; offset, index = offset+1, index+1 {
		columnWidth := baseWidth
		if offset < remainder {
			columnWidth++
		}
		if x >= left && x < left+columnWidth {
			return index, true
		}
		left += columnWidth
	}
	return 0, false
}

func (model *Model) clickBoard(event tea.MouseEvent) (tea.Model, tea.Cmd) {
	columnIndex, ok := model.mouseColumnAt(event.X)
	if !ok {
		return model, nil
	}
	if event.Y <= 2 {
		if columnIndex == model.columnIndex {
			column := model.columns[columnIndex]
			model.detail = columnDetail(column, len(model.visibleCards(column.ID)))
		} else {
			model.columnIndex = columnIndex
		}
		return model, nil
	}
	cardIndex, ok := model.mouseCardAt(columnIndex, event.Y)
	if !ok {
		model.columnIndex = columnIndex
		return model, nil
	}
	column := model.columns[columnIndex]
	wasSelected := columnIndex == model.columnIndex && model.cardIndexes[column.ID] == cardIndex
	model.columnIndex = columnIndex
	model.cardIndexes[column.ID] = cardIndex
	if wasSelected {
		model.openSelectedDetail()
	}
	return model, nil
}

func (model *Model) mouseCardAt(columnIndex, screenY int) (int, bool) {
	column := model.columns[columnIndex]
	cards := model.visibleCards(column.ID)
	if len(cards) == 0 {
		return 0, false
	}
	_, height := model.dimensions()
	maxRows := max(max(height-4, 1)-5, 1)
	selected := clampIndex(model.cardIndexes[column.ID], len(cards))
	rows, selectedRow := model.cardDisplayRows(cards, selected)
	heights := make([]int, len(rows))
	cardWidth := max(model.mouseColumnWidth(columnIndex)-6, 1)
	for index, row := range rows {
		rendered := model.renderCardDisplayRow(row, columnIndex == model.columnIndex && index == selectedRow, cardWidth, maxRows)
		heights[index] = lipgloss.Height(rendered)
	}
	start, end := visibleCardRowRange(heights, selectedRow, maxRows)
	y := 3
	for index := start; index < end; index++ {
		if screenY >= y && screenY < y+heights[index] && rows[index].group == "" {
			return rows[index].cardIndex, true
		}
		y += heights[index]
	}
	return 0, false
}

func (model *Model) mouseColumnWidth(target int) int {
	const minimumColumnWidth = 22
	width, _ := model.dimensions()
	visible := min(len(model.columns), max(1, width/minimumColumnWidth))
	start := max(0, model.columnIndex-visible+1)
	start = min(start, len(model.columns)-visible)
	baseWidth, remainder := width/visible, width%visible
	for offset, index := 0, start; index < start+visible; offset, index = offset+1, index+1 {
		columnWidth := baseWidth
		if offset < remainder {
			columnWidth++
		}
		if index == target {
			return columnWidth
		}
	}
	return baseWidth
}

func (model *Model) clickForm(event tea.MouseEvent, width, height int) (tea.Model, tea.Cmd) {
	if model.form.control != nil {
		return model.clickFormControl(event, width, height)
	}
	lineCount := 3 + len(model.form.fields)
	if model.form.err != "" {
		lineCount += 2
	}
	top := centeredMouseTop(height, lineCount)
	boxWidth := min(70, max(width-4, 26))
	innerWidth := max(boxWidth-6, 20)
	left := max((width-innerWidth)/2, 0)
	if event.Y == top+3 {
		hint := "Tab fields · ←/→ edit · Ctrl-S save · Esc cancel"
		if model.form.fields[model.form.focus].kind != textField {
			hint = "Tab fields · Enter open · Ctrl-S save · Esc cancel"
		}
		position := event.X - (left + 3)
		if insideMouseLabel(hint, "Ctrl-S save", position) {
			return model.handleFormKey(mouseKey("ctrl+s"))
		}
		if insideMouseLabel(hint, "Esc cancel", position) {
			return model.handleFormKey(tea.KeyMsg{Type: tea.KeyEsc})
		}
	}
	index := event.Y - (top + 5)
	if index < 0 || index >= len(model.form.fields) {
		return model, nil
	}
	wasFocused := model.form.focus == index
	model.form.focus = index
	field := &model.form.fields[index]
	if field.kind == textField {
		labelWidth := lipgloss.Width(field.label + ": ")
		cell := max(event.X-(left+3+labelWidth), 0)
		field.cursor = mouseRuneIndex(field.value, cell)
	} else if field.kind == checkboxField {
		toggleCheckboxField(field)
	} else if wasFocused {
		model.form.openControl()
	}
	return model, nil
}

func (model *Model) clickFormControl(event tea.MouseEvent, width, height int) (tea.Model, tea.Cmd) {
	control := model.form.control
	switch control.kind {
	case dropdownControl:
		options := model.form.fields[control.field].options
		index := event.Y - (centeredMouseTop(height, 3+len(options)) + 5)
		if index >= 0 && index < len(options) {
			if control.selection == index {
				return model.handleDropdownKey(mouseKey("enter"))
			}
			control.selection = index
		}
	case calendarControl:
		top := centeredMouseTop(height, 9)
		if event.Y == top+3 {
			hint := "Arrows day/week · PgUp/PgDn month · Enter apply · x no due date"
			left := max((width-min(54, width))/2, 0)
			if insideMouseLabel(hint, "x no due date", event.X-(left+3)) {
				return model.handleCalendarKey(mouseKey("x"))
			}
		}
		week := event.Y - (top + 5)
		left := max((width-min(54, width))/2, 0)
		day := (event.X - (left + 3)) / 3
		if week >= 0 && week < 6 && day >= 0 && day < 7 {
			first := time.Date(control.date.Year(), control.date.Month(), 1, 0, 0, 0, 0, time.Local)
			start := first.AddDate(0, 0, -int(first.Weekday()))
			date := start.AddDate(0, 0, week*7+day)
			if sameDay(date, control.date) {
				return model.handleCalendarKey(mouseKey("enter"))
			}
			control.date = date
		}
	case linksControl:
		matches := model.linkMatches(control.query)
		count := min(len(matches), max(height-10, 1))
		index := event.Y - (centeredMouseTop(height, 4+count) + 6)
		if index >= 0 && index < count {
			control.selection = index
			control.selected[matches[index].id] = !control.selected[matches[index].id]
		}
	case checklistControl:
		count := min(len(control.checklist), max(height-11, 2))
		index := event.Y - (centeredMouseTop(height, 3+count) + 5)
		if index >= 0 && index < count {
			control.selection = index
			control.checklist[index].Done = !control.checklist[index].Done
		}
	}
	return model, nil
}

func (model *Model) clickDetail(event tea.MouseEvent, width, height int) (tea.Model, tea.Cmd) {
	layout := detailLayoutForSize(width, height, model.detail.expanded)
	top := max((height-(layout.insideHeight+4))/2, 0)
	if event.Y != top+layout.insideHeight+1 {
		return model, nil
	}
	left := max((width-layout.boxWidth)/2, 0)
	position := event.X - (left + 3)
	hint := "Esc / d / Enter close"
	if insideMouseLabel(hint, "close", position) {
		model.detail = nil
		return model, nil
	}
	position -= lipgloss.Width(hint + " · ")
	if model.detail.kind == "card" {
		edit := "e edit"
		if position >= 0 && position < lipgloss.Width(edit) {
			return model.openSelectedCardEdit()
		}
		position -= lipgloss.Width(edit + " · ")
	}
	size := "Shift+E expand"
	if model.detail.expanded {
		size = "Shift+E compact"
	}
	if position >= 0 && position < lipgloss.Width(size) {
		model.detail.expanded = !model.detail.expanded
		model.clampDetailForCurrentLayout()
	}
	return model, nil
}

func (model *Model) clickPalette(event tea.MouseEvent, height int) (tea.Model, tea.Cmd) {
	matches := model.paletteMatches()
	selected := clampIndex(model.commandIndex, len(matches))
	maxRows := max(1, height-10)
	start := max(0, selected-maxRows+1)
	count := min(maxRows, len(matches)-start)
	index := start + event.Y - (centeredMouseTop(height, 4+count) + 6)
	if index < start || index >= len(matches) {
		return model, nil
	}
	if model.commandIndex == index {
		return model.handleCommandKey(mouseKey("enter"))
	}
	model.commandIndex = index
	return model, nil
}

func (model *Model) clickMovePicker(event tea.MouseEvent, height int) (tea.Model, tea.Cmd) {
	maxRows := max(height-10, 2)
	start := max(model.movePicker.targetColumnIndex-maxRows+1, 0)
	start = min(start, max(len(model.columns)-maxRows, 0))
	end := min(start+maxRows, len(model.columns))
	index := start + event.Y - (centeredMouseTop(height, 4+end-start) + 6)
	if index < start || index >= end {
		return model, nil
	}
	if model.movePicker.targetColumnIndex == index {
		return model.handleMovePickerKey(mouseKey("enter"))
	}
	model.movePicker.targetColumnIndex = index
	return model, nil
}

func (model *Model) clickDecision(event tea.MouseEvent, discard bool) (tea.Model, tea.Cmd) {
	width, height := model.dimensions()
	if event.Y != centeredMouseTop(height, 5)+6 {
		return model, nil
	}
	key := "n"
	if event.X < width/2 {
		key = "y"
	}
	if discard {
		return model.handleDiscardKey(mouseKey(key))
	}
	return model.handleConfirmKey(mouseKey(key))
}

func centeredMouseTop(height, contentLines int) int {
	return max((height-(contentLines+4))/2, 0)
}

func insideMouseLabel(line, label string, position int) bool {
	start := lipgloss.Width(line[:strings.Index(line, label)])
	return position >= start && position < start+lipgloss.Width(label)
}

func mouseRuneIndex(value string, cell int) int {
	width := 0
	for index, character := range []rune(value) {
		next := width + lipgloss.Width(string(character))
		if cell < next {
			return index
		}
		width = next
	}
	return len([]rune(value))
}

func mouseKey(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
