package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

type batchMoveDoneMsg struct{ moved int }

type batchArchiveDoneMsg struct{ archived int }

func (model *Model) toggleMarkedCard() {
	card := model.selectedCard()
	if card.ID == "" {
		return
	}
	if model.marked == nil {
		model.marked = map[string]bool{}
	}
	if model.marked[card.ID] {
		delete(model.marked, card.ID)
	} else {
		model.marked[card.ID] = true
	}
}

func (model *Model) markedCardIDs() []string {
	ids := []string{}
	for _, cards := range model.cards {
		for _, card := range cards {
			if model.marked[card.ID] {
				ids = append(ids, card.ID)
			}
		}
	}
	return ids
}

// moveMarkedCardsTo appends every marked card to the target column. Cards
// that fail (for example a full WIP limit) are skipped and not counted.
func (model *Model) moveMarkedCardsTo(columnID string) tea.Cmd {
	ids := model.markedCardIDs()
	return func() tea.Msg {
		moved := 0
		for _, id := range ids {
			if err := model.repo.MoveCard(model.ctx, id, columnID, 0); err == nil {
				moved++
			}
		}
		return batchMoveDoneMsg{moved: moved}
	}
}

func (model *Model) archiveMarkedCards() tea.Cmd {
	ids := model.markedCardIDs()
	return func() tea.Msg {
		archived := 0
		for _, id := range ids {
			if _, err := model.repo.ArchiveCard(model.ctx, id); err == nil {
				archived++
			}
		}
		return batchArchiveDoneMsg{archived: archived}
	}
}
