package app

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/epoxsizer/kan/internal/domain"
)

// The full-screen card editor replaces the compact card form on terminals at
// least this large. Smaller terminals keep the centered form.
const (
	cardEditorMinWidth  = 100
	cardEditorMinHeight = 24
)

const (
	cardEditorFieldsHint      = "Tab fields · ←/→ edit · Enter open · Ctrl-S save · Esc cancel"
	cardEditorDescriptionHint = "Enter edit description · Tab fields · Ctrl-S save · Esc cancel"
	cardEditorChecklistHint   = "a add · e edit · Space toggle · D delete · J/K reorder · Tab fields · Ctrl-S save · Esc cancel"
)

type cardEditorLayout struct {
	bodyTop      int
	bodyHeight   int
	fieldsLeft   int
	fieldsWidth  int
	previewLeft  int
	previewWidth int
}

func cardEditorLayoutForSize(width, height int) cardEditorLayout {
	fieldsWidth := min(max(44, width*2/5), width-40)
	return cardEditorLayout{
		bodyTop: 2, bodyHeight: max(height-3, 6),
		fieldsLeft: 0, fieldsWidth: fieldsWidth,
		previewLeft: fieldsWidth, previewWidth: width - fieldsWidth,
	}
}

func (layout cardEditorLayout) contentHeight() int {
	return max(layout.bodyHeight-2, 1)
}

func isCardForm(form *formModal) bool {
	return form != nil && (form.kind == createCardForm || form.kind == editCardForm)
}

func (model *Model) cardEditorActive(width, height int) bool {
	return isCardForm(model.form) && width >= cardEditorMinWidth && height >= cardEditorMinHeight
}

func (model *Model) cardEditorActiveNow() bool {
	width, height := model.dimensions()
	return model.cardEditorActive(width, height)
}

// cardEditorFieldIndexes lists the fields shown as rows in the left pane. The
// description has its own pane and the checklist is listed below the rows.
func cardEditorFieldIndexes(form *formModal) []int {
	indexes := []int{}
	for index, field := range form.fields {
		if field.kind == commentField || field.kind == checklistField {
			continue
		}
		indexes = append(indexes, index)
	}
	return indexes
}

func cardEditorDescriptionIndex(form *formModal) int {
	for index, field := range form.fields {
		if field.kind == commentField {
			return index
		}
	}
	return -1
}

func cardEditorChecklistIndex(form *formModal) int {
	for index, field := range form.fields {
		if field.kind == checklistField {
			return index
		}
	}
	return -1
}

// cardEditorFieldRow returns the left-pane content row of a field.
func cardEditorFieldRow(form *formModal, fieldIndex int) int {
	for row, index := range cardEditorFieldIndexes(form) {
		if index == fieldIndex {
			return row
		}
	}
	return -1
}

func cardEditorChecklistHeaderRow(form *formModal) int {
	return len(cardEditorFieldIndexes(form)) + 1
}

// cardEditorChecklistRow returns the left-pane content row of a checklist item
// when no scrolling is needed.
func cardEditorChecklistRow(form *formModal, item int) int {
	return cardEditorChecklistHeaderRow(form) + 1 + item
}

func (form *formModal) editorChecklist() []domain.ChecklistItem {
	if form.control != nil && form.control.kind == checklistControl {
		return form.control.checklist
	}
	index := cardEditorChecklistIndex(form)
	if index < 0 {
		return nil
	}
	var items []domain.ChecklistItem
	_ = json.Unmarshal([]byte(form.fields[index].value), &items)
	return items
}

func (form *formModal) inlineChecklist() bool {
	return form.control != nil && form.control.inline
}

// checklistWindow returns the first visible item and the number of item rows.
func (form *formModal) checklistWindow(layout cardEditorLayout) (int, int) {
	rows := layout.contentHeight() - cardEditorChecklistHeaderRow(form) - 1
	if form.inlineChecklist() && form.control.inputMode {
		rows -= 2
	}
	rows = max(rows, 1)
	start := 0
	if form.inlineChecklist() {
		start = max(0, form.control.selection-rows+1)
	}
	return start, rows
}

// syncCardEditorChecklist starts inline checklist editing when the
// full-screen editor focuses the checklist.
func (model *Model) syncCardEditorChecklist() {
	form := model.form
	if !model.cardEditorActiveNow() || form.control != nil || form.focus != cardEditorChecklistIndex(form) {
		return
	}
	form.openControl()
	form.control.inline = true
}

func (model *Model) commitInlineChecklist() {
	control := model.form.control
	encoded, _ := json.Marshal(nonNilChecklist(control.checklist))
	model.form.fields[control.field].value = string(encoded)
	model.form.control = nil
}

