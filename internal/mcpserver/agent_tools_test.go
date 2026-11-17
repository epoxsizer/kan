package mcpserver

import (
	"context"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAgentToolsLifecycle(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Queue", Position: 1024}
	require.NoError(t, coordinator.CreateColumn(ctx, &column))
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Agent work", Position: 1024}
	require.NoError(t, coordinator.CreateCardAtEnd(ctx, &card))

	session := connectTestSession(t, coordinator, nil)
	var result result[cardData]
	session.mustCall("kan_queue_card", map[string]any{"card_id": card.ID}, &result)
	require.Equal(t, domain.AgentQueued, result.Data.Card.AgentStatus)

	session.mustCall("kan_claim_card", map[string]any{"card_id": card.ID, "agent_name": "mcp-runner"}, &result)
	require.Equal(t, domain.AgentRunning, result.Data.Card.AgentStatus)
	require.Equal(t, "mcp-runner", result.Data.Card.AgentName)

	conflict := session.call("kan_claim_card", map[string]any{"card_id": card.ID, "agent_name": "other"}, &result)
	require.True(t, conflict.IsError)
	require.Equal(t, "conflict", result.Error.Code)

	session.mustCall("kan_complete_card", map[string]any{"card_id": card.ID, "success": true, "result": "shipped"}, &result)
	require.Equal(t, domain.AgentDone, result.Data.Card.AgentStatus)
	require.Equal(t, "shipped", result.Data.Card.AgentResult)
}
