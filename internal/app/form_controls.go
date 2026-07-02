package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/google/uuid"
)

type controlKind uint8

const (
	commentControl controlKind = iota
	dropdownControl
	calendarControl
	linksControl
	checklistControl
)

type formControl struct {
	kind          controlKind
	field         int
	value         string
	original      string
	cursor        int
	selection     int
	date          time.Time
	query         string
	queryCursor   int
	selected      map[string]bool
	standalone    bool
	checklist     []domain.ChecklistItem
	inputMode     bool
	input         string
	inputCursor   int
	inputOriginal string
	editIndex     int
	markdown      bool
	preview       bool
	previewOffset int
	undo          []editorState
	redo          []editorState
	searching     bool
	search        string
	searchCursor  int
	previewSource string
	previewWidth  int
	previewLines  []string
	previewErr    error
}

type editorState struct {
	value  string
	cursor int
}

type linkCandidate struct {
	id    string
	label string
}

func loadLinkCandidates(ctx context.Context, repo domain.Repository, projectID, excludeID string) tea.Cmd {
	return func() tea.Msg {
		boards, err := repo.ListBoards(ctx, projectID)
		if err != nil {
			return linkCandidatesLoadedMsg{err: err}
		}
		values := []linkCandidate{}
		for _, board := range boards {
			cards, listErr := repo.ListCards(ctx, board.ID)
			if listErr != nil {
				return linkCandidatesLoadedMsg{err: listErr}
			}
			for _, card := range cards {
				if card.ID != excludeID {
					values = append(values, linkCandidate{id: card.ID, label: board.Name + " / " + card.Title})
				}
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i].label < values[j].label })
		return linkCandidatesLoadedMsg{candidates: values}
	}
}

func (form *formModal) openControl() {
	field := form.fields[form.focus]
	control := &formControl{field: form.focus, value: field.value, original: field.value, cursor: utf8.RuneCountInString(field.value)}
	switch field.kind {
	case commentField:
		control.kind = commentControl
		control.markdown = field.markdown
	case dropdownField:
		control.kind = dropdownControl
		for index, option := range field.options {
			if strings.EqualFold(option, field.value) {
				control.selection = index
			}
		}
	case calendarField:
		control.kind = calendarControl
		control.date, _ = time.ParseInLocation("2006-01-02", field.value, time.Local)
		if control.date.IsZero() {
			control.date = time.Now()
		}
	case linksField:
		control.kind = linksControl
		control.selected = map[string]bool{}
		for _, id := range splitIDs(field.value) {
			control.selected[id] = true
		}
	case checklistField:
		control.kind = checklistControl
		control.editIndex = -1
		_ = json.Unmarshal([]byte(field.value), &control.checklist)
	}
	form.control = control
}

func (model *Model) handleFormControlKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	switch control.kind {
	case commentControl:
		return model.handleCommentKey(key)
	case dropdownControl:
		return model.handleDropdownKey(key)
	case calendarControl:
		return model.handleCalendarKey(key)
	case linksControl:
		return model.handleLinksKey(key)
	case checklistControl:
		return model.handleChecklistKey(key)
	}
	return model, nil
}

