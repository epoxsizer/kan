package tasks

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSetupBoardAppendsBoardWithOrderedColumnsAndTemplates(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, _, _ := createHierarchy(t, coordinator)

	limit := 2
	result, err := coordinator.SetupBoardInProject(ctx, board.ProjectID, BoardSetup{
		Name: "Sprint", Description: "One shot",
		Columns: []ColumnSetup{
			{Name: "Todo"},
			{Name: "Doing", WIPLimit: &limit},
			{Name: "Done", AutoArchive: true, ArchiveAfterDays: 7},
		},
		Templates: []TemplateSetup{{Name: "Bug", Title: "Fix bug", Tags: []string{"bug"}, Checklist: []string{"Reproduce", "Fix"}}},
	})
	require.NoError(t, err)
	require.Equal(t, "Sprint", result.Board.Name)
	require.Equal(t, 2048.0, result.Board.Position, "the new board appends after the existing one")
	require.Len(t, result.Columns, 3)
	require.Equal(t, 1024.0, result.Columns[0].Position)
	require.Equal(t, 3072.0, result.Columns[2].Position)
	require.Equal(t, result.Board.ID, result.Columns[0].BoardID)
	require.Equal(t, 2, *result.Columns[1].WIPLimit)
	require.True(t, result.Columns[2].AutoArchive)
	require.Equal(t, 7, result.Columns[2].ArchiveAfterDays)
	require.Len(t, result.Templates, 1)
	require.Equal(t, result.Board.ID, result.Templates[0].BoardID)
	require.Len(t, result.Templates[0].Checklist, 2)
	require.NotEmpty(t, result.Templates[0].Checklist[0].ID)
	require.NotEqual(t, result.Templates[0].Checklist[0].ID, result.Templates[0].Checklist[1].ID)
	require.Less(t, result.Templates[0].Checklist[0].Position, result.Templates[0].Checklist[1].Position)

	stored, err := coordinator.ListColumns(ctx, result.Board.ID)
	require.NoError(t, err)
	require.Len(t, stored, 3)
}

func TestSetupBoardRejectsUnknownProject(t *testing.T) {
	coordinator := openTestCoordinator(t)
	_, err := coordinator.SetupBoardInProject(context.Background(), "missing", BoardSetup{Name: "Sprint", Columns: []ColumnSetup{{Name: "Todo"}}})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSetupBoardValidationFailureCreatesNothing(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, _, _ := createHierarchy(t, coordinator)

	_, err := coordinator.SetupBoardInProject(ctx, board.ProjectID, BoardSetup{Name: "Sprint", Columns: []ColumnSetup{{Name: "Todo"}, {Name: ""}}})
	require.ErrorIs(t, err, domain.ErrValidation)

	boards, err := coordinator.ListBoards(ctx, board.ProjectID)
	require.NoError(t, err)
	require.Len(t, boards, 1, "only the original board survives")
}

func TestSetupBoardConcurrentCallsSerialize(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, coordinator.CreateProject(ctx, &project))

	const callers = 8
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for index := range callers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := coordinator.SetupBoardInProject(ctx, project.ID, BoardSetup{Name: fmt.Sprintf("Board %d", index), Columns: []ColumnSetup{{Name: "Todo"}}})
			errs <- err
		}(index)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	boards, err := coordinator.ListBoards(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, boards, callers)
	for index := 1; index < len(boards); index++ {
		require.Greater(t, boards[index].Position, boards[index-1].Position, "positions stay unique under concurrency")
	}
}
