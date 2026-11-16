package sqlite

import (
	"context"

	"github.com/epoxsizer/kan/internal/domain"
)

// SetupBoard inserts a board, its columns, and its card templates in one
// transaction. Any validation or constraint failure rolls everything back.
// The board's new ID is written into the columns and templates.
func (repo *Repository) SetupBoard(ctx context.Context, board *domain.Board, columns []domain.Column, templates []domain.CardTemplate) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertBoard(ctx, tx, board); err != nil {
		return err
	}
	for index := range columns {
		columns[index].BoardID = board.ID
		if err := insertColumn(ctx, tx, &columns[index]); err != nil {
			return err
		}
	}
	for index := range templates {
		templates[index].BoardID = board.ID
		if err := insertCardTemplate(ctx, tx, &templates[index]); err != nil {
			return err
		}
	}
	return tx.Commit()
}
