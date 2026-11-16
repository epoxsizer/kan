package app

// Selection is identified by persisted IDs, independently of list positions.
func (model *Model) selectedIDs() (string, string) {
	if len(model.columns) == 0 {
		return "", ""
	}
	columnID := model.columns[clampIndex(model.columnIndex, len(model.columns))].ID
	cards := model.visibleCards(columnID)
	if len(cards) == 0 {
		return columnID, ""
	}
	return columnID, cards[clampIndex(model.cardIndexes[columnID], len(cards))].ID
}

func (model *Model) restoreSelection(columnID, cardID string) {
	for index, column := range model.columns {
		if column.ID == columnID {
			model.columnIndex = index
		}
		cards := model.visibleCards(column.ID)
		model.cardIndexes[column.ID] = clampIndex(model.cardIndexes[column.ID], len(cards))
		for cardIndex, card := range cards {
			if card.ID == cardID {
				model.columnIndex = index
				model.cardIndexes[column.ID] = cardIndex
				return
			}
		}
	}
}
