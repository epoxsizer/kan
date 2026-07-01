package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/epoxsizer/kan/internal/domain"
)

const cardColumns = `c.id,c.board_id,c.column_id,c.title,c.description,c.position,c.priority,c.due_date,c.tags,c.fields,c.checklist,c.created_at,c.updated_at,c.deleted_at,c.column_entered_at`

func (repo *Repository) CreateCard(ctx context.Context, value *domain.Card) error {
	if value.Tags == nil {
		value.Tags = []string{}
	}
	if value.Fields == nil {
		value.Fields = map[string]domain.FieldValue{}
	}
	if value.Checklist == nil {
		value.Checklist = []domain.ChecklistItem{}
	}
	prepareIdentity(&value.ID, &value.CreatedAt, &value.UpdatedAt)
	if value.ColumnEnteredAt.IsZero() {
		value.ColumnEnteredAt = value.CreatedAt
	}
	if err := domain.ValidateCard(*value); err != nil {
		return err
	}
	tags, err := jsonValue(value.Tags, `[]`)
	if err != nil {
		return err
	}
	fields, err := jsonValue(value.Fields, `{}`)
	if err != nil {
		return err
	}
	checklist, err := jsonValue(value.Checklist, `[]`)
	if err != nil {
		return err
	}
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO cards(id,board_id,column_id,title,description,position,priority,due_date,tags,fields,checklist,created_at,updated_at,deleted_at,column_entered_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.BoardID, value.ColumnID, value.Title, value.Description, value.Position, value.Priority, encodeOptionalTime(value.DueDate), tags, fields, checklist, encodeTime(value.CreatedAt), encodeTime(value.UpdatedAt), encodeOptionalTime(value.DeletedAt), encodeTime(value.ColumnEnteredAt)); err != nil {
		return mapError(err)
	}
	if err = replaceCardLinks(ctx, tx, value.ID, value.RelatedCardIDs); err != nil {
		return err
	}
	return tx.Commit()
}

func scanCard(row scanner) (domain.Card, error) {
	var value domain.Card
	var priority, due, deleted sql.NullString
	var tags, fields, checklist, created, updated, entered string
	err := row.Scan(&value.ID, &value.BoardID, &value.ColumnID, &value.Title, &value.Description, &value.Position, &priority, &due, &tags, &fields, &checklist, &created, &updated, &deleted, &entered)
	if err != nil {
		return value, mapError(err)
	}
	if priority.Valid {
		value.Priority = &priority.String
	}
	if err = json.Unmarshal([]byte(tags), &value.Tags); err != nil {
		return value, err
	}
	if err = json.Unmarshal([]byte(fields), &value.Fields); err != nil {
		return value, err
	}
	if err = json.Unmarshal([]byte(checklist), &value.Checklist); err != nil {
		return value, err
	}
	value.CreatedAt, err = parseTime(created)
	if err != nil {
		return value, err
	}
	value.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return value, err
	}
	value.DueDate, err = parseOptionalTime(due)
	if err != nil {
		return value, err
	}
	value.DeletedAt, err = parseOptionalTime(deleted)
	if err != nil {
		return value, err
	}
	value.ColumnEnteredAt, err = parseTime(entered)
	return value, err
}

