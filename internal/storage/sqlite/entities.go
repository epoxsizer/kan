package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

func (repo *Repository) CreateProject(ctx context.Context, value *domain.Project) error {
	prepareIdentity(&value.ID, &value.CreatedAt, &value.UpdatedAt)
	if err := domain.ValidateProject(*value); err != nil {
		return err
	}
	_, err := repo.db.ExecContext(ctx, `INSERT INTO projects(id,name,description,position,created_at,updated_at) VALUES(?,?,?,?,?,?)`, value.ID, value.Name, value.Description, value.Position, encodeTime(value.CreatedAt), encodeTime(value.UpdatedAt))
	return mapError(err)
}

func scanProject(row scanner) (domain.Project, error) {
	var value domain.Project
	var created, updated string
	err := row.Scan(&value.ID, &value.Name, &value.Description, &value.Position, &created, &updated)
	if err != nil {
		return value, mapError(err)
	}
	value.CreatedAt, err = parseTime(created)
	if err != nil {
		return value, err
	}
	value.UpdatedAt, err = parseTime(updated)
	return value, err
}

func (repo *Repository) GetProject(ctx context.Context, id string) (domain.Project, error) {
	return scanProject(repo.db.QueryRowContext(ctx, `SELECT id,name,description,position,created_at,updated_at FROM projects WHERE id=?`, id))
}

func (repo *Repository) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT id,name,description,position,created_at,updated_at FROM projects ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.Project{}
	for rows.Next() {
		value, scanErr := scanProject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (repo *Repository) UpdateProject(ctx context.Context, value *domain.Project) error {
	value.UpdatedAt = domain.UTCNow()
	if err := domain.ValidateProject(*value); err != nil {
		return err
	}
	return ensureAffected(repo.db.ExecContext(ctx, `UPDATE projects SET name=?,description=?,position=?,updated_at=? WHERE id=?`, value.Name, value.Description, value.Position, encodeTime(value.UpdatedAt), value.ID))
}

func (repo *Repository) DeleteProject(ctx context.Context, id string) error {
	return ensureAffected(repo.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id))
}

