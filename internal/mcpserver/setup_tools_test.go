package mcpserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupBoardToolCreatesBoardColumnsAndTemplatesInOneCall(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	var changes []Change
	session := connectTestSession(t, coordinator, func(change Change) { changes = append(changes, change) })

	var setup result[setupBoardData]
	session.mustCall("kan_setup_board", map[string]any{
		"project_id": project.ID, "name": "Sprint", "description": "One shot",
		"columns": []map[string]any{
			{"name": "Todo"},
			{"name": "Doing", "wip_limit": 2, "color": "Green"},
			{"name": "Done", "auto_archive": true, "archive_after_days": 7},
		},
		"templates": []map[string]any{{"name": "Bug", "title": "Fix bug", "tags": []string{"bug"}, "checklist": []string{"Reproduce", "Fix"}}},
	}, &setup)
	require.Equal(t, "Sprint", setup.Data.Board.Name)
	require.Equal(t, "One shot", setup.Data.Board.Description)
	require.Len(t, setup.Data.Columns, 3)
	require.Equal(t, 2, *setup.Data.Columns[1].WIPLimit)
	require.Equal(t, "Green", *setup.Data.Columns[1].Color)
	require.True(t, setup.Data.Columns[2].AutoArchive)
	require.Len(t, setup.Data.Templates, 1)
	require.Equal(t, []string{"bug"}, setup.Data.Templates[0].Tags)
	require.Len(t, setup.Data.Templates[0].Checklist, 2)

	columns, err := coordinator.ListColumns(ctx, setup.Data.Board.ID)
	require.NoError(t, err)
	require.Len(t, columns, 3)
	require.Len(t, changes, 1, "one setup call notifies once")
	require.Equal(t, "board setup", changes[0].Action)
	require.Equal(t, setup.Data.Board.ID, changes[0].BoardID)
}

func TestSetupBoardToolFailureCreatesNothing(t *testing.T) {
	coordinator, project := openTestCoordinator(t)
	session := connectTestSession(t, coordinator, nil)

	var setup result[setupBoardData]
	invalid := session.call("kan_setup_board", map[string]any{
		"project_id": project.ID, "name": "Sprint",
		"columns": []map[string]any{{"name": "Todo"}, {"name": ""}},
	}, &setup)
	require.True(t, invalid.IsError)
	require.Equal(t, "validation", setup.Error.Code)

	boards, err := coordinator.ListBoards(context.Background(), project.ID)
	require.NoError(t, err)
	require.Empty(t, boards, "a failed setup leaves the project untouched")
}