func (model *Model) handleChecklistKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	if key.String() == "ctrl+c" {
		return model, tea.Quit
	}
	if control.inputMode {
		switch key.String() {
		case "esc":
			if control.input != control.inputOriginal {
				model.discard = &discardModal{kind: discardChecklistInput, title: "Discard checklist text?", message: "The checklist item text has not been applied."}
			} else {
				control.inputMode = false
				control.input = ""
				control.inputCursor = 0
				control.editIndex = -1
			}
		case "enter":
			text := strings.TrimSpace(control.input)
			if text != "" {
				if control.editIndex >= 0 {
					control.checklist[control.editIndex].Text = text
				} else {
					control.checklist = append(control.checklist, domain.ChecklistItem{ID: uuid.NewString(), Text: text, Position: nextChecklistPosition(control.checklist)})
					control.selection = len(control.checklist) - 1
				}
			}
			control.inputMode = false
			control.input = ""
			control.inputCursor = 0
			control.inputOriginal = ""
			control.editIndex = -1
		default:
			result := editText(control.input, control.inputCursor, key, false)
			if result.handled {
				control.input, control.inputCursor = result.value, result.cursor
			}
		}
		return model, nil
	}
	switch key.String() {
	case "esc":
		model.form.control = nil
	case "ctrl+s":
		encoded, _ := json.Marshal(control.checklist)
		model.form.fields[control.field].value = string(encoded)
		model.form.control = nil
	case "up", "k":
		control.selection = max(control.selection-1, 0)
	case "down", "j":
		control.selection = min(control.selection+1, max(len(control.checklist)-1, 0))
	case " ", "enter":
		if len(control.checklist) > 0 {
			control.checklist[control.selection].Done = !control.checklist[control.selection].Done
		}
	case "a":
		control.inputMode = true
		control.input = ""
		control.inputCursor = 0
		control.inputOriginal = ""
		control.editIndex = -1
	case "e":
		if len(control.checklist) > 0 {
			control.inputMode = true
			control.editIndex = control.selection
			control.input = control.checklist[control.selection].Text
			control.inputCursor = len([]rune(control.input))
			control.inputOriginal = control.input
		}
	case "D":
		if len(control.checklist) > 0 {
			control.checklist = append(control.checklist[:control.selection], control.checklist[control.selection+1:]...)
			renumberChecklist(control.checklist)
			control.selection = clampIndex(control.selection, len(control.checklist))
		}
	case "J":
		if control.selection < len(control.checklist)-1 {
			control.checklist[control.selection], control.checklist[control.selection+1] = control.checklist[control.selection+1], control.checklist[control.selection]
			control.selection++
			renumberChecklist(control.checklist)
		}
	case "K":
		if control.selection > 0 {
			control.checklist[control.selection], control.checklist[control.selection-1] = control.checklist[control.selection-1], control.checklist[control.selection]
			control.selection--
			renumberChecklist(control.checklist)
		}
	}
	return model, nil
}

func (model *Model) handleCommentKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	if key.String() == "ctrl+c" {
		return model, tea.Quit
	}
	if control.searching {
		switch key.String() {
		case "esc":
			control.searching = false
		case "enter":
			control.cursor = moveToSearchMatch(control.value, control.search, control.cursor, false)
		case "shift+enter":
			control.cursor = moveToSearchMatch(control.value, control.search, control.cursor, true)
		default:
			result := editText(control.search, control.searchCursor, key, false)
			if result.handled {
				control.search, control.searchCursor = result.value, result.cursor
			}
		}
		return model, nil
	}
	switch key.String() {
	case "ctrl+p":
		if control.markdown {
			control.preview = !control.preview
		}
	case "ctrl+f":
		if control.markdown {
			control.searching = true
			control.searchCursor = len([]rune(control.search))
		}
	case "ctrl+z":
		if control.markdown {
			control.undoEdit()
		}
	case "ctrl+y":
		if control.markdown {
			control.redoEdit()
		}
	case "ctrl+g":
		model.form.err = ""
		return model, prepareExternalEditor(control.value)
	case "esc":
		if control.value != control.original {
			model.discard = &discardModal{kind: discardControl, title: "Discard editor changes?", message: "The text in this editor has not been applied."}
		} else {
			model.form.control = nil
		}
	case "ctrl+s":
		model.form.fields[control.field].value = control.value
		model.form.fields[control.field].cursor = len([]rune(control.value))
		model.form.control = nil
		if control.standalone {
			return model.submitForm()
		}
	default:
		if control.markdown && control.preview {
			_, height := model.dimensions()
			switch key.String() {
			case "up", "k":
				control.previewOffset = max(control.previewOffset-1, 0)
			case "down", "j":
				control.previewOffset++
			case "pgup":
				control.previewOffset = max(control.previewOffset-max(height-8, 1), 0)
			case "pgdown":
				control.previewOffset += max(height-8, 1)
			case "home", "g":
				control.previewOffset = 0
			case "end", "G":
				control.previewOffset = int(^uint(0) >> 1)
			}
			return model, nil
		}
		if control.markdown && (key.String() == "pgup" || key.String() == "pgdown") {
			_, height := model.dimensions()
			delta := max(height-8, 1)
			if key.String() == "pgup" {
				delta = -delta
			}
			control.cursor = moveEditorCursorLines(control.value, control.cursor, delta)
			return model, nil
		}
		result, special := textEditResult{}, false
		if control.markdown {
			result, special = markdownEdit(control.value, control.cursor, key.String())
		}
		if !special {
			result = editText(control.value, control.cursor, key, true)
		}
		if result.handled {
			if result.changed {
				control.pushUndo()
			}
			control.value, control.cursor = result.value, result.cursor
			if result.changed {
				model.form.err = ""
			}
		}
	}
	return model, nil
}

