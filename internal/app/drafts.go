package app

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
)

type cardDraftReloadedMsg struct {
	form   *formModal
	card   domain.Card
	column domain.Column
	err    error
}

func (model *Model) reloadCardDraft() tea.Cmd {
	form, ctx, repo := model.form, model.ctx, model.repo
	form.saving = true
	return func() tea.Msg {
		card, err := repo.GetCard(ctx, form.entityID)
		var column domain.Column
		if err == nil {
			column, err = repo.GetColumn(ctx, card.ColumnID)
		}
		return cardDraftReloadedMsg{form: form, card: card, column: column, err: err}
	}
}

func (model *Model) draftDirty() bool {
	if model.formDirty() {
		return true
	}
	if model.form == nil || model.form.control == nil {
		return false
	}
	control := model.form.control
	switch control.kind {
	case commentControl:
		return control.value != control.original
	case checklistControl:
		encoded, _ := json.Marshal(control.checklist)
		return string(encoded) != model.form.fields[control.field].value || control.inputMode && control.input != control.inputOriginal
	case linksControl:
		original := splitIDs(model.form.fields[control.field].value)
		count := 0
		for _, selected := range control.selected {
			if selected {
				count++
			}
		}
		if count != len(original) {
			return true
		}
		for _, id := range original {
			if !control.selected[id] {
				return true
			}
		}
	}
	return false
}

func (model *Model) requestQuit() (tea.Model, tea.Cmd) {
	if model.form != nil && model.form.saving {
		return model, nil
	}
	if model.draftDirty() {
		model.discard = &discardModal{kind: discardQuit, title: "Quit and discard changes?", message: "Your draft has not been saved. Choose n to keep editing."}
		return model, nil
	}
	return model, tea.Quit
}
