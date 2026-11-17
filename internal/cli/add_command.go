package cli

import (
	"fmt"
	"strings"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/spf13/cobra"
)

// newAddCommand captures a card with minimal friction: the destination comes
// from the configured inbox unless --board/--column override it.
func newAddCommand(opts *options) *cobra.Command {
	var description, boardID, columnID, priority, due, tags string
	command := &cobra.Command{Use: "add <title>", Short: "Capture a card into the inbox (or a chosen column)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		res, err := open(cmd.Context(), *opts)
		if err != nil {
			return err
		}
		defer res.Close()
		if boardID == "" {
			boardID = res.config.Inbox.BoardID
		}
		if columnID == "" {
			columnID = res.config.Inbox.ColumnID
		}
		if boardID == "" || columnID == "" {
			return fmt.Errorf("no capture destination: set inbox.board and inbox.column in the config, or pass --board and --column")
		}
		parsedDue, err := parseDueDate(due)
		if err != nil {
			return err
		}
		card := domain.Card{
			BoardID: boardID, ColumnID: columnID, Title: args[0], Description: description,
			DueDate: parsedDue, Tags: parseTags(tags),
		}
		if strings.TrimSpace(priority) != "" {
			card.Priority = &priority
		}
		if err := res.tasks.CreateCardAtEnd(cmd.Context(), &card); err != nil {
			return err
		}
		return writeJSON(cmd, card)
	}}
	command.Flags().StringVar(&description, "description", "", "card description (Markdown)")
	command.Flags().StringVar(&boardID, "board", "", "destination board ID (defaults to inbox.board)")
	command.Flags().StringVar(&columnID, "column", "", "destination column ID (defaults to inbox.column)")
	command.Flags().StringVar(&priority, "priority", "", "card priority")
	command.Flags().StringVar(&due, "due", "", "due date (YYYY-MM-DD)")
	command.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return command
}
