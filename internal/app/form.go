package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
)

type formKind uint8

const (
	createProjectForm formKind = iota
	editProjectForm
	createBoardForm
	editBoardForm
	createColumnForm
	editColumnForm
	createCardForm
	editCardForm
	settingsForm
)

type formField struct {
	label   string
	value   string
	cursor  int
	kind    fieldKind
	options []string
}

type fieldKind uint8

const (
	textField fieldKind = iota
	commentField
	dropdownField
	calendarField
	linksField
	checklistField
)

var columnColors = []string{"Blue", "Green", "Yellow", "Orange", "Red", "Purple", "Gray"}
var priorities = []string{"Low", "Medium", "High", "Urgent"}
var layoutOptions = []string{"Table", "Cards"}
var booleanOptions = []string{"Enabled", "Disabled"}
var sortOptions = []string{"Position", "Priority", "Due date", "Title"}
var groupOptions = []string{"None", "Priority", "Due date", "First tag"}

type formModal struct {
	kind           formKind
	title          string
	fields         []formField
	focus          int
	err            string
	control        *formControl
	linkCandidates []linkCandidate
	linksLoading   bool
	initialValues  []string
}

type deleteKind uint8

const (
	deleteProject deleteKind = iota
	deleteBoard
	deleteColumn
	deleteCard
)

type confirmModal struct {
	kind    deleteKind
	title   string
	message string
	id      string
}

type discardKind uint8

const (
	discardForm discardKind = iota
	discardControl
	discardChecklistInput
)

type discardModal struct {
	kind    discardKind
	title   string
	message string
}

func (model *Model) startProjectForm(edit bool) {
	form := &formModal{kind: createProjectForm, title: "Add project", fields: []formField{{label: "Name"}, {label: "Comments", kind: commentField}}}
	if edit {
		project := model.projects[model.projectIndex]
		form.kind, form.title = editProjectForm, "Edit project"
		form.fields[0].value, form.fields[1].value = project.Name, project.Description
	}
	model.activateForm(form)
}

func (model *Model) startBoardForm(edit bool) {
	form := &formModal{kind: createBoardForm, title: "Add board", fields: []formField{{label: "Name"}, {label: "Comments", kind: commentField}}}
	if edit {
		board := model.boards[model.boardIndex]
		form.kind, form.title = editBoardForm, "Edit board"
		form.fields[0].value, form.fields[1].value = board.Name, board.Description
	}
	model.activateForm(form)
}

func (model *Model) startColumnForm(edit bool) {
	form := &formModal{kind: createColumnForm, title: "Add column", fields: []formField{{label: "Name"}, {label: "WIP limit", value: "10"}, {label: "Color", value: "Blue", kind: dropdownField, options: columnColors}}}
	if edit {
		column := model.columns[model.columnIndex]
		form.kind, form.title = editColumnForm, "Edit column"
		form.fields[0].value = column.Name
		if column.WIPLimit != nil {
			form.fields[1].value = strconv.Itoa(*column.WIPLimit)
		} else {
			form.fields[1].value = ""
		}
		if column.Color != nil {
			form.fields[2].value = *column.Color
			form.fields[2].options = withLegacyOption(columnColors, *column.Color)
		} else {
			form.fields[2].value = ""
		}
	}
	model.activateForm(form)
}

func (model *Model) startCardForm(edit bool) tea.Cmd {
	column := model.columns[model.columnIndex]
	form := &formModal{kind: createCardForm, title: "Add card", fields: []formField{
		{label: "Title"}, {label: "Comments", kind: commentField}, {label: "Status", value: column.Name, kind: dropdownField, options: columnNames(model.columns)}, {label: "Priority", value: "Medium", kind: dropdownField, options: priorities}, {label: "Due date", value: time.Now().Format("2006-01-02"), kind: calendarField}, {label: "Tags comma-separated"}, {label: "Related cards", kind: linksField}, {label: "Checklist", value: "[]", kind: checklistField},
	}}
	if edit {
		card := model.selectedCard()
		form.kind, form.title = editCardForm, "Edit card"
		form.fields[0].value, form.fields[1].value = card.Title, card.Description
		form.fields[2].value = column.Name
		if card.Priority != nil {
			form.fields[3].value = *card.Priority
			form.fields[3].options = withLegacyOption(priorities, *card.Priority)
		} else {
			form.fields[3].value = ""
		}
		if card.DueDate != nil {
			form.fields[4].value = card.DueDate.Format("2006-01-02")
		} else {
			form.fields[4].value = ""
		}
		form.fields[5].value = strings.Join(card.Tags, ", ")
		form.fields[6].value = strings.Join(card.RelatedCardIDs, ",")
		checklist, _ := json.Marshal(card.Checklist)
		form.fields[7].value = string(checklist)
	}
	form.linksLoading = true
	model.activateForm(form)
	return loadLinkCandidates(model.ctx, model.repo, model.project.ID, selectedCardID(form, model))
}

