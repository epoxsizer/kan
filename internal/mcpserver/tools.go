package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type result[T any] struct {
	Data  T          `json:"data"`
	Error *toolError `json:"error,omitempty"`
}

type emptyInput struct{}

type projectsData struct {
	Projects []domain.Project `json:"projects"`
}

type projectInput struct {
	ProjectID string `json:"project_id" jsonschema:"required,ID of the parent project"`
}

type boardsData struct {
	Boards []domain.Board `json:"boards"`
}

type boardInput struct {
	BoardID string `json:"board_id" jsonschema:"required,ID of the board"`
}

type columnSummary struct {
	Column        domain.Column `json:"column"`
	ActiveCards   int           `json:"active_cards"`
	ArchivedCards int           `json:"archived_cards"`
}

type boardData struct {
	Board     domain.Board      `json:"board"`
	Columns   []columnSummary   `json:"columns"`
	FieldDefs []domain.FieldDef `json:"field_defs"`
}

type listCardsInput struct {
	BoardID  string `json:"board_id" jsonschema:"required,ID of the board"`
	ColumnID string `json:"column_id,omitempty" jsonschema:"optional column ID filter"`
	State    string `json:"state,omitempty" jsonschema:"active, archived, or all; defaults to active"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum rows to return; defaults to 100 and is capped at 200"`
	Offset   int    `json:"offset,omitempty" jsonschema:"zero-based result offset"`
}

type cardsData struct {
	Cards      []domain.Card `json:"cards"`
	NextOffset *int          `json:"next_offset,omitempty"`
}

type getCardInput struct {
	CardID          string `json:"card_id" jsonschema:"required,ID of the card"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"include archived cards"`
}

type cardData struct {
	Card domain.Card `json:"card"`
}

type searchCardsInput struct {
	BoardID string `json:"board_id" jsonschema:"required,ID of the board"`
	Query   string `json:"query" jsonschema:"required,FTS5 search query"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum rows to return; defaults to 50 and is capped at 100"`
	Offset  int    `json:"offset,omitempty" jsonschema:"zero-based result offset"`
}

type createCardInput struct {
	BoardID        string                       `json:"board_id" jsonschema:"required,ID of the board"`
	ColumnID       string                       `json:"column_id" jsonschema:"required,ID of the target column"`
	Title          string                       `json:"title" jsonschema:"required,card title"`
	Description    string                       `json:"description,omitempty" jsonschema:"Markdown card description"`
	Priority       *string                      `json:"priority,omitempty" jsonschema:"priority; defaults to Medium"`
	DueDate        *string                      `json:"due_date,omitempty" jsonschema:"due date in YYYY-MM-DD format"`
	Tags           []string                     `json:"tags,omitempty"`
	RelatedCardIDs []string                     `json:"related_card_ids,omitempty"`
	Checklist      []domain.ChecklistItem       `json:"checklist,omitempty"`
	Fields         map[string]domain.FieldValue `json:"fields,omitempty"`
}

type updateCardInput struct {
	CardID         string                        `json:"card_id" jsonschema:"required,ID of the card"`
	Title          *string                       `json:"title,omitempty"`
	Description    *string                       `json:"description,omitempty" jsonschema:"Markdown card description"`
	Priority       *string                       `json:"priority,omitempty"`
	ClearPriority  bool                          `json:"clear_priority,omitempty"`
	DueDate        *string                       `json:"due_date,omitempty" jsonschema:"due date in YYYY-MM-DD format"`
	ClearDueDate   bool                          `json:"clear_due_date,omitempty"`
	Tags           *[]string                     `json:"tags,omitempty"`
	RelatedCardIDs *[]string                     `json:"related_card_ids,omitempty"`
	Checklist      *[]domain.ChecklistItem       `json:"checklist,omitempty"`
	Fields         *map[string]domain.FieldValue `json:"fields,omitempty"`
}

type moveCardInput struct {
	CardID         string `json:"card_id" jsonschema:"required,ID of the card"`
	TargetColumnID string `json:"target_column_id" jsonschema:"required,ID of the destination column"`
	TargetIndex    *int   `json:"target_index,omitempty" jsonschema:"zero-based destination index; omit to append"`
}

type cardIDInput struct {
	CardID string `json:"card_id" jsonschema:"required,ID of the card"`
}

