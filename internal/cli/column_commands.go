package cli

import (
	"context"

	"github.com/spf13/cobra"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

func newColumnCommand(opts *options) *cobra.Command {
	command := &cobra.Command{Use: "column", Short: "Manage board columns for automation"}
	command.AddCommand(newColumnListCommand(opts), newColumnGetCommand(opts), newColumnCreateCommand(opts), newColumnUpdateCommand(opts), newColumnDeleteCommand(opts))
	return command
}

func newColumnListCommand(opts *options) *cobra.Command {
	var boardID string
	command := &cobra.Command{Use: "list", Short: "List columns as JSON", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			values, err := repo.ListColumns(ctx, boardID)
			if err != nil {
				return err
			}
			return writeJSON(cmd, values)
		})
	}}
	command.Flags().StringVar(&boardID, "board", "", "parent board ID")
	_ = command.MarkFlagRequired("board")
	return command
}

func newColumnGetCommand(opts *options) *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Get one column as JSON", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			value, err := repo.GetColumn(ctx, args[0])
			if err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
}

func newColumnCreateCommand(opts *options) *cobra.Command {
	var boardID, name string
	color := "Blue"
	var position float64
	wip := 10
	command := &cobra.Command{Use: "create", Short: "Create a board column", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			if !cmd.Flags().Changed("position") {
				values, err := repo.ListColumns(ctx, boardID)
				if err != nil {
					return err
				}
				position = nextPosition(values, func(value domain.Column) float64 { return value.Position })
			}
			value := domain.Column{BoardID: boardID, Name: name, Position: position, WIPLimit: &wip, Color: &color}
			if err := repo.CreateColumn(ctx, &value); err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
	command.Flags().StringVar(&boardID, "board", "", "parent board ID")
	command.Flags().StringVar(&name, "name", "", "column name")
	command.Flags().StringVar(&color, "color", color, "column color")
	command.Flags().IntVar(&wip, "wip-limit", wip, "positive WIP limit")
	command.Flags().Float64Var(&position, "position", 0, "explicit ordering position")
	_ = command.MarkFlagRequired("board")
	_ = command.MarkFlagRequired("name")
	return command
}

func newColumnUpdateCommand(opts *options) *cobra.Command {
	var name, color string
	var position float64
	var wip int
	var clearWIP, clearColor bool
	command := &cobra.Command{Use: "update <id>", Short: "Update a board column", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			value, err := repo.GetColumn(ctx, args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("name") {
				value.Name = name
			}
			if cmd.Flags().Changed("position") {
				value.Position = position
			}
			if cmd.Flags().Changed("wip-limit") {
				value.WIPLimit = &wip
			}
			if clearWIP {
				value.WIPLimit = nil
			}
			if cmd.Flags().Changed("color") {
				value.Color = &color
			}
			if clearColor {
				value.Color = nil
			}
			if err = repo.UpdateColumn(ctx, &value); err != nil {
				return err
			}
			return writeJSON(cmd, value)
		})
	}}
	command.Flags().StringVar(&name, "name", "", "new column name")
	command.Flags().StringVar(&color, "color", "", "new color")
	command.Flags().IntVar(&wip, "wip-limit", 0, "new positive WIP limit")
	command.Flags().BoolVar(&clearWIP, "clear-wip-limit", false, "clear the WIP limit")
	command.Flags().BoolVar(&clearColor, "clear-color", false, "clear the color")
	command.Flags().Float64Var(&position, "position", 0, "new ordering position")
	return command
}

func newColumnDeleteCommand(opts *options) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete <id>", Short: "Delete a column and its cards", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireDeleteConfirmation(yes); err != nil {
			return err
		}
		return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
			if err := repo.DeleteColumn(ctx, args[0]); err != nil {
				return err
			}
			return writeJSON(cmd, deletedResult(args[0]))
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm destructive deletion")
	return command
}
