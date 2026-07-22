package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
)

func templateNames(templates []domain.CardTemplate) []string {
	names := make([]string, len(templates))
	for index, template := range templates {
		names[index] = template.Name
	}
	return names
}

func templateByNameOrID(templates []domain.CardTemplate, value string) (domain.CardTemplate, bool) {
	for _, template := range templates {
		if template.ID == value || strings.EqualFold(template.Name, value) {
			return template, true
		}
	}
	return domain.CardTemplate{}, false
}

func nextTemplatePosition(templates []domain.CardTemplate) float64 {
	maximum := 0.0
	for _, template := range templates {
		if template.Position > maximum {
			maximum = template.Position
		}
	}
	return maximum + 1024
}

func cardFromTemplate(template domain.CardTemplate, boardID, columnID string, position float64) domain.Card {
	card := domain.Card{
		BoardID: boardID, ColumnID: columnID, Title: template.Title, Description: template.Description, Position: position,
		Tags: append([]string(nil), template.Tags...), Checklist: append([]domain.ChecklistItem(nil), template.Checklist...), Fields: map[string]domain.FieldValue{},
	}
	if template.Priority != nil {
		priority := *template.Priority
		card.Priority = &priority
	}
	if template.DueOffsetDays != nil {
		now := time.Now().In(time.Local)
		due := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, *template.DueOffsetDays).UTC()
		card.DueDate = &due
	}
	return card
}

func (model *Model) saveSelectedCardAsTemplate() tea.Cmd {
	if model.screen != boardScreen || model.board == nil || len(model.columns) == 0 || model.selectedCard().ID == "" {
		model.err = fmt.Errorf("select a card first")
		return nil
	}
	source := model.selectedCard()
	template := domain.CardTemplate{
		BoardID: model.board.ID, Name: source.Title, Title: source.Title, Description: source.Description,
		Tags: append([]string(nil), source.Tags...), Checklist: append([]domain.ChecklistItem(nil), source.Checklist...),
	}
	if source.Priority != nil {
		priority := *source.Priority
		template.Priority = &priority
	}
	model.loading = true
	return mutationCommand(boardScreen, "Template saved", func() error {
		templates, err := model.repo.ListCardTemplates(model.ctx, model.board.ID)
		if err != nil {
			return err
		}
		template.Position = nextTemplatePosition(templates)
		return model.repo.CreateCardTemplate(model.ctx, &template)
	})
}

func templatesDetail(templates []domain.CardTemplate) *detailPopup {
	lines := []string{}
	if len(templates) == 0 {
		lines = append(lines, "No templates. Use :card new-template or :card save-template.")
	}
	for _, template := range templates {
		priority := "none"
		if template.Priority != nil && *template.Priority != "" {
			priority = *template.Priority
		}
		due := "none"
		if template.DueOffsetDays != nil {
			due = fmt.Sprintf("+%dd", *template.DueOffsetDays)
		}
		lines = append(lines,
			template.Name,
			"  ID: "+template.ID,
			"  Title: "+template.Title,
			"  Priority: "+priority+" · Due: "+due,
			fmt.Sprintf("  Tags: %s · Checklist: %d", fallbackValue(strings.Join(template.Tags, ", ")), len(template.Checklist)),
			"",
		)
	}
	return &detailPopup{kind: "templates", title: "Card templates", lines: lines}
}

func (model *Model) handleTemplatesLoaded(message cardTemplatesLoadedMsg) (tea.Model, tea.Cmd) {
	if message.err != nil {
		model.err = message.err
		model.loading = false
		return model, nil
	}
	model.loading = false
	switch message.purpose {
	case templateLoadChoose:
		if len(message.templates) == 0 {
			model.err = fmt.Errorf("no templates on this board; use :card new-template or :card save-template")
			return model, nil
		}
		model.startCardFromTemplateForm(message.templates)
	case templateLoadList:
		model.detail = templatesDetail(message.templates)
	}
	return model, nil
}
