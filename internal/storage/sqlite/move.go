package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"github.com/epoxsizer/kan/internal/domain"
)

const positionSpacing = 1024.0

type positionedCard struct {
	id       string
	position float64
}

func (repo *Repository) MoveCard(ctx context.Context, cardID, targetColumnID string, targetIndex int) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin card move: %w", err)
	}
	defer tx.Rollback()

	var boardID string
	if err = tx.QueryRowContext(ctx, `SELECT board_id FROM cards WHERE id=? AND deleted_at IS NULL`, cardID).Scan(&boardID); err != nil {
		return mapError(err)
	}
	var targetBoardID string
	var wipLimit sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT board_id,wip_limit FROM board_columns WHERE id=?`, targetColumnID).Scan(&targetBoardID, &wipLimit); err != nil {
		return mapError(err)
	}
	if targetBoardID != boardID {
		return fmt.Errorf("%w: target column belongs to another board", domain.ErrConflict)
	}

	rows, err := tx.QueryContext(ctx, `SELECT id,position FROM cards WHERE column_id=? AND deleted_at IS NULL AND id<>? ORDER BY position,id`, targetColumnID, cardID)
	if err != nil {
		return err
	}
	cards := []positionedCard{}
	for rows.Next() {
		var card positionedCard
		if err = rows.Scan(&card.id, &card.position); err != nil {
			rows.Close()
			return err
		}
		cards = append(cards, card)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	if wipLimit.Valid && len(cards) >= int(wipLimit.Int64) {
		return fmt.Errorf("%w: target column WIP limit %d reached", domain.ErrConflict, wipLimit.Int64)
	}
	targetIndex = min(max(targetIndex, 0), len(cards))
	position, renormalize := insertionPosition(cards, targetIndex)
	if renormalize {
		for index := range cards {
			cards[index].position = float64(index+1) * positionSpacing
			if _, err = tx.ExecContext(ctx, `UPDATE cards SET position=?,updated_at=? WHERE id=?`, cards[index].position, encodeTime(domain.UTCNow()), cards[index].id); err != nil {
				return mapError(err)
			}
		}
		position, _ = insertionPosition(cards, targetIndex)
	}
	result, err := tx.ExecContext(ctx, `UPDATE cards SET column_id=?,position=?,updated_at=? WHERE id=? AND deleted_at IS NULL`, targetColumnID, position, encodeTime(domain.UTCNow()), cardID)
	if err = ensureAffected(result, err); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit card move: %w", err)
	}
	return nil
}

func insertionPosition(cards []positionedCard, index int) (float64, bool) {
	if len(cards) == 0 {
		return positionSpacing, false
	}
	if index <= 0 {
		if cards[0].position > 1 {
			return cards[0].position / 2, false
		}
		return cards[0].position - positionSpacing, false
	}
	if index >= len(cards) {
		return cards[len(cards)-1].position + positionSpacing, false
	}
	left, right := cards[index-1].position, cards[index].position
	if math.Abs(right-left) < 1e-6 {
		return 0, true
	}
	middle := left + (right-left)/2
	if middle == left || middle == right {
		return 0, true
	}
	return middle, false
}
