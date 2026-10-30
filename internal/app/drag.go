package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
)

// cardDrag tracks a card pressed on the board. It becomes active on the first
// pointer motion; a press and release without motion stays a plain click.
type cardDrag struct {
	cardID       string
	title        string
	fromColumn   int
	fromIndex    int
	active       bool
	openedDetail bool
	targetColumn int
	targetIndex  int
}

func (model *Model) startCardDrag(columnIndex, cardIndex int) {
	column := model.columns[columnIndex]
	cards := model.visibleCards(column.ID)
	if cardIndex >= len(cards) {
		return
	}
	model.drag = &cardDrag{
		cardID: cards[cardIndex].ID, title: cards[cardIndex].Title,
		fromColumn: columnIndex, fromIndex: cardIndex,
		targetColumn: columnIndex, targetIndex: cardIndex,
	}
}

func (model *Model) handleCardDragMotion(event tea.MouseEvent) (tea.Model, tea.Cmd) {
	drag := model.drag
	if model.screen != boardScreen || model.loading {
		model.drag = nil
		return model, nil
	}
	if model.derivedBoardView() {
		model.drag = nil
		model.notice = "Clear filter and use position/no grouping before dragging cards"
		return model, nil
	}
	if drag.openedDetail {
		// The press on the selected card opened its detail; dragging replaces it.
		model.detail = nil
		drag.openedDetail = false
	}
	drag.active = true
	if columnIndex, ok := model.mouseColumnAt(event.X); ok {
		drag.targetColumn = columnIndex
	}
	model.columnIndex = drag.targetColumn
	drag.targetIndex = model.dropIndex(drag, event.Y)
	model.notice = fmt.Sprintf("Drag %q → %s, position %d · release to drop · Esc cancel", drag.title, model.columns[drag.targetColumn].Name, drag.targetIndex+1)
	return model, nil
}

// dropIndex returns the insertion index among the target column's cards
// without the dragged card, matching the repository's MoveCard semantics.
func (model *Model) dropIndex(drag *cardDrag, screenY int) int {
	column := model.columns[drag.targetColumn]
	cards := model.visibleCards(column.ID)
	others := 0
	for _, card := range cards {
		if card.ID != drag.cardID {
			others++
		}
	}
	if screenY <= 2 {
		return 0
	}
	cardIndex, top, height, ok := model.mouseCardBounds(drag.targetColumn, screenY)
	if !ok {
		if len(cards) > 0 {
			if firstIndex, firstTop, _, found := model.firstVisibleCardBounds(drag.targetColumn); found && screenY < firstTop {
				return model.indexWithout(cards, firstIndex, drag.cardID)
			}
		}
		return others
	}
	if cards[cardIndex].ID == drag.cardID {
		return model.indexWithout(cards, cardIndex, drag.cardID)
	}
	index := model.indexWithout(cards, cardIndex, drag.cardID)
	if (screenY-top)*2 >= height && height > 1 {
		index++
	}
	return min(index, others)
}

// indexWithout converts an index in cards to an index in cards without the
// dragged card.
func (model *Model) indexWithout(cards []domain.Card, index int, draggedID string) int {
	result := 0
	for position := 0; position < index && position < len(cards); position++ {
		if cards[position].ID != draggedID {
			result++
		}
	}
	return result
}

func (model *Model) finishCardDrag() (tea.Model, tea.Cmd) {
	drag := model.drag
	model.drag = nil
	if !drag.active || model.screen != boardScreen || model.loading || model.derivedBoardView() {
		return model, nil
	}
	fromColumn := model.columns[drag.fromColumn]
	toColumn := model.columns[drag.targetColumn]
	fromIndex := -1
	for index, card := range model.cards[fromColumn.ID] {
		if card.ID == drag.cardID {
			fromIndex = index
		}
	}
	if fromIndex < 0 {
		model.columnIndex = drag.fromColumn
		return model, nil
	}
	if fromColumn.ID == toColumn.ID && fromIndex == drag.targetIndex {
		model.notice = "Card left in place"
		return model, nil
	}
	notice := "Card moved"
	if fromColumn.ID == toColumn.ID {
		notice = "Card reordered"
	}
	record := cardMoveRecord{
		boardID: model.board.ID, cardID: drag.cardID,
		fromColumn: fromColumn.ID, fromIndex: fromIndex,
		toColumn: toColumn.ID, toIndex: drag.targetIndex,
		notice: notice,
	}
	model.pendingColumn, model.pendingCard = toColumn.ID, drag.cardID
	model.loading = true
	return model, model.moveCardCommand(record, false)
}

func (model *Model) cancelCardDrag() {
	if model.drag.active {
		model.columnIndex = model.drag.fromColumn
		model.notice = "Drag cancelled"
	}
	model.drag = nil
}

// mouseCardBounds returns the card under screenY with its first row and
// height on screen.
func (model *Model) mouseCardBounds(columnIndex, screenY int) (int, int, int, bool) {
	found := false
	var cardIndex, top, height int
	model.eachVisibleCardRow(columnIndex, func(index, rowTop, rowHeight int) bool {
		if screenY >= rowTop && screenY < rowTop+rowHeight {
			cardIndex, top, height, found = index, rowTop, rowHeight, true
			return false
		}
		return true
	})
	return cardIndex, top, height, found
}

func (model *Model) firstVisibleCardBounds(columnIndex int) (int, int, int, bool) {
	found := false
	var cardIndex, top, height int
	model.eachVisibleCardRow(columnIndex, func(index, rowTop, rowHeight int) bool {
		cardIndex, top, height, found = index, rowTop, rowHeight, true
		return false
	})
	return cardIndex, top, height, found
}

// eachVisibleCardRow walks the card rows rendered in a board column, skipping
// group headers, until visit returns false.
func (model *Model) eachVisibleCardRow(columnIndex int, visit func(cardIndex, top, height int) bool) {
	column := model.columns[columnIndex]
	cards := model.visibleCards(column.ID)
	if len(cards) == 0 {
		return
	}
	_, height := model.dimensions()
	maxRows := max(max(height-4, 1)-5, 1)
	selected := clampIndex(model.cardIndexes[column.ID], len(cards))
	rows, selectedRow := model.cardDisplayRows(cards, selected)
	heights := make([]int, len(rows))
	cardWidth := max(model.mouseColumnWidth(columnIndex)-6, 1)
	for index, row := range rows {
		rendered := model.renderCardDisplayRow(row, columnIndex == model.columnIndex && index == selectedRow, cardWidth, maxRows)
		heights[index] = lipgloss.Height(rendered)
	}
	start, end := visibleCardRowRange(heights, selectedRow, maxRows)
	y := 3
	for index := start; index < end; index++ {
		if rows[index].group == "" && !visit(rows[index].cardIndex, y, heights[index]) {
			return
		}
		y += heights[index]
	}
}
