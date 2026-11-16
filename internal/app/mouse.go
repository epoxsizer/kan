package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// doubleClickWindow is the longest gap between two presses on the same card
// that still counts as a double-click and opens the card editor.
const doubleClickWindow = 400 * time.Millisecond

// cardClickRecord remembers the last pressed board card for double-click
// detection. Dragging clears it so a drag never turns into a double-click.
type cardClickRecord struct {
	cardID string
	at     time.Time
}

func (model *Model) handleMouse(message tea.MouseMsg) (tea.Model, tea.Cmd) {
	if model.form != nil && model.form.saving {
		return model, nil
	}
	event := tea.MouseEvent(message)
	if model.drag != nil && event.Action != tea.MouseActionPress {
		switch {
		case event.Action == tea.MouseActionRelease:
			return model.finishCardDrag()
		case event.Action == tea.MouseActionMotion && event.Button == tea.MouseButtonLeft:
			return model.handleCardDragMotion(event)
		}
		return model, nil
	}
	model.drag = nil
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
	if model.screen == boardScreen {
		width, height := model.dimensions()
		layout := model.workspaceLayout(width, height)
		if event.Y >= height-3 || event.Y < 1 {
			return model, nil
		}
		if layout.inspectorWidth > 0 && event.X >= layout.inspectorX {
			model.scrollInspector(delta * 3)
			return model, nil
		}
		if layout.navigationWidth > 0 && event.X < layout.navigationWidth {
			model.workspace.navigationIndex = clampIndex(model.workspace.navigationIndex+delta, len(model.boards)+3)
			return model, nil
		}
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
		model.syncCardEditorChecklist()
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
		if model.detail.clickOpened && model.lastCardClick != nil && model.now().Sub(model.lastCardClick.at) <= doubleClickWindow {
			model.lastCardClick = nil
			return model.openSelectedCardEdit()
		}
		return model.clickDetail(event, width, height)
	case model.help:
		return model, nil
	}
	footerY := height - 2
	if model.filterMode {
		footerY = height - 1
	}
	if event.Y >= height-3 {
		if event.Y == footerY {
			return model.clickShortcut(event.X, width)
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
		layout := model.workspaceLayout(width, height)
		if event.X < layout.navigationWidth {
			return model.clickNavigation(event)
		}
		if layout.inspectorWidth > 0 && event.X >= layout.inspectorX {
			model.workspace.focus = inspectorFocus
			return model, nil
		}
		model.workspace.focus = boardFocus
		return model.clickBoard(event)
	}
	return model, nil
}

func (model *Model) mouseListIndex(event tea.MouseEvent, total int) (int, bool) {
	if total == 0 {
		return 0, false
	}
	width, height := model.dimensions()
	window := model.listWindow(width, total)
	if event.Y >= height-3 {
		return 0, false
	}
	if model.listLayout == tableLayout {
		index := event.Y - 5 + window.start
		return index, index >= window.start && index < window.end
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
	index := window.start + row*columns + column
	return index, column < columns && index >= window.start && index < window.end
}

func (model *Model) mouseColumnAt(x int) (int, bool) {
	width, height := model.dimensions()
	return model.workspaceLayout(width, height).columnAt(x)
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
	card := model.visibleCards(column.ID)[cardIndex]
	if model.lastCardClick != nil && model.lastCardClick.cardID == card.ID && model.now().Sub(model.lastCardClick.at) <= doubleClickWindow {
		model.lastCardClick = nil
		model.drag = nil
		model.columnIndex = columnIndex
		model.cardIndexes[column.ID] = cardIndex
		return model.openSelectedCardEdit()
	}
	model.lastCardClick = &cardClickRecord{cardID: card.ID, at: model.now()}
	wasSelected := columnIndex == model.columnIndex && model.cardIndexes[column.ID] == cardIndex
	model.columnIndex = columnIndex
	model.cardIndexes[column.ID] = cardIndex
	model.startCardDrag(columnIndex, cardIndex)
	if wasSelected {
		model.openSelectedDetail()
		if model.detail != nil {
			model.detail.clickOpened = true
		}
		if model.drag != nil {
			model.drag.openedDetail = model.detail != nil
		}
	}
	return model, nil
}

func (model *Model) mouseCardAt(columnIndex, screenY int) (int, bool) {
	cardIndex, _, _, ok := model.mouseCardBounds(columnIndex, screenY)
	return cardIndex, ok
}

func (model *Model) mouseColumnWidth(target int) int {
	width, height := model.dimensions()
	return model.workspaceLayout(width, height).columnWidth(target)
}

func (model *Model) clickForm(event tea.MouseEvent, width, height int) (tea.Model, tea.Cmd) {
	if model.cardEditorActive(width, height) && (model.form.control == nil || model.form.control.inline) {
		return model.clickCardEditor(event, width, height)
	}
	if model.form.control != nil {
		return model.clickFormControl(event, width, height)
	}
	window := model.compactFormWindow(height)
	top := centeredMouseTop(height, window.lineCount)
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
	index := event.Y - (top + 5) + window.start
	if index < window.start || index >= window.end {
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
		start, end := controlWindow(len(options), control.selection, height-7)
		index := start + event.Y - (centeredMouseTop(height, 3+end-start) + 5)
		if index >= start && index < end {
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
		start, end := controlWindow(len(matches), control.selection, height-10)
		index := start + event.Y - (centeredMouseTop(height, 4+end-start) + 6)
		if index >= start && index < end {
			control.selection = index
			control.selected[matches[index].id] = !control.selected[matches[index].id]
		}
	case checklistControl:
		count := min(len(control.checklist), max(height-11, 2))
		start := max(control.selection-max(height-11, 2)+1, 0)
		lineCount := 3 + count
		if control.inputMode {
			lineCount += 3
		}
		if count == 0 && !control.inputMode {
			lineCount++
		}
		index := start + event.Y - (centeredMouseTop(height, lineCount) + 5)
		if index >= start && index < min(start+count, len(control.checklist)) {
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

func (model *Model) clickShortcut(x, width int) (tea.Model, tea.Cmd) {
	for _, region := range model.shortcutRegions(width) {
		if x < region.start || x >= region.end {
			continue
		}
		key := region.item.key
		switch key {
		case "Enter":
			return model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		case "Esc":
			return model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
		case "Tab":
			return model.handleKey(tea.KeyMsg{Type: tea.KeyTab})
		case "Ctrl-U":
			return model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		case "Ctrl-C":
			return model.requestQuit()
		}
		if len([]rune(key)) == 1 {
			return model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
		return model, nil
	}
	return model, nil
}
