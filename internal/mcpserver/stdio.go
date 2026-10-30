package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// lockWait is how long a stdio call waits for the database when no running
// TUI accepted the forwarded call, for example while another CLI command or
// stdio call briefly holds the lock.
const lockWait = 5 * time.Second

// OpenFunc opens the database for one tool call and returns a function that
// closes it. It returns domain.ErrLocked when another Kan process holds the
// database for longer than wait.
type OpenFunc func(ctx context.Context, wait time.Duration) (*tasks.Coordinator, func() error, error)

// ForwardFunc sends one tool call to the MCP server of a running TUI.
type ForwardFunc func(ctx context.Context, params *mcp.CallToolParamsRaw) (*mcp.CallToolResult, error)

type coordinatorKey struct{}

// NewStdioServer builds an MCP server for `kan mcp`. It holds the database
// only during a tool call so the TUI can start at any time. While the TUI
// holds the database, calls are forwarded to its MCP server when forward is
// not nil.
func NewStdioServer(version string, open OpenFunc, forward ForwardFunc, logger *slog.Logger) *mcp.Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := mcp.NewServer(
		&mcp.Implementation{Name: "kan", Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions, Logger: logger},
	)
	registerTools(server, func(ctx context.Context) *tasks.Coordinator {
		return ctx.Value(coordinatorKey{}).(*tasks.Coordinator)
	}, logger, nil)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			call, ok := request.(*mcp.CallToolRequest)
			if !ok {
				return next(ctx, method, request)
			}
			coordinator, closeDatabase, err := open(ctx, 0)
			if errors.Is(err, domain.ErrLocked) && forward != nil {
				forwarded, forwardErr := forward(ctx, call.Params)
				if forwardErr == nil {
					return forwarded, nil
				}
				logger.Warn("forwarding MCP call to the running TUI failed", "tool", call.Params.Name, "error", forwardErr)
			}
			if errors.Is(err, domain.ErrLocked) {
				coordinator, closeDatabase, err = open(ctx, lockWait)
			}
			if err != nil {
				return lockedResult(err), nil
			}
			defer func() {
				if closeErr := closeDatabase(); closeErr != nil {
					logger.Error("close database after MCP call", "error", closeErr)
				}
			}()
			return next(context.WithValue(ctx, coordinatorKey{}, coordinator), method, request)
		}
	})
	return server
}

func lockedResult(err error) *mcp.CallToolResult {
	failure := classify(err)
	if errors.Is(err, domain.ErrLocked) {
		failure.Message = "the Kan database is in use, usually by the running TUI; enable the [mcp] section in config.toml so kan mcp can send requests through the TUI, or retry after closing it"
	}
	payload := result[struct{}]{Error: failure}
	encoded, _ := json.Marshal(payload)
	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		StructuredContent: payload,
	}
}

// NewForwarder returns a ForwardFunc for the TUI MCP server described by cfg,
// or nil when that server is disabled. The returned close function ends the
// forwarding session.
func NewForwarder(cfg config.MCP, version string) (ForwardFunc, func() error) {
	if !cfg.Enabled {
		return nil, func() error { return nil }
	}
	forwarder := &forwarder{cfg: cfg, version: version}
	return forwarder.call, forwarder.close
}

type forwarder struct {
	cfg     config.MCP
	version string
	mu      sync.Mutex
	session *mcp.ClientSession
}

func (forwarder *forwarder) call(ctx context.Context, params *mcp.CallToolParamsRaw) (*mcp.CallToolResult, error) {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	var lastErr error
	// A stale session from a TUI that has since restarted fails once; the
	// second attempt reconnects.
	for range 2 {
		if forwarder.session == nil {
			session, err := forwarder.connect(ctx)
			if err != nil {
				return nil, err
			}
			forwarder.session = session
		}
		result, err := forwarder.session.CallTool(ctx, &mcp.CallToolParams{Meta: params.Meta, Name: params.Name, Arguments: params.Arguments})
		if err == nil {
			return result, nil
		}
		lastErr = err
		_ = forwarder.session.Close()
		forwarder.session = nil
	}
	return nil, lastErr
}

func (forwarder *forwarder) connect(ctx context.Context) (*mcp.ClientSession, error) {
	connectContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "kan-stdio", Version: forwarder.version}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   "http://" + forwarder.cfg.Address + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token: forwarder.cfg.Token, next: http.DefaultTransport}},
		MaxRetries: -1,
	}
	session, err := client.Connect(connectContext, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to TUI MCP server at %s: %w", forwarder.cfg.Address, err)
	}
	return session, nil
}

func (forwarder *forwarder) close() error {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	if forwarder.session == nil {
		return nil
	}
	err := forwarder.session.Close()
	forwarder.session = nil
	return err
}

type bearerTransport struct {
	token string
	next  http.RoundTripper
}

func (transport bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.next.RoundTrip(clone)
}
