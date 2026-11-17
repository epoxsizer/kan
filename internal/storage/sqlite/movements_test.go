package sqlite

import (
	"context"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestMovementsRecordCreateMoveAndColumnEdit(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	_, board, column := createHierarchy(t, repo)
	second := domain.Column{BoardID: board.ID, Name: "Doing", Position: 2048}
	require.NoError(t, repo.CreateColumn(ctx, &second))

	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Tracked", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))
	require.NoError(t, repo.MoveCard(ctx, card.ID, second.ID, 0))

	stored, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	stored.Title = "Renamed"
	require.NoError(t, repo.UpdateCard(ctx, &stored))

	movements, err := repo.ListCardMovements(ctx, card.ID)
	require.NoError(t, err)
	require.Len(t, movements, 2, "creation and move are recorded; a same-column edit is not")
	require.Empty(t, movements[0].FromColumnID)
	require.Equal(t, column.ID, movements[0].ToColumnID)
	require.Equal(t, column.ID, movements[1].FromColumnID)
	require.Equal(t, second.ID, movements[1].ToColumnID)
	require.False(t, movements[0].MovedAt.After(movements[1].MovedAt))

	stored.ColumnID = column.ID
	require.NoError(t, repo.UpdateCard(ctx, &stored))
	movements, err = repo.ListCardMovements(ctx, card.ID)
	require.NoError(t, err)
	require.Len(t, movements, 3, "a column change through update is recorded too")

	boardMovements, err := repo.ListBoardMovements(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, boardMovements, 3)
}
