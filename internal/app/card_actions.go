package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (model *Model) moveSelectedCard(columnDelta int) (tea.Model, tea.Cmd) {
	if model.derivedBoardView() {
		model.notice = "Clear filter and use position/no grouping before moving cards"
		return model, nil
	}
	if len(model.columns) == 0 {
		return model, nil
	}
	column := model.columns[model.columnIndex]
	cards := model.cards[column.ID]
	if len(cards) == 0 {
		return model, nil
	}
	targetColumnIndex := model.columnIndex + columnDelta
	if targetColumnIndex < 0 || targetColumnIndex >= len(model.columns) {
		return model, nil
	}
	cardIndex := clampIndex(model.cardIndexes[column.ID], len(cards))
	card := cards[cardIndex]
	targetColumn := model.columns[targetColumnIndex]
	targetIndex := min(cardIndex, len(model.cards[targetColumn.ID]))
	model.pendingColumn, model.pendingCard = targetColumn.ID, card.ID
	model.loading = true
	return model, mutationCommand(boardScreen, "Card moved", func() error {
		return model.repo.MoveCard(model.ctx, card.ID, targetColumn.ID, targetIndex)
	})
}

func (model *Model) reorderSelectedCard(delta int) (tea.Model, tea.Cmd) {
	if model.derivedBoardView() {
		model.notice = "Clear filter and use position/no grouping before reordering cards"
		return model, nil
	}
	if len(model.columns) == 0 {
		return model, nil
	}
	column := model.columns[model.columnIndex]
	cards := model.cards[column.ID]
	if len(cards) < 2 {
		return model, nil
	}
	index := clampIndex(model.cardIndexes[column.ID], len(cards))
	target := index + delta
	if target < 0 || target >= len(cards) {
		return model, nil
	}
	card := cards[index]
	model.pendingColumn, model.pendingCard = column.ID, card.ID
	model.loading = true
	return model, mutationCommand(boardScreen, "Card reordered", func() error {
		return model.repo.MoveCard(model.ctx, card.ID, column.ID, target)
	})
}
