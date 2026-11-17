package mcpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverInstructions = "Kan is the user's local task tracker. Discover project, board, column, and card IDs before mutations. Update tools use patch semantics; pass expected_updated_at from your last read to avoid overwriting the user's edits. Respect WIP limits. Boards and columns can be created, renamed, configured, and reordered; projects are read-only. Archive is reversible; permanent deletion is unavailable."

type Change struct {
	Action   string
	BoardID  string
	ColumnID string
	CardID   string
}

type Server struct {
	httpServer *http.Server
	mcpServer  *mcp.Server
	cancel     context.CancelFunc
	listener   net.Listener
	done       chan error
}

func Start(cfg config.MCP, version string, coordinator *tasks.Coordinator, logger *slog.Logger, notify func(Change), intake Intake) (*Server, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	mcpServer := newMCPServer(version, coordinator, logger, notify)
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return mcpServer },
		&mcp.StreamableHTTPOptions{
			Logger:         logger,
			SessionTimeout: 30 * time.Minute,
		},
	)
	handler := authenticate(cfg.Token, protectOrigin(streamable))
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle("/cards", authenticate(cfg.Token, protectOrigin(intakeHandler(coordinator, intake, logger, notify))))
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("start MCP listener on %s: %w", cfg.Address, err)
	}
	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	requestContext, cancel := context.WithCancel(context.Background())
	httpServer.BaseContext = func(net.Listener) context.Context { return requestContext }
	server := &Server{httpServer: httpServer, mcpServer: mcpServer, cancel: cancel, listener: listener, done: make(chan error, 1)}
	go func() {
		err := httpServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		server.done <- err
		close(server.done)
	}()
	logger.Info("MCP server started", "address", cfg.Address, "path", "/mcp")
	return server, nil
}

func newMCPServer(version string, coordinator *tasks.Coordinator, logger *slog.Logger, notify func(Change)) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "kan", Version: version},
		&mcp.ServerOptions{
			Instructions: serverInstructions,
			Logger:       logger,
			KeepAlive:    30 * time.Second,
		},
	)
	registerTools(server, func(context.Context) *tasks.Coordinator { return coordinator }, logger, notify)
	return server
}

// Address returns the address the server listens on.
func (server *Server) Address() string {
	return server.listener.Addr().String()
}

func (server *Server) Shutdown(ctx context.Context) error {
	if server == nil {
		return nil
	}
	// Cancel long-lived SSE requests and close protocol sessions before waiting
	// for HTTP handlers. Otherwise connected agents keep graceful shutdown open.
	server.cancel()
	for session := range server.mcpServer.Sessions() {
		_ = session.Close()
	}
	err := server.httpServer.Shutdown(ctx)
	if err != nil {
		_ = server.httpServer.Close()
	}
	serveErr := <-server.done
	if err != nil {
		return err
	}
	return serveErr
}

func authenticate(token string, next http.Handler) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		actual := []byte(request.Header.Get("Authorization"))
		if len(actual) != len(expected) || subtle.ConstantTimeCompare(actual, expected) != 1 {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="kan-mcp"`)
			http.Error(writer, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func protectOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := strings.TrimSpace(request.Header.Get("Origin"))
		if origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || !isLoopbackHost(parsed.Hostname()) {
				http.Error(writer, "Forbidden: non-loopback Origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
