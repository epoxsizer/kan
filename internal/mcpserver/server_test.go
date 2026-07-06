package mcpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/epoxsizer/kan/internal/tasks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationAndOriginProtection(t *testing.T) {
	handler := authenticate("12345678901234567890123456789012", protectOrigin(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})))

	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/mcp", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1/mcp", nil)
	request.Header.Set("Authorization", "Bearer 12345678901234567890123456789012")
	request.Header.Set("Origin", "https://example.com")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1/mcp", nil)
	request.Header.Set("Authorization", "Bearer 12345678901234567890123456789012")
	request.Header.Set("Origin", "http://127.0.0.1:3000")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestMCPToolsExposeSafeCardWorkflow(t *testing.T) {
	ctx := context.Background()
	repo, err := storage.Open(ctx, filepath.Join(t.TempDir(), "kan.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	coordinator := tasks.New(repo)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, coordinator.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &board))
	first := domain.Column{BoardID: board.ID, Name: "Backlog", Position: 1024}
	second := domain.Column{BoardID: board.ID, Name: "Doing", Position: 2048}
	require.NoError(t, coordinator.CreateColumn(ctx, &first))
	require.NoError(t, coordinator.CreateColumn(ctx, &second))

	var changes []Change
	server := newMCPServer("test", coordinator, slog.New(slog.NewTextHandler(io.Discard, nil)), func(change Change) {
		changes = append(changes, change)
	})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	_, err = server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	listed, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	require.Len(t, names, 11)
	for _, expected := range []string{"kan_list_projects", "kan_list_boards", "kan_get_board", "kan_list_cards", "kan_get_card", "kan_search_cards", "kan_create_card", "kan_update_card", "kan_move_card", "kan_archive_card", "kan_restore_card"} {
		require.True(t, slices.Contains(names, expected), expected)
	}
	require.False(t, slices.Contains(names, "kan_delete_card"))
	require.False(t, slices.Contains(names, "kan_create_board"))

	call := func(name string, arguments map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, callErr := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
		require.NoError(t, callErr)
		require.False(t, result.IsError, "%s: %#v", name, result.StructuredContent)
		return result
	}
	call("kan_create_card", map[string]any{
		"board_id": board.ID, "column_id": first.ID, "title": "Pair with model", "description": "Use **MCP**",
	})
	cards, err := coordinator.ListCards(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	cardID := cards[0].ID

	call("kan_update_card", map[string]any{"card_id": cardID, "title": "Paired work", "tags": []string{"agent"}})
	call("kan_move_card", map[string]any{"card_id": cardID, "target_column_id": second.ID})
	call("kan_archive_card", map[string]any{"card_id": cardID})
	call("kan_restore_card", map[string]any{"card_id": cardID})

	card, err := coordinator.GetCard(ctx, cardID)
	require.NoError(t, err)
	require.Equal(t, "Paired work", card.Title)
	require.Equal(t, second.ID, card.ColumnID)
	require.Equal(t, []string{"agent"}, card.Tags)
	require.Len(t, changes, 5)
}

func TestStartReturnsListenerFailure(t *testing.T) {
	_, err := Start(config.MCP{
		Enabled: true,
		Address: "127.0.0.1:99999",
		Token:   "12345678901234567890123456789012",
	}, "test", nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	require.ErrorContains(t, err, "start MCP listener")
}