func nonNilChecklist(items []domain.ChecklistItem) []domain.ChecklistItem {
	if items == nil {
		return []domain.ChecklistItem{}
	}
	return items
}

func (model *Model) handleInlineChecklistKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	if control.inputMode {
		return model.handleChecklistKey(key)
	}
	switch key.String() {
	case "tab", "shift+tab", "esc":
		model.commitInlineChecklist()
		return model.handleFormKey(key)
	case "up", "k":
		if control.selection == 0 {
			model.commitInlineChecklist()
			return model.handleFormKey(tea.KeyMsg{Type: tea.KeyUp})
		}
	case "ctrl+s":
		model.commitInlineChecklist()
		return model.submitForm()
	}
	return model.handleChecklistKey(key)
}

func (model *Model) cardEditorHint() string {
	form := model.form
	switch form.focus {
	case cardEditorChecklistIndex(form):
		return cardEditorChecklistHint
	case cardEditorDescriptionIndex(form):
		return cardEditorDescriptionHint
	}
	return cardEditorFieldsHint
}

func (model *Model) renderCardEditor(width, height int) string {
	form := model.form
	layout := cardEditorLayoutForSize(width, height)
	location := ""
	if model.board != nil {
		location = model.board.Name
	}
	if status := strings.TrimSpace(form.fields[2].value); status != "" {
		location += " › " + status
	}
	header := model.styles.header.Render(form.title)
	if location != "" {
		header += model.styles.subtle.Render(" · " + location)
	}
	lines := []string{
		ansi.Truncate(header, width, "…"),
		model.styles.subtle.Render(truncate(model.cardEditorHint(), width)),
	}
	fields := model.renderCardEditorFields(layout)
	preview := model.renderCardEditorPreview(layout)
	body := lipgloss.JoinHorizontal(lipgloss.Top, fields, preview)
	lines = append(lines, strings.Split(body, "\n")...)
	footer := ""
	if form.err != "" {
		footer = model.styles.error.Render(truncate(form.err, width))
	}
	lines = append(lines, footer)
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:height], "\n")
}

func (model *Model) cardEditorPanel(focused bool, width, height int, content []string) string {
	style := model.styles.panel
	if focused {
		style = model.styles.focusedPanel
	}
	contentWidth := max(width-4, 1)
	rows := max(height-2, 1)
	for index := range content {
		content[index] = ansi.Truncate(content[index], contentWidth, "…")
	}
	if len(content) > rows {
		content = content[:rows]
	}
	return style.Width(max(width-2, 1)).Height(rows).Render(strings.Join(content, "\n"))
}

func (model *Model) renderCardEditorFields(layout cardEditorLayout) string {
	form := model.form
	contentWidth := max(layout.fieldsWidth-4, 10)
	indexes := cardEditorFieldIndexes(form)
	labelWidth := 0
	for _, index := range indexes {
		labelWidth = max(labelWidth, lipgloss.Width(cardEditorLabel(form.fields[index])))
	}
	content := []string{}
	for _, index := range indexes {
		field := form.fields[index]
		label := fmt.Sprintf("%-*s ", labelWidth, cardEditorLabel(field))
		inputWidth := max(contentWidth-lipgloss.Width(label), 1)
		value := truncate(fieldDisplayValue(field, form.linkCandidates), inputWidth)
		if index == form.focus {
			if field.kind == textField {
				value = textViewport(field.value, field.cursor, inputWidth)
			}
			content = append(content, model.styles.selected.Copy().Padding(0).Width(contentWidth).Render(label+value))
			continue
		}
		content = append(content, model.styles.subtle.Render(label)+value)
	}

	checklistFocused := form.focus == cardEditorChecklistIndex(form)
	items := form.editorChecklist()
	done := 0
	for _, item := range items {
		if item.Done {
			done++
		}
	}
	heading := fmt.Sprintf("Checklist %d/%d", done, len(items))
	if checklistFocused {
		heading = model.styles.header.Render(heading)
	}
	content = append(content, "", heading)
	start, rows := form.checklistWindow(layout)
	for index := start; index < min(start+rows, len(items)); index++ {
		mark := "[ ]"
		if items[index].Done {
			mark = "[x]"
		}
		line := mark + " " + items[index].Text
		if checklistFocused && form.inlineChecklist() && index == form.control.selection {
			line = model.styles.selected.Copy().Padding(0).Width(contentWidth).Render(truncate(line, contentWidth))
		}
		content = append(content, line)
	}
	if len(items) == 0 {
		content = append(content, model.styles.subtle.Render("No items · Tab here and press a to add"))
	}
	if form.inlineChecklist() && form.control.inputMode {
		verb := "Add"
		if form.control.editIndex >= 0 {
			verb = "Edit"
		}
		inputWidth := max(contentWidth-lipgloss.Width(verb+": "), 4)
		content = append(content, "", model.styles.command.Render(verb+": "+textViewport(form.control.input, form.control.inputCursor, inputWidth)))
	}
	focused := form.focus != cardEditorDescriptionIndex(form)
	return model.cardEditorPanel(focused, layout.fieldsWidth, layout.bodyHeight, content)
}

