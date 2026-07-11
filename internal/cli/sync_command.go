package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type syncStatusReport struct {
	Enabled  bool             `json:"enabled"`
	Interval string           `json:"interval"`
	Target   syncStatusTarget `json:"target"`
	State    *syncStatusState `json:"state,omitempty"`
	Local    syncStatusData   `json:"local"`
	Remote   syncStatusRemote `json:"remote"`
	Relation string           `json:"relation"`
}

type syncStatusTarget struct {
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	Endpoint  string `json:"endpoint,omitempty"`
}

type syncStatusState struct {
	ETag     string    `json:"etag"`
	DataHash string    `json:"data_hash"`
	SyncedAt time.Time `json:"synced_at"`
}

type syncStatusData struct {
	DataHash string `json:"data_hash"`
	Empty    bool   `json:"empty"`
}

type syncStatusRemote struct {
	Exists   bool   `json:"exists"`
	ETag     string `json:"etag,omitempty"`
	DataHash string `json:"data_hash,omitempty"`
	Error    string `json:"error,omitempty"`
}

type syncResult struct {
	Action string `json:"action"`
	Status string `json:"status"`
}

func newSyncCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:   "sync",
		Short: "Inspect or manually synchronize JSON data with S3",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(newSyncStatusCommand(opts), newSyncPullCommand(opts), newSyncPushCommand(opts))
	return command
}

func newSyncStatusCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local, remote, and last-synced state as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withSyncEngine(cmd, opts, func(engine *syncEngine) error {
				report, err := engine.status(cmd.Context())
				if err != nil {
					return err
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			})
		},
	}
}

func newSyncPullCommand(opts *options) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "pull",
		Short: "Replace local data with the remote JSON object",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return errors.New("sync pull replaces local data and requires --yes")
			}
			return withSyncEngine(cmd, opts, func(engine *syncEngine) error {
				if err := engine.pull(cmd.Context()); err != nil {
					return err
				}
				return writeSyncResult(cmd, "pull")
			})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm replacement of local data")
	return command
}

func newSyncPushCommand(opts *options) *cobra.Command {
	var force, yes bool
	command := &cobra.Command{
		Use:   "push",
		Short: "Upload local JSON data when the remote object is unchanged",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if force && !yes {
				return errors.New("sync push --force overwrites remote data and requires --yes")
			}
			return withSyncEngine(cmd, opts, func(engine *syncEngine) error {
				if err := engine.push(cmd.Context(), force); err != nil {
					return err
				}
				return writeSyncResult(cmd, "push")
			})
		},
	}
	command.Flags().BoolVar(&force, "force", false, "overwrite remote data without an ETag precondition")
	command.Flags().BoolVar(&yes, "yes", false, "confirm a forced remote overwrite")
	return command
}

func writeSyncResult(cmd *cobra.Command, action string) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(syncResult{Action: action, Status: "complete"})
}

func withSyncEngine(cmd *cobra.Command, opts *options, action func(*syncEngine) error) error {
	res, err := open(cmd.Context(), *opts)
	if err != nil {
		return err
	}
	defer res.Close()
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	engine, err := newSyncEngine(res.store, res.tasks, res.config.Database, res.config.Sync, workingDirectory, res.logger, realS3SyncClient{})
	if err != nil {
		return err
	}
	return action(engine)
}

func (engine *syncEngine) status(ctx context.Context) (syncStatusReport, error) {
	report := syncStatusReport{
		Enabled: engine.config.Enabled, Interval: engine.config.Interval,
		Target: syncStatusTarget{Bucket: engine.config.S3.Bucket, ObjectKey: engine.config.ObjectKey, Endpoint: engine.config.S3.Endpoint},
	}
	local, err := engine.snapshot(ctx)
	if err != nil {
		return report, err
	}
	report.Local = syncStatusData{DataHash: local.DataHash, Empty: local.Empty}
	state, err := engine.readState()
	if err != nil {
		return report, err
	}
	if state != nil {
		report.State = &syncStatusState{ETag: state.ETag, DataHash: state.DataHash, SyncedAt: state.SyncedAt}
		if !engine.matchesTarget(*state) {
			report.Relation = "target_mismatch"
		}
	}
	remote, err := engine.client.Get(ctx, engine.config.S3, engine.config.ObjectKey, "")
	if err != nil {
		report.Remote.Error = err.Error()
		report.Relation = "remote_unavailable"
		return report, nil
	}
	if remote.NotFound {
		report.Relation = "remote_missing"
		return report, nil
	}
	report.Remote.Exists = true
	report.Remote.ETag = remote.ETag
	_, remoteHash, err := decodeSyncDocument(ctx, remote.Body)
	if err != nil {
		report.Remote.Error = err.Error()
		report.Relation = "remote_invalid"
		return report, nil
	}
	report.Remote.DataHash = remoteHash
	if report.Relation == "target_mismatch" {
		return report, nil
	}
	switch {
	case local.DataHash == remoteHash:
		report.Relation = "in_sync"
	case state == nil:
		report.Relation = "unrelated"
	case local.DataHash != state.DataHash && remoteHash != state.DataHash:
		report.Relation = "both_changed"
	case local.DataHash != state.DataHash:
		report.Relation = "local_changed"
	case remoteHash != state.DataHash:
		report.Relation = "remote_changed"
	default:
		report.Relation = "etag_changed"
	}
	return report, nil
}
