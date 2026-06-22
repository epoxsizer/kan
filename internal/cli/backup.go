package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	storage "gitlab.digital-spirit.ru/solutions/common/kan/internal/storage/sqlite"
)

var backupNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func newBackupCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "backup [name]",
		Short: "Back up all data into ./backup",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "kan"
			if len(args) == 1 {
				name = strings.TrimSpace(args[0])
			}
			if !backupNamePattern.MatchString(name) {
				return fmt.Errorf("backup name must start with a letter or number and contain only letters, numbers, dots, underscores, or hyphens")
			}
			workingDirectory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			destination := filepath.Join(
				storage.BackupDirectory(workingDirectory),
				fmt.Sprintf("%s-%s.db", name, time.Now().Format("20060102-150405")),
			)

			res, err := open(cmd.Context(), *opts)
			if err != nil {
				return err
			}
			defer res.Close()
			if err = res.repo.Backup(cmd.Context(), destination); err != nil {
				return err
			}
			res.logger.Info("database backup created", "path", destination)
			relative, relativeErr := filepath.Rel(workingDirectory, destination)
			if relativeErr != nil {
				relative = destination
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup created: %s\n", relative)
			return nil
		},
	}
}