func (model *Model) handleDropdownKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	options := model.form.fields[control.field].options
	switch key.String() {
	case "esc":
		model.form.control = nil
	case "up", "k", "left", "h":
		control.selection = max(control.selection-1, 0)
	case "down", "j", "right", "l", "tab":
		control.selection = min(control.selection+1, len(options)-1)
	case "enter", "ctrl+s":
		if len(options) > 0 {
			model.form.fields[control.field].value = options[control.selection]
		}
		model.form.control = nil
	}
	return model, nil
}

func (model *Model) handleCalendarKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	switch key.String() {
	case "esc":
		model.form.control = nil
	case "left", "h":
		control.date = control.date.AddDate(0, 0, -1)
	case "right", "l":
		control.date = control.date.AddDate(0, 0, 1)
	case "up", "k":
		control.date = control.date.AddDate(0, 0, -7)
	case "down", "j":
		control.date = control.date.AddDate(0, 0, 7)
	case "pgup":
		control.date = control.date.AddDate(0, -1, 0)
	case "pgdown":
		control.date = control.date.AddDate(0, 1, 0)
	case "x", "delete":
		model.form.fields[control.field].value = ""
		model.form.control = nil
	case "enter", "ctrl+s":
		model.form.fields[control.field].value = control.date.Format("2006-01-02")
		model.form.control = nil
	}
	return model, nil
}

func (model *Model) handleLinksKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	control := model.form.control
	matches := model.linkMatches(control.query)
	switch key.String() {
	case "esc":
		model.form.control = nil
	case "ctrl+s":
		ids := []string{}
		for _, candidate := range model.form.linkCandidates {
			if control.selected[candidate.id] {
				ids = append(ids, candidate.id)
			}
		}
		model.form.fields[control.field].value = strings.Join(ids, ",")
		model.form.control = nil
	case "up":
		control.selection = max(control.selection-1, 0)
	case "down", "tab":
		control.selection = min(control.selection+1, max(len(matches)-1, 0))
	case "enter":
		if len(matches) > 0 {
			id := matches[clampIndex(control.selection, len(matches))].id
			control.selected[id] = !control.selected[id]
		}
	default:
		result := editText(control.query, control.queryCursor, key, false)
		if result.handled {
			control.query, control.queryCursor = result.value, result.cursor
		}
		if result.changed {
			control.selection = 0
		}
	}
	return model, nil
}

func (model *Model) linkMatches(query string) []linkCandidate {
	query = strings.ToLower(strings.TrimSpace(query))
	values := []linkCandidate{}
	for _, candidate := range model.form.linkCandidates {
		if fuzzyScore(query, strings.ToLower(candidate.label)) >= 0 {
			values = append(values, candidate)
		}
	}
	return values
}

