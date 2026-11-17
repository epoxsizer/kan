package tasks

import (
	"context"
	"fmt"

	"github.com/epoxsizer/kan/internal/domain"
)

// QueueCard marks a card for agent processing and clears any previous run.
func (coordinator *Coordinator) QueueCard(ctx context.Context, id string) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		card, err := coordinator.Repository.GetCard(ctx, id)
		if err != nil {
			return err
		}
		card.AgentStatus = domain.AgentQueued
		card.AgentName = ""
		card.AgentResult = ""
		if err := coordinator.Repository.UpdateCard(ctx, &card); err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, err
}

// ClaimCard atomically assigns a queued card to an agent. A card that is not
// queued yields ErrConflict so concurrent runners never share a card.
func (coordinator *Coordinator) ClaimCard(ctx context.Context, id, agentName string) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		card, err := coordinator.Repository.GetCard(ctx, id)
		if err != nil {
			return err
		}
		if card.AgentStatus != domain.AgentQueued {
			return fmt.Errorf("%w: card %q is not queued (status %q)", domain.ErrConflict, id, card.AgentStatus)
		}
		card.AgentStatus = domain.AgentRunning
		card.AgentName = agentName
		if err := coordinator.Repository.UpdateCard(ctx, &card); err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, err
}

// CompleteCard records an agent run's outcome on a running card.
func (coordinator *Coordinator) CompleteCard(ctx context.Context, id string, success bool, agentResult string) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		card, err := coordinator.Repository.GetCard(ctx, id)
		if err != nil {
			return err
		}
		if card.AgentStatus != domain.AgentRunning {
			return fmt.Errorf("%w: card %q is not running (status %q)", domain.ErrConflict, id, card.AgentStatus)
		}
		card.AgentStatus = domain.AgentFailed
		if success {
			card.AgentStatus = domain.AgentDone
		}
		card.AgentResult = agentResult
		if err := coordinator.Repository.UpdateCard(ctx, &card); err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, err
}
