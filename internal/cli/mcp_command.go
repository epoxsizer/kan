package cli

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/logging"
	"github.com/epoxsizer/kan/internal/mcpserver"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand(version string, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve the Kan MCP tools over stdio for a local model client",
		Long: `Serve the Kan MCP tools over standard input and output.

Model clients such as Claude Code and Codex start this command themselves.
The database is opened only for the duration of each tool call, so the TUI can
be started and closed at any time. While the TUI is running and its [mcp]
server is enabled in config.toml, calls are sent through the TUI so it
refreshes immediately.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(config.Overrides{ConfigFile: opts.config, Database: opts.db, LogFile: opts.log})
			if err != nil {
				return err
			}
			logger, closer, err := logging.Open(cfg.LogFile, cfg.LogLevel)
			if err != nil {
				return err
			}
			defer closer.Close()
			forward, closeForward := mcpserver.NewForwarder(cfg.MCP, version, storage.DatabaseIdentity(cfg.Database))
			defer closeForward()
			open := func(ctx context.Context, wait time.Duration) (*tasks.Coordinator, func() error, error) {
				lock, lockErr := storage.AcquireLockTimeout(cfg.Database, wait)
				if lockErr != nil {
					return nil, nil, lockErr
				}
				repo, openErr := storage.Open(ctx, cfg.Database)
				if openErr != nil {
					lock.Close()
					return nil, nil, openErr
				}
				return tasks.New(repo), func() error { return errors.Join(repo.Close(), lock.Close()) }, nil
			}
			logger.Info("stdio MCP server starting", "database", cfg.Database, "forwarding", forward != nil)
			server := mcpserver.NewStdioServer(version, open, forward, logger)
			input := &eofReader{reader: cmd.InOrStdin()}
			transport := &mcp.IOTransport{Reader: io.NopCloser(input), Writer: nopWriteCloser{cmd.OutOrStdout()}}
			err = server.Run(cmd.Context(), transport)
			if input.closed.Load() {
				// The client closed stdin, which is the normal way to stop. Errors
				// from requests still in flight at that moment are expected.
				err = nil
			}
			logger.Info("stdio MCP server stopped", "error", err)
			return err
		},
	}
}

type eofReader struct {
	reader io.Reader
	closed atomic.Bool
}

func (input *eofReader) Read(buffer []byte) (int, error) {
	count, err := input.reader.Read(buffer)
	if errors.Is(err, io.EOF) {
		input.closed.Store(true)
	}
	return count, err
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
