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

type backupFlags struct {
	storage           string
	s3Bucket          string
	s3Prefix          string
	s3Region          string
	s3Endpoint        string
	s3AccessKeyID     string
	s3SecretAccessKey string
	s3ForcePathStyle  bool
}

func newBackupCommand(opts *options) *cobra.Command {
	var flags backupFlags
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
			backupConfig := backupConfigWithOverrides(res.config.Backup, flags)
			if err = ensureBackupConfig(backupConfig); err != nil {
				return err
			}
			result, err := createConfiguredBackup(cmd.Context(), res.repo, backupConfig, workingDirectory, name, time.Now(), nil)
			if err != nil {
				return err
			}
			res.logger.Info("database backup created", "path", result.localPath, "s3", result.s3URI)
			if removed, rotateErr := rotateBackups(storage.BackupDirectory(workingDirectory), backupConfig, time.Now()); rotateErr != nil {
				res.logger.Error("backup rotation failed", "error", rotateErr)
			} else if removed > 0 {
				res.logger.Info("expired backups removed", "count", removed)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup created: %s\n", result.localRelative)
			if result.s3URI != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "backup uploaded: %s\n", result.s3URI)
			}
			return nil
		},
	}
	command.Flags().StringVar(&flags.storage, "storage", "", "backup storage override: local or s3")
	command.Flags().StringVar(&flags.s3Bucket, "s3-bucket", "", "S3 bucket for backup upload")
	command.Flags().StringVar(&flags.s3Prefix, "s3-prefix", "", "S3 key prefix for backup upload")
	command.Flags().StringVar(&flags.s3Region, "s3-region", "", "S3 region for backup upload")
	command.Flags().StringVar(&flags.s3Endpoint, "s3-endpoint", "", "S3-compatible endpoint URL")
	command.Flags().StringVar(&flags.s3AccessKeyID, "s3-access-key-id", "", "S3 access key ID")
	command.Flags().StringVar(&flags.s3SecretAccessKey, "s3-secret-access-key", "", "S3 secret access key")
	command.Flags().BoolVar(&flags.s3ForcePathStyle, "s3-force-path-style", false, "use path-style S3 URLs")
	return command
}
