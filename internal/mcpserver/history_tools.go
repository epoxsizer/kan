package mcpserver

import (
	"context"
	"log/slog"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type cardHistoryInput struct {
	CardID string `json:"card_id" jsonschema:"required,ID of the card"`
}

type movementsData struct {
	Movements []domain.CardMovement `json:"movements"`
}

func registerHistoryTools(server *mcp.Server, use coordinatorFunc, logger *slog.Logger, notify func(Change)) {
	readAnnotations := annotations(true, false, true)

	mcp.AddTool(server, &mcp.Tool{Name: "kan_get_card_history", Description: "List a card's column movements in order, from creation to the latest move.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input cardHistoryInput) (*mcp.CallToolResult, result[movementsData], error) {
			movements, err := use(ctx).ListCardMovements(ctx, input.CardID)
			if err != nil {
				return failure[movementsData](err)
			}
			return nil, result[movementsData]{Data: &movementsData{Movements: nonNilSlice(movements)}}, nil
		})
}
