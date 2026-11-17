package sqlite

import (
	"context"
	"database/sql"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/google/uuid"
)

const movementColumns = `id,card_id,from_column_id,to_column_id,actor,moved_at`

// recordMovement inserts a column transition inside the caller's transaction.
// Same-column updates are not movements.
func recordMovement(ctx context.Context, db execer, cardID, fromColumn, toColumn string) error {
	if fromColumn == toColumn {
		return nil
	}
	_, err := db.ExecContext(ctx, `INSERT INTO card_movements(id,card_id,from_column_id,to_column_id,actor,moved_at) VALUES(?,?,?,?,?,?)`,
		uuid.NewString(), cardID, fromColumn, toColumn, "", encodeTime(domain.UTCNow()))
	return mapError(err)
}

func scanMovement(row scanner) (domain.CardMovement, error) {
	var value domain.CardMovement
	var from sql.NullString
	var moved string
	err := row.Scan(&value.ID, &value.CardID, &from, &value.ToColumnID, &value.Actor, &moved)
	if err != nil {
		return value, mapError(err)
	}
	if from.Valid {
		value.FromColumnID = from.String
	}
	value.MovedAt, err = parseTime(moved)
	return value, err
}

func (repo *Repository) ListCardMovements(ctx context.Context, cardID string) ([]domain.CardMovement, error) {
	return repo.listMovements(ctx, `SELECT `+movementColumns+` FROM card_movements WHERE card_id=? ORDER BY moved_at,id`, cardID)
}

func (repo *Repository) ListBoardMovements(ctx context.Context, boardID string) ([]domain.CardMovement, error) {
	return repo.listMovements(ctx, `SELECT m.id,m.card_id,m.from_column_id,m.to_column_id,m.actor,m.moved_at FROM card_movements m JOIN cards c ON c.id=m.card_id WHERE c.board_id=? ORDER BY m.moved_at,m.id`, boardID)
}

func (repo *Repository) listMovements(ctx context.Context, query, id string) ([]domain.CardMovement, error) {
	rows, err := repo.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.CardMovement{}
	for rows.Next() {
		value, scanErr := scanMovement(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
