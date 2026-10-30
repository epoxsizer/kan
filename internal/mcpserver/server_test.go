package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

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
	require.ElementsMatch(t, expectedToolNames, names)
	for _, forbidden := range []string{"kan_delete_card", "kan_delete_board", "kan_delete_column", "kan_create_project", "kan_update_project"} {
		require.False(t, slices.Contains(names, forbidden), forbidden)
	}

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

var expectedToolNames = []string{
	"kan_list_projects", "kan_list_boards", "kan_get_board", "kan_list_cards", "kan_get_card", "kan_search_cards",
	"kan_create_card", "kan_update_card", "kan_move_card", "kan_archive_card", "kan_restore_card",
	"kan_create_board", "kan_update_board", "kan_create_column", "kan_update_column", "kan_move_column",
	"kan_add_checklist_item", "kan_update_checklist_item", "kan_list_card_templates", "kan_create_card_from_template",
}

type testSession struct {
	t       *testing.T
	session *mcp.ClientSession
}

func connectTestSession(t *testing.T, coordinator *tasks.Coordinator, notify func(Change)) *testSession {
	t.Helper()
	ctx := context.Background()
	server := newMCPServer("test", coordinator, slog.New(slog.NewTextHandler(io.Discard, nil)), notify)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })
	return &testSession{t: t, session: session}
}

// call invokes a tool and decodes its structured result into out.
func (session *testSession) call(name string, arguments map[string]any, out any) *mcp.CallToolResult {
	session.t.Helper()
	result, err := session.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	require.NoError(session.t, err)
	if out != nil {
		reflect.ValueOf(out).Elem().SetZero()
		encoded, marshalErr := json.Marshal(result.StructuredContent)
		require.NoError(session.t, marshalErr)
		require.NoError(session.t, json.Unmarshal(encoded, out))
	}
	return result
}

func (session *testSession) mustCall(name string, arguments map[string]any, out any) {
	session.t.Helper()
	result := session.call(name, arguments, out)
	require.False(session.t, result.IsError, "%s: %#v %s", name, result.StructuredContent, contentText(result))
}