func (model *Model) startSettingsForm() {
	showTags := "Disabled"
	if model.showCardTags {
		showTags = "Enabled"
	}
	model.activateForm(&formModal{kind: settingsForm, title: "Settings", fields: []formField{
		{label: "Projects/boards layout", value: titleCase(model.listLayout.String()), kind: dropdownField, options: layoutOptions},
		{label: "Card title tags", value: showTags, kind: dropdownField, options: booleanOptions},
		{label: "Card sort", value: titleCase(model.sortMode.String()), kind: dropdownField, options: sortOptions},
		{label: "Card group", value: titleCase(model.groupMode.String()), kind: dropdownField, options: groupOptions},
	}})
}

func (model *Model) activateForm(form *formModal) {
	for index := range form.fields {
		form.fields[index].cursor = len([]rune(form.fields[index].value))
	}
	form.initialValues = textValues(form.fields)
	model.form = form
}

func (model *Model) handleFormKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := model.form
	if form.control != nil {
		return model.handleFormControlKey(key)
	}
	switch key.String() {
	case "ctrl+c":
		return model, tea.Quit
	case "esc":
		if model.formDirty() {
			model.discard = &discardModal{kind: discardForm, title: "Discard form changes?", message: "Your edits have not been saved."}
		} else {
			model.form = nil
		}
		return model, nil
	case "ctrl+s":
		return model.submitForm()
	case "tab", "down":
		form.focus = min(form.focus+1, len(form.fields)-1)
		return model, nil
	case "shift+tab", "up":
		form.focus = max(form.focus-1, 0)
		return model, nil
	case "enter":
		if form.fields[form.focus].kind != textField {
			form.openControl()
			return model, nil
		}
		if form.focus == len(form.fields)-1 {
			return model.submitForm()
		}
		form.focus++
		return model, nil
	}
	field := &form.fields[form.focus]
	if field.kind == textField {
		result := editText(field.value, field.cursor, key, false)
		if result.handled {
			field.value, field.cursor = result.value, result.cursor
		}
	}
	form.err = ""
	return model, nil
}

func (model *Model) formDirty() bool {
	return model.form != nil && !equalTextValues(model.form.initialValues, textValues(model.form.fields))
}

