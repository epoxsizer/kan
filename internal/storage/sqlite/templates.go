package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/epoxsizer/kan/internal/domain"
)

const cardTemplateColumns = `id,board_id,name,title,description,priority,due_offset_days,tags,checklist,position,created_at,updated_at`

func (repo *Repository) CreateCardTemplate(ctx context.Context, value *domain.CardTemplate) error {
	if value.Tags == nil {
		value.Tags = []string{}
	}
	if value.Checklist == nil {
		value.Checklist = []domain.ChecklistItem{}
	}
	prepareIdentity(&value.ID, &value.CreatedAt, &value.UpdatedAt)
	if err := domain.ValidateCardTemplate(*value); err != nil {
		return err
	}
	tags, err := jsonValue(value.Tags, `[]`)
	if err != nil {
		return err
	}
	checklist, err := jsonValue(value.Checklist, `[]`)
	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, `INSERT INTO card_templates(id,board_id,name,title,description,priority,due_offset_days,tags,checklist,position,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.BoardID, value.Name, value.Title, value.Description, value.Priority, value.DueOffsetDays, tags, checklist, value.Position, encodeTime(value.CreatedAt), encodeTime(value.UpdatedAt))
	return mapError(err)
}

func scanCardTemplate(row scanner) (domain.CardTemplate, error) {
	var value domain.CardTemplate
	var priority sql.NullString
	var dueOffset sql.NullInt64
	var tags, checklist, created, updated string
	err := row.Scan(&value.ID, &value.BoardID, &value.Name, &value.Title, &value.Description, &priority, &dueOffset, &tags, &checklist, &value.Position, &created, &updated)
	if err != nil {
		return value, mapError(err)
	}
	if priority.Valid {
		value.Priority = &priority.String
	}
	if dueOffset.Valid {
		v := int(dueOffset.Int64)
		value.DueOffsetDays = &v
	}
	if err = json.Unmarshal([]byte(tags), &value.Tags); err != nil {
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
	return value, err
}

func (repo *Repository) GetCardTemplate(ctx context.Context, id string) (domain.CardTemplate, error) {
	return scanCardTemplate(repo.db.QueryRowContext(ctx, `SELECT `+cardTemplateColumns+` FROM card_templates WHERE id=?`, id))
}

func (repo *Repository) ListCardTemplates(ctx context.Context, boardID string) ([]domain.CardTemplate, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT `+cardTemplateColumns+` FROM card_templates WHERE board_id=? ORDER BY position,id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.CardTemplate{}
	for rows.Next() {
		value, scanErr := scanCardTemplate(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (repo *Repository) UpdateCardTemplate(ctx context.Context, value *domain.CardTemplate) error {
	updated := *value
	updated.UpdatedAt = domain.UTCNow()
	if updated.Tags == nil {
		updated.Tags = []string{}
	}
	if updated.Checklist == nil {
		updated.Checklist = []domain.ChecklistItem{}
	}
	if err := domain.ValidateCardTemplate(updated); err != nil {
		return err
	}
	tags, err := jsonValue(updated.Tags, `[]`)
	if err != nil {
		return err
	}
	checklist, err := jsonValue(updated.Checklist, `[]`)
	if err != nil {
		return err
	}
	if err = ensureAffected(repo.db.ExecContext(ctx, `UPDATE card_templates SET board_id=?,name=?,title=?,description=?,priority=?,due_offset_days=?,tags=?,checklist=?,position=?,updated_at=? WHERE id=?`, updated.BoardID, updated.Name, updated.Title, updated.Description, updated.Priority, updated.DueOffsetDays, tags, checklist, updated.Position, encodeTime(updated.UpdatedAt), updated.ID)); err != nil {
		return err
	}
	*value = updated
	return nil
}

func (repo *Repository) DeleteCardTemplate(ctx context.Context, id string) error {
	return ensureAffected(repo.db.ExecContext(ctx, `DELETE FROM card_templates WHERE id=?`, id))
}