func registerTools(server *mcp.Server, coordinator *tasks.Coordinator, logger *slog.Logger, notify func(Change)) {
	readAnnotations := annotations(true, false, true)
	writeAnnotations := annotations(false, false, false)
	archiveAnnotations := annotations(false, true, false)

	mcp.AddTool(server, &mcp.Tool{Name: "kan_list_projects", Description: "List Kan projects and their IDs.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, result[projectsData], error) {
			values, err := coordinator.ListProjects(ctx)
			if err != nil {
				return failure[projectsData](err)
			}
			return nil, result[projectsData]{Data: projectsData{Projects: values}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_list_boards", Description: "List boards in a Kan project.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, result[boardsData], error) {
			values, err := coordinator.ListBoards(ctx, input.ProjectID)
			if err != nil {
				return failure[boardsData](err)
			}
			return nil, result[boardsData]{Data: boardsData{Boards: values}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_get_board", Description: "Get a board, its columns, field definitions, and active/archived card counts.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input boardInput) (*mcp.CallToolResult, result[boardData], error) {
			board, err := coordinator.GetBoard(ctx, input.BoardID)
			if err != nil {
				return failure[boardData](err)
			}
			columns, err := coordinator.ListColumns(ctx, input.BoardID)
			if err != nil {
				return failure[boardData](err)
			}
			fields, err := coordinator.ListFieldDefs(ctx, input.BoardID)
			if err != nil {
				return failure[boardData](err)
			}
			cards, err := coordinator.ListCardsIncludingDeleted(ctx, input.BoardID)
			if err != nil {
				return failure[boardData](err)
			}
			summaries := make([]columnSummary, len(columns))
			for index, column := range columns {
				summaries[index].Column = column
				for _, card := range cards {
					if card.ColumnID != column.ID {
						continue
					}
					if card.DeletedAt == nil {
						summaries[index].ActiveCards++
					} else {
						summaries[index].ArchivedCards++
					}
				}
			}
			return nil, result[boardData]{Data: boardData{Board: board, Columns: summaries, FieldDefs: fields}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_list_cards", Description: "List active, archived, or all cards in a board with optional column filtering and pagination.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input listCardsInput) (*mcp.CallToolResult, result[cardsData], error) {
			state := strings.ToLower(strings.TrimSpace(input.State))
			if state == "" {
				state = "active"
			}
			if state != "active" && state != "archived" && state != "all" {
				return failure[cardsData](fmt.Errorf("%w: state must be active, archived, or all", domain.ErrValidation))
			}
			values, err := coordinator.ListCardsIncludingDeleted(ctx, input.BoardID)
			if err != nil {
				return failure[cardsData](err)
			}
			filtered := make([]domain.Card, 0, len(values))
			for _, card := range values {
				if input.ColumnID != "" && card.ColumnID != input.ColumnID {
					continue
				}
				if state == "active" && card.DeletedAt != nil || state == "archived" && card.DeletedAt == nil {
					continue
				}
				filtered = append(filtered, card)
			}
			page, next := paginate(filtered, input.Offset, boundedLimit(input.Limit, 100, 200))
			return nil, result[cardsData]{Data: cardsData{Cards: page, NextOffset: next}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_get_card", Description: "Get one Kan card by ID.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input getCardInput) (*mcp.CallToolResult, result[cardData], error) {
			var card domain.Card
			var err error
			if input.IncludeArchived {
				card, err = coordinator.GetCardIncludingArchived(ctx, input.CardID)
			} else {
				card, err = coordinator.GetCard(ctx, input.CardID)
			}
			if err != nil {
				return failure[cardData](err)
			}
			return nil, result[cardData]{Data: cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_search_cards", Description: "Search active cards in a board using Kan full-text search.", Annotations: readAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input searchCardsInput) (*mcp.CallToolResult, result[cardsData], error) {
			values, err := coordinator.SearchCards(ctx, input.BoardID, input.Query)
			if err != nil {
				return failure[cardsData](err)
			}
			page, next := paginate(values, input.Offset, boundedLimit(input.Limit, 50, 100))
			return nil, result[cardsData]{Data: cardsData{Cards: page, NextOffset: next}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_create_card", Description: "Create a card at the end of a column while enforcing board and WIP constraints.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createCardInput) (*mcp.CallToolResult, result[cardData], error) {
			priority := "Medium"
			if input.Priority != nil {
				priority = *input.Priority
			}
			due, err := parseDate(input.DueDate)
			if err != nil {
				return failure[cardData](err)
			}
			card := domain.Card{
				BoardID: input.BoardID, ColumnID: input.ColumnID, Title: input.Title, Description: input.Description,
				Priority: &priority, DueDate: due, Tags: nonNilSlice(input.Tags), RelatedCardIDs: nonNilSlice(input.RelatedCardIDs),
				Checklist: nonNilSlice(input.Checklist), Fields: nonNilMap(input.Fields),
			}
			if err = coordinator.CreateCardAtEnd(ctx, &card); err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card created", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_update_card", Description: "Patch a card. Omitted fields stay unchanged; use clear flags for priority or due date.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input updateCardInput) (*mcp.CallToolResult, result[cardData], error) {
			if input.Priority != nil && input.ClearPriority {
				return failure[cardData](fmt.Errorf("%w: priority and clear_priority are mutually exclusive", domain.ErrValidation))
			}
			if input.DueDate != nil && input.ClearDueDate {
				return failure[cardData](fmt.Errorf("%w: due_date and clear_due_date are mutually exclusive", domain.ErrValidation))
			}
			due, err := parseDate(input.DueDate)
			if err != nil {
				return failure[cardData](err)
			}
			card, err := coordinator.PatchCard(ctx, input.CardID, tasks.CardPatch{
				Title: input.Title, Description: input.Description, Priority: input.Priority, ClearPriority: input.ClearPriority,
				DueDate: due, ClearDueDate: input.ClearDueDate, Tags: input.Tags, RelatedCardIDs: input.RelatedCardIDs,
				Checklist: input.Checklist, Fields: input.Fields,
			})
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card updated", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_move_card", Description: "Move or reorder a card within its board. Omit target_index to append.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input moveCardInput) (*mcp.CallToolResult, result[cardData], error) {
			if input.TargetIndex != nil && *input.TargetIndex < 0 {
				return failure[cardData](fmt.Errorf("%w: target_index must not be negative", domain.ErrValidation))
			}
			card, err := coordinator.MoveCardTo(ctx, input.CardID, input.TargetColumnID, input.TargetIndex)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card moved", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_archive_card", Description: "Archive a card. This removes it from active views but can be reversed with kan_restore_card.", Annotations: archiveAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input cardIDInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := coordinator.ArchiveCard(ctx, input.CardID)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card archived", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: cardData{Card: card}}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "kan_restore_card", Description: "Restore an archived card to active views.", Annotations: writeAnnotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input cardIDInput) (*mcp.CallToolResult, result[cardData], error) {
			card, err := coordinator.RestoreArchivedCard(ctx, input.CardID)
			if err != nil {
				return failure[cardData](err)
			}
			changed(logger, notify, Change{Action: "card restored", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
			return nil, result[cardData]{Data: cardData{Card: card}}, nil
		})
}

func annotations(readOnly, destructive, idempotent bool) *mcp.ToolAnnotations {
	openWorld := false
	return &mcp.ToolAnnotations{
		ReadOnlyHint: readOnly, DestructiveHint: &destructive, IdempotentHint: idempotent, OpenWorldHint: &openWorld,
	}
}

func failure[T any](err error) (*mcp.CallToolResult, result[T], error) {
	return &mcp.CallToolResult{IsError: true}, result[T]{Error: classify(err)}, nil
}

func classify(err error) *toolError {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return &toolError{Code: "not_found", Message: err.Error()}
	case errors.Is(err, domain.ErrValidation):
		return &toolError{Code: "validation", Message: err.Error()}
	case errors.Is(err, domain.ErrConflict):
		return &toolError{Code: "conflict", Message: err.Error()}
	case errors.Is(err, domain.ErrLocked):
		return &toolError{Code: "locked", Message: err.Error(), Retryable: true}
	default:
		return &toolError{Code: "internal", Message: "internal Kan error"}
	}
}

func changed(logger *slog.Logger, notify func(Change), change Change) {
	logger.Info("MCP mutation complete", "action", change.Action, "board_id", change.BoardID, "column_id", change.ColumnID, "card_id", change.CardID)
	if notify != nil {
		notify(change)
	}
}

func parseDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, fmt.Errorf("%w: due_date must use YYYY-MM-DD", domain.ErrValidation)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func boundedLimit(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func paginate[T any](values []T, offset, limit int) ([]T, *int) {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(values) {
		return []T{}, nil
	}
	end := min(offset+limit, len(values))
	page := values[offset:end]
	if end == len(values) {
		return page, nil
	}
	return page, &end
}

func nonNilSlice[T any](value []T) []T {
	if value == nil {
		return []T{}
	}
	return value
}

func nonNilMap[K comparable, V any](value map[K]V) map[K]V {
	if value == nil {
		return map[K]V{}
	}
	return value
}
