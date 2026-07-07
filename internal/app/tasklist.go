package app

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/google/uuid"
)

var markdownTaskPattern = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[([ xX])\]\s+(.+?)\s*$`)

func markdownTasksToChecklist(markdown string, startPosition float64) []domain.ChecklistItem {
	items := []domain.ChecklistItem{}
	position := startPosition
	for _, line := range strings.Split(markdown, "\n") {
		matches := markdownTaskPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		text := strings.TrimSpace(matches[2])
		if text == "" {
			continue
		}
		done := strings.EqualFold(matches[1], "x")
		position += 1024
		items = append(items, domain.ChecklistItem{ID: uuid.NewString(), Text: text, Done: done, Position: position})
	}
	return items
}

func (model *Model) importMarkdownTasksToChecklist(markdown string) {
	if model.form == nil {
		return
	}
	checklistIndex := -1
	for index, field := range model.form.fields {
		if field.kind == checklistField {
			checklistIndex = index
			break
		}
	}
	if checklistIndex < 0 {
		model.form.err = "this form has no checklist field"
		return
	}
	var checklist []domain.ChecklistItem
	if err := json.Unmarshal([]byte(model.form.fields[checklistIndex].value), &checklist); err != nil {
		model.form.err = "invalid existing checklist: " + err.Error()
		return
	}
	position := 0.0
	for _, item := range checklist {
		if item.Position > position {
			position = item.Position
		}
	}
	items := markdownTasksToChecklist(markdown, position)
	if len(items) == 0 {
		model.form.err = "no Markdown task list items found"
		return
	}
	checklist = append(checklist, items...)
	contents, err := json.Marshal(checklist)
	if err != nil {
		model.form.err = "encode checklist: " + err.Error()
		return
	}
	model.form.fields[checklistIndex].value = string(contents)
	model.form.fields[checklistIndex].cursor = len([]rune(model.form.fields[checklistIndex].value))
	model.form.err = "Imported Markdown tasks to checklist; description text was kept"
}