func (model *Model) renderFormControl(width, height int) string {
	control := model.form.control
	switch control.kind {
	case commentControl:
		return model.renderCommentEditor(width, height)
	case dropdownControl:
		return model.renderDropdown(width, height)
	case calendarControl:
		return model.renderCalendar(width, height)
	case linksControl:
		return model.renderLinks(width, height)
	case checklistControl:
		return model.renderChecklist(width, height)
	}
	return ""
}

func (model *Model) renderChecklist(width, height int) string {
	control := model.form.control
	lines := []string{model.styles.header.Render("Checklist"), model.styles.subtle.Render("a add · e edit · Space toggle · D delete · J/K reorder · Ctrl-S apply"), ""}
	maxRows := max(height-11, 2)
	start := max(0, control.selection-maxRows+1)
	for index := start; index < min(start+maxRows, len(control.checklist)); index++ {
		item := control.checklist[index]
		mark := "[ ]"
		if item.Done {
			mark = "[x]"
		}
		line := mark + " " + item.Text
		if index == control.selection {
			line = model.styles.selected.Copy().Padding(0).Render(line)
		}
		lines = append(lines, line)
	}
	if len(control.checklist) == 0 && !control.inputMode {
		lines = append(lines, model.styles.subtle.Render("No checklist items. Press a to add one."))
	}
	if control.inputMode {
		verb := "Add"
		if control.editIndex >= 0 {
			verb = "Edit"
		}
		inputWidth := max(min(width-12, 66)-lipgloss.Width(verb+": "), 8)
		lines = append(lines, "", model.styles.command.Render(verb+": "+textViewport(control.input, control.inputCursor, inputWidth)), model.styles.subtle.Render("←/→ cursor · Home/End · Enter accept · Esc cancel"))
	}
	return centeredControl(model, width, height, 78, strings.Join(lines, "\n"))
}

func nextChecklistPosition(items []domain.ChecklistItem) float64 {
	maximum := 0.0
	for _, item := range items {
		maximum = max(maximum, item.Position)
	}
	return maximum + 1024
}

func renumberChecklist(items []domain.ChecklistItem) {
	for index := range items {
		items[index].Position = float64(index+1) * 1024
	}
}

