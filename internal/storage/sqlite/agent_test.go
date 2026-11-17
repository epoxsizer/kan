package sqlite

import (
	"context"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCardAgentFieldsRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	_, board, column := createHierarchy(t, repo)

	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Agent task", Position: 1024, AgentStatus: domain.AgentQueued}
	require.NoError(t, repo.CreateCard(ctx, &card))
	stored, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentQueued, stored.AgentStatus)

	stored.AgentStatus = domain.AgentRunning
	stored.AgentName = "tester"
	stored.AgentResult = "partial output"
	require.NoError(t, repo.UpdateCard(ctx, &stored))
	again, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentRunning, again.AgentStatus)
	require.Equal(t, "tester", again.AgentName)
	require.Equal(t, "partial output", again.AgentResult)
}

func TestCardAgentStatusValidation(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	_, board, column := createHierarchy(t, repo)

	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Bad status", Position: 1024, AgentStatus: "bogus"}
	require.ErrorIs(t, repo.CreateCard(ctx, &card), domain.ErrValidation)
}
