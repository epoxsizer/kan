package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/spf13/cobra"
)

func newImportCommand(opts *options) *cobra.Command {
	var replace, yes bool
	command := &cobra.Command{
		Use:   "import <file.json>",
		Short: "Import a complete kan JSON export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if replace && !yes {
				return fmt.Errorf("--replace is destructive and requires --yes")
			}
			var reader io.Reader
			var file *os.File
			if args[0] == "-" {
				reader = cmd.InOrStdin()
			} else {
				var err error
				file, err = os.Open(args[0])
				if err != nil {
					return fmt.Errorf("open import: %w", err)
				}
				defer file.Close()
				reader = file
			}
			var document domain.ExportDocument
			decoder := json.NewDecoder(reader)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&document); err != nil {
				return fmt.Errorf("decode import: %w", err)
			}
			if err := ensureJSONEOF(decoder); err != nil {
				return err
			}

			res, err := open(cmd.Context(), *opts)
			if err != nil {
				return err
			}
			defer res.Close()
			if replace {
				workingDirectory, cwdErr := os.Getwd()
				if cwdErr != nil {
					return fmt.Errorf("get working directory: %w", cwdErr)
				}
				result, backupErr := createLocalBackup(cmd.Context(), res.repo, workingDirectory, "kan-pre-import", time.Now())
				if backupErr != nil {
					return fmt.Errorf("backup before import: %w", backupErr)
				}
				res.logger.Info("pre-import backup created", "path", result.localPath)
			}
			if err = res.repo.ImportDocument(cmd.Context(), document, replace); err != nil {
				return fmt.Errorf("import data: %w", err)
			}
			projects, boards, cards := exportCounts(document)
			res.logger.Info("JSON import complete", "projects", projects, "boards", boards, "cards", cards)
			fmt.Fprintf(cmd.OutOrStdout(), "import complete: %d projects, %d boards, %d cards\n", projects, boards, cards)
			return nil
		},
	}
	command.Flags().BoolVar(&replace, "replace", false, "replace all existing data")
	command.Flags().BoolVar(&yes, "yes", false, "confirm destructive replacement")
	return command
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode import: multiple JSON documents are not allowed")
		}
		return fmt.Errorf("decode import: %w", err)
	}
	return nil
}

func exportCounts(document domain.ExportDocument) (projects, boards, cards int) {
	projects = len(document.Projects)
	for _, project := range document.Projects {
		boards += len(project.Boards)
		for _, board := range project.Boards {
			for _, column := range board.Columns {
				cards += len(column.Cards)
			}
		}
	}
	return projects, boards, cards
}
