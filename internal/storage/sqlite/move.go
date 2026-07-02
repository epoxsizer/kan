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

func (repo *Repository) MoveColumn(ctx context.Context, columnID string, targetIndex int) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin column move: %w", err)
	}
	defer tx.Rollback()

	var boardID string
	if err = tx.QueryRowContext(ctx, `SELECT board_id FROM board_columns WHERE id=?`, columnID).Scan(&boardID); err != nil {
		return mapError(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM board_columns WHERE board_id=? ORDER BY position,id`, boardID)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Close(); err != nil {
		return err
	}

	currentIndex := -1
	for index, id := range ids {
		if id == columnID {
			currentIndex = index
			break
		}
	}
	if currentIndex < 0 {
		return domain.ErrNotFound
	}
	targetIndex = min(max(targetIndex, 0), len(ids)-1)
	if targetIndex == currentIndex {
		return nil
	}
	ids = append(ids[:currentIndex], ids[currentIndex+1:]...)
	ids = append(ids, "")
	copy(ids[targetIndex+1:], ids[targetIndex:])
	ids[targetIndex] = columnID

	now := encodeTime(domain.UTCNow())
	for index, id := range ids {
		position := float64(index+1) * positionSpacing
		if _, err = tx.ExecContext(ctx, `UPDATE board_columns SET position=?,updated_at=? WHERE id=?`, position, now, id); err != nil {
			return mapError(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit column move: %w", err)
	}
	return nil
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
	now := encodeTime(domain.UTCNow())
	result, err := tx.ExecContext(ctx, `UPDATE cards SET column_id=?,position=?,updated_at=?,column_entered_at=? WHERE id=? AND deleted_at IS NULL`, targetColumnID, position, now, now, cardID)
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
