package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
)

// cardQueuedMsg reports a card queued for agent processing.
type cardQueuedMsg struct {
	card domain.Card
	err  error
}

func agentStatusLabel(card domain.Card) string {
	if card.AgentName != "" {
		return card.AgentStatus + " (" + card.AgentName + ")"
	}
	return card.AgentStatus
}

// queueSelectedCard hands the selected card to the agent queue.
func (model *Model) queueSelectedCard() tea.Cmd {
	id := model.selectedCard().ID
	return func() tea.Msg {
		card, err := model.repo.QueueCard(model.ctx, id)
		return cardQueuedMsg{card: card, err: err}
	}
}
