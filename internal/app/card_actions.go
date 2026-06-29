package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type cardMoveRecord struct {
	boardID    string
	cardID     string
	fromColumn string
	fromIndex  int
	toColumn   string
	toIndex    int
	notice     string
}

type cardMoveDoneMsg struct {
	record cardMoveRecord
	undo   bool
	err    error
}

type movePicker struct {
	targetColumnIndex int
}

func (model *Model) moveCardCommand(record cardMoveRecord, undo bool) tea.Cmd {
	return func() tea.Msg {
		columnID, index := record.toColumn, record.toIndex
		if undo {
			columnID, index = record.fromColumn, record.fromIndex
		}
		return cardMoveDoneMsg{
			record: record,
			undo:   undo,
			err:    model.repo.MoveCard(model.ctx, record.cardID, columnID, index),
		}
	}
}

func (model *Model) moveSelectedCard(columnDelta int) (tea.Model, tea.Cmd) {
	if model.derivedBoardView() {
		model.notice = "Clear filter and use position/no grouping before moving cards"
		return model, nil
	}
	targetColumnIndex := model.columnIndex + columnDelta
	return model.moveSelectedCardTo(targetColumnIndex)
}

func (model *Model) moveSelectedCardTo(targetColumnIndex int) (tea.Model, tea.Cmd) {
	if len(model.columns) == 0 || targetColumnIndex < 0 || targetColumnIndex >= len(model.columns) || targetColumnIndex == model.columnIndex {
		return model, nil
	}
	column := model.columns[model.columnIndex]
	cards := model.cards[column.ID]
	if len(cards) == 0 {
		return model, nil
	}
	cardIndex := clampIndex(model.cardIndexes[column.ID], len(cards))
	card := cards[cardIndex]
	targetColumn := model.columns[targetColumnIndex]
	targetIndex := min(cardIndex, len(model.cards[targetColumn.ID]))
	record := cardMoveRecord{
		boardID:    model.board.ID,
		cardID:     card.ID,
		fromColumn: column.ID,
		fromIndex:  cardIndex,
		toColumn:   targetColumn.ID,
		toIndex:    targetIndex,
		notice:     "Card moved",
	}
	model.movePicker = nil
	model.pendingColumn, model.pendingCard = targetColumn.ID, card.ID
	model.loading = true
	return model, model.moveCardCommand(record, false)
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
	record := cardMoveRecord{
		boardID:    model.board.ID,
		cardID:     card.ID,
		fromColumn: column.ID,
		fromIndex:  index,
		toColumn:   column.ID,
		toIndex:    target,
		notice:     "Card reordered",
	}
	model.pendingColumn, model.pendingCard = column.ID, card.ID
	model.loading = true
	return model, model.moveCardCommand(record, false)
}

func (model *Model) openMovePicker() {
	if model.derivedBoardView() {
		model.notice = "Clear filter and use position/no grouping before moving cards"
		return
	}
	if len(model.columns) < 2 || model.selectedCard().ID == "" {
		model.notice = "No card destination available"
		return
	}
	targets := model.availableMoveTargets()
	if len(targets) == 0 {
		model.notice = "No column has available WIP capacity"
		return
	}
	target := targets[0]
	for _, candidate := range targets {
		if candidate > model.columnIndex {
			target = candidate
			break
		}
	}
	model.movePicker = &movePicker{
		targetColumnIndex: target,
	}
	model.notice = ""
}

func (model *Model) availableMoveTargets() []int {
	targets := []int{}
	for index, column := range model.columns {
		if index == model.columnIndex {
			continue
		}
		if column.WIPLimit != nil && len(model.cards[column.ID]) >= *column.WIPLimit {
			continue
		}
		targets = append(targets, index)
	}
	return targets
}

func (model *Model) handleMovePickerKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		model.movePicker = nil
		return model, nil
	case "enter":
		return model.moveSelectedCardTo(model.movePicker.targetColumnIndex)
	case "j", "down", "l", "right":
		model.movePicker.targetColumnIndex = model.nextMoveTarget(1)
	case "k", "up", "h", "left":
		model.movePicker.targetColumnIndex = model.nextMoveTarget(-1)
	}
	return model, nil
}

func (model *Model) nextMoveTarget(delta int) int {
	targets := model.availableMoveTargets()
	if len(targets) == 0 {
		return model.movePicker.targetColumnIndex
	}
	position := 0
	for index, target := range targets {
		if target == model.movePicker.targetColumnIndex {
			position = index
			break
		}
	}
	position = (position + delta + len(targets)) % len(targets)
	return targets[position]
}

func (model *Model) renderMovePicker(width, height int) string {
	card := model.selectedCard()
	boxWidth := min(70, max(width-4, 30))
	contentWidth := max(boxWidth-6, 24)
	lines := []string{
		model.styles.header.Render("Move card"),
		truncate(card.Title, contentWidth),
		model.styles.subtle.Render(truncate("j/k or h/l select · Enter move · Esc cancel", contentWidth)),
		"",
	}
	maxRows := max(height-10, 2)
	start := max(model.movePicker.targetColumnIndex-maxRows+1, 0)
	start = min(start, max(len(model.columns)-maxRows, 0))
	end := min(start+maxRows, len(model.columns))
	nameWidth := max(contentWidth-17, 6)
	for index := start; index < end; index++ {
		column := model.columns[index]
		count := len(model.cards[column.ID])
		capacity := fmt.Sprintf("%d", count)
		state := ""
		if column.WIPLimit != nil {
			capacity = fmt.Sprintf("%d/%d", count, *column.WIPLimit)
			if count >= *column.WIPLimit && index != model.columnIndex {
				state = " FULL"
			}
		}
		if index == model.columnIndex {
			state = " CURRENT"
		}
		line := fmt.Sprintf("  %-*s %7s%s", nameWidth, truncate(column.Name, nameWidth), capacity, state)
		if index == model.movePicker.targetColumnIndex {
			line = model.styles.selected.Copy().Padding(0).Render("> " + strings.TrimPrefix(line, "  "))
		}
		lines = append(lines, line)
	}
	if start > 0 || end < len(model.columns) {
		lines = append(lines, model.styles.subtle.Render(fmt.Sprintf("columns %d-%d/%d", start+1, end, len(model.columns))))
	}
	popup := model.styles.help.Width(max(boxWidth-6, 24)).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}

func (model *Model) undoLastCardMove() (tea.Model, tea.Cmd) {
	if model.lastMove == nil || model.board == nil || model.lastMove.boardID != model.board.ID {
		model.notice = "No card move to undo"
		return model, nil
	}
	if model.derivedBoardView() {
		model.notice = "Clear filter and use position/no grouping before undoing a move"
		return model, nil
	}
	record := *model.lastMove
	model.pendingColumn, model.pendingCard = record.fromColumn, record.cardID
	model.loading = true
	return model, model.moveCardCommand(record, true)
}
