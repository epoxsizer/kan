package app

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCardDraftSurvivesUnrelatedAgentChangeAndSavesOriginalCard(t *testing.T) {
	model, repo, original := cardEditorModel(t)
	model.form.fields[0].value = "Human draft"
	other := domain.Card{BoardID: original.BoardID, ColumnID: original.ColumnID, Title: "Agent card", Position: 2048}
	require.NoError(t, repo.CreateCard(context.Background(), &other))
	_, command := model.Update(ExternalChangeMsg{Action: "card created", BoardID: other.BoardID, ColumnID: other.ColumnID, CardID: other.ID})
	runCommands(model, command)
	require.NotNil(t, model.form)
	require.Equal(t, original.ID, model.selectedCard().ID, "external changes must preserve human selection")
	// The draft must keep its target even if selection changes independently.
	model.cardIndexes[other.ColumnID] = 1
	_, command = model.Update(key("ctrl+s"))
	require.NotNil(t, model.form, "retain the draft until persistence succeeds")
	runCommands(model, command)
	require.Nil(t, model.form)
	stored, err := repo.GetCard(context.Background(), original.ID)
	require.NoError(t, err)
	require.Equal(t, "Human draft", stored.Title)
	stored, err = repo.GetCard(context.Background(), other.ID)
	require.NoError(t, err)
	require.Equal(t, "Agent card", stored.Title)
}

func TestCardDraftSurvivesConflictAndFailedSave(t *testing.T) {
	model, repo, card := cardEditorModel(t)
	model.form.fields[0].value = "Unsaved human work"
	card.Title = "Agent update"
	require.NoError(t, repo.UpdateCard(context.Background(), &card))
	_, command := model.Update(ExternalChangeMsg{Action: "card updated", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
	runCommands(model, command)
	require.NotNil(t, model.form)
	require.Equal(t, "Unsaved human work", model.form.fields[0].value)
	require.Contains(t, model.form.err, "changed")
	_, command = model.Update(key("ctrl+s"))
	runCommands(model, command)
	require.NotNil(t, model.form)
	require.Equal(t, "Unsaved human work", model.form.fields[0].value)
	require.Contains(t, model.form.err, "changed")
	stored, err := repo.GetCard(context.Background(), card.ID)
	require.NoError(t, err)
	require.Equal(t, "Agent update", stored.Title)
}

func TestCardDraftRejectsDuplicateSavesWhilePending(t *testing.T) {
	model, _, _ := cardEditorModel(t)
	model.form.fields[0].value = "Save once"
	_, first := model.Update(key("ctrl+s"))
	require.NotNil(t, first)
	_, second := model.Update(key("ctrl+s"))
	require.Nil(t, second)
	runCommands(model, first)
	require.Nil(t, model.form)
}

func TestConflictReloadRequiresConfirmationAndUsesOriginalCard(t *testing.T) {
	model, repo, card := cardEditorModel(t)
	model.form.fields[0].value = "My draft"
	card.Title = "Latest version"
	require.NoError(t, repo.UpdateCard(context.Background(), &card))
	_, command := model.Update(ExternalChangeMsg{Action: "card updated", BoardID: card.BoardID, CardID: card.ID})
	runCommands(model, command)
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	require.NotNil(t, model.discard)
	model.Update(key("n"))
	require.Equal(t, "My draft", model.form.fields[0].value)
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	_, command = model.Update(key("y"))
	runCommands(model, command)
	require.NotNil(t, model.form)
	require.Equal(t, "Latest version", model.form.fields[0].value)
	require.Empty(t, model.form.err)
}

func TestQuitProtectsUnsavedFormAndNestedEditor(t *testing.T) {
	for _, nested := range []bool{false, true} {
		model, _, _ := cardEditorModel(t)
		if nested {
			model.form.focus = 1
			model.Update(key("enter"))
			model.Update(key("Unsaved description"))
		} else {
			model.form.fields[0].value = "Unsaved title"
		}
		_, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		require.Nil(t, command)
		require.NotNil(t, model.discard)
		model.Update(key("n"))
		require.NotNil(t, model.form)
	}
}
