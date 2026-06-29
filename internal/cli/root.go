package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/app"
	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/logging"
	"github.com/epoxsizer/kan/internal/seed"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type options struct {
	config string
	db     string
	log    string
}

type resources struct {
	logger *slog.Logger
	closer interface{ Close() error }
	lock   *storage.Lock
	repo   *storage.Repository
	config config.Config
}

func New(version, commit, date string) *cobra.Command {
	var opts options
	buildVersion := fmt.Sprintf("%s (commit %s, built %s, %s)", version, commit, date, runtime.Version())
	root := &cobra.Command{
		Use:           "kan",
		Short:         "Local-first terminal kanban task tracker",
		Version:       buildVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := open(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer res.Close()

			workingDirectory, workingDirectoryErr := os.Getwd()
			var syncer *syncEngine
			var startupNotice string
			if res.config.Sync.Enabled {
				if workingDirectoryErr != nil {
					return fmt.Errorf("get working directory for sync backups: %w", workingDirectoryErr)
				}
				syncer, err = newSyncEngine(res.repo, res.config.Database, res.config.Sync, workingDirectory, res.logger, realS3SyncClient{})
				if err != nil {
					return err
				}
				startupContext, cancelStartup := context.WithTimeout(cmd.Context(), 30*time.Second)
				err = syncer.startup(startupContext)
				cancelStartup()
				if err != nil {
					if isTransientS3Error(err) {
						startupNotice = "S3 sync unavailable; working locally and retrying"
						res.logger.Error("startup JSON sync unavailable", "error", err)
					} else {
						return fmt.Errorf("startup JSON sync: %w", err)
					}
				} else {
					res.logger.Info("startup JSON sync complete")
				}
			}

			var cancelBackups context.CancelFunc
			var backupsDone <-chan struct{}
			if workingDirectoryErr != nil {
				res.logger.Error("automatic backups disabled", "error", workingDirectoryErr)
			} else {
				backupContext, cancel := context.WithCancel(cmd.Context())
				backupConfig := res.config.Backup
				if res.config.Sync.Enabled {
					backupConfig.Storage = "local"
				}
				backupsDone = startAutomaticBackups(backupContext, res.repo, res.logger, storage.BackupDirectory(workingDirectory), backupConfig)
				cancelBackups = cancel
			}
			res.logger.Info("TUI starting")
			program := tea.NewProgram(
				app.NewWithOptions(cmd.Context(), res.repo, res.logger, app.Options{ShowCardTags: res.config.ShowCardTags, Theme: app.Theme{
					Primary: res.config.Theme.Primary, Muted: res.config.Theme.Muted, Text: res.config.Theme.Text, Background: res.config.Theme.Background, SelectedForeground: res.config.Theme.SelectedForeground, SelectedBackground: res.config.Theme.SelectedBackground, Danger: res.config.Theme.Danger, Border: res.config.Theme.Border,
					SelectedColumnForeground: res.config.Theme.SelectedColumnForeground, SelectedColumnBackground: res.config.Theme.SelectedColumnBackground, SelectedColumnBorder: res.config.Theme.SelectedColumnBorder,
					SelectedCardForeground: res.config.Theme.SelectedCardForeground, SelectedCardBackground: res.config.Theme.SelectedCardBackground, PanelBorder: res.config.Theme.PanelBorder, FocusedPanelBorder: res.config.Theme.FocusedPanelBorder,
					StatusForeground: res.config.Theme.StatusForeground, StatusBackground: res.config.Theme.StatusBackground, StatusAccentForeground: res.config.Theme.StatusAccentForeground, StatusAccentBackground: res.config.Theme.StatusAccentBackground,
					ShortcutKeyForeground: res.config.Theme.ShortcutKeyForeground, ShortcutKeyBackground: res.config.Theme.ShortcutKeyBackground, ShortcutText: res.config.Theme.ShortcutText, HelpText: res.config.Theme.HelpText, HelpBorder: res.config.Theme.HelpBorder,
					Command: res.config.Theme.Command, ColumnDefault: res.config.Theme.ColumnDefault,
				}, StartupNotice: startupNotice}),
				tea.WithAltScreen(),
				tea.WithContext(cmd.Context()),
			)

			var cancelSync context.CancelFunc
			var syncDone <-chan struct{}
			if syncer != nil {
				syncContext, cancel := context.WithCancel(cmd.Context())
				cancelSync = cancel
				syncDone = startAutomaticSync(syncContext, syncer, res.logger, func(message string) {
					program.Send(app.NoticeMsg{Text: message})
				})
			}

			_, runErr := program.Run()
			if cancelSync != nil {
				cancelSync()
				<-syncDone
			}
			if cancelBackups != nil {
				cancelBackups()
				<-backupsDone
			}
			if syncer != nil {
				shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
				if syncErr := syncer.push(shutdownContext, false); syncErr != nil {
					res.logger.Error("final JSON sync failed", "error", syncErr)
				} else {
					res.logger.Info("final JSON sync complete")
				}
				cancelShutdown()
			}

			if runErr != nil {
				err = runErr
				res.logger.Error("TUI stopped with error", "error", err)
				return fmt.Errorf("run TUI: %w", err)
			}
			res.logger.Info("TUI stopped")
			return nil
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.PersistentFlags().StringVar(&opts.config, "config", "", "config file path (env: KAN_CONFIG)")
	root.PersistentFlags().StringVar(&opts.db, "db", "", "SQLite database path (env: KAN_DB)")
	root.PersistentFlags().StringVar(&opts.log, "log", "", "log file path (env: KAN_LOG)")

	root.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Apply pending database migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := open(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer res.Close()
			res.logger.Info("database migrations complete")
			fmt.Fprintln(cmd.OutOrStdout(), "migrations applied")
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "seed",
		Short: "Load the deterministic demo dataset",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := open(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer res.Close()
			if err = seed.Demo(cmd.Context(), res.repo); err != nil {
				return fmt.Errorf("seed demo data: %w", err)
			}
			res.logger.Info("demo dataset ready")
			fmt.Fprintln(cmd.OutOrStdout(), "demo data ready")
			return nil
		},
	})
	root.AddCommand(newBackupCommand(&opts))
	root.AddCommand(newExportCommand(&opts))
	root.AddCommand(newImportCommand(&opts))
	root.AddCommand(newSyncCommand(&opts))
	root.AddCommand(
		newProjectCommand(&opts),
		newBoardCommand(&opts),
		newColumnCommand(&opts),
		newCardCommand(&opts),
	)
	return root
}

func open(ctx context.Context, opts options) (*resources, error) {
	cfg, err := config.Load(config.Overrides{ConfigFile: opts.config, Database: opts.db, LogFile: opts.log})
	if err != nil {
		return nil, err
	}
	logger, closer, err := logging.Open(cfg.LogFile, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	lock, err := storage.AcquireLockTimeout(cfg.Database, 5*time.Second)
	if err != nil {
		closer.Close()
		return nil, err
	}
	repo, err := storage.Open(ctx, cfg.Database)
	if err != nil {
		lock.Close()
		closer.Close()
		return nil, err
	}
	logger.Info("database opened", "path", cfg.Database)
	return &resources{logger: logger, closer: closer, lock: lock, repo: repo, config: cfg}, nil
}

func (res *resources) Close() error {
	var first error
	if res.repo != nil {
		first = res.repo.Close()
	}
	if res.lock != nil {
		if err := res.lock.Close(); first == nil {
			first = err
		}
	}
	if res.closer != nil {
		if err := res.closer.Close(); first == nil {
			first = err
		}
	}
	return first
}