func (repo *Repository) GetCard(ctx context.Context, id string) (domain.Card, error) {
	value, err := scanCard(repo.db.QueryRowContext(ctx, `SELECT `+cardColumns+` FROM cards c WHERE c.id=? AND c.deleted_at IS NULL`, id))
	if err != nil {
		return value, err
	}
	value.RelatedCardIDs, err = listRelatedCardIDs(ctx, repo.db, id, false)
	return value, err
}
func (repo *Repository) ListCards(ctx context.Context, boardID string) ([]domain.Card, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT `+cardColumns+` FROM cards c WHERE c.board_id=? AND c.deleted_at IS NULL ORDER BY c.column_id,c.position,c.id`, boardID)
	if err != nil {
		return nil, err
	}
	values, err := collectCards(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return repo.attachCardLinks(ctx, values, false)
}

func (repo *Repository) ListCardsIncludingDeleted(ctx context.Context, boardID string) ([]domain.Card, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT `+cardColumns+` FROM cards c WHERE c.board_id=? ORDER BY c.column_id,c.position,c.id`, boardID)
	if err != nil {
		return nil, err
	}
	values, err := collectCards(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return repo.attachCardLinks(ctx, values, true)
}
func collectCards(rows *sql.Rows) ([]domain.Card, error) {
	values := []domain.Card{}
	for rows.Next() {
		value, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (repo *Repository) UpdateCard(ctx context.Context, value *domain.Card) error {
	value.UpdatedAt = domain.UTCNow()
	if value.Tags == nil {
		value.Tags = []string{}
	}
	if value.Fields == nil {
		value.Fields = map[string]domain.FieldValue{}
	}
	if value.Checklist == nil {
		value.Checklist = []domain.ChecklistItem{}
	}
	if err := domain.ValidateCard(*value); err != nil {
		return err
	}
	tags, err := jsonValue(value.Tags, `[]`)
	if err != nil {
		return err
	}
	fields, err := jsonValue(value.Fields, `{}`)
	if err != nil {
		return err
	}
	checklist, err := jsonValue(value.Checklist, `[]`)
	if err != nil {
		return err
	}
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = ensureAffected(tx.ExecContext(ctx, `UPDATE cards SET board_id=?,column_id=?,title=?,description=?,position=?,priority=?,due_date=?,tags=?,fields=?,checklist=?,updated_at=?,column_entered_at=CASE WHEN column_id<>? THEN ? ELSE column_entered_at END WHERE id=? AND deleted_at IS NULL`, value.BoardID, value.ColumnID, value.Title, value.Description, value.Position, value.Priority, encodeOptionalTime(value.DueDate), tags, fields, checklist, encodeTime(value.UpdatedAt), value.ColumnID, encodeTime(value.UpdatedAt), value.ID)); err != nil {
		return err
	}
	if err = replaceCardLinks(ctx, tx, value.ID, value.RelatedCardIDs); err != nil {
		return err
	}
	return tx.Commit()
}
func (repo *Repository) DeleteCard(ctx context.Context, id string) error {
	now := domain.UTCNow()
	return ensureAffected(repo.db.ExecContext(ctx, `UPDATE cards SET deleted_at=?,updated_at=? WHERE id=? AND deleted_at IS NULL`, encodeTime(now), encodeTime(now), id))
}

func (repo *Repository) RestoreCard(ctx context.Context, id string) error {
	now := domain.UTCNow()
	return ensureAffected(repo.db.ExecContext(ctx, `UPDATE cards SET deleted_at=NULL,updated_at=?,column_entered_at=? WHERE id=? AND deleted_at IS NOT NULL`, encodeTime(now), encodeTime(now), id))
}

func (repo *Repository) ArchiveCardsInColumn(ctx context.Context, columnID string) (int, error) {
	now := encodeTime(domain.UTCNow())
	result, err := repo.db.ExecContext(ctx, `UPDATE cards SET deleted_at=?,updated_at=? WHERE column_id=? AND deleted_at IS NULL`, now, now, columnID)
	if err != nil {
		return 0, mapError(err)
	}
	count, err := result.RowsAffected()
	return int(count), err
}

func (repo *Repository) ArchiveExpiredCards(ctx context.Context, boardID string) (int, error) {
	now := encodeTime(domain.UTCNow())
	result, err := repo.db.ExecContext(ctx, `UPDATE cards SET deleted_at=?,updated_at=?
		WHERE board_id=? AND deleted_at IS NULL AND column_id IN (
			SELECT id FROM board_columns WHERE board_id=? AND auto_archive=1
		) AND julianday(column_entered_at) <= julianday(?) - (
			SELECT archive_after_days FROM board_columns WHERE id=cards.column_id
		)`, now, now, boardID, boardID, now)
	if err != nil {
		return 0, mapError(err)
	}
	count, err := result.RowsAffected()
	return int(count), err
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func replaceCardLinks(ctx context.Context, tx *sql.Tx, cardID string, relatedIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM card_links WHERE card_id=? OR related_card_id=?`, cardID, cardID); err != nil {
		return err
	}
	var projectID string
	if err := tx.QueryRowContext(ctx, `SELECT b.project_id FROM cards c JOIN boards b ON b.id=c.board_id WHERE c.id=? AND c.deleted_at IS NULL`, cardID).Scan(&projectID); err != nil {
		return mapError(err)
	}
	seen := map[string]struct{}{}
	for _, relatedID := range relatedIDs {
		if relatedID == cardID {
			return fmt.Errorf("%w: a card cannot relate to itself", domain.ErrValidation)
		}
		if _, exists := seen[relatedID]; exists {
			continue
		}
		seen[relatedID] = struct{}{}
		var relatedProjectID string
		if err := tx.QueryRowContext(ctx, `SELECT b.project_id FROM cards c JOIN boards b ON b.id=c.board_id WHERE c.id=? AND c.deleted_at IS NULL`, relatedID).Scan(&relatedProjectID); err != nil {
			return mapError(err)
		}
		if relatedProjectID != projectID {
			return fmt.Errorf("%w: related cards must belong to the same project", domain.ErrConflict)
		}
		left, right := cardID, relatedID
		if left > right {
			left, right = right, left
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO card_links(card_id,related_card_id,created_at) VALUES(?,?,?)`, left, right, encodeTime(domain.UTCNow())); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func listRelatedCardIDs(ctx context.Context, db queryer, cardID string, includeDeleted bool) ([]string, error) {
	deletedFilter := "AND other.deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	rows, err := db.QueryContext(ctx, `SELECT CASE WHEN l.card_id=? THEN l.related_card_id ELSE l.card_id END
		FROM card_links l
		JOIN cards other ON other.id=CASE WHEN l.card_id=? THEN l.related_card_id ELSE l.card_id END
		WHERE (l.card_id=? OR l.related_card_id=?) `+deletedFilter+`
		ORDER BY other.title,other.id`, cardID, cardID, cardID, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (repo *Repository) attachCardLinks(ctx context.Context, cards []domain.Card, includeDeleted bool) ([]domain.Card, error) {
	for index := range cards {
		links, err := listRelatedCardIDs(ctx, repo.db, cards[index].ID, includeDeleted)
		if err != nil {
			return nil, err
		}
		cards[index].RelatedCardIDs = links
		sort.Strings(cards[index].RelatedCardIDs)
	}
	return cards, nil
}

func (repo *Repository) SearchCards(ctx context.Context, boardID, query string) ([]domain.Card, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT `+cardColumns+` FROM card_fts JOIN cards c ON c.id=card_fts.card_id WHERE card_fts MATCH ? AND card_fts.board_id=? AND c.deleted_at IS NULL ORDER BY rank,c.position`, query, boardID)
	if err != nil {
		return nil, mapError(err)
	}
	values, err := collectCards(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return repo.attachCardLinks(ctx, values, false)
}