func contentText(result *mcp.CallToolResult) string {
	parts := []string{}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func openTestCoordinator(t *testing.T) (*tasks.Coordinator, domain.Project) {
	t.Helper()
	ctx := context.Background()
	repo, err := storage.Open(ctx, filepath.Join(t.TempDir(), "kan.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	coordinator := tasks.New(repo)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, coordinator.CreateProject(ctx, &project))
	return coordinator, project
}

func TestMCPToolsManageBoardStructure(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	var changes []Change
	session := connectTestSession(t, coordinator, func(change Change) { changes = append(changes, change) })

	var board result[boardData]
	session.mustCall("kan_create_board", map[string]any{"project_id": project.ID, "name": "Roadmap", "description": "Plans"}, &board)
	require.Equal(t, "Roadmap", board.Data.Board.Name)
	boardID := board.Data.Board.ID

	session.mustCall("kan_update_board", map[string]any{"board_id": boardID, "name": "Roadmap 2027"}, &board)
	require.Equal(t, "Roadmap 2027", board.Data.Board.Name)
	require.Equal(t, "Plans", board.Data.Board.Description)

	var column result[columnData]
	session.mustCall("kan_create_column", map[string]any{"board_id": boardID, "name": "Todo"}, &column)
	todoID := column.Data.Column.ID
	session.mustCall("kan_create_column", map[string]any{"board_id": boardID, "name": "Doing", "wip_limit": 2, "color": "Green"}, &column)
	doingID := column.Data.Column.ID
	require.Equal(t, 2, *column.Data.Column.WIPLimit)

	session.mustCall("kan_update_column", map[string]any{"column_id": doingID, "clear_wip_limit": true, "auto_archive": true, "archive_after_days": 7}, &column)
	require.Nil(t, column.Data.Column.WIPLimit)
	require.True(t, column.Data.Column.AutoArchive)
	require.Equal(t, 7, column.Data.Column.ArchiveAfterDays)

	session.mustCall("kan_move_column", map[string]any{"column_id": doingID, "target_index": 0}, &column)
	columns, err := coordinator.ListColumns(ctx, boardID)
	require.NoError(t, err)
	require.Equal(t, []string{doingID, todoID}, []string{columns[0].ID, columns[1].ID})

	invalid := session.call("kan_update_column", map[string]any{"column_id": doingID, "wip_limit": 2, "clear_wip_limit": true}, &column)
	require.True(t, invalid.IsError)
	require.Equal(t, "validation", column.Error.Code)

	require.Len(t, changes, 6)
	require.Equal(t, "column moved", changes[5].Action)
	require.Equal(t, boardID, changes[5].BoardID)
}

func TestMCPToolsManageChecklistsTemplatesAndConflicts(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Backlog", Position: 1024}
	require.NoError(t, coordinator.CreateColumn(ctx, &column))
	session := connectTestSession(t, coordinator, nil)

	var card result[cardData]
	session.mustCall("kan_create_card", map[string]any{
		"board_id": board.ID, "column_id": column.ID, "title": "Checklist",
		"checklist": []map[string]any{{"text": "First"}, {"text": "Second", "done": true}},
	}, &card)
	require.Len(t, card.Data.Card.Checklist, 2)
	require.NotEmpty(t, card.Data.Card.Checklist[0].ID)
	require.NotEqual(t, card.Data.Card.Checklist[0].ID, card.Data.Card.Checklist[1].ID)
	require.Less(t, card.Data.Card.Checklist[0].Position, card.Data.Card.Checklist[1].Position)
	cardID := card.Data.Card.ID

	var added result[checklistItemData]
	session.mustCall("kan_add_checklist_item", map[string]any{"card_id": cardID, "text": "Third"}, &added)
	require.Len(t, added.Data.Card.Checklist, 3)
	require.Equal(t, "Third", added.Data.Item.Text)

	session.mustCall("kan_update_checklist_item", map[string]any{"card_id": cardID, "item_id": added.Data.Item.ID, "done": true}, &card)
	require.True(t, card.Data.Card.Checklist[2].Done)

	stale := card.Data.Card.UpdatedAt.Add(-time.Second).Format(time.RFC3339Nano)
	conflict := session.call("kan_update_card", map[string]any{"card_id": cardID, "title": "Stale", "expected_updated_at": stale}, &card)
	require.True(t, conflict.IsError)
	require.Equal(t, "conflict", card.Error.Code)
	current, err := coordinator.GetCard(ctx, cardID)
	require.NoError(t, err)
	session.mustCall("kan_update_card", map[string]any{"card_id": cardID, "title": "Fresh", "expected_updated_at": current.UpdatedAt.Format(time.RFC3339Nano)}, &card)
	require.Equal(t, "Fresh", card.Data.Card.Title)

	template := domain.CardTemplate{BoardID: board.ID, Name: "Bug", Title: "Fix bug", Tags: []string{"bug"}, Position: 1024}
	require.NoError(t, coordinator.CreateCardTemplate(ctx, &template))
	var templates result[templatesData]
	session.mustCall("kan_list_card_templates", map[string]any{"board_id": board.ID}, &templates)
	require.Len(t, templates.Data.Templates, 1)
	session.mustCall("kan_create_card_from_template", map[string]any{"template_id": template.ID, "column_id": column.ID}, &card)
	require.Equal(t, "Fix bug", card.Data.Card.Title)
	require.Equal(t, []string{"bug"}, card.Data.Card.Tags)
}

func TestStartReturnsListenerFailure(t *testing.T) {
	_, err := Start(config.MCP{
		Enabled: true,
		Address: "127.0.0.1:99999",
		Token:   "12345678901234567890123456789012",
	}, "test", nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	require.ErrorContains(t, err, "start MCP listener")
}