func (model *Model) submitForm() (tea.Model, tea.Cmd) {
	form := model.form
	value := func(index int) string { return strings.TrimSpace(form.fields[index].value) }
	raw := func(index int) string { return form.fields[index].value }
	invalid := func(err error) (tea.Model, tea.Cmd) {
		form.err = err.Error()
		return model, nil
	}
	switch form.kind {
	case settingsForm:
		layout, err := parseListLayout(value(0))
		if err != nil {
			return invalid(err)
		}
		showTags, err := parseEnabled(value(1))
		if err != nil {
			return invalid(err)
		}
		sortMode, err := parseCardSort(value(2))
		if err != nil {
			return invalid(err)
		}
		groupMode, err := parseCardGroup(value(3))
		if err != nil {
			return invalid(err)
		}
		model.listLayout = layout
		model.showCardTags = showTags
		model.sortMode = sortMode
		model.groupMode = groupMode
		model.cardIndexes = make(map[string]int, len(model.columns))
		model.form = nil
		model.notice = "Settings applied"
		return model, nil
	case createProjectForm:
		project := domain.Project{Name: value(0), Description: raw(1), Position: nextProjectPosition(model.projects)}
		if err := domain.ValidateProject(project); err != nil {
			return invalid(err)
		}
		model.form = nil
		model.loading = true
		return model, mutationCommand(projectsScreen, "Project added", func() error { return model.repo.CreateProject(model.ctx, &project) })
	case editProjectForm:
		project := model.projects[model.projectIndex]
		project.Name, project.Description = value(0), raw(1)
		if err := domain.ValidateProject(project); err != nil {
			return invalid(err)
		}
		model.form = nil
		model.loading = true
		return model, mutationCommand(projectsScreen, "Project updated", func() error { return model.repo.UpdateProject(model.ctx, &project) })
	case createBoardForm:
		board := domain.Board{ProjectID: model.project.ID, Name: value(0), Description: raw(1), Position: nextBoardPosition(model.boards)}
		if err := domain.ValidateBoard(board); err != nil {
			return invalid(err)
		}
		model.form = nil
		model.loading = true
		return model, mutationCommand(boardsScreen, "Board added", func() error { return model.repo.CreateBoard(model.ctx, &board) })
	case editBoardForm:
		board := model.boards[model.boardIndex]
		board.Name, board.Description = value(0), raw(1)
		if err := domain.ValidateBoard(board); err != nil {
			return invalid(err)
		}
		model.form = nil
		model.loading = true
		return model, mutationCommand(boardsScreen, "Board updated", func() error { return model.repo.UpdateBoard(model.ctx, &board) })
	case createColumnForm, editColumnForm:
		column := domain.Column{BoardID: model.board.ID, Position: nextColumnPosition(model.columns)}
		if form.kind == editColumnForm {
			column = model.columns[model.columnIndex]
		}
		column.Name = value(0)
		if value(1) == "" {
			column.WIPLimit = nil
		} else {
			wip, err := strconv.Atoi(value(1))
			if err != nil || wip < 1 {
				return invalid(fmt.Errorf("WIP limit must be a positive integer"))
			}
			column.WIPLimit = &wip
		}
		if value(2) == "" {
			column.Color = nil
		} else {
			color := value(2)
			column.Color = &color
		}
		if err := domain.ValidateColumn(column); err != nil {
			return invalid(err)
		}
		model.form = nil
		model.loading = true
		if form.kind == createColumnForm {
			return model, mutationCommand(boardScreen, "Column added", func() error { return model.repo.CreateColumn(model.ctx, &column) })
		}
		return model, mutationCommand(boardScreen, "Column updated", func() error { return model.repo.UpdateColumn(model.ctx, &column) })
	case createCardForm, editCardForm:
		column, ok := model.columnByNameOrID(value(2))
		if !ok {
			return invalid(fmt.Errorf("status must match a column name or ID"))
		}
		card := domain.Card{BoardID: model.board.ID, ColumnID: column.ID, Position: nextCardPosition(model.cards[column.ID]), Tags: []string{}, Checklist: []domain.ChecklistItem{}, Fields: map[string]domain.FieldValue{}}
		originalColumnID := ""
		if form.kind == editCardForm {
			card = model.selectedCard()
			originalColumnID = card.ColumnID
		}
		if column.ID != originalColumnID && column.WIPLimit != nil && len(model.cards[column.ID]) >= *column.WIPLimit {
			return invalid(fmt.Errorf("%w: target column WIP limit reached", domain.ErrConflict))
		}
		card.Title, card.Description, card.ColumnID = value(0), raw(1), column.ID
		if form.kind == editCardForm && card.ColumnID != originalColumnID {
			card.Position = nextCardPosition(model.cards[column.ID])
		}
		if value(3) == "" {
			card.Priority = nil
		} else {
			priority := value(3)
			card.Priority = &priority
		}
		if value(4) == "" {
			card.DueDate = nil
		} else {
			due, err := time.Parse("2006-01-02", value(4))
			if err != nil {
				return invalid(fmt.Errorf("due date must use YYYY-MM-DD"))
			}
			due = due.UTC()
			card.DueDate = &due
		}
		card.Tags = splitTags(value(5))
		card.RelatedCardIDs = splitIDs(value(6))
		if err := json.Unmarshal([]byte(raw(7)), &card.Checklist); err != nil {
			return invalid(fmt.Errorf("invalid checklist: %w", err))
		}
		if err := domain.ValidateCard(card); err != nil {
			return invalid(err)
		}
		model.form = nil
		model.loading = true
		if form.kind == createCardForm {
			return model, mutationCommand(boardScreen, "Card added", func() error { return model.repo.CreateCard(model.ctx, &card) })
		}
		return model, mutationCommand(boardScreen, "Card updated", func() error { return model.repo.UpdateCard(model.ctx, &card) })
	}
	return model, nil
}

