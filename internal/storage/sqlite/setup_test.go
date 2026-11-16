package sqlite

import (
	"context"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSetupBoardCreatesBoardColumnsAndTemplatesInOneTransaction(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))

	limit := 2
	board := domain.Board{ProjectID: project.ID, Name: "Setup", Position: 1024}
	columns := []domain.Column{
		{Name: "Todo", Position: 1024},
		{Name: "Done", Position: 2048, WIPLimit: &limit},
	}
	templates := []domain.CardTemplate{{
		Name: "Bug", Title: "Fix bug", Position: 1024,
		Checklist: []domain.ChecklistItem{{ID: "item", Text: "Reproduce", Position: 1024}},
	}}
	require.NoError(t, repo.SetupBoard(ctx, &board, columns, templates))
	require.NotEmpty(t, board.ID)
	require.Equal(t, board.ID, columns[0].BoardID)
	require.NotEmpty(t, columns[0].ID)
	require.Equal(t, board.ID, templates[0].BoardID)
	require.NotEmpty(t, templates[0].ID)

	storedColumns, err := repo.ListColumns(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, storedColumns, 2)
	require.Equal(t, "Todo", storedColumns[0].Name)
	require.Equal(t, 2, *storedColumns[1].WIPLimit)
	storedTemplates, err := repo.ListCardTemplates(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, storedTemplates, 1)
	require.Equal(t, "Reproduce", storedTemplates[0].Checklist[0].Text)
}

func TestSetupBoardRollsBackWhenAnyColumnIsInvalid(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))

	board := domain.Board{ProjectID: project.ID, Name: "Setup", Position: 1024}
	columns := []domain.Column{{Name: "Todo", Position: 1024}, {Name: " ", Position: 2048}}
	err := repo.SetupBoard(ctx, &board, columns, nil)
	require.ErrorIs(t, err, domain.ErrValidation)

	boards, err := repo.ListBoards(ctx, project.ID)
	require.NoError(t, err)
	require.Empty(t, boards, "the board insert must roll back with the invalid column")
}

func TestSetupBoardRollsBackWhenAnyTemplateIsInvalid(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))

	board := domain.Board{ProjectID: project.ID, Name: "Setup", Position: 1024}
	columns := []domain.Column{{Name: "Todo", Position: 1024}}
	templates := []domain.CardTemplate{{Name: "Bug", Title: "", Position: 1024}}
	err := repo.SetupBoard(ctx, &board, columns, templates)
	require.ErrorIs(t, err, domain.ErrValidation)

	boards, err := repo.ListBoards(ctx, project.ID)
	require.NoError(t, err)
	require.Empty(t, boards, "the board and columns must roll back with the invalid template")
}