func (repo *Repository) CreateBoard(ctx context.Context, value *domain.Board) error {
	prepareIdentity(&value.ID, &value.CreatedAt, &value.UpdatedAt)
	if err := domain.ValidateBoard(*value); err != nil {
		return err
	}
	_, err := repo.db.ExecContext(ctx, `INSERT INTO boards(id,project_id,name,description,position,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, value.ID, value.ProjectID, value.Name, value.Description, value.Position, encodeTime(value.CreatedAt), encodeTime(value.UpdatedAt))
	return mapError(err)
}

func scanBoard(row scanner) (domain.Board, error) {
	var value domain.Board
	var created, updated string
	err := row.Scan(&value.ID, &value.ProjectID, &value.Name, &value.Description, &value.Position, &created, &updated)
	if err != nil {
		return value, mapError(err)
	}
	value.CreatedAt, err = parseTime(created)
	if err != nil {
		return value, err
	}
	value.UpdatedAt, err = parseTime(updated)
	return value, err
}

func (repo *Repository) GetBoard(ctx context.Context, id string) (domain.Board, error) {
	return scanBoard(repo.db.QueryRowContext(ctx, `SELECT id,project_id,name,description,position,created_at,updated_at FROM boards WHERE id=?`, id))
}
func (repo *Repository) ListBoards(ctx context.Context, projectID string) ([]domain.Board, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT id,project_id,name,description,position,created_at,updated_at FROM boards WHERE project_id=? ORDER BY position,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.Board{}
	for rows.Next() {
		value, scanErr := scanBoard(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (repo *Repository) UpdateBoard(ctx context.Context, value *domain.Board) error {
	value.UpdatedAt = domain.UTCNow()
	if err := domain.ValidateBoard(*value); err != nil {
		return err
	}
	return ensureAffected(repo.db.ExecContext(ctx, `UPDATE boards SET project_id=?,name=?,description=?,position=?,updated_at=? WHERE id=?`, value.ProjectID, value.Name, value.Description, value.Position, encodeTime(value.UpdatedAt), value.ID))
}
func (repo *Repository) DeleteBoard(ctx context.Context, id string) error {
	return ensureAffected(repo.db.ExecContext(ctx, `DELETE FROM boards WHERE id=?`, id))
}

func (repo *Repository) CreateColumn(ctx context.Context, value *domain.Column) error {
	prepareIdentity(&value.ID, &value.CreatedAt, &value.UpdatedAt)
	if err := domain.ValidateColumn(*value); err != nil {
		return err
	}
	_, err := repo.db.ExecContext(ctx, `INSERT INTO board_columns(id,board_id,name,position,wip_limit,color,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, value.ID, value.BoardID, value.Name, value.Position, value.WIPLimit, value.Color, encodeTime(value.CreatedAt), encodeTime(value.UpdatedAt))
	return mapError(err)
}
func scanColumn(row scanner) (domain.Column, error) {
	var value domain.Column
	var wip sql.NullInt64
	var color sql.NullString
	var created, updated string
	err := row.Scan(&value.ID, &value.BoardID, &value.Name, &value.Position, &wip, &color, &created, &updated)
	if err != nil {
		return value, mapError(err)
	}
	if wip.Valid {
		v := int(wip.Int64)
		value.WIPLimit = &v
	}
	if color.Valid {
		value.Color = &color.String
	}
	value.CreatedAt, err = parseTime(created)
	if err != nil {
		return value, err
	}
	value.UpdatedAt, err = parseTime(updated)
	return value, err
}
func (repo *Repository) GetColumn(ctx context.Context, id string) (domain.Column, error) {
	return scanColumn(repo.db.QueryRowContext(ctx, `SELECT id,board_id,name,position,wip_limit,color,created_at,updated_at FROM board_columns WHERE id=?`, id))
}
func (repo *Repository) ListColumns(ctx context.Context, boardID string) ([]domain.Column, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT id,board_id,name,position,wip_limit,color,created_at,updated_at FROM board_columns WHERE board_id=? ORDER BY position,id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.Column{}
	for rows.Next() {
		value, scanErr := scanColumn(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (repo *Repository) UpdateColumn(ctx context.Context, value *domain.Column) error {
	value.UpdatedAt = domain.UTCNow()
	if err := domain.ValidateColumn(*value); err != nil {
		return err
	}
	return ensureAffected(repo.db.ExecContext(ctx, `UPDATE board_columns SET board_id=?,name=?,position=?,wip_limit=?,color=?,updated_at=? WHERE id=?`, value.BoardID, value.Name, value.Position, value.WIPLimit, value.Color, encodeTime(value.UpdatedAt), value.ID))
}
func (repo *Repository) DeleteColumn(ctx context.Context, id string) error {
	return ensureAffected(repo.db.ExecContext(ctx, `DELETE FROM board_columns WHERE id=?`, id))
}

func (repo *Repository) CreateFieldDef(ctx context.Context, value *domain.FieldDef) error {
	if len(value.Options) == 0 {
		value.Options = json.RawMessage(`[]`)
	}
	prepareIdentity(&value.ID, &value.CreatedAt, &value.UpdatedAt)
	if err := domain.ValidateFieldDef(*value); err != nil {
		return err
	}
	_, err := repo.db.ExecContext(ctx, `INSERT INTO field_defs(id,board_id,key,label,type,options,required,position,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, value.ID, value.BoardID, value.Key, value.Label, value.Type, string(value.Options), value.Required, value.Position, encodeTime(value.CreatedAt), encodeTime(value.UpdatedAt))
	return mapError(err)
}
func scanFieldDef(row scanner) (domain.FieldDef, error) {
	var value domain.FieldDef
	var options, created, updated string
	err := row.Scan(&value.ID, &value.BoardID, &value.Key, &value.Label, &value.Type, &options, &value.Required, &value.Position, &created, &updated)
	if err != nil {
		return value, mapError(err)
	}
	value.Options = json.RawMessage(options)
	value.CreatedAt, err = parseTime(created)
	if err != nil {
		return value, err
	}
	value.UpdatedAt, err = parseTime(updated)
	return value, err
}
func (repo *Repository) GetFieldDef(ctx context.Context, id string) (domain.FieldDef, error) {
	return scanFieldDef(repo.db.QueryRowContext(ctx, `SELECT id,board_id,key,label,type,options,required,position,created_at,updated_at FROM field_defs WHERE id=?`, id))
}
func (repo *Repository) ListFieldDefs(ctx context.Context, boardID string) ([]domain.FieldDef, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT id,board_id,key,label,type,options,required,position,created_at,updated_at FROM field_defs WHERE board_id=? ORDER BY position,id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.FieldDef{}
	for rows.Next() {
		value, scanErr := scanFieldDef(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (repo *Repository) UpdateFieldDef(ctx context.Context, value *domain.FieldDef) error {
	if len(value.Options) == 0 {
		value.Options = json.RawMessage(`[]`)
	}
	value.UpdatedAt = domain.UTCNow()
	if err := domain.ValidateFieldDef(*value); err != nil {
		return err
	}
	return ensureAffected(repo.db.ExecContext(ctx, `UPDATE field_defs SET board_id=?,key=?,label=?,type=?,options=?,required=?,position=?,updated_at=? WHERE id=?`, value.BoardID, value.Key, value.Label, value.Type, string(value.Options), value.Required, value.Position, encodeTime(value.UpdatedAt), value.ID))
}
func (repo *Repository) DeleteFieldDef(ctx context.Context, id string) error {
	return ensureAffected(repo.db.ExecContext(ctx, `DELETE FROM field_defs WHERE id=?`, id))
}
