package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/config"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

func newExportCommand(opts *options) *cobra.Command {
	var outputPath string
	var force bool
	command := &cobra.Command{
		Use:   "export",
		Short: "Export the complete kan hierarchy as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withRepository(cmd, opts, func(ctx context.Context, repo domain.Repository) error {
				document, err := buildExport(ctx, repo, time.Now().UTC())
				if err != nil {
					return err
				}
				contents, err := json.MarshalIndent(document, "", "  ")
				if err != nil {
					return fmt.Errorf("encode export: %w", err)
				}
				contents = append(contents, '\n')
				if outputPath == "" || outputPath == "-" {
					_, err = cmd.OutOrStdout().Write(contents)
					return err
				}
				if err = writeExportFile(outputPath, contents, force); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "export created: %s\n", outputPath)
				return nil
			})
		},
	}
	command.Flags().StringVar(&outputPath, "out", "", "write JSON to this file instead of stdout; use - for stdout")
	command.Flags().BoolVar(&force, "force", false, "overwrite an existing output file")
	return command
}

func buildExport(ctx context.Context, repo domain.Repository, exportedAt time.Time) (domain.ExportDocument, error) {
	document := domain.ExportDocument{Format: "kan", Version: domain.ExportVersion, ExportedAt: exportedAt.UTC(), Projects: []domain.ExportProject{}}
	projects, err := repo.ListProjects(ctx)
	if err != nil {
		return document, err
	}
	for _, project := range projects {
		exportedProject := domain.ExportProject{Project: project, Boards: []domain.ExportBoard{}}
		boards, listErr := repo.ListBoards(ctx, project.ID)
		if listErr != nil {
			return document, listErr
		}
		for _, board := range boards {
			exportedBoard := domain.ExportBoard{Board: board, FieldDefs: []domain.FieldDef{}, Columns: []domain.ExportColumn{}}
			exportedBoard.FieldDefs, listErr = repo.ListFieldDefs(ctx, board.ID)
			if listErr != nil {
				return document, listErr
			}
			columns, columnErr := repo.ListColumns(ctx, board.ID)
			if columnErr != nil {
				return document, columnErr
			}
			cards, cardErr := repo.ListCardsIncludingDeleted(ctx, board.ID)
			if cardErr != nil {
				return document, cardErr
			}
			cardsByColumn := make(map[string][]domain.Card, len(columns))
			for _, card := range cards {
				cardsByColumn[card.ColumnID] = append(cardsByColumn[card.ColumnID], card)
			}
			for _, column := range columns {
				columnCards := cardsByColumn[column.ID]
				if columnCards == nil {
					columnCards = []domain.Card{}
				}
				exportedBoard.Columns = append(exportedBoard.Columns, domain.ExportColumn{Column: column, Cards: columnCards})
			}
			exportedProject.Boards = append(exportedProject.Boards, exportedBoard)
		}
		document.Projects = append(document.Projects, exportedProject)
	}
	return document, nil
}

func writeExportFile(path string, contents []byte, force bool) error {
	if err := config.EnsureParent(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("export file %q already exists; use --force to overwrite", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat export file: %w", err)
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".kan-export-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary export: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(contents)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close export: %w", closeErr)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish export: %w", err)
	}
	return nil
}
