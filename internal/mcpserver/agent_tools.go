package mcpserver

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type queueCardInput struct {
	CardID string `json:"card_id" jsonschema:"required,ID of the card to queue for agent processing"`
}

type claimCardInput struct {
	CardID    string `json:"card_id" jsonschema:"required,ID of the queued card"`
	AgentName string `json:"agent_name" jsonschema:"required,name of the agent claiming the card"`
}

type completeCardInput struct {
	CardID  string `json:"card_id" jsonschema:"required,ID of the running card"`
	Success bool   `json:"success" jsonschema:"whether the agent run succeeded"`
	Result  string `json:"result,omitempty" jsonschema:"agent output or summary stored on the card"`
}

func registerAgentTools(server *mcp.Server, use coordinatorFunc, logger *slog.Logger, notify func(Change)) {
	writeAnnotations := annotations(false, false, false)
	replacementAnnotations := annotations(false, true, false)

	mcp.AddTool(server, &mcp.Tool{Name: "kan_queue_card", Description: "Queue a card for agent processing. Clears any previous agent result.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input queueCardInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := use(ctx).QueueCard(ctx, input.CardID)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card queued", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: &cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_claim_card", Description: "Atomically claim a queued card for an agent. Fails with conflict unless the card is queued, so concurrent runners never share a card.", Annotations: replacementAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input claimCardInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := use(ctx).ClaimCard(ctx, input.CardID, input.AgentName)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card claimed", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: &cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_complete_card", Description: "Record the outcome of an agent run on a running card: done or failed, with the agent output stored as the result.", Annotations: replacementAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input completeCardInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := use(ctx).CompleteCard(ctx, input.CardID, input.Success, input.Result)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card completed", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: &cardData{Card: card}}, nil
		})
}
