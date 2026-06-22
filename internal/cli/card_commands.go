package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

func newCardCommand(opts *options) *cobra.Command {
	command := &cobra.Command{Use: "card", Short: "Manage cards for automation"}
	command.AddCommand(
		newCardListCommand(opts),
		newCardGetCommand(opts),
		newCardSearchCommand(opts),
		newCardCreateCommand(opts),
		newCardUpdateCommand(opts),
		newCardDeleteCommand(opts),
	)
	return command
}

func newCardListCommand(opts *options) *cobra.Command {
	var boardID, columnID string
	command := &cobra.Command{Use: "list", Short: "List cards as JSON", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			values, err := repo.ListCards(ctx, boardID)
			if err != nil {
				return err
			}
			if columnID != "" {
				filtered := make([]domain.Card, 0, len(values))
				for _, value := range values {
					if value.ColumnID == columnID {
						filtered = append(filtered, value)
					}
				}
				values = filtered
			}
			return writeJSON(cmd, values)
		})
	}}
	command.Flags().StringVar(&boardID, "board", "", "parent board ID")
	command.Flags().StringVar(&columnID, "column", "", "optional column ID filter")
	_ = command.MarkFlagRequired("board")
	return command
}

func newCardGetCommand(opts *options) *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Get one card as JSON", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			value, err := repo.GetCard(ctx, args[0])
			if err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
}

func newCardSearchCommand(opts *options) *cobra.Command {
	var boardID, query string
	command := &cobra.Command{Use: "search", Short: "Search cards with FTS5 and return JSON", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			values, err := repo.SearchCards(ctx, boardID, query)
			if err != nil {
				return err
			}
			return writeJSON(cmd, values)
		})
	}}
	command.Flags().StringVar(&boardID, "board", "", "board ID")
	command.Flags().StringVar(&query, "query", "", "FTS5 search query")
	_ = command.MarkFlagRequired("board")
	_ = command.MarkFlagRequired("query")
	return command
}

type cardFlags struct {
	board, column, title, comment                 string
	priority, due, tags, fields, links, checklist string
	position                                      float64
}

func addCardFlags(command *cobra.Command, flags *cardFlags, create bool) {
	priorityDefault, dueDefault := "", ""
	if create {
		priorityDefault = "Medium"
		dueDefault = time.Now().Format("2006-01-02")
	}
	command.Flags().StringVar(&flags.board, "board", "", "board ID")
	command.Flags().StringVar(&flags.column, "column", "", "column/status ID")
	command.Flags().StringVar(&flags.title, "title", "", "card title")
	command.Flags().StringVar(&flags.comment, "comment", "", "card description/comment")
	command.Flags().StringVar(&flags.priority, "priority", priorityDefault, "card priority; empty clears on update")
	command.Flags().StringVar(&flags.due, "due", dueDefault, "due date in YYYY-MM-DD; empty clears on update")
	command.Flags().StringVar(&flags.tags, "tags", "", "comma-separated tags; empty clears on update")
	command.Flags().StringVar(&flags.fields, "fields", "", `fields JSON object, e.g. {"key":{"type":"text","value":"value"}}`)
	command.Flags().StringVar(&flags.links, "links", "", "comma-separated related card IDs from the same project")
	command.Flags().StringVar(&flags.checklist, "checklist", "", `checklist JSON array, e.g. [{"id":"step-1","text":"Verify","done":false,"position":1024}]`)
	command.Flags().Float64Var(&flags.position, "position", 0, "explicit ordering position")
	if create {
		_ = command.MarkFlagRequired("board")
		_ = command.MarkFlagRequired("column")
		_ = command.MarkFlagRequired("title")
	}
}

func newCardCreateCommand(opts *options) *cobra.Command {
	var flags cardFlags
	command := &cobra.Command{Use: "create", Short: "Create a card", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			if !cmd.Flags().Changed("position") {
				values, err := repo.ListCards(ctx, flags.board)
				if err != nil {
					return err
				}
				flags.position = nextCardPosition(values, flags.column)
			}
			fields, err := parseFields(flags.fields)
			if err != nil {
				return err
			}
			checklist, err := parseChecklist(flags.checklist)
			if err != nil {
				return err
			}
			due, err := parseDueDate(flags.due)
			if err != nil {
				return err
			}
			value := domain.Card{BoardID: flags.board, ColumnID: flags.column, Title: flags.title, Description: flags.comment, Position: flags.position, DueDate: due, Tags: parseTags(flags.tags), RelatedCardIDs: parseTags(flags.links), Checklist: checklist, Fields: fields}
			if strings.TrimSpace(flags.priority) != "" {
				value.Priority = &flags.priority
			}
			if err = repo.CreateCard(ctx, &value); err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
	addCardFlags(command, &flags, true)
	return command
}

func newCardUpdateCommand(opts *options) *cobra.Command {
	var flags cardFlags
	command := &cobra.Command{Use: "update <id>", Short: "Update or move a card", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			value, err := repo.GetCard(ctx, args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("board") {
				value.BoardID = flags.board
			}
			if cmd.Flags().Changed("column") {
				value.ColumnID = flags.column
				if !cmd.Flags().Changed("position") {
					values, listErr := repo.ListCards(ctx, value.BoardID)
					if listErr != nil {
						return listErr
					}
					value.Position = nextCardPosition(values, flags.column)
				}
			}
			if cmd.Flags().Changed("title") {
				value.Title = flags.title
			}
			if cmd.Flags().Changed("comment") {
				value.Description = flags.comment
			}
			if cmd.Flags().Changed("priority") {
				if flags.priority == "" {
					value.Priority = nil
				} else {
					value.Priority = &flags.priority
				}
			}
			if cmd.Flags().Changed("due") {
				value.DueDate, err = parseDueDate(flags.due)
				if err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("tags") {
				value.Tags = parseTags(flags.tags)
			}
			if cmd.Flags().Changed("fields") {
				value.Fields, err = parseFields(flags.fields)
				if err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("links") {
				value.RelatedCardIDs = parseTags(flags.links)
			}
			if cmd.Flags().Changed("checklist") {
				value.Checklist, err = parseChecklist(flags.checklist)
				if err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("position") {
				value.Position = flags.position
			}
			if err = repo.UpdateCard(ctx, &value); err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
	addCardFlags(command, &flags, false)
	return command
}

func newCardDeleteCommand(opts *options) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete <id>", Short: "Soft-delete a card", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireDeleteConfirmation(yes); err != nil {
			return err
		}
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			if err := repo.DeleteCard(ctx, args[0]); err != nil {
				return err
			}
			return writeJSON(cmd, deletedResult(args[0]))
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return command
}

func nextCardPosition(values []domain.Card, columnID string) float64 {
	maximum := 0.0
	for _, value := range values {
		if value.ColumnID == columnID && value.Position > maximum {
			maximum = value.Position
		}
	}
	return maximum + positionSpacing
}