func cardEditorLabel(field formField) string {
	if field.label == "Tags comma-separated" {
		return "Tags"
	}
	return field.label
}

func (model *Model) renderCardEditorPreview(layout cardEditorLayout) string {
	form := model.form
	index := cardEditorDescriptionIndex(form)
	focused := form.focus == index
	contentWidth := max(layout.previewWidth-4, 10)
	heading := "Description"
	if focused {
		heading = model.styles.header.Render(heading) + model.styles.subtle.Render(" · Enter edit")
	}
	content := []string{heading, ""}
	source := form.fields[index].value
	if form.previewSource != source || form.previewWidth != contentWidth || form.previewLines == nil {
		form.previewSource, form.previewWidth = source, contentWidth
		form.previewLines, form.previewErr = renderMarkdown(source, contentWidth)
		if form.previewLines == nil {
			form.previewLines = []string{}
		}
	}
	switch {
	case form.previewErr != nil:
		content = append(content, model.styles.error.Render(form.previewErr.Error()))
	case len(form.previewLines) == 0:
		content = append(content, model.styles.subtle.Render("No description. Press Enter to write Markdown."))
	default:
		content = append(content, form.previewLines...)
	}
	return model.cardEditorPanel(focused, layout.previewWidth, layout.bodyHeight, content)
}

func (model *Model) clickCardEditor(event tea.MouseEvent, width, height int) (tea.Model, tea.Cmd) {
	form := model.form
	layout := cardEditorLayoutForSize(width, height)
	if event.Y == 1 {
		hint := model.cardEditorHint()
		if insideMouseLabel(hint, "Ctrl-S save", event.X) {
			if form.inlineChecklist() {
				model.commitInlineChecklist()
			}
			return model.submitForm()
		}
		if insideMouseLabel(hint, "Esc cancel", event.X) {
			if form.inlineChecklist() {
				model.commitInlineChecklist()
			}
			return model.handleFormKey(tea.KeyMsg{Type: tea.KeyEsc})
		}
		return model, nil
	}
	row := event.Y - (layout.bodyTop + 1)
	if row < 0 || row >= layout.contentHeight() {
		return model, nil
	}
	if event.X >= layout.previewLeft {
		return model.focusCardEditorField(cardEditorDescriptionIndex(form), event.X)
	}
	indexes := cardEditorFieldIndexes(form)
	if row < len(indexes) {
		return model.focusCardEditorField(indexes[row], event.X)
	}
	checklistIndex := cardEditorChecklistIndex(form)
	if row < cardEditorChecklistHeaderRow(form) {
		return model, nil
	}
	wasInline := form.inlineChecklist()
	if form.control != nil && !wasInline {
		return model, nil
	}
	form.focus = checklistIndex
	model.syncCardEditorChecklist()
	control := form.control
	start, rows := form.checklistWindow(layout)
	item := start + row - cardEditorChecklistHeaderRow(form) - 1
	if row == cardEditorChecklistHeaderRow(form) || item >= start+rows || item >= len(control.checklist) || control.inputMode {
		return model, nil
	}
	if wasInline && control.selection == item {
		control.checklist[item].Done = !control.checklist[item].Done
		return model, nil
	}
	control.selection = item
	return model, nil
}

func (model *Model) focusCardEditorField(index, x int) (tea.Model, tea.Cmd) {
	form := model.form
	if form.inlineChecklist() {
		model.commitInlineChecklist()
	}
	wasFocused := form.focus == index
	form.focus = index
	field := &form.fields[index]
	switch {
	case field.kind == textField:
		labelWidth := 0
		for _, fieldIndex := range cardEditorFieldIndexes(form) {
			labelWidth = max(labelWidth, lipgloss.Width(cardEditorLabel(form.fields[fieldIndex])))
		}
		field.cursor = mouseRuneIndex(field.value, max(x-(2+labelWidth+1), 0))
	case field.kind == checkboxField:
		toggleCheckboxField(field)
	case wasFocused:
		form.openControl()
	}
	return model, nil
}
