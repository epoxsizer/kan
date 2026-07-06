package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var backupNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func newBackupCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
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

			res, err := open(cmd.Context(), *opts)
			if err != nil {
				return err
			}
			defer res.Close()
			result, err := createLocalBackup(cmd.Context(), res.store, workingDirectory, name, time.Now())
			if err != nil {
				return err
			}
			res.logger.Info("database backup created", "path", result.localPath)
			if removed, rotateErr := rotateBackups(storage.BackupDirectory(workingDirectory), time.Now()); rotateErr != nil {
				res.logger.Error("backup rotation failed", "error", rotateErr)
			} else if removed > 0 {
				res.logger.Info("expired backups removed", "count", removed)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup created: %s\n", result.localRelative)
			return nil
		},
	}
	return command
}