func (model *Model) handleConfirmKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return model, tea.Quit
	}
	if key.String() == "esc" || key.String() == "n" {
		model.confirm = nil
		return model, nil
	}
	if key.String() != "y" {
		return model, nil
	}
	confirmation := *model.confirm
	model.confirm = nil
	model.loading = true
	switch confirmation.kind {
	case deleteProject:
		model.project = nil
		return model, mutationCommand(projectsScreen, "Project deleted", func() error { return model.repo.DeleteProject(model.ctx, confirmation.id) })
	case deleteBoard:
		model.board = nil
		return model, mutationCommand(boardsScreen, "Board deleted", func() error { return model.repo.DeleteBoard(model.ctx, confirmation.id) })
	case deleteColumn:
		return model, mutationCommand(boardScreen, "Column deleted", func() error { return model.repo.DeleteColumn(model.ctx, confirmation.id) })
	case deleteCard:
		return model, mutationCommand(boardScreen, "Card deleted", func() error { return model.repo.DeleteCard(model.ctx, confirmation.id) })
	}
	return model, nil
}

func (model *Model) handleDiscardKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return model, tea.Quit
	}
	if key.String() == "esc" || key.String() == "n" {
		model.discard = nil
		return model, nil
	}
	if key.String() != "y" {
		return model, nil
	}
	discard := model.discard
	model.discard = nil
	switch discard.kind {
	case discardForm:
		model.form = nil
	case discardControl:
		if model.form != nil {
			model.form.control = nil
		}
	case discardChecklistInput:
		if model.form != nil && model.form.control != nil {
			control := model.form.control
			control.inputMode = false
			control.input = ""
			control.inputCursor = 0
			control.editIndex = -1
		}
	}
	return model, nil
}

func (model *Model) renderForm(width, height int) string {
	if model.form.control != nil {
		return model.renderFormControl(width, height)
	}
	boxWidth := min(70, max(width-4, 26))
	innerWidth := max(boxWidth-6, 20)
	contentWidth := max(innerWidth-4, 16)
	hint := "Tab fields · ←/→ edit · Ctrl-S save · Esc cancel"
	if model.form.fields[model.form.focus].kind != textField {
		hint = "Tab fields · Enter open · Ctrl-S save · Esc cancel"
	}
	lines := []string{model.styles.header.Render(model.form.title), model.styles.subtle.Render(truncate(hint, contentWidth)), ""}
	for index, field := range model.form.fields {
		label := field.label + ": "
		if index == model.form.focus {
			inputWidth := max(contentWidth-lipgloss.Width(label)-2, 1)
			input := fieldDisplayValue(field, model.form.linkCandidates)
			if field.kind == textField {
				input = textViewport(field.value, field.cursor, inputWidth)
			} else {
				input = truncate(input, inputWidth)
			}
			lines = append(lines, model.styles.selected.Copy().Padding(0).Width(contentWidth).Render("> "+label+input))
		} else {
			input := fieldDisplayValue(field, model.form.linkCandidates)
			input = truncate(input, max(contentWidth-lipgloss.Width(label)-2, 1))
			lines = append(lines, "  "+label+input)
		}
	}
	if model.form.err != "" {
		lines = append(lines, "", model.styles.error.Render(truncate(model.form.err, contentWidth)))
	}
	popup := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}

func (model *Model) renderConfirm(width, height int) string {
	boxWidth := min(64, max(width-4, 24))
	innerWidth := max(boxWidth-6, 18)
	contentWidth := max(innerWidth-4, 14)
	lines := []string{model.styles.error.Render(model.confirm.title), "", truncate(model.confirm.message, contentWidth), "", model.styles.subtle.Render("y confirm · n / Esc cancel")}
	popup := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}

