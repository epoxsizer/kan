package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/notify"
	"github.com/spf13/cobra"
)

// sendNotification is swapped in tests to observe notifications.
var sendNotification = notify.Send

func newAgentCommand(opts *options) *cobra.Command {
	command := &cobra.Command{Use: "agent", Short: "Run queued cards through an external agent command"}
	command.AddCommand(newAgentRunCommand(opts))
	return command
}

// newAgentRunCommand drains a queue column: claim a queued card, run the
// command with the card as its prompt, record the outcome, and optionally
// move the card to a done/failed column.
func newAgentRunCommand(opts *options) *cobra.Command {
	var commandSpec, boardID, columnID, agentName, doneColumn, failedColumn string
	var limit int
	run := &cobra.Command{Use: "run", Short: "Claim queued cards and run a command for each", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		res, err := open(cmd.Context(), *opts)
		if err != nil {
			return err
		}
		defer res.Close()
		if columnID == "" {
			columnID = res.config.Inbox.ColumnID
		}
		if columnID == "" {
			return fmt.Errorf("no queue column: pass --column or configure inbox.column")
		}
		column, err := res.tasks.GetColumn(cmd.Context(), columnID)
		if err != nil {
			return err
		}
		if boardID == "" {
			boardID = column.BoardID
		}
		cards, err := res.tasks.ListCards(cmd.Context(), boardID)
		if err != nil {
			return err
		}
		summaries := []map[string]any{}
		for _, card := range cards {
			if len(summaries) >= limit {
				break
			}
			if card.ColumnID != columnID || card.AgentStatus != domain.AgentQueued {
				continue
			}
			claimed, err := res.tasks.ClaimCard(cmd.Context(), card.ID, agentName)
			if err != nil {
				continue // another runner won the claim race
			}
			output, runErr := runAgentCommand(cmd.Context(), commandSpec, claimed)
			success := runErr == nil
			completed, err := res.tasks.CompleteCard(cmd.Context(), claimed.ID, success, tailOutput(output, 2000))
			if err != nil {
				return err
			}
			destination := ""
			if success {
				destination = doneColumn
			} else {
				destination = failedColumn
			}
			if destination != "" {
				if _, err := res.tasks.MoveCardTo(cmd.Context(), completed.ID, destination, nil); err != nil {
					return err
				}
			}
			if res.config.Notifications.Enabled {
				sendNotification("Kan: agent "+completed.AgentStatus, completed.Title)
			}
			summaries = append(summaries, map[string]any{
				"card_id": completed.ID, "title": completed.Title, "status": completed.AgentStatus, "success": success,
			})
		}
		return writeJSON(cmd, summaries)
	}}
	run.Flags().StringVar(&commandSpec, "command", "", "shell command run per card; the prompt arrives on stdin and in KAN_PROMPT")
	run.Flags().StringVar(&boardID, "board", "", "board ID (defaults to the queue column's board)")
	run.Flags().StringVar(&columnID, "column", "", "queue column ID (defaults to inbox.column)")
	run.Flags().StringVar(&agentName, "agent", "agent", "agent name recorded on claimed cards")
	run.Flags().IntVar(&limit, "limit", 1, "maximum cards to process")
	run.Flags().StringVar(&doneColumn, "done-column", "", "move successful cards to this column")
	run.Flags().StringVar(&failedColumn, "failed-column", "", "move failed cards to this column")
	_ = run.MarkFlagRequired("command")
	return run
}

func runAgentCommand(ctx context.Context, spec string, card domain.Card) (string, error) {
	prompt := agentPrompt(card)
	command := exec.CommandContext(ctx, "sh", "-c", spec)
	command.Stdin = strings.NewReader(prompt)
	command.Env = append(os.Environ(), "KAN_CARD_ID="+card.ID, "KAN_CARD_TITLE="+card.Title, "KAN_PROMPT="+prompt)
	output, err := command.CombinedOutput()
	return string(output), err
}

// agentPrompt renders the card as a Markdown work order for the agent.
func agentPrompt(card domain.Card) string {
	var builder strings.Builder
	builder.WriteString("# " + card.Title + "\n")
	if strings.TrimSpace(card.Description) != "" {
		builder.WriteString("\n" + strings.TrimRight(card.Description, "\n") + "\n")
	}
	if len(card.Checklist) > 0 {
		builder.WriteString("\nChecklist:\n")
		for _, item := range card.Checklist {
			mark := "[ ]"
			if item.Done {
				mark = "[x]"
			}
			builder.WriteString("- " + mark + " " + item.Text + "\n")
		}
	}
	return builder.String()
}

func tailOutput(output string, limit int) string {
	if len(output) <= limit {
		return output
	}
	return output[len(output)-limit:]
}
