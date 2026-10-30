package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const columnColorHint = "Blue, Green, Yellow, Orange, Red, Purple, or Gray"

type checklistItemInput struct {
	ID       string   `json:"id,omitempty" jsonschema:"existing item ID; omit for a new item"`
	Text     string   `json:"text" jsonschema:"required,checklist item text"`
	Done     bool     `json:"done,omitempty"`
	Position *float64 `json:"position,omitempty" jsonschema:"ordering position; omit to keep the given order"`
}

type columnData struct {
	Column domain.Column `json:"column"`
}

type checklistItemData struct {
	Card domain.Card          `json:"card"`
	Item domain.ChecklistItem `json:"item"`
}

type templatesData struct {
	Templates []domain.CardTemplate `json:"templates"`
}

type singleBoardData struct {
	Board domain.Board `json:"board"`
}

type createBoardInput struct {
	ProjectID   string `json:"project_id" jsonschema:"required,ID of the parent project"`
	Name        string `json:"name" jsonschema:"required,board name"`
	Description string `json:"description,omitempty"`
}

type updateBoardInput struct {
	BoardID     string  `json:"board_id" jsonschema:"required,ID of the board"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type createColumnInput struct {
	BoardID          string  `json:"board_id" jsonschema:"required,ID of the board"`
	Name             string  `json:"name" jsonschema:"required,column name"`
	WIPLimit         *int    `json:"wip_limit,omitempty" jsonschema:"positive work-in-progress limit; omit for no limit"`
	Color            *string `json:"color,omitempty" jsonschema:"column color: Blue, Green, Yellow, Orange, Red, Purple, or Gray"`
	AutoArchive      bool    `json:"auto_archive,omitempty" jsonschema:"archive cards automatically after archive_after_days in this column"`
	ArchiveAfterDays int     `json:"archive_after_days,omitempty" jsonschema:"days before automatic archival; defaults to 14"`
}

type updateColumnInput struct {
	ColumnID         string  `json:"column_id" jsonschema:"required,ID of the column"`
	Name             *string `json:"name,omitempty"`
	WIPLimit         *int    `json:"wip_limit,omitempty" jsonschema:"positive work-in-progress limit"`
	ClearWIPLimit    bool    `json:"clear_wip_limit,omitempty"`
	Color            *string `json:"color,omitempty" jsonschema:"column color: Blue, Green, Yellow, Orange, Red, Purple, or Gray"`
	ClearColor       bool    `json:"clear_color,omitempty"`
	AutoArchive      *bool   `json:"auto_archive,omitempty"`
	ArchiveAfterDays *int    `json:"archive_after_days,omitempty"`
}

type moveColumnInput struct {
	ColumnID    string `json:"column_id" jsonschema:"required,ID of the column"`
	TargetIndex int    `json:"target_index" jsonschema:"required,zero-based destination index within the board"`
}

type addChecklistItemInput struct {
	CardID string `json:"card_id" jsonschema:"required,ID of the card"`
	Text   string `json:"text" jsonschema:"required,checklist item text"`
	Done   bool   `json:"done,omitempty"`
}

type updateChecklistItemInput struct {
	CardID string  `json:"card_id" jsonschema:"required,ID of the card"`
	ItemID string  `json:"item_id" jsonschema:"required,ID of the checklist item"`
	Text   *string `json:"text,omitempty"`
	Done   *bool   `json:"done,omitempty"`
}

type createCardFromTemplateInput struct {
	TemplateID string `json:"template_id" jsonschema:"required,ID of the card template"`
	ColumnID   string `json:"column_id" jsonschema:"required,ID of the target column on the template's board"`
}

func registerStructureTools(server *mcp.Server, use coordinatorFunc, logger *slog.Logger, notify func(Change)) {
	readAnnotations := annotations(true, false, true)
	writeAnnotations := annotations(false, false, false)

	mcp.AddTool(server, &mcp.Tool{Name: "kan_create_board", Description: "Create a board at the end of a project. Add columns with kan_create_column.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createBoardInput) (*mcp.CallToolResult, result[singleBoardData], error) {
			board := domain.Board{ProjectID: input.ProjectID, Name: strings.TrimSpace(input.Name), Description: input.Description}
			if err := use(ctx).CreateBoardAtEnd(ctx, &board); err != nil {
				return failure[singleBoardData](err)
			}
			changed(logger, notify, Change{Action: "board created", BoardID: board.ID})
			return nil, result[singleBoardData]{Data: &singleBoardData{Board: board}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_update_board", Description: "Rename a board or change its description. Omitted fields stay unchanged.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input updateBoardInput) (*mcp.CallToolResult, result[singleBoardData], error) {
			board, err := use(ctx).PatchBoard(ctx, input.BoardID, tasks.BoardPatch{Name: input.Name, Description: input.Description})
			if err != nil {
				return failure[singleBoardData](err)
			}
			changed(logger, notify, Change{Action: "board updated", BoardID: board.ID})
			return nil, result[singleBoardData]{Data: &singleBoardData{Board: board}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_create_column", Description: "Create a column at the right end of a board.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createColumnInput) (*mcp.CallToolResult, result[columnData], error) {
			if input.ArchiveAfterDays < 0 {
				return failure[columnData](fmt.Errorf("%w: archive_after_days must be positive", domain.ErrValidation))
			}
			column := domain.Column{
				BoardID: input.BoardID, Name: strings.TrimSpace(input.Name), WIPLimit: input.WIPLimit, Color: input.Color,
				AutoArchive: input.AutoArchive, ArchiveAfterDays: input.ArchiveAfterDays,
			}
			if err := use(ctx).CreateColumnAtEnd(ctx, &column); err != nil {
				return failure[columnData](err)
			}
			changed(logger, notify, Change{Action: "column created", BoardID: column.BoardID, ColumnID: column.ID})
			return nil, result[columnData]{Data: &columnData{Column: column}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_update_column", Description: "Patch a column's name, WIP limit, color (" + columnColorHint + "), or automatic archiving. Use clear flags to remove the WIP limit or color.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input updateColumnInput) (*mcp.CallToolResult, result[columnData], error) {
			if input.WIPLimit != nil && input.ClearWIPLimit {
				return failure[columnData](fmt.Errorf("%w: wip_limit and clear_wip_limit are mutually exclusive", domain.ErrValidation))
			}
			if input.Color != nil && input.ClearColor {
				return failure[columnData](fmt.Errorf("%w: color and clear_color are mutually exclusive", domain.ErrValidation))
			}
			column, err := use(ctx).PatchColumn(ctx, input.ColumnID, tasks.ColumnPatch{
				Name: input.Name, WIPLimit: input.WIPLimit, ClearWIPLimit: input.ClearWIPLimit, Color: input.Color, ClearColor: input.ClearColor,
				AutoArchive: input.AutoArchive, ArchiveAfterDays: input.ArchiveAfterDays,
			})
			if err != nil {
				return failure[columnData](err)
			}
			changed(logger, notify, Change{Action: "column updated", BoardID: column.BoardID, ColumnID: column.ID})
			return nil, result[columnData]{Data: &columnData{Column: column}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_move_column", Description: "Reorder a column within its board.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input moveColumnInput) (*mcp.CallToolResult, result[columnData], error) {
			if input.TargetIndex < 0 {
				return failure[columnData](fmt.Errorf("%w: target_index must not be negative", domain.ErrValidation))
			}
			column, err := use(ctx).MoveColumnTo(ctx, input.ColumnID, input.TargetIndex)
			if err != nil {
				return failure[columnData](err)
			}
			changed(logger, notify, Change{Action: "column moved", BoardID: column.BoardID, ColumnID: column.ID})
			return nil, result[columnData]{Data: &columnData{Column: column}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_add_checklist_item", Description: "Append one checklist item to a card.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input addChecklistItemInput) (*mcp.CallToolResult, result[checklistItemData], error) {
			card, item, err := use(ctx).AddChecklistItem(ctx, input.CardID, input.Text, input.Done)
			if err != nil {
				return failure[checklistItemData](err)
			}
			changed(logger, notify, Change{Action: "checklist item added", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[checklistItemData]{Data: &checklistItemData{Card: card, Item: item}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_update_checklist_item", Description: "Check, uncheck, or rename one checklist item on a card.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input updateChecklistItemInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := use(ctx).PatchChecklistItem(ctx, input.CardID, input.ItemID, tasks.ChecklistItemPatch{Text: input.Text, Done: input.Done})
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "checklist item updated", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: &cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_list_card_templates", Description: "List card templates available on a board.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input boardInput) (*mcp.CallToolResult, result[templatesData], error) {
			if _, err := use(ctx).GetBoard(ctx, input.BoardID); err != nil {
				return failure[templatesData](err)
			}
			values, err := use(ctx).ListCardTemplates(ctx, input.BoardID)
			if err != nil {
				return failure[templatesData](err)
			}
			return nil, result[templatesData]{Data: &templatesData{Templates: nonNilSlice(values)}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_create_card_from_template", Description: "Create a card from a board template at the end of a column. Patch it afterwards with kan_update_card.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createCardFromTemplateInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := use(ctx).CreateCardFromTemplate(ctx, input.TemplateID, input.ColumnID)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card created", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: &cardData{Card: card}}, nil
		})
}

// normalizeChecklist assigns IDs and increasing positions to items a model
// sent without them, keeping explicit values.
func normalizeChecklist(items []checklistItemInput) []domain.ChecklistItem {
	result := make([]domain.ChecklistItem, len(items))
	maximum := 0.0
	for _, item := range items {
		if item.Position != nil {
			maximum = math.Max(maximum, *item.Position)
		}
	}
	for index, item := range items {
		normalized := domain.ChecklistItem{ID: item.ID, Text: strings.TrimSpace(item.Text), Done: item.Done}
		if normalized.ID == "" {
			normalized.ID = uuid.NewString()
		}
		if item.Position != nil {
			normalized.Position = *item.Position
		} else {
			maximum += 1024
			normalized.Position = maximum
		}
		result[index] = normalized
	}
	return result
}
