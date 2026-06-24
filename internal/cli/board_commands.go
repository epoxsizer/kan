package cli

import (
	"context"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/spf13/cobra"
)

func newBoardCommand(opts *options) *cobra.Command {
	command := &cobra.Command{Use: "board", Short: "Manage boards for automation"}
	command.AddCommand(newBoardListCommand(opts), newBoardGetCommand(opts), newBoardCreateCommand(opts), newBoardUpdateCommand(opts), newBoardDeleteCommand(opts))
	return command
}

func newBoardListCommand(opts *options) *cobra.Command {
	var projectID string
	command := &cobra.Command{Use: "list", Short: "List boards as JSON", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			values, err := repo.ListBoards(ctx, projectID)
			if err != nil {
				return err
			}
			return writeJSON(cmd, values)
		})
	}}
	command.Flags().StringVar(&projectID, "project", "", "parent project ID")
	_ = command.MarkFlagRequired("project")
	return command
}

func newBoardGetCommand(opts *options) *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Get one board as JSON", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			value, err := repo.GetBoard(ctx, args[0])
			if err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
}

func newBoardCreateCommand(opts *options) *cobra.Command {
	var projectID, name, comment string
	var position float64
	command := &cobra.Command{Use: "create", Short: "Create a board", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			if !cmd.Flags().Changed("position") {
				values, err := repo.ListBoards(ctx, projectID)
				if err != nil {
					return err
				}
				position = nextPosition(values, func(value domain.Board) float64 { return value.Position })
			}
			value := domain.Board{ProjectID: projectID, Name: name, Description: comment, Position: position}
			if err := repo.CreateBoard(ctx, &value); err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
	command.Flags().StringVar(&projectID, "project", "", "parent project ID")
	command.Flags().StringVar(&name, "name", "", "board name")
	command.Flags().StringVar(&comment, "comment", "", "board description/comment")
	command.Flags().Float64Var(&position, "position", 0, "explicit ordering position")
	_ = command.MarkFlagRequired("project")
	_ = command.MarkFlagRequired("name")
	return command
}

func newBoardUpdateCommand(opts *options) *cobra.Command {
	var name, comment string
	var position float64
	command := &cobra.Command{Use: "update <id>", Short: "Update a board", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			value, err := repo.GetBoard(ctx, args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("name") {
				value.Name = name
			}
			if cmd.Flags().Changed("comment") {
				value.Description = comment
			}
			if cmd.Flags().Changed("position") {
				value.Position = position
			}
			if err = repo.UpdateBoard(ctx, &value); err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
	command.Flags().StringVar(&name, "name", "", "new board name")
	command.Flags().StringVar(&comment, "comment", "", "new description/comment; empty clears it")
	command.Flags().Float64Var(&position, "position", 0, "new ordering position")
	return command
}

func newBoardDeleteCommand(opts *options) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete <id>", Short: "Delete a board and all nested data", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireDeleteConfirmation(yes); err != nil {
			return err
		}
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			if err := repo.DeleteBoard(ctx, args[0]); err != nil {
				return err
			}
			return writeJSON(cmd, deletedResult(args[0]))
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm destructive deletion")
	return command
}