func (model *Model) renderCommentEditor(width, height int) string {
	control := model.form.control
	layout := commentEditorLayoutForSize(width, height)
	title := model.form.fields[control.field].label + " editor"
	hint := "Arrows · Ctrl-G $EDITOR · Ctrl-S apply · Esc cancel"
	if control.markdown {
		hint = "Ctrl-P edit/preview · Ctrl-F find · Ctrl-Z/Y undo/redo · Ctrl-G $EDITOR · Ctrl-S apply"
	}
	hintStyle := model.styles.subtle
	if model.form.err != "" {
		hint = "Editor: " + model.form.err
		hintStyle = model.styles.error
	}
	wide := control.markdown && width >= 100
	bodyWidth := layout.contentWidth
	editorHeight := layout.viewportHeight
	if wide {
		bodyWidth = max((layout.contentWidth-3)/2, 10)
		editorHeight = max(editorHeight-1, 1)
	}
	editor := editorViewport(control.value, control.cursor, bodyWidth, editorHeight)
	body := editor
	if control.markdown {
		preview, renderErr := control.markdownPreview(bodyWidth)
		if renderErr != nil {
			preview = []string{model.styles.error.Render(renderErr.Error())}
		}
		control.previewOffset = clampDetailOffset(control.previewOffset, len(preview), layout.viewportHeight)
		end := min(control.previewOffset+layout.viewportHeight, len(preview))
		visiblePreview := strings.Join(preview[control.previewOffset:end], "\n")
		if wide {
			editorTitle, previewTitle := "EDIT", "PREVIEW"
			if control.preview {
				previewTitle += " *"
			} else {
				editorTitle += " *"
			}
			left := model.styles.subtle.Render(editorTitle) + "\n" + editor
			right := model.styles.subtle.Render(previewTitle) + "\n" + visiblePreview
			body = lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Width(bodyWidth).Render(left),
				" │ ",
				lipgloss.NewStyle().Width(bodyWidth).Render(right),
			)
		} else if control.preview {
			body = visiblePreview
		}
	}
	line, column := editorLineColumn(control.value, control.cursor)
	status := fmt.Sprintf("Ln %d, Col %d · %d chars", line, column, len([]rune(control.value)))
	if control.value != control.original {
		status += " · modified"
	}
	if control.searching {
		status = "Find: " + textViewport(control.search, control.searchCursor, max(layout.contentWidth-6, 1)) + " · Enter next · Shift-Enter previous · Esc close"
	}
	lines := []string{
		model.styles.header.Render(truncate(title, layout.contentWidth)),
		hintStyle.Render(truncate(hint, layout.contentWidth)),
		model.styles.subtle.Render(truncate(status, layout.contentWidth)),
	}
	if body != "" {
		lines = append(lines, strings.Split(body, "\n")...)
	}
	for len(lines) < layout.insideHeight {
		lines = append(lines, "")
	}
	if len(lines) > layout.insideHeight {
		lines = lines[:layout.insideHeight]
	}
	content := model.styles.help.Width(layout.boxWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

type commentEditorLayout struct {
	boxWidth       int
	contentWidth   int
	insideHeight   int
	viewportHeight int
}

func commentEditorLayoutForSize(width, height int) commentEditorLayout {
	width = max(width, 20)
	height = max(height, 6)
	boxWidth := max(width-2, 14)
	insideHeight := max(height-4, 1)
	return commentEditorLayout{
		boxWidth:       boxWidth,
		contentWidth:   max(boxWidth-4, 10),
		insideHeight:   insideHeight,
		viewportHeight: max(insideHeight-3, 1),
	}
}

func (model *Model) renderDropdown(width, height int) string {
	control := model.form.control
	field := model.form.fields[control.field]
	lines := []string{model.styles.header.Render(field.label), model.styles.subtle.Render("Arrows select · Enter apply · Esc cancel"), ""}
	for index, option := range field.options {
		line := option
		if index == control.selection {
			line = model.styles.selected.Copy().Padding(0).Render(line)
		}
		lines = append(lines, line)
	}
	return centeredControl(model, width, height, 46, strings.Join(lines, "\n"))
}

func (model *Model) renderCalendar(width, height int) string {
	selected := model.form.control.date
	first := time.Date(selected.Year(), selected.Month(), 1, 0, 0, 0, 0, time.Local)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	lines := []string{model.styles.header.Render(selected.Format("January 2006")), model.styles.subtle.Render("Arrows day/week · PgUp/PgDn month · Enter apply · x no due date"), "Su Mo Tu We Th Fr Sa"}
	for week := 0; week < 6; week++ {
		cells := []string{}
		for day := 0; day < 7; day++ {
			date := start.AddDate(0, 0, week*7+day)
			cell := fmt.Sprintf("%2d", date.Day())
			if date.Month() != selected.Month() {
				cell = model.styles.subtle.Render(cell)
			} else if sameDay(date, selected) {
				cell = model.styles.selected.Copy().Padding(0).Render(cell)
			}
			cells = append(cells, cell)
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	return centeredControl(model, width, height, 54, strings.Join(lines, "\n"))
}

func (model *Model) renderLinks(width, height int) string {
	control := model.form.control
	queryWidth := max(min(width-12, 64), 8)
	lines := []string{model.styles.header.Render("Related cards"), model.styles.command.Render("/" + textViewport(control.query, control.queryCursor, queryWidth)), model.styles.subtle.Render("Type to filter · ←/→ cursor · Enter toggle · Ctrl-S apply"), ""}
	if model.form.linksLoading {
		lines = append(lines, "Loading cards...")
	} else {
		matches := model.linkMatches(control.query)
		for index, candidate := range matches[:min(len(matches), max(height-10, 1))] {
			mark := "[ ]"
			if control.selected[candidate.id] {
				mark = "[x]"
			}
			line := mark + " " + candidate.label
			if index == control.selection {
				line = model.styles.selected.Copy().Padding(0).Render(line)
			}
			lines = append(lines, line)
		}
	}
	return centeredControl(model, width, height, 76, strings.Join(lines, "\n"))
}

func centeredControl(model *Model, width, height, maximum int, content string) string {
	boxWidth := min(maximum, max(width-4, 24))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, model.styles.help.Width(max(boxWidth-6, 18)).Render(content))
}

func verticalCursor(value []rune, cursor, direction int) int {
	start := lineStart(value, cursor)
	column := cursor - start
	if direction < 0 {
		if start == 0 {
			return cursor
		}
		previousEnd := start - 1
		previousStart := lineStart(value, previousEnd)
		return min(previousStart+column, previousEnd)
	}
	end := lineEnd(value, cursor)
	if end >= len(value) {
		return cursor
	}
	nextStart := end + 1
	return min(nextStart+column, lineEnd(value, nextStart))
}

func lineStart(value []rune, cursor int) int {
	for cursor > 0 && value[cursor-1] != '\n' {
		cursor--
	}
	return cursor
}

func lineEnd(value []rune, cursor int) int {
	for cursor < len(value) && value[cursor] != '\n' {
		cursor++
	}
	return cursor
}

func sameDay(left, right time.Time) bool {
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func columnNames(columns []domain.Column) []string {
	values := make([]string, len(columns))
	for index, column := range columns {
		values[index] = column.Name
	}
	return values
}

func selectedCardID(form *formModal, model *Model) string {
	if form.kind == editCardForm {
		return model.selectedCard().ID
	}
	return ""
}

func splitIDs(value string) []string {
	values := []string{}
	seen := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		id := strings.TrimSpace(part)
		if id != "" && !seen[id] {
			seen[id] = true
			values = append(values, id)
		}
	}
	return values
}

func (model *Model) startStandaloneCommentEdit() tea.Cmd {
	switch model.screen {
	case projectsScreen:
		if len(model.projects) == 0 {
			return nil
		}
		model.startProjectForm(true)
	case boardsScreen:
		if len(model.boards) == 0 {
			return nil
		}
		model.startBoardForm(true)
	case boardScreen:
		if len(model.columns) == 0 || model.selectedCard().ID == "" {
			return nil
		}
		command := model.startCardForm(true)
		model.form.focus = 1
		model.form.openControl()
		model.form.control.standalone = true
		return command
	}
	model.form.focus = 1
	model.form.openControl()
	model.form.control.standalone = true
	return nil
}

func editorViewport(value string, cursor, width, height int) string {
	width = max(width, 1)
	runes := []rune(value)
	lines := []string{}
	line := []rune{}
	lineWidth := 0
	cursorLine := 0
	appendRune := func(value rune, atCursor bool) {
		runeWidth := lipgloss.Width(string(value))
		if lineWidth > 0 && lineWidth+runeWidth > width {
			lines = append(lines, string(line))
			line = []rune{}
			lineWidth = 0
		}
		if atCursor {
			cursorLine = len(lines)
		}
		line = append(line, value)
		lineWidth += runeWidth
	}
	for index, valueRune := range runes {
		if index == cursor {
			appendRune('█', true)
		}
		if (valueRune < ' ' && valueRune != '\n' && valueRune != '\t') || valueRune == 0x7f {
			valueRune = '�'
		}
		if valueRune == '\n' {
			lines = append(lines, string(line))
			line = []rune{}
			lineWidth = 0
			continue
		}
		if valueRune == '\t' {
			for range 4 {
				appendRune(' ', false)
			}
			continue
		}
		appendRune(valueRune, false)
	}
	if cursor == len(runes) {
		appendRune('█', true)
	}
	lines = append(lines, string(line))
	start := max(cursorLine-height+1, 0)
	end := min(start+height, len(lines))
	return strings.Join(lines[start:end], "\n")
}
