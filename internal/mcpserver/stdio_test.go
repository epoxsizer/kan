package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestForwardedWriteWithLostResponseIsNeverRepeatedLocally(t *testing.T) {
	coordinator, project := openTestCoordinator(t)
	opens := 0
	open := func(context.Context, time.Duration) (*tasks.Coordinator, func() error, error) {
		opens++
		if opens == 1 {
			return nil, nil, domain.ErrLocked
		}
		return coordinator, func() error { return nil }, nil
	}
	forward := func(ctx context.Context, _ *mcp.CallToolParamsRaw) (*mcp.CallToolResult, error) {
		board := domain.Board{ProjectID: project.ID, Name: "Created once"}
		if err := coordinator.CreateBoardAtEnd(ctx, &board); err != nil {
			return nil, err
		}
		return nil, errors.New("response lost after commit")
	}
	session := connectStdioSession(t, open, forward)
	var response result[singleBoardData]
	failed := session.call("kan_create_board", map[string]any{"project_id": project.ID, "name": "Created once"}, &response)
	require.True(t, failed.IsError)
	require.Equal(t, "outcome_unknown", response.Error.Code)
	require.False(t, response.Error.Retryable)
	require.Equal(t, 1, opens, "an ambiguous write must not fall back to another database call")
	boards, err := coordinator.ListBoards(context.Background(), project.ID)
	require.NoError(t, err)
	require.Len(t, boards, 1)
}

const testToken = "12345678901234567890123456789012"

func connectStdioSession(t *testing.T, open OpenFunc, forward ForwardFunc) *testSession {
	t.Helper()
	ctx := context.Background()
	server := NewStdioServer("test", open, forward, slog.New(slog.NewTextHandler(io.Discard, nil)))
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })
	return &testSession{t: t, session: session}
}

func TestStdioServerOpensDatabasePerToolCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kan.db")
	opens, closes := 0, 0
	open := func(ctx context.Context, _ time.Duration) (*tasks.Coordinator, func() error, error) {
		repo, err := storage.Open(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		opens++
		return tasks.New(repo), func() error { closes++; return repo.Close() }, nil
	}
	session := connectStdioSession(t, open, nil)

	listed, err := session.session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, listed.Tools, len(expectedToolNames))
	require.Zero(t, opens, "listing tools must not open the database")

	var projects result[projectsData]
	session.mustCall("kan_list_projects", map[string]any{}, &projects)
	require.Empty(t, projects.Data.Projects)
	require.Equal(t, 1, opens)
	require.Equal(t, 1, closes)

	failed := session.call("kan_list_boards", map[string]any{"project_id": "missing"}, nil)
	require.False(t, failed.IsError, "listing boards of an unknown project returns an empty list")
	require.Equal(t, 2, opens)
	require.Equal(t, 2, closes)
}

func TestStdioServerReportsLockedDatabaseWithoutTUIServer(t *testing.T) {
	waits := []time.Duration{}
	open := func(_ context.Context, wait time.Duration) (*tasks.Coordinator, func() error, error) {
		waits = append(waits, wait)
		return nil, nil, domain.ErrLocked
	}
	session := connectStdioSession(t, open, nil)

	var projects result[projectsData]
	locked := session.call("kan_list_projects", map[string]any{}, &projects)
	require.True(t, locked.IsError)
	require.Equal(t, "locked", projects.Error.Code)
	require.True(t, projects.Error.Retryable)
	require.Contains(t, projects.Error.Message, "[mcp]")
	require.Equal(t, []time.Duration{0, lockWait}, waits)
}

func TestStdioServerForwardsToRunningTUIServer(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	var changes []Change
	httpServer, err := Start(config.MCP{Enabled: true, Address: "127.0.0.1:0", Token: testToken}, "test", coordinator,
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(change Change) { changes = append(changes, change) })
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, httpServer.Shutdown(context.Background())) })

	forward, closeForward := NewForwarder(config.MCP{Enabled: true, Address: httpServer.Address(), Token: testToken}, "test")
	require.NotNil(t, forward)
	t.Cleanup(func() { require.NoError(t, closeForward()) })
	opens := 0
	open := func(context.Context, time.Duration) (*tasks.Coordinator, func() error, error) {
		opens++
		return nil, nil, domain.ErrLocked
	}
	session := connectStdioSession(t, open, forward)

	var board result[singleBoardData]
	session.mustCall("kan_create_board", map[string]any{"project_id": project.ID, "name": "Forwarded"}, &board)
	require.Equal(t, "Forwarded", board.Data.Board.Name)
	session.mustCall("kan_update_board", map[string]any{"board_id": board.Data.Board.ID, "description": "via TUI"}, &board)

	stored, err := coordinator.GetBoard(ctx, board.Data.Board.ID)
	require.NoError(t, err)
	require.Equal(t, "via TUI", stored.Description)
	require.Len(t, changes, 2, "the TUI server must notify its own UI")
	require.Equal(t, 2, opens, "each call tries the local database first without waiting")

	var missing result[cardData]
	notFound := session.call("kan_get_card", map[string]any{"card_id": "missing"}, &missing)
	require.True(t, notFound.IsError)
	require.Equal(t, "not_found", missing.Error.Code)
}

func TestNewForwarderIsDisabledWithoutMCPConfiguration(t *testing.T) {
	forward, closeForward := NewForwarder(config.MCP{}, "test")
	require.Nil(t, forward)
	require.NoError(t, closeForward())
}

func TestForwarderRejectsAnotherDatabase(t *testing.T) {
	coordinator, project := openTestCoordinator(t)
	server, err := Start(config.MCP{Enabled: true, Address: "127.0.0.1:0", Token: testToken}, "test", coordinator, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Shutdown(context.Background())) })
	forward, closeForward := NewForwarder(config.MCP{Enabled: true, Address: server.Address(), Token: testToken}, "test", "another-database")
	defer closeForward()
	response, err := forward(context.Background(), &mcp.CallToolParamsRaw{Name: "kan_create_board", Arguments: json.RawMessage(`{"project_id":"` + project.ID + `","name":"Wrong database"}`)})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, contentText(response), "wrong_database")
	boards, err := coordinator.ListBoards(context.Background(), project.ID)
	require.NoError(t, err)
	require.Empty(t, boards)
}
