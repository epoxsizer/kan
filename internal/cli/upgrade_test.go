package cli

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	appupgrade "github.com/epoxsizer/kan/internal/upgrade"
	"github.com/stretchr/testify/require"
)

type fakeUpgradeService struct {
	result       appupgrade.Result
	checkErr     error
	upgradeErr   error
	checkCalls   int
	upgradeCalls int
}

func (service *fakeUpgradeService) Check(context.Context, string) (appupgrade.Result, error) {
	service.checkCalls++
	return service.result, service.checkErr
}

func (service *fakeUpgradeService) Upgrade(context.Context, string) (appupgrade.Result, error) {
	service.upgradeCalls++
	return service.result, service.upgradeErr
}

func executeUpgradeCommand(t *testing.T, service versionUpgradeService, arguments ...string) (string, error) {
	t.Helper()
	command := newUpgradeCommand("1.0.0", service, nil)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs(arguments)
	return output.String(), command.Execute()
}

func TestUpgradeCommandChecksAndInstalls(t *testing.T) {
	service := &fakeUpgradeService{result: appupgrade.Result{Current: "1.0.0", Latest: "1.1.0", Available: true}}
	command := newUpgradeCommand("1.0.0", service, nil)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"--check"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "update available: 1.0.0 -> 1.1.0")
	require.Equal(t, 1, service.checkCalls)
	require.Zero(t, service.upgradeCalls)

	output.Reset()
	command = newUpgradeCommand("1.0.0", service, nil)
	command.SetOut(&output)
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "kan upgraded: 1.0.0 -> 1.1.0")
	require.Equal(t, 1, service.upgradeCalls)
}

func TestUpgradeCommandReportsUpToDateAndPermissionGuidance(t *testing.T) {
	service := &fakeUpgradeService{result: appupgrade.Result{Current: "1.0.0", Latest: "1.0.0"}}
	command := newUpgradeCommand("1.0.0", service, nil)
	var output bytes.Buffer
	command.SetOut(&output)
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "kan is up to date (1.0.0)")

	service.upgradeErr = fs.ErrPermission
	command = newUpgradeCommand("1.0.0", service, nil)
	err := command.Execute()
	require.ErrorContains(t, err, "install the release manually")
}

func TestUpgradeCommandDoesNotCreateApplicationState(t *testing.T) {
	directory := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	service := &fakeUpgradeService{result: appupgrade.Result{Current: "1.0.0", Latest: "1.0.0"}}
	command := newUpgradeCommand("1.0.0", service, nil)
	command.SetArgs([]string{"--check"})
	require.NoError(t, command.Execute())
	require.NoFileExists(t, filepath.Join(directory, "config.toml"))
	require.NoFileExists(t, filepath.Join(directory, "kan.db"))
	require.NoFileExists(t, filepath.Join(directory, "kan.log"))
}

func TestBackgroundVersionCheckUsesFreshCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "version-check.json")
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	result := appupgrade.Result{Current: "1.0.0", Latest: "1.1.0", Available: true}
	require.NoError(t, appupgrade.WriteCache(cachePath, result, now))
	service := &fakeUpgradeService{checkErr: errors.New("must not be called")}
	notices := make(chan string, 1)

	done := startVersionCheckWithOptions(context.Background(), "1.0.0", service, slog.Default(), func(message string) {
		notices <- message
	}, versionCheckOptions{cachePath: cachePath, now: now.Add(time.Hour)})
	<-done

	require.Zero(t, service.checkCalls)
	require.Equal(t, "kan v1.1.0 available — run: kan upgrade", <-notices)
}

func TestBackgroundVersionCheckRefreshesStaleCacheAndIgnoresFailure(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "version-check.json")
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	service := &fakeUpgradeService{result: appupgrade.Result{Current: "1.0.0", Latest: "1.1.0", Available: true}}
	notices := make(chan string, 1)

	done := startVersionCheckWithOptions(context.Background(), "1.0.0", service, nil, func(message string) {
		notices <- message
	}, versionCheckOptions{cachePath: cachePath, now: now})
	<-done
	require.Equal(t, 1, service.checkCalls)
	require.NotEmpty(t, <-notices)

	service = &fakeUpgradeService{checkErr: errors.New("offline")}
	done = startVersionCheckWithOptions(context.Background(), "1.0.1", service, nil, func(message string) {
		t.Fatalf("unexpected notice: %s", message)
	}, versionCheckOptions{cachePath: cachePath, now: now.Add(time.Hour)})
	<-done
	require.Equal(t, 1, service.checkCalls)
}