func (model *Model) renderDiscard(width, height int) string {
	boxWidth := min(64, max(width-4, 24))
	innerWidth := max(boxWidth-6, 18)
	contentWidth := max(innerWidth-4, 14)
	lines := []string{model.styles.error.Render(model.discard.title), "", truncate(model.discard.message, contentWidth), "", model.styles.subtle.Render("y discard · n / Esc keep editing")}
	popup := model.styles.help.Width(innerWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}

func nextProjectPosition(values []domain.Project) float64 {
	maximum := 0.0
	for _, value := range values {
		maximum = max(maximum, value.Position)
	}
	return maximum + 1024
}
func nextBoardPosition(values []domain.Board) float64 {
	maximum := 0.0
	for _, value := range values {
		maximum = max(maximum, value.Position)
	}
	return maximum + 1024
}
func nextColumnPosition(values []domain.Column) float64 {
	maximum := 0.0
	for _, value := range values {
		maximum = max(maximum, value.Position)
	}
	return maximum + 1024
}
func nextCardPosition(values []domain.Card) float64 {
	maximum := 0.0
	for _, value := range values {
		maximum = max(maximum, value.Position)
	}
	return maximum + 1024
}

func splitTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	tags := []string{}
	seen := map[string]struct{}{}
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func fieldDisplayValue(field formField, candidates []linkCandidate) string {
	switch field.kind {
	case commentField:
		if field.value == "" {
			return "(empty · Enter editor)"
		}
		return fmt.Sprintf("%d characters · Enter editor", len([]rune(field.value)))
	case dropdownField, calendarField:
		return field.value + " ▾"
	case linksField:
		ids := splitIDs(field.value)
		if len(ids) == 0 {
			return "none · Enter selector"
		}
		labels := map[string]string{}
		for _, candidate := range candidates {
			labels[candidate.id] = candidate.label
		}
		if label := labels[ids[0]]; label != "" {
			if len(ids) == 1 {
				return label + " · Enter selector"
			}
			return fmt.Sprintf("%s +%d · Enter selector", label, len(ids)-1)
		}
		return fmt.Sprintf("%d selected · Enter selector", len(ids))
	case checklistField:
		var items []domain.ChecklistItem
		if json.Unmarshal([]byte(field.value), &items) != nil || len(items) == 0 {
			return "0 items · Enter checklist"
		}
		done := 0
		for _, item := range items {
			if item.Done {
				done++
			}
		}
		return fmt.Sprintf("%d/%d done · Enter checklist", done, len(items))
	default:
		return field.value
	}
}

func withLegacyOption(options []string, value string) []string {
	for _, option := range options {
		if strings.EqualFold(option, value) {
			return options
		}
	}
	return append(append([]string{}, options...), value)
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func parseListLayout(value string) (listLayout, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "table":
		return tableLayout, nil
	case "cards":
		return cardsLayout, nil
	default:
		return tableLayout, fmt.Errorf("layout must be Table or Cards")
	}
}

func parseEnabled(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enabled", "yes", "true", "on":
		return true, nil
	case "disabled", "no", "false", "off":
		return false, nil
	default:
		return false, fmt.Errorf("value must be Enabled or Disabled")
	}
}

func parseCardSort(value string) (cardSort, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "position":
		return sortPosition, nil
	case "priority":
		return sortPriority, nil
	case "due date":
		return sortDue, nil
	case "title":
		return sortTitle, nil
	default:
		return sortPosition, fmt.Errorf("card sort must be Position, Priority, Due date, or Title")
	}
}

func parseCardGroup(value string) (cardGroup, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return groupNone, nil
	case "priority":
		return groupPriority, nil
	case "due date":
		return groupDue, nil
	case "first tag":
		return groupTag, nil
	default:
		return groupNone, fmt.Errorf("card group must be None, Priority, Due date, or First tag")
	}
}

func (model *Model) selectedCard() domain.Card {
	column := model.columns[model.columnIndex]
	cards := model.visibleCards(column.ID)
	if len(cards) == 0 {
		return domain.Card{}
	}
	return cards[clampIndex(model.cardIndexes[column.ID], len(cards))]
}
func (model *Model) columnByNameOrID(value string) (domain.Column, bool) {
	for _, column := range model.columns {
		if column.ID == value || strings.EqualFold(column.Name, value) {
			return column, true
		}
	}
	return domain.Column{}, false
}
