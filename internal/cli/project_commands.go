package cli

import (
	"context"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/spf13/cobra"
)

func newProjectCommand(opts *options) *cobra.Command {
	command := &cobra.Command{Use: "project", Short: "Manage projects for automation"}
	command.AddCommand(
		newProjectListCommand(opts),
		newProjectGetCommand(opts),
		newProjectCreateCommand(opts),
		newProjectUpdateCommand(opts),
		newProjectDeleteCommand(opts),
	)
	return command
}

func newProjectListCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List projects as JSON", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
				values, err := repo.ListProjects(ctx)
				if err != nil {
					return err
				}
				return writeJSON(cmd, values)
			})
		},
	}
}

func newProjectGetCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "get <id>", Short: "Get one project as JSON", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
				value, err := repo.GetProject(ctx, args[0])
				if err != nil {
					return err
				}
				return writeJSON(cmd, value)
			})
		},
	}
}

func newProjectCreateCommand(opts *options) *cobra.Command {
	var name, comment string
	var position float64
	command := &cobra.Command{
		Use: "create", Short: "Create a project", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
				if !cmd.Flags().Changed("position") {
					values, err := repo.ListProjects(ctx)
					if err != nil {
						return err
					}
					position = nextPosition(values, func(value domain.Project) float64 { return value.Position })
				}
				value := domain.Project{Name: name, Description: comment, Position: position}
				if err := repo.CreateProject(ctx, &value); err != nil {
					return err
				}
				return writeJSON(cmd, value)
			})
		},
	}
	command.Flags().StringVar(&name, "name", "", "project name")
	command.Flags().StringVar(&comment, "comment", "", "project description/comment")
	command.Flags().Float64Var(&position, "position", 0, "explicit ordering position")
	_ = command.MarkFlagRequired("name")
	return command
}

func newProjectUpdateCommand(opts *options) *cobra.Command {
	var name, comment string
	var position float64
	command := &cobra.Command{
		Use: "update <id>", Short: "Update a project", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
				value, err := repo.GetProject(ctx, args[0])
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
				if err = repo.UpdateProject(ctx, &value); err != nil {
					return err
				}
				return writeJSON(cmd, value)
			})
		},
	}
	command.Flags().StringVar(&name, "name", "", "new project name")
	command.Flags().StringVar(&comment, "comment", "", "new description/comment; empty clears it")
	command.Flags().Float64Var(&position, "position", 0, "new ordering position")
	return command
}

func newProjectDeleteCommand(opts *options) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use: "delete <id>", Short: "Delete a project and all nested data", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDeleteConfirmation(yes); err != nil {
				return err
			}
			return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
				if err := repo.DeleteProject(ctx, args[0]); err != nil {
					return err
				}
				return writeJSON(cmd, deletedResult(args[0]))
			})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm destructive deletion")
	return command
}
