package mcpserver

import (
	"context"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCardHistoryTool(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Backlog", Position: 1024}
	require.NoError(t, coordinator.CreateColumn(ctx, &column))
	other := domain.Column{BoardID: board.ID, Name: "Doing", Position: 2048}
	require.NoError(t, coordinator.CreateColumn(ctx, &other))
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Tracked", Position: 1024}
	require.NoError(t, coordinator.CreateCardAtEnd(ctx, &card))
	_, err := coordinator.MoveCardTo(ctx, card.ID, other.ID, nil)
	require.NoError(t, err)

	session := connectTestSession(t, coordinator, nil)
	var history result[movementsData]
	session.mustCall("kan_get_card_history", map[string]any{"card_id": card.ID}, &history)
	require.Len(t, history.Data.Movements, 2)
	require.Equal(t, other.ID, history.Data.Movements[1].ToColumnID)
}
