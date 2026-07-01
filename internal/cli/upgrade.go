package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	appupgrade "github.com/epoxsizer/kan/internal/upgrade"
	"github.com/spf13/cobra"
)

const (
	versionCheckTimeout = 15 * time.Second
	upgradeTimeout      = 5 * time.Minute
)

type versionUpgradeService interface {
	Check(context.Context, string) (appupgrade.Result, error)
	Upgrade(context.Context, string) (appupgrade.Result, error)
}

func newUpgradeCommand(currentVersion string, service versionUpgradeService, serviceErr error) *cobra.Command {
	var checkOnly bool
	command := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install the latest stable kan release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if serviceErr != nil {
				return serviceErr
			}
			if service == nil {
				return errors.New("updater is unavailable")
			}
			timeout := upgradeTimeout
			if checkOnly {
				timeout = versionCheckTimeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			var result appupgrade.Result
			var err error
			if checkOnly {
				result, err = service.Check(ctx, currentVersion)
			} else {
				result, err = service.Upgrade(ctx, currentVersion)
			}
			if err != nil {
				return upgradeCommandError(err)
			}
			if !result.Available {
				fmt.Fprintf(cmd.OutOrStdout(), "kan is up to date (%s)\n", result.Current)
				return nil
			}
			if checkOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "update available: %s -> %s\nrun \"kan upgrade\" to install\n", result.Current, result.Latest)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "kan upgraded: %s -> %s\n", result.Current, result.Latest)
			return nil
		},
	}
	command.Flags().BoolVar(&checkOnly, "check", false, "check for a newer stable release without installing it")
	return command
}

func upgradeCommandError(err error) error {
	if errors.Is(err, appupgrade.ErrDevelopmentBuild) {
		return err
	}
	message := strings.ToLower(err.Error())
	if errors.Is(err, fs.ErrPermission) || strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted") || strings.Contains(message, "access is denied") {
		return fmt.Errorf("%w; cannot replace the current executable—install the release manually from https://github.com/epoxsizer/kan/releases or run as the account that owns the file", err)
	}
	return err
}
